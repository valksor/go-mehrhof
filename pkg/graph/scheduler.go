package graph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/valksor/kvelmo/pkg/worker"
)

const (
	// DefaultMaxParallel is the maximum number of nodes running concurrently.
	DefaultMaxParallel = 10
)

// SchedulerEventType identifies scheduler-level events.
type SchedulerEventType string

const (
	EventNodeQueued           SchedulerEventType = "node_queued"
	EventNodeStarted          SchedulerEventType = "node_started"
	EventNodeCompleted        SchedulerEventType = "node_completed"
	EventNodeFailed           SchedulerEventType = "node_failed"
	EventNodeSkipped          SchedulerEventType = "node_skipped"
	EventNodeFailRouted       SchedulerEventType = "node_fail_routed"       // Failure routed to handler node
	EventNodeOutput           SchedulerEventType = "node_output"            // Streaming output from node's job
	EventNodeIteration        SchedulerEventType = "node_iteration"         // Node re-executing (iteration loop)
	EventNodeRetry            SchedulerEventType = "node_retry"             // Node retrying after failure
	EventNodeApprovalRequired SchedulerEventType = "node_approval_required" // Waiting for human approval
	EventPhaseProgress        SchedulerEventType = "phase_progress"         // Aggregate progress
	EventAllDone              SchedulerEventType = "all_done"               // All nodes finished
)

// SubTaskExecutor runs a sub-task in an isolated worktree and returns its result.
// Provided by the conductor layer; the graph package does not depend on conductor.
type SubTaskExecutor func(ctx context.Context, config SubTaskConfig) (string, error)

// SchedulerEvent is emitted during graph execution.
type SchedulerEvent struct {
	Type      SchedulerEventType `json:"type"`
	NodeID    NodeID             `json:"node_id,omitempty"`
	NodeLabel string             `json:"node_label,omitempty"`
	JobID     string             `json:"job_id,omitempty"`
	Content   string             `json:"content,omitempty"`
	Error     string             `json:"error,omitempty"`
	Done      int                `json:"done,omitempty"`  // Completed + failed + skipped count
	Total     int                `json:"total,omitempty"` // Total node count
	Timestamp time.Time          `json:"timestamp"`
}

// JobOpts provides execution context passed to each submitted job.
type JobOpts struct {
	WorktreeID  string
	WorkDir     string
	Environment map[string]string
	Metadata    map[string]any

	// ResumeFrom pre-populates results for completed nodes, skipping their
	// execution. Keys are node IDs, values are cached results from a previous
	// run. This enables partial re-execution when a graph fails midway.
	ResumeFrom map[NodeID]string
}

// Option configures a Scheduler.
type Option func(*Scheduler)

// WithMaxParallel sets the maximum concurrent node executions.
func WithMaxParallel(n int) Option {
	return func(s *Scheduler) {
		if n > 0 && n <= DefaultMaxParallel {
			s.maxParallel = n
		}
	}
}

// WithSubTaskExecutor sets the function used to execute sub-task nodes.
// Without this, sub-task nodes will fail with "no sub-task executor configured".
func WithSubTaskExecutor(exec SubTaskExecutor) Option {
	return func(s *Scheduler) {
		s.subTaskExecutor = exec
	}
}

// Scheduler executes a dependency graph by dispatching ready nodes to a worker pool.
type Scheduler struct {
	graph       *Graph
	pool        *worker.Pool
	state       *StateManager
	maxParallel int
	running     int // Number of currently executing nodes
	mu          sync.Mutex
	events      chan SchedulerEvent
	nodeJobs    map[NodeID]string // NodeID → worker job ID
	nodeWg      sync.WaitGroup    // tracks in-flight watchNode goroutines
	ctx         context.Context   //nolint:containedctx // needed by emit's blocking EventAllDone send; threading ctx through all callers is impractical

	// Approval gates: channels for pending approval decisions.
	approvals   map[NodeID]chan bool // NodeID → approval channel (true=approve, false=reject)
	approvalsMu sync.Mutex

	// SubTask executor: provided by the conductor layer to handle sub-task nodes.
	subTaskExecutor SubTaskExecutor
}

// NewScheduler creates a scheduler for the given graph and worker pool.
func NewScheduler(g *Graph, pool *worker.Pool, opts ...Option) *Scheduler {
	s := &Scheduler{
		graph:       g,
		pool:        pool,
		state:       NewStateManager(g),
		maxParallel: DefaultMaxParallel,
		events:      make(chan SchedulerEvent, 100),
		nodeJobs:    make(map[NodeID]string),
		approvals:   make(map[NodeID]chan bool),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// ID returns a stable identifier for this scheduler instance, derived from
// the graph's root nodes. For single-node graphs this is the node ID.
func (s *Scheduler) ID() string {
	roots := s.graph.Roots()
	if len(roots) == 1 {
		return string(roots[0])
	}
	ids := make([]string, len(roots))
	for i, r := range roots {
		ids[i] = string(r)
	}

	return strings.Join(ids, "+")
}

// Run executes the graph. Returns a channel of scheduler events.
// The channel is closed when all nodes are complete (or context is canceled).
func (s *Scheduler) Run(ctx context.Context, opts JobOpts) <-chan SchedulerEvent {
	go s.run(ctx, opts)

	return s.events
}

// State returns the state manager for querying node states.
func (s *Scheduler) State() *StateManager {
	return s.state
}

func (s *Scheduler) run(ctx context.Context, opts JobOpts) {
	s.ctx = ctx // Set before any child goroutine is spawned (happens-before via goroutine creation)
	defer func() {
		s.nodeWg.Wait() // Wait for all watchNode goroutines before closing the event channel
		close(s.events)
	}()

	// Validate graph before executing.
	if err := s.graph.Validate(); err != nil {
		s.emit(SchedulerEvent{
			Type:    EventAllDone,
			Error:   err.Error(),
			Content: "graph validation failed",
		})

		return
	}

	// Preload cached results from a previous run for partial re-execution.
	if len(opts.ResumeFrom) > 0 {
		s.state.PreloadResults(opts.ResumeFrom)

		// Emit skip events for preloaded nodes and mark their outgoing edges.
		for _, id := range s.graph.NodeIDs() {
			if s.state.Get(id) != StateDone {
				continue
			}

			if _, cached := opts.ResumeFrom[id]; !cached {
				continue
			}

			node := s.graph.Nodes[id]

			// Mark outgoing edges as taken so dependents become eligible.
			for _, dep := range s.graph.Dependents(id) {
				s.state.SetEdgeState(id, dep, EdgeTaken)
			}

			s.emit(SchedulerEvent{
				Type:      EventNodeSkipped,
				NodeID:    id,
				NodeLabel: node.Label,
				Content:   "cached",
			})
		}

		slog.Info("graph: resumed from cached results", "cached_nodes", len(opts.ResumeFrom))
	}

	// Seed the ready queue with root nodes.
	s.enqueueReady(ctx, opts)

	// Wait for all nodes to complete.
	// The loop exits when the state manager reports all done.
	for {
		select {
		case <-ctx.Done():
			s.emit(SchedulerEvent{
				Type:  EventAllDone,
				Error: ctx.Err().Error(),
			})

			return
		default:
		}

		if s.state.AllDone() {
			s.emitProgress()
			s.emit(SchedulerEvent{
				Type:  EventAllDone,
				Error: s.summaryError(),
			})

			return
		}

		// Brief sleep to avoid busy-waiting. Node completions are
		// detected on the next iteration via state checks.
		time.Sleep(50 * time.Millisecond)
	}
}

// enqueueReady finds pending nodes whose dependencies are satisfied and dispatches them.
// Nodes are prioritized by transitive dependent count (critical path first).
func (s *Scheduler) enqueueReady(ctx context.Context, opts JobOpts) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// First pass: resolve frozen/skipped nodes and collect dispatch candidates.
	type candidate struct {
		id   NodeID
		node *Node
	}

	var candidates []candidate

	for _, id := range s.graph.NodeIDs() {
		if s.state.Get(id) != StatePending {
			continue
		}

		node := s.graph.Nodes[id]
		if !s.state.DependenciesSatisfied(node) {
			continue
		}

		// Frozen nodes with cached results skip execution entirely.
		if node.Frozen {
			cachedResult := s.state.Result(id)
			if cachedResult != "" {
				if err := s.state.Transition(id, StateDone); err != nil {
					slog.Error("graph: failed to complete frozen node", "node", id, "error", err)

					continue
				}

				s.emit(SchedulerEvent{
					Type:      EventNodeCompleted,
					NodeID:    id,
					NodeLabel: node.Label,
					Content:   cachedResult,
				})

				continue
			}
			// No cached result — fall through to normal execution.
		}

		// Auto-skip nodes whose ALL incoming edges are skipped (recursive propagation).
		if s.state.AllIncomingEdgesSkipped(node) {
			s.skipNodeAndPropagate(id, node)

			continue
		}

		// Check condition function.
		if node.Condition != nil && !node.Condition(s.state.Results()) {
			s.skipNodeAndPropagate(id, node)

			continue
		}

		candidates = append(candidates, candidate{id: id, node: node})
	}

	// Sort by transitive dependent count (descending) — critical path first.
	// Ties broken by node ID for determinism.
	slices.SortFunc(candidates, func(a, b candidate) int {
		ac, bc := s.graph.TransitiveDepCount(a.id), s.graph.TransitiveDepCount(b.id)
		if ac != bc {
			return bc - ac // higher count first (descending)
		}

		return strings.Compare(string(a.id), string(b.id))
	})

	// Second pass: dispatch up to maxParallel.
	for _, c := range candidates {
		if s.running >= s.maxParallel {
			break
		}

		if err := s.state.Transition(c.id, StateQueued); err != nil {
			slog.Error("graph: failed to queue node", "node", c.id, "error", err)

			continue
		}

		s.emit(SchedulerEvent{
			Type:      EventNodeQueued,
			NodeID:    c.id,
			NodeLabel: c.node.Label,
		})

		// Approval gate: wait for human decision before dispatching.
		if c.node.RequiresApproval {
			s.waitForApproval(ctx, c.id, c.node, opts)

			continue
		}

		s.dispatchNode(ctx, c.id, c.node, opts)
	}
}

// dispatchNode submits a node's job to the worker pool and monitors it.
// For sub-task nodes, it invokes the SubTaskExecutor instead.
func (s *Scheduler) dispatchNode(ctx context.Context, id NodeID, node *Node, opts JobOpts) {
	// Handle sub-task nodes via the executor callback.
	if node.IsSubTask() {
		s.dispatchSubTask(ctx, id, node, opts)

		return
	}

	jobOpts := &worker.JobOptions{
		WorkDir:     opts.WorkDir,
		Environment: opts.Environment,
		Metadata:    opts.Metadata,
	}

	if jobOpts.Metadata == nil {
		jobOpts.Metadata = make(map[string]any)
	}

	jobOpts.Metadata["graph_node_id"] = string(id)
	jobOpts.Metadata["graph_node_label"] = node.Label

	job, err := s.pool.SubmitWithOptions(node.JobType, opts.WorktreeID, node.Prompt, jobOpts)
	if err != nil {
		slog.Error("graph: failed to submit job", "node", id, "error", err)

		_ = s.state.Transition(id, StateRunning) // queued → running (briefly)
		s.handleNodeFailure(ctx, id, node, err, opts)

		// Check if fail-branch or skipped dependencies unlock new nodes.
		// Goroutine avoids recursive s.mu acquisition from enqueueReady.
		s.nodeWg.Go(func() {
			s.enqueueReady(ctx, opts)
		})

		return
	}

	s.nodeJobs[id] = job.ID
	s.running++

	if err := s.state.Transition(id, StateRunning); err != nil {
		slog.Error("graph: failed to transition to running", "node", id, "error", err)
	}

	s.emit(SchedulerEvent{
		Type:      EventNodeStarted,
		NodeID:    id,
		NodeLabel: node.Label,
		JobID:     job.ID,
	})

	s.emitProgress()

	// Monitor the job in a goroutine.
	s.nodeWg.Go(func() {
		s.watchNode(ctx, id, node, job.ID, opts)
	})
}

// watchNode monitors a running job and updates state on completion.
func (s *Scheduler) watchNode(ctx context.Context, id NodeID, node *Node, jobID string, opts JobOpts) {
	stream := s.pool.Stream(jobID)
	if stream == nil {
		s.completeNode(ctx, id, node, "", fmt.Errorf("no stream for job %s", jobID), opts)

		return
	}

	var result string

	for {
		select {
		case <-ctx.Done():
			s.completeNode(ctx, id, node, "", ctx.Err(), opts)

			return
		case evt, ok := <-stream:
			if !ok {
				// Stream closed — check job status.
				job := s.pool.GetJob(jobID)
				if job != nil && job.Status == worker.JobStatusFailed {
					s.completeNode(ctx, id, node, "", fmt.Errorf("%s", job.Error), opts)
				} else {
					s.completeNode(ctx, id, node, result, nil, opts)
				}

				return
			}

			switch evt.Type {
			case "job_completed":
				job := s.pool.GetJob(jobID)
				if job != nil {
					result = job.Result
				}

				s.completeNode(ctx, id, node, result, nil, opts)

				return
			case "job_failed":
				s.completeNode(ctx, id, node, "", fmt.Errorf("%s", evt.Content), opts)

				return
			default:
				// Forward streaming output.
				s.emit(SchedulerEvent{
					Type:      EventNodeOutput,
					NodeID:    id,
					NodeLabel: node.Label,
					JobID:     jobID,
					Content:   evt.Content,
				})
			}
		}
	}
}

// completeNode handles node completion (success or failure).
func (s *Scheduler) completeNode(ctx context.Context, id NodeID, node *Node, result string, err error, opts JobOpts) {
	s.mu.Lock()
	s.running--
	s.mu.Unlock()

	if err != nil {
		s.handleNodeFailure(ctx, id, node, err, opts)
	} else {
		s.handleNodeSuccess(ctx, id, node, result, opts)
	}

	s.emitProgress()

	// Enqueue newly unblocked nodes.
	s.enqueueReady(ctx, opts)
}

// handleNodeSuccess processes a successful node completion, including iteration checks.
func (s *Scheduler) handleNodeSuccess(_ context.Context, id NodeID, node *Node, result string, _ JobOpts) {
	// Check iteration loop: should this node re-execute?
	if node.MaxIterations > 0 && node.IterationCheck != nil && node.IterationCheck(result) {
		iteration := s.state.IncrementIteration(id)
		if iteration < node.MaxIterations {
			slog.Info("graph: node iterating",
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
		slog.Info("graph: node failed, using default output",
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
			slog.Info("graph: node retrying",
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
