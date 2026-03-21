package conductor

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/agent/strategy"
	"github.com/valksor/kvelmo/pkg/git"
	"github.com/valksor/kvelmo/pkg/graph"
	"github.com/valksor/kvelmo/pkg/memory"
	"github.com/valksor/kvelmo/pkg/varpool"
	"github.com/valksor/kvelmo/pkg/worker"
)

// watchGraph monitors a graph scheduler execution and handles completion.
// This is the graph-aware equivalent of watchJob. It creates checkpoints,
// dispatches the completion event, and triggers auto-advance.
//

func (c *Conductor) watchGraph(ctx context.Context, sched *graph.Scheduler, completionEvent Event) {
	// Pre-job safety checkpoint is created by the caller (Plan, etc.) before
	// starting this goroutine, matching the pattern used by watchJob callers.
	events := sched.Run(ctx, c.buildGraphJobOpts())

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
		// Graph had failures — save partial work checkpoint.
		c.createSafetyCheckpoint(ctx, fmt.Sprintf("partial work before %s failure", completionEvent))
		c.mu.Unlock()

		// Apply per-phase failure policy before default error handling.
		if c.applyFailurePolicy(ctx, completionEvent) {
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

	// Success path — same as watchJob completion.
	if completionEvent == EventPlanDone {
		c.detectSpecificationFiles()
		c.copySpecsToRepo()
		c.copyPlanToRepo()
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

	if combinedOutput != "" && c.evaluateAndMaybeIterate(ctx, completionEvent, combinedOutput) {
		return // Re-submitted; skip normal completion path
	}

	c.mu.Lock()

	// Mark implementation as done for guard checks (matches watchJob)
	if completionEvent == EventImplementDone {
		c.workUnit.HasImplemented = true
	}

	// Clear stale PriorStableState on successful completion (matches watchJob)
	c.machine.ClearPriorStableState()

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

// buildGraphJobOpts creates graph.JobOpts from conductor state.
// Must be called with c.mu held or from a safe context.
func (c *Conductor) buildGraphJobOpts() graph.JobOpts {
	opts := graph.JobOpts{
		WorktreeID: c.getWorkDir(),
		WorkDir:    c.getWorkDir(),
		Metadata:   make(map[string]any),
	}

	if c.workUnit != nil {
		opts.Metadata["task_id"] = c.workUnit.ID
		opts.Metadata["task_title"] = c.workUnit.Title
		if c.workUnit.ExternalID != "" {
			opts.Metadata["external_id"] = c.workUnit.ExternalID
		}
		if c.workUnit.Source != nil {
			opts.Metadata["provider"] = c.workUnit.Source.Provider
			opts.Metadata["reference"] = c.workUnit.Source.Reference
		}
	}

	return opts
}

// createSafetyCheckpoint stages and commits all changes as a safety checkpoint.
// Must be called with c.mu held.
func (c *Conductor) createSafetyCheckpoint(ctx context.Context, message string) {
	workDir := c.getWorkDir()
	repo, err := git.Open(workDir)
	if err != nil {
		return
	}

	if err := repo.StageAll(ctx); err != nil {
		return
	}

	hasChanges, _ := repo.HasUncommittedChanges(ctx)
	if !hasChanges {
		return
	}

	sha, err := repo.Commit(ctx, c.formatCheckpointMessage(message))
	if err != nil {
		return
	}

	c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, sha)
	slog.Info("checkpoint created", "sha", sha, "message", message)
}

// createCompletionCheckpoint creates the post-job checkpoint.
// Must be called with c.mu held.
func (c *Conductor) createCompletionCheckpoint(ctx context.Context, completionEvent Event) {
	workDir := c.getWorkDir()
	repo, err := git.Open(workDir)
	if err != nil {
		slog.Debug("checkpoint: git open failed", "error", err, "workDir", workDir)

		return
	}

	if stageErr := repo.StageAll(ctx); stageErr != nil {
		slog.Warn("checkpoint: stage failed", "error", stageErr, "workDir", workDir)

		return
	}

	hasChanges, _ := repo.HasUncommittedChanges(ctx)
	if hasChanges {
		sha, commitErr := repo.Commit(ctx, c.formatCheckpointMessage(fmt.Sprintf("%s complete", completionEvent)))
		if commitErr == nil {
			c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, sha)
			slog.Info("checkpoint created", "sha", sha, "event", completionEvent)
		} else {
			slog.Warn("checkpoint: commit failed", "error", commitErr, "workDir", workDir)
		}
	} else {
		// Capture agent commits if any.
		if headSHA, headErr := repo.CurrentCommit(ctx); headErr == nil && headSHA != "" {
			isNew := true
			for _, cp := range c.workUnit.Checkpoints {
				if cp == headSHA {
					isNew = false

					break
				}
			}
			if isNew {
				c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, headSHA)
				slog.Info("checkpoint captured (agent commit)", "sha", headSHA, "event", completionEvent)
			}
		}
	}
}

// buildPhaseGraph creates a graph for a phase. Currently returns a single-node
// graph for backward compatibility. Future: read sub-task definitions from
// task config or plan output to build multi-node graphs.
func buildPhaseGraph(jobType worker.JobType, label, prompt string) *graph.Graph {
	return graph.SingleNode(graph.NodeID(string(jobType)), label, jobType, prompt)
}

// resolveStrategy returns the strategy for a given phase.
// Checks per-phase overrides first, then conductor default, then global default.
// Must be called with c.mu held (at least RLock).
func (c *Conductor) resolveStrategy(phase string) strategy.Strategy {
	if s, ok := c.phaseStrategies[phase]; ok {
		return s
	}
	if c.strategy != nil {
		return c.strategy
	}

	return strategy.Default()
}

// applyStrategy wraps a raw prompt through the resolved strategy for the phase.
// Must be called with c.mu held (at least RLock).
func (c *Conductor) applyStrategy(phase, prompt string) string {
	s := c.resolveStrategy(phase)

	vars := make(map[string]string)
	if c.varPool != nil {
		for _, v := range c.varPool.List() {
			vars[v.Name] = fmt.Sprintf("%v", v.Value)
		}
	}

	return s.BuildPrompt(strategy.Input{
		Task:      prompt,
		Phase:     phase,
		Variables: vars,
	})
}

// VarPool returns the conductor's variable pool.
// The pool is initialized when the conductor is created and persisted
// alongside task state.
func (c *Conductor) VarPool() *varpool.Pool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.varPool
}

// populateStandardVars sets standard variables from the current work unit.
// Must be called with c.mu held.
func (c *Conductor) populateStandardVars() {
	if c.workUnit == nil || c.varPool == nil {
		return
	}

	// Scoped system variables (new convention).
	c.varPool.SetScoped(varpool.ScopeSystem, "task_id", c.workUnit.ID, "conductor")
	c.varPool.SetScoped(varpool.ScopeSystem, "task_title", c.workUnit.Title, "conductor")
	c.varPool.SetScoped(varpool.ScopeSystem, "task_description", c.workUnit.Description, "conductor")

	// Legacy flat keys for backward compatibility.
	c.varPool.Set("task.id", c.workUnit.ID, "conductor")
	c.varPool.Set("task.title", c.workUnit.Title, "conductor")
	c.varPool.Set("task.description", c.workUnit.Description, "conductor")

	if c.workUnit.Branch != "" {
		c.varPool.SetScoped(varpool.ScopeSystem, "branch", c.workUnit.Branch, "conductor")
		c.varPool.Set("task.branch", c.workUnit.Branch, "conductor")
	}
}

// persistVarPool saves the variable pool to disk.
// Must be called with c.mu held.
func (c *Conductor) persistVarPool() {
	if c.varPool == nil || c.workUnit == nil || c.store == nil {
		return
	}

	path := filepath.Join(c.store.WorkDir(c.workUnit.ID), "varpool.json")
	if err := c.varPool.Save(path); err != nil {
		slog.Warn("persist varpool failed", "task_id", c.workUnit.ID, "error", err)
	}

	c.workUnit.VarPoolPath = path
}

// applyFailurePolicy checks the per-phase failure policy and handles the error
// accordingly. Returns true if the error was handled (retry or skip), in which
// case the caller should skip the default error path.
// Must NOT be called with c.mu held.
func (c *Conductor) applyFailurePolicy(ctx context.Context, completionEvent Event) bool {
	phase := phaseFromEvent(completionEvent)
	if phase == "" {
		return false
	}

	c.mu.RLock()
	policy, ok := c.phasePolicies[phase]
	c.mu.RUnlock()

	if !ok {
		return false
	}

	switch policy.Policy {
	case FailurePolicyRetry:
		c.mu.Lock()
		c.retryCount[phase]++
		attempt := c.retryCount[phase]
		c.mu.Unlock()

		if attempt <= policy.MaxRetries {
			slog.Info("retrying failed phase", "phase", phase, "attempt", attempt, "max", policy.MaxRetries)
			c.emit(ConductorEvent{
				Type:    "phase_retry",
				Message: fmt.Sprintf("Retrying %s (attempt %d/%d)", phase, attempt, policy.MaxRetries),
			})
			time.AfterFunc(policy.RetryDelay, func() {
				c.dispatchAutoAdvance(c.lifecycleCtx, phase)
			})

			return true
		}
		// Exhausted retries — fall through to default error handling.
		slog.Warn("retry limit reached", "phase", phase, "attempts", attempt)
		c.mu.Lock()
		delete(c.retryCount, phase)
		c.mu.Unlock()

		return false

	case FailurePolicySkip:
		slog.Info("skipping failed phase", "phase", phase)
		c.emit(ConductorEvent{
			Type:    "phase_skipped",
			Message: fmt.Sprintf("Skipping failed %s phase", phase),
		})
		// Dispatch the completion event to advance past this phase.
		// Verify the machine is in the expected in-progress state before dispatching
		// to avoid corrupting the state machine if a retry cycle changed the state.
		c.mu.Lock()
		currentState := c.machine.State()
		expectedState := expectedInProgressState(phase)
		if expectedState == "" || currentState != expectedState {
			slog.Warn("skip policy: unexpected state, falling through to fail",
				"phase", phase, "current", currentState, "expected", expectedState)
			c.mu.Unlock()

			return false
		}
		// Clear priorStableState before dispatching completion to prevent
		// it leaking into subsequent operations (the skip advances past
		// the phase, so rollback is no longer relevant).
		c.machine.ClearPriorStableState()
		_ = c.machine.Dispatch(ctx, completionEvent)
		c.persistState()
		c.mu.Unlock()
		c.maybeAutoAdvance(ctx, completionEvent)

		return true

	case FailurePolicyFail:
		return false // Default behavior: stop and wait for user
	}

	return false
}

// expectedInProgressState returns the state the machine should be in
// while a phase is actively running. Used by skip policy to verify the
// machine hasn't been moved by a retry cycle before dispatching completion.
func expectedInProgressState(phase string) State {
	switch phase {
	case "plan":
		return StatePlanning
	case "implement":
		return StateImplementing
	case "simplify":
		return StateSimplifying
	case "optimize":
		return StateOptimizing
	default:
		return ""
	}
}

// phaseFromEvent maps a completion event to its phase name.
func phaseFromEvent(event Event) string {
	switch event { //nolint:exhaustive // Only completion events map to phases
	case EventPlanDone:
		return "plan"
	case EventImplementDone:
		return "implement"
	case EventSimplifyDone:
		return "simplify"
	case EventOptimizeDone:
		return "optimize"
	default:
		return ""
	}
}

// evaluateAndMaybeIterate checks the job output using the phase strategy's
// EvaluateOutput. If the strategy signals "needs_iteration" and we haven't
// exceeded maxIterations, it re-submits the phase and returns true (caller
// should skip the normal completion path). Returns false if the result is
// accepted (either complete or max iterations reached).
// Must NOT be called with c.mu held.
func (c *Conductor) evaluateAndMaybeIterate(_ context.Context, completionEvent Event, output string) bool {
	phase := phaseFromEvent(completionEvent)
	if phase == "" {
		return false
	}

	c.mu.RLock()
	start := c.resolveStrategy(phase)
	c.mu.RUnlock()

	result := start.EvaluateOutput(output)
	if result.Status != "needs_iteration" {
		// Reset iteration count on successful completion.
		c.mu.Lock()
		delete(c.iterationCount, phase)
		c.mu.Unlock()

		return false
	}

	c.mu.Lock()
	c.iterationCount[phase]++
	attempt := c.iterationCount[phase]
	maxIter := c.maxIterations
	c.mu.Unlock()

	if attempt >= maxIter {
		slog.Warn("max iterations reached, accepting result",
			"phase", phase, "attempts", attempt, "marker", result.Metadata["marker"])
		c.mu.Lock()
		delete(c.iterationCount, phase)
		c.mu.Unlock()

		return false
	}

	slog.Info("strategy requests iteration",
		"phase", phase, "attempt", attempt, "marker", result.Metadata["marker"])

	c.emit(ConductorEvent{
		Type:    "iteration_retry",
		Message: fmt.Sprintf("Retrying %s (attempt %d/%d): unresolved %s marker", phase, attempt, maxIter, result.Metadata["marker"]),
	})

	// Re-submit the phase in a goroutine to avoid blocking.
	// Use lifecycleCtx so re-submission survives request context cancellation.
	go func() {
		// Guard against re-submission on a closed conductor.
		if c.closed.Load() {
			return
		}
		select {
		case <-c.lifecycleCtx.Done():
			return
		default:
		}

		var err error
		switch phase {
		case "implement":
			_, err = c.Implement(c.lifecycleCtx, true) // force re-run
		case "simplify":
			_, err = c.Simplify(c.lifecycleCtx)
		case "optimize":
			_, err = c.Optimize(c.lifecycleCtx)
		case "plan":
			_, err = c.Plan(c.lifecycleCtx, true)
		}
		if err != nil {
			slog.Warn("iteration re-submit failed", "phase", phase, "error", err)
		}
	}()

	return true
}

// loadVarPool loads the variable pool from disk if a path is set.
// Must be called with c.mu held.
func (c *Conductor) loadVarPool() {
	if c.workUnit == nil || c.workUnit.VarPoolPath == "" {
		return
	}

	if c.varPool == nil {
		c.varPool = varpool.New()
	}

	if err := c.varPool.Load(c.workUnit.VarPoolPath); err != nil {
		slog.Debug("load varpool failed (may not exist yet)", "path", c.workUnit.VarPoolPath, "error", err)
	}
}
