package graph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// handleNodeSuccess processes a successful node completion, including iteration checks.
func (s *Scheduler) handleNodeSuccess(_ context.Context, id NodeID, node *Node, result string, _ JobOpts) {
	// Check iteration loop: should this node re-execute?
	if node.MaxIterations > 0 && node.IterationCheck != nil && node.IterationCheck(result) {
		iteration := s.state.IncrementIteration(id)
		if iteration < node.MaxIterations {
			slog.Info(
				"graph: node iterating",
				"node", id,
				"iteration", iteration+1,
				"max", node.MaxIterations,
			)

			s.emit(SchedulerEvent{
				Type:      EventNodeIteration,
				NodeID:    id,
				NodeLabel: node.Label,
				Content:   fmt.Sprintf("iteration %d/%d", iteration+1, node.MaxIterations),
			})

			// Transition Running→Done, store result, then reset to Pending for re-execution.
			_ = s.state.Transition(id, StateDone)
			s.state.SetResult(id, result)

			if err := s.state.ResetForIteration(id); err != nil {
				slog.Error("graph: failed to reset node for iteration", "node", id, "error", err)
			}
			// Node will be re-enqueued by the next enqueueReady cycle.
			return
		}

		slog.Info("graph: node iteration limit reached", "node", id, "iterations", iteration)
	}

	// Normal completion.
	_ = s.state.Transition(id, StateDone)
	s.state.SetResult(id, result)

	// Mark outgoing edges as taken.
	for _, dep := range s.graph.Dependents(id) {
		s.state.SetEdgeState(id, dep, EdgeTaken)
	}

	s.emit(SchedulerEvent{
		Type:      EventNodeCompleted,
		NodeID:    id,
		NodeLabel: node.Label,
	})
}

// handleNodeFailure processes a node failure with error strategy support.
func (s *Scheduler) handleNodeFailure(ctx context.Context, id NodeID, node *Node, err error, opts JobOpts) {
	switch node.ErrorStrategy {
	case ErrorFail:
		// Fall through to default failure handling below.

	case ErrorDefaultValue:
		// Use fallback output and treat as success.
		slog.Info(
			"graph: node failed, using default output",
			"node", id,
			"error", err,
			"default", node.DefaultOutput,
		)

		_ = s.state.Transition(id, StateDone)
		s.state.SetResult(id, node.DefaultOutput)

		for _, dep := range s.graph.Dependents(id) {
			s.state.SetEdgeState(id, dep, EdgeTaken)
		}

		s.emit(SchedulerEvent{
			Type:      EventNodeCompleted,
			NodeID:    id,
			NodeLabel: node.Label,
			Content:   "default-value",
		})

		return

	case ErrorRetryThenFail:
		retryCount := s.state.IncrementRetry(id)
		if retryCount <= node.MaxRetries {
			slog.Info(
				"graph: node retrying",
				"node", id,
				"retry", retryCount,
				"max", node.MaxRetries,
				"error", err,
			)

			s.emit(SchedulerEvent{
				Type:      EventNodeRetry,
				NodeID:    id,
				NodeLabel: node.Label,
				Content:   fmt.Sprintf("retry %d/%d", retryCount, node.MaxRetries),
				Error:     err.Error(),
			})

			// Transition to failed first (valid from Running), then reset to pending.
			if s.state.Get(id) == StateRunning {
				_ = s.state.Transition(id, StateFailed)
			}

			if err := s.state.ResetForIteration(id); err != nil {
				slog.Error("graph: failed to reset node for retry", "node", id, "error", err)
			}

			// Schedule retry with delay if configured.
			if node.RetryDelay > 0 {
				s.nodeWg.Go(func() {
					select {
					case <-time.After(node.RetryDelay):
						s.enqueueReady(ctx, opts)
					case <-ctx.Done():
					}
				})

				return
			}

			// Node will be re-enqueued by the next enqueueReady cycle.
			return
		}
		// Fall through to normal failure after retries exhausted.
	}

	// Default failure handling (ErrorFail or retries exhausted).
	_ = s.state.Transition(id, StateFailed)
	s.state.SetError(id, err.Error())
	s.markFailureEdges(id, err.Error())

	s.emit(SchedulerEvent{
		Type:      EventNodeFailed,
		NodeID:    id,
		NodeLabel: node.Label,
		Error:     err.Error(),
	})
}

// markFailureEdges sets edge states after a node failure.
// Normal dependents get EdgeSkipped; the fail-branch target (if any) gets EdgeTaken.
func (s *Scheduler) markFailureEdges(id NodeID, errMsg string) {
	failTarget, hasFailBranch := s.graph.FailBranchTarget(id)

	for _, dep := range s.graph.Dependents(id) {
		if hasFailBranch && dep == failTarget {
			s.state.SetEdgeState(id, dep, EdgeTaken)
		} else {
			s.state.SetEdgeState(id, dep, EdgeSkipped)
		}
	}

	if hasFailBranch {
		s.state.SetResult(FailErrorKey(id), errMsg)
		s.emit(SchedulerEvent{
			Type:    EventNodeFailRouted,
			NodeID:  id,
			Content: string(failTarget),
		})
	}
}

// isFailHandled returns true if a failed node has a fail-branch target that completed.
func (s *Scheduler) isFailHandled(id NodeID) bool {
	target, ok := s.graph.FailBranchTarget(id)
	if !ok {
		return false
	}

	return s.state.Get(target) == StateDone
}

// skipNodeAndPropagate skips a node and marks all its outgoing edges as skipped.
// Dependent nodes may be auto-skipped on the next enqueueReady cycle via AllIncomingEdgesSkipped.
// Must be called with s.mu held.
func (s *Scheduler) skipNodeAndPropagate(id NodeID, node *Node) {
	if err := s.state.Transition(id, StateSkipped); err != nil {
		slog.Error("graph: failed to skip node", "node", id, "error", err)

		return
	}

	// Mark outgoing edges as skipped.
	for _, dep := range s.graph.Dependents(id) {
		s.state.SetEdgeState(id, dep, EdgeSkipped)
	}

	s.emit(SchedulerEvent{
		Type:      EventNodeSkipped,
		NodeID:    id,
		NodeLabel: node.Label,
	})
}

// dispatchSubTask handles execution of sub-task nodes via the SubTaskExecutor.
func (s *Scheduler) dispatchSubTask(ctx context.Context, id NodeID, node *Node, opts JobOpts) {
	if s.subTaskExecutor == nil {
		slog.Error("graph: no sub-task executor configured", "node", id)

		_ = s.state.Transition(id, StateRunning)
		s.handleNodeFailure(ctx, id, node, errors.New("no sub-task executor configured"), opts)

		s.nodeWg.Go(func() {
			s.enqueueReady(ctx, opts)
		})

		return
	}

	// Note: s.running is accessed without lock because caller (enqueueReady) holds s.mu.
	s.running++

	if err := s.state.Transition(id, StateRunning); err != nil {
		slog.Error("graph: failed to transition sub-task to running", "node", id, "error", err)
	}

	s.emit(SchedulerEvent{
		Type:      EventNodeStarted,
		NodeID:    id,
		NodeLabel: node.Label,
		Content:   "sub-task: " + node.SubTask.Title,
	})

	s.emitProgress()

	// Run the sub-task in a goroutine.
	s.nodeWg.Go(func() {
		result, err := s.subTaskExecutor(ctx, *node.SubTask)
		s.completeNode(ctx, id, node, result, err, opts)
	})
}

// waitForApproval emits an approval event and spawns a goroutine that blocks
// until approved, rejected, or timed out. Must be called with s.mu held;
// the caller's deferred unlock releases the lock after this function returns.
func (s *Scheduler) waitForApproval(ctx context.Context, id NodeID, node *Node, opts JobOpts) {
	ch := make(chan bool, 1)

	s.approvalsMu.Lock()
	s.approvals[id] = ch
	s.approvalsMu.Unlock()

	prompt := node.ApprovalPrompt
	if prompt == "" {
		prompt = fmt.Sprintf("Approve execution of node %q?", id)
	}

	s.emit(SchedulerEvent{
		Type:      EventNodeApprovalRequired,
		NodeID:    id,
		NodeLabel: node.Label,
		Content:   prompt,
	})

	s.nodeWg.Go(func() {
		defer func() {
			s.approvalsMu.Lock()
			delete(s.approvals, id)
			s.approvalsMu.Unlock()
		}()

		var timeoutCh <-chan time.Time
		if node.ApprovalTimeout > 0 {
			timer := time.NewTimer(node.ApprovalTimeout)
			defer timer.Stop()
			timeoutCh = timer.C
		}

		select {
		case approved := <-ch:
			if approved {
				slog.Info("graph: node approved", "node", id)
				s.mu.Lock()
				s.dispatchNode(ctx, id, node, opts)
				s.mu.Unlock()
			} else {
				slog.Info("graph: node rejected", "node", id)
				_ = s.state.Transition(id, StateRunning) // queued → running (briefly)
				s.handleNodeFailure(ctx, id, node, errors.New("approval rejected"), opts)
				s.emitProgress()
				s.enqueueReady(ctx, opts)
			}
		case <-timeoutCh:
			slog.Info("graph: node approval timed out", "node", id, "timeout", node.ApprovalTimeout)
			_ = s.state.Transition(id, StateRunning)
			s.handleNodeFailure(ctx, id, node, fmt.Errorf("approval timed out after %v", node.ApprovalTimeout), opts)
			s.emitProgress()
			s.enqueueReady(ctx, opts)
		case <-ctx.Done():
			_ = s.state.Transition(id, StateRunning)
			s.handleNodeFailure(ctx, id, node, ctx.Err(), opts)
			s.emitProgress()
		}
	})
}

// ApproveNode approves a pending approval gate, allowing the node to execute.
func (s *Scheduler) ApproveNode(id NodeID) bool {
	s.approvalsMu.Lock()
	ch, ok := s.approvals[id]
	s.approvalsMu.Unlock()

	if !ok {
		return false
	}

	select {
	case ch <- true:
		return true
	default:
		return false
	}
}

// RejectNode rejects a pending approval gate, failing the node.
func (s *Scheduler) RejectNode(id NodeID) bool {
	s.approvalsMu.Lock()
	ch, ok := s.approvals[id]
	s.approvalsMu.Unlock()

	if !ok {
		return false
	}

	select {
	case ch <- false:
		return true
	default:
		return false
	}
}

// emitProgress sends an aggregate progress event.
func (s *Scheduler) emitProgress() {
	counts := s.state.CountByState()
	done := counts[StateDone] + counts[StateFailed] + counts[StateSkipped]

	s.emit(SchedulerEvent{
		Type:  EventPhaseProgress,
		Done:  done,
		Total: s.graph.NodeCount(),
	})
}

// summaryError returns a combined error string if any unhandled nodes failed, or empty string.
// Nodes whose failure was handled by a successful fail-branch target are excluded.
func (s *Scheduler) summaryError() string {
	if !s.state.HasFailures() {
		return ""
	}

	var msgs []string

	for _, id := range s.graph.NodeIDs() {
		if s.state.Get(id) == StateFailed && !s.isFailHandled(id) {
			msgs = append(msgs, fmt.Sprintf("%s: %s", id, s.state.Error(id)))
		}
	}

	if len(msgs) == 0 {
		return ""
	}

	return fmt.Sprintf("graph: %d node(s) failed: %s", len(msgs), joinErrors(msgs))
}

func joinErrors(msgs []string) string {
	if len(msgs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(msgs[0])
	for _, m := range msgs[1:] {
		sb.WriteString("; ")
		sb.WriteString(m)
	}

	return sb.String()
}

func (s *Scheduler) emit(evt SchedulerEvent) {
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}

	// Critical events (completion) must not be dropped — block to ensure delivery,
	// but respect context cancellation to avoid goroutine leaks.
	if evt.Type == EventAllDone {
		select {
		case s.events <- evt:
		case <-s.ctx.Done():
		}

		return
	}

	select {
	case s.events <- evt:
	default:
		slog.Warn("graph: scheduler event channel full, dropping event", "type", evt.Type)
	}
}
