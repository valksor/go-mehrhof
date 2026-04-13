package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/valksor/kvelmo/internal/graph"
	"github.com/valksor/kvelmo/internal/memory"
)

// watchGraph monitors a graph scheduler execution and handles completion.
// This is the graph-aware equivalent of watchJob. It creates checkpoints,
// dispatches the completion event, and triggers auto-advance.
//

func (c *Conductor) watchGraph(ctx context.Context, sched *graph.Scheduler, completionEvent Event) {
	// Store active scheduler for node approval RPCs.
	c.mu.Lock()
	c.activeScheduler = sched
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.activeScheduler = nil
		c.mu.Unlock()
	}()

	// Pre-job safety checkpoint is created by the caller (Plan, etc.) before
	// starting this goroutine, matching the pattern used by watchJob callers.
	// Load cached partial results and build job opts under lock.
	c.mu.RLock()
	phase := phaseFromEvent(completionEvent)
	opts := c.buildGraphJobOptsForPhase(phase)
	opts.ResumeFrom = c.loadPartialResults(completionEvent)
	c.mu.RUnlock()

	events := sched.Run(ctx, opts)

	for evt := range events {
		switch evt.Type {
		case graph.EventNodeOutput:
			c.emit(ConductorEvent{
				Type:    "job_output",
				JobID:   evt.JobID,
				NodeID:  string(evt.NodeID),
				Message: evt.Content,
			})

		case graph.EventNodeQueued:
			c.emit(ConductorEvent{
				Type:    "node_queued",
				NodeID:  string(evt.NodeID),
				Message: "Node queued: " + evt.NodeLabel,
			})

		case graph.EventNodeStarted:
			c.emit(ConductorEvent{
				Type:    "node_started",
				JobID:   evt.JobID,
				NodeID:  string(evt.NodeID),
				Message: "Node started: " + evt.NodeLabel,
			})

		case graph.EventNodeCompleted:
			c.emit(ConductorEvent{
				Type:    "node_completed",
				NodeID:  string(evt.NodeID),
				Message: "Node completed: " + evt.NodeLabel,
			})
			// Check for mid-execution spec changes between graph nodes.
			if c.specWatcher != nil && c.specWatcher.Check() {
				c.emit(ConductorEvent{
					Type:    "spec_changed",
					Message: "Specification modified during execution — agent will adapt",
				})
				c.specWatcher.Reset()
			}

		case graph.EventNodeFailed:
			c.emit(ConductorEvent{
				Type:    "node_failed",
				NodeID:  string(evt.NodeID),
				Error:   evt.Error,
				Message: "Node failed: " + evt.NodeLabel,
			})

		case graph.EventNodeFailRouted:
			c.emit(ConductorEvent{
				Type:    "node_fail_routed",
				NodeID:  string(evt.NodeID),
				Message: "Failure routed to handler: " + evt.Content,
			})

		case graph.EventNodeSkipped:
			c.emit(ConductorEvent{
				Type:    "node_skipped",
				NodeID:  string(evt.NodeID),
				Message: "Node skipped: " + evt.NodeLabel,
			})

		case graph.EventNodeIteration:
			c.emit(ConductorEvent{
				Type:    "node_iteration",
				NodeID:  string(evt.NodeID),
				Message: "Node iterating: " + evt.Content,
			})

		case graph.EventNodeRetry:
			c.emit(ConductorEvent{
				Type:    "node_retry",
				NodeID:  string(evt.NodeID),
				Error:   evt.Error,
				Message: "Node retrying: " + evt.Content,
			})

		case graph.EventNodeApprovalRequired:
			c.emit(ConductorEvent{
				Type:    "node_approval_required",
				NodeID:  string(evt.NodeID),
				Message: evt.Content,
			})

		case graph.EventPhaseProgress:
			c.emit(ConductorEvent{
				Type:    "phase_progress",
				Message: fmt.Sprintf("%d/%d nodes complete", evt.Done, evt.Total),
			})

		case graph.EventAllDone:
			c.handleGraphCompletion(ctx, sched, completionEvent, evt.Error)

			return
		}
	}
}

// handleGraphCompletion processes the final state of a graph execution.
//
//nolint:contextcheck // Intentionally creates detached context for background indexing (same pattern as watchJob)
func (c *Conductor) handleGraphCompletion(ctx context.Context, sched *graph.Scheduler, completionEvent Event, errMsg string) {
	c.mu.Lock()

	c.activeJobID = ""

	if c.workUnit == nil {
		c.mu.Unlock()

		return
	}

	if errMsg != "" {
		// Graph had failures — save partial results for resume-on-retry.
		c.savePartialResults(sched, completionEvent)

		// Save partial work checkpoint.
		c.createSafetyCheckpoint(ctx, fmt.Sprintf("partial work before %s failure", completionEvent))
		c.mu.Unlock()

		// Apply per-phase failure policy before default error handling.
		if c.applyFailurePolicy(ctx, completionEvent, errMsg) {
			return // Policy handled it (retry or skip)
		}

		c.mu.Lock()
		_ = c.machine.Dispatch(ctx, EventError)
		c.persistState()
		c.mu.Unlock()

		c.emit(ConductorEvent{
			Type:    "job_failed",
			Error:   errMsg,
			Message: "Graph execution failed",
		})
		c.emitEnrichedError(fmt.Errorf("%s", errMsg), string(completionEvent))

		return
	}

	// Success — clear cached partial results since they are no longer needed.
	c.clearPartialResults(completionEvent)

	// Capture pre-completion checkpoint ref and validation params under lock
	var preJobCheckpoint string
	if len(c.workUnit.Checkpoints) > 0 {
		preJobCheckpoint = c.workUnit.Checkpoints[len(c.workUnit.Checkpoints)-1]
	}
	commitValParams := c.prepareCommitValidation(preJobCheckpoint)

	// Success path — same as watchJob completion.
	if completionEvent == EventPlanDone {
		c.detectSpecificationFiles()
		if len(c.workUnit.Specifications) == 0 {
			err := errors.New("plan phase produced no specification file (agent did not write required deliverable)")
			c.emitEnrichedError(err, "plan")
			_ = c.machine.Dispatch(ctx, EventError)
			c.persistState()
			c.mu.Unlock()

			return
		}
		c.copySpecsToRepo()
		c.copyPlanToRepo()
		c.commitRepoSpecs(ctx)
	}

	// Create completion checkpoint.
	c.createCompletionCheckpoint(ctx, completionEvent)

	// Evaluate output via strategy (collect all node results as output).
	var combinedOutput string
	if sched != nil {
		var sb strings.Builder
		for _, result := range sched.State().Results() {
			sb.WriteString(result)
		}
		combinedOutput = sb.String()
	}
	c.mu.Unlock()

	// Validate agent commits without holding the lock (runs git subprocess)
	c.validateAgentCommits(ctx, commitValParams)

	if combinedOutput != "" && c.evaluateAndMaybeIterate(ctx, completionEvent, combinedOutput) {
		return // Re-submitted; skip normal completion path
	}

	// Router-based adaptive phase progression: evaluate output and decide next action.
	// This runs AFTER strategy evaluation completes, wrapping the normal advance path.
	if c.router != nil {
		phase := phaseFromEvent(completionEvent)
		decision := c.router.Route(ctx, phase, combinedOutput, 0)
		if c.applyRouteDecision(ctx, decision, completionEvent) {
			return // Router handled it (retry/skip/rollback)
		}
	}

	c.mu.Lock()

	// Mark implementation as done for guard checks (matches watchJob)
	if completionEvent == EventImplementDone {
		c.workUnit.HasImplemented = true
	}

	// Clear stale PriorStableState on successful completion (matches watchJob)
	c.machine.ClearPriorStableState()

	// Record per-phase execution metrics from graph scheduler.
	c.recordPhaseMetricsFromGraph(completionEvent, sched)

	// Dispatch completion event.
	_ = c.machine.Dispatch(ctx, completionEvent)
	c.persistState()

	// Snapshot for async indexing.
	var (
		wuSnapshot *WorkUnit
		indexer    *memory.Indexer
	)
	if completionEvent == EventPlanDone || completionEvent == EventImplementDone {
		wuSnapshot = c.workUnit
		indexer = c.memoryIndexer
	}

	baseBranch, baseBranchErr := c.getBaseBranch(ctx)
	c.mu.Unlock()

	// Async memory indexing (same pattern as watchJob).
	if baseBranchErr != nil {
		slog.Debug("skipping memory indexing - cannot detect base branch", "error", baseBranchErr)
	} else if indexer != nil && wuSnapshot != nil {
		asyncCtx, asyncCancel := context.WithTimeout(context.Background(), 30*time.Second)

		go func(wu *WorkUnit, idx *memory.Indexer, event Event, base string) {
			defer asyncCancel()
			if err := idx.IndexTask(asyncCtx, wu.ID, wu.Title, wu.Description, wu.Branch, base); err != nil {
				slog.Warn("memory indexing failed", "task_id", wu.ID, "event", event, "error", err)
			}
		}(wuSnapshot, indexer, completionEvent, baseBranch)
	}

	c.maybeAutoAdvance(ctx, completionEvent)
}
