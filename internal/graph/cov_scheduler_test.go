package graph

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/worker"
)

// subTaskNode builds a sub-task node whose executor behaviour is keyed off its
// Title: "ok:<result>" succeeds with the result; "fail:<msg>" returns an error.
func subTaskNode(id string, title string, deps ...NodeID) *Node {
	return &Node{
		ID:        NodeID(id),
		Label:     id,
		DependsOn: deps,
		SubTask:   &SubTaskConfig{Title: title},
	}
}

// runWithSubExecutor runs a graph to completion using a deterministic sub-task
// executor and collects all emitted events. It fails the test if AllDone is
// not observed within the timeout.
func runWithSubExecutor(t *testing.T, g *Graph) ([]SchedulerEvent, *Scheduler) {
	t.Helper()

	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	exec := func(_ context.Context, cfg SubTaskConfig) (string, error) {
		mode, rest, _ := strings.Cut(cfg.Title, ":")
		switch mode {
		case "fail":
			return "", errors.New(rest)
		case "ok":
			return rest, nil
		default:
			return cfg.Title, nil
		}
	}

	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })

	sched := NewScheduler(g, pool, WithSubTaskExecutor(exec))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var events []SchedulerEvent
	sawDone := false
	for evt := range sched.Run(ctx, JobOpts{}) {
		events = append(events, evt)
		if evt.Type == EventAllDone {
			sawDone = true
		}
	}
	if !sawDone {
		t.Fatal("never observed EventAllDone")
	}

	return events, sched
}

func countEvents(events []SchedulerEvent, typ SchedulerEventType) int {
	n := 0
	for _, e := range events {
		if e.Type == typ {
			n++
		}
	}

	return n
}

func TestScheduler_LinearSuccessChain(t *testing.T) {
	g := New()
	_ = g.AddNode(subTaskNode("A", "ok:result-a"))
	_ = g.AddNode(subTaskNode("B", "ok:result-b", "A"))
	_ = g.AddNode(subTaskNode("C", "ok:result-c", "B"))

	events, sched := runWithSubExecutor(t, g)

	if got := countEvents(events, EventNodeCompleted); got != 3 {
		t.Errorf("completed events = %d, want 3", got)
	}
	results := sched.State().Results()
	if results["A"] != "result-a" || results["C"] != "result-c" {
		t.Errorf("unexpected results: %v", results)
	}
	for _, id := range []NodeID{"A", "B", "C"} {
		if sched.State().Get(id) != StateDone {
			t.Errorf("node %s state = %v, want Done", id, sched.State().Get(id))
		}
	}
	// Outgoing edges of completed nodes must be marked EdgeTaken.
	if sched.State().GetEdgeState("A", "B") != EdgeTaken {
		t.Errorf("edge A->B = %v, want EdgeTaken", sched.State().GetEdgeState("A", "B"))
	}
}

// TestScheduler_PlainFailureLeavesDependentsStuck documents the CURRENT
// behavior of an ErrorFail node with non-fail-branch dependents.
//
// Known limitation (reported, not fixed): when a node fails with the default
// ErrorFail strategy, markFailureEdges marks the edge to a normal dependent as
// EdgeSkipped, but enqueueReady consults AllIncomingEdgesSkipped only AFTER
// DependenciesSatisfied returns true. Because a failed (non-fail-branch)
// dependency makes DependenciesSatisfied return false, the dependent is never
// auto-skipped and remains StatePending forever. The Run loop then only
// terminates via context cancellation, not via clean AllDone. This test pins
// the observed behavior so a future fix is a deliberate, visible change.
func TestScheduler_PlainFailureLeavesDependentsStuck(t *testing.T) {
	g := New()
	_ = g.AddNode(subTaskNode("A", "fail:boom"))
	_ = g.AddNode(subTaskNode("B", "ok:b", "A"))
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	exec := func(_ context.Context, cfg SubTaskConfig) (string, error) {
		if strings.HasPrefix(cfg.Title, "fail") {
			return "", errors.New("boom")
		}

		return "ok", nil
	}
	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })
	sched := NewScheduler(g, pool, WithSubTaskExecutor(exec))

	// The graph cannot complete cleanly (the bug strands B), so the run loop only
	// ends via cancellation. Use a generous timeout so a slow CI runner still
	// observes A's failure event before cancellation fires.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var sawFailed bool
	for evt := range sched.Run(ctx, JobOpts{}) {
		if evt.Type == EventNodeFailed && evt.NodeID == "A" {
			sawFailed = true
		}
	}

	if !sawFailed {
		t.Error("expected a node-failed event for A")
	}
	if sched.State().Get("A") != StateFailed {
		t.Errorf("A state = %v, want Failed", sched.State().Get("A"))
	}
	// Observed (buggy) behavior: B is never skipped, it stays Pending.
	if got := sched.State().Get("B"); got != StatePending {
		t.Errorf("B state = %v; current behavior leaves it Pending (see BUG note)", got)
	}
	// The edge A->B WAS marked skipped, even though B never auto-skips.
	if sched.State().GetEdgeState("A", "B") != EdgeSkipped {
		t.Errorf("edge A->B = %v, want EdgeSkipped", sched.State().GetEdgeState("A", "B"))
	}
}

func TestScheduler_FailBranchRouting(t *testing.T) {
	g := New()
	handler := NodeID("handler")
	a := subTaskNode("a", "fail:exploded")
	a.FailBranch = &handler
	_ = g.AddNode(a)
	_ = g.AddNode(subTaskNode("handler", "ok:recovered", "a"))

	events, sched := runWithSubExecutor(t, g)

	if sched.State().Get("a") != StateFailed {
		t.Errorf("a state = %v, want Failed", sched.State().Get("a"))
	}
	// The handler (fail-branch target) must run and complete.
	if sched.State().Get("handler") != StateDone {
		t.Errorf("handler state = %v, want Done", sched.State().Get("handler"))
	}
	// A fail-routed event must be emitted.
	if countEvents(events, EventNodeFailRouted) != 1 {
		t.Errorf("fail-routed events = %d, want 1", countEvents(events, EventNodeFailRouted))
	}
	// The fail-branch edge must be marked EdgeTaken (not skipped).
	if sched.State().GetEdgeState("a", "handler") != EdgeTaken {
		t.Errorf("edge a->handler = %v, want EdgeTaken", sched.State().GetEdgeState("a", "handler"))
	}
	// Since the failure was handled, the summary error should be empty.
	var doneErr string
	for _, e := range events {
		if e.Type == EventAllDone {
			doneErr = e.Error
		}
	}
	if doneErr != "" {
		t.Errorf("handled failure should leave empty summary error, got %q", doneErr)
	}
}

func TestScheduler_ErrorDefaultValue(t *testing.T) {
	g := New()
	a := subTaskNode("a", "fail:nope")
	a.ErrorStrategy = ErrorDefaultValue
	a.DefaultOutput = "fallback-output"
	_ = g.AddNode(a)
	_ = g.AddNode(subTaskNode("b", "ok:b", "a"))

	events, sched := runWithSubExecutor(t, g)

	// Node a should be Done (treated as success via default value).
	if sched.State().Get("a") != StateDone {
		t.Errorf("a state = %v, want Done", sched.State().Get("a"))
	}
	if sched.State().Result("a") != "fallback-output" {
		t.Errorf("a result = %q, want fallback-output", sched.State().Result("a"))
	}
	// b should run normally since a succeeded.
	if sched.State().Get("b") != StateDone {
		t.Errorf("b state = %v, want Done", sched.State().Get("b"))
	}
	// A completed event with default-value content should be present.
	found := false
	for _, e := range events {
		if e.Type == EventNodeCompleted && e.NodeID == "a" && e.Content == "default-value" {
			found = true
		}
	}
	if !found {
		t.Error("expected default-value completion event for node a")
	}
}

func TestScheduler_ErrorRetryThenFail(t *testing.T) {
	// Executor fails the first two attempts, succeeds on the third.
	var mu sync.Mutex
	attempts := 0
	exec := func(_ context.Context, _ SubTaskConfig) (string, error) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n < 3 {
			return "", errors.New("transient")
		}

		return "succeeded-after-retries", nil
	}

	g := New()
	a := &Node{
		ID:            "a",
		Label:         "a",
		ErrorStrategy: ErrorRetryThenFail,
		MaxRetries:    3,
		SubTask:       &SubTaskConfig{Title: "retry-node"},
	}
	_ = g.AddNode(a)
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })
	sched := NewScheduler(g, pool, WithSubTaskExecutor(exec))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var retryEvents int
	for evt := range sched.Run(ctx, JobOpts{}) {
		if evt.Type == EventNodeRetry {
			retryEvents++
		}
	}

	if sched.State().Get("a") != StateDone {
		t.Errorf("a state = %v, want Done after successful retry", sched.State().Get("a"))
	}
	if sched.State().Result("a") != "succeeded-after-retries" {
		t.Errorf("a result = %q", sched.State().Result("a"))
	}
	if retryEvents < 2 {
		t.Errorf("expected >= 2 retry events, got %d", retryEvents)
	}
}

func TestScheduler_ErrorRetryExhausted(t *testing.T) {
	// Always fails -> exhausts retries -> node ends Failed.
	exec := func(_ context.Context, _ SubTaskConfig) (string, error) {
		return "", errors.New("always-fails")
	}

	g := New()
	_ = g.AddNode(&Node{
		ID:            "a",
		Label:         "a",
		ErrorStrategy: ErrorRetryThenFail,
		MaxRetries:    2,
		SubTask:       &SubTaskConfig{Title: "doomed"},
	})
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })
	sched := NewScheduler(g, pool, WithSubTaskExecutor(exec))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var doneErr string
	for evt := range sched.Run(ctx, JobOpts{}) {
		if evt.Type == EventAllDone {
			doneErr = evt.Error
		}
	}

	if sched.State().Get("a") != StateFailed {
		t.Errorf("a state = %v, want Failed after exhausting retries", sched.State().Get("a"))
	}
	if !strings.Contains(doneErr, "always-fails") {
		t.Errorf("summary error = %q, want it to mention always-fails", doneErr)
	}
}

func TestScheduler_RetryWithDelay(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	exec := func(_ context.Context, _ SubTaskConfig) (string, error) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n < 2 {
			return "", errors.New("retry-me")
		}

		return "ok", nil
	}

	g := New()
	_ = g.AddNode(&Node{
		ID:            "a",
		Label:         "a",
		ErrorStrategy: ErrorRetryThenFail,
		MaxRetries:    2,
		RetryDelay:    20 * time.Millisecond,
		SubTask:       &SubTaskConfig{Title: "delayed"},
	})
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })
	sched := NewScheduler(g, pool, WithSubTaskExecutor(exec))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for range sched.Run(ctx, JobOpts{}) {
	}

	if sched.State().Get("a") != StateDone {
		t.Errorf("a state = %v, want Done", sched.State().Get("a"))
	}
}

func TestScheduler_IterationLoop(t *testing.T) {
	// Re-executes until IterationCheck returns false (or MaxIterations hit).
	var mu sync.Mutex
	runs := 0
	exec := func(_ context.Context, _ SubTaskConfig) (string, error) {
		mu.Lock()
		runs++
		n := runs
		mu.Unlock()

		return "iteration output " + string(rune('0'+n)), nil
	}

	iterChecks := 0
	g := New()
	_ = g.AddNode(&Node{
		ID:            "loop",
		Label:         "loop",
		MaxIterations: 3,
		IterationCheck: func(string) bool {
			// Re-iterate the first two times, then stop.
			mu.Lock()
			defer mu.Unlock()
			iterChecks++

			return iterChecks < 3
		},
		SubTask: &SubTaskConfig{Title: "iter-node"},
	})
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })
	sched := NewScheduler(g, pool, WithSubTaskExecutor(exec))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var iterations int
	for evt := range sched.Run(ctx, JobOpts{}) {
		if evt.Type == EventNodeIteration {
			iterations++
		}
	}

	if iterations < 1 {
		t.Errorf("expected iteration events, got %d", iterations)
	}
	if sched.State().Get("loop") != StateDone {
		t.Errorf("loop state = %v, want Done", sched.State().Get("loop"))
	}
	mu.Lock()
	totalRuns := runs
	mu.Unlock()
	if totalRuns < 2 {
		t.Errorf("expected node to execute multiple times, got %d", totalRuns)
	}
}

func TestScheduler_ConditionSkipsNode(t *testing.T) {
	g := New()
	_ = g.AddNode(subTaskNode("a", "ok:done-a"))
	conditional := subTaskNode("b", "ok:done-b", "a")
	// Condition returns false -> node b is skipped.
	conditional.Condition = func(map[NodeID]string) bool { return false }
	_ = g.AddNode(conditional)

	_, sched := runWithSubExecutor(t, g)

	if sched.State().Get("a") != StateDone {
		t.Errorf("a state = %v, want Done", sched.State().Get("a"))
	}
	if sched.State().Get("b") != StateSkipped {
		t.Errorf("b state = %v, want Skipped (condition false)", sched.State().Get("b"))
	}
}

func TestScheduler_SkipPropagatesToDownstream(t *testing.T) {
	// A is skipped by its condition; B depends only on A, so B's single incoming
	// edge becomes EdgeSkipped and B auto-skips via AllIncomingEdgesSkipped.
	g := New()
	a := subTaskNode("a", "ok:a")
	a.Condition = func(map[NodeID]string) bool { return false }
	_ = g.AddNode(a)
	_ = g.AddNode(subTaskNode("b", "ok:b", "a"))

	events, sched := runWithSubExecutor(t, g)

	if sched.State().Get("a") != StateSkipped {
		t.Errorf("a state = %v, want Skipped", sched.State().Get("a"))
	}
	if sched.State().Get("b") != StateSkipped {
		t.Errorf("b state = %v, want Skipped (auto-skip from skipped dependency)", sched.State().Get("b"))
	}
	if countEvents(events, EventNodeSkipped) < 2 {
		t.Errorf("expected >= 2 skip events, got %d", countEvents(events, EventNodeSkipped))
	}
}

func TestScheduler_FrozenNodeUsesCachedResult(t *testing.T) {
	g := New()
	frozen := subTaskNode("f", "ok:should-not-run")
	frozen.Frozen = true
	_ = g.AddNode(frozen)
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })

	executed := false
	exec := func(_ context.Context, _ SubTaskConfig) (string, error) {
		executed = true

		return "executed", nil
	}
	sched := NewScheduler(g, pool, WithSubTaskExecutor(exec))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Preload a cached result so the frozen node short-circuits.
	for range sched.Run(ctx, JobOpts{ResumeFrom: map[NodeID]string{"f": "cached-frozen"}}) {
	}

	if executed {
		t.Error("frozen node with cached result should not execute the sub-task")
	}
	if sched.State().Get("f") != StateDone {
		t.Errorf("frozen node state = %v, want Done", sched.State().Get("f"))
	}
}

func TestScheduler_NoSubTaskExecutorConfigured(t *testing.T) {
	g := New()
	_ = g.AddNode(subTaskNode("a", "ok:x"))
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })

	// No WithSubTaskExecutor -> sub-task nodes fail.
	sched := NewScheduler(g, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var doneErr string
	for evt := range sched.Run(ctx, JobOpts{}) {
		if evt.Type == EventAllDone {
			doneErr = evt.Error
		}
	}

	if sched.State().Get("a") != StateFailed {
		t.Errorf("a state = %v, want Failed", sched.State().Get("a"))
	}
	if !strings.Contains(doneErr, "no sub-task executor") {
		t.Errorf("summary error = %q, want it to mention missing executor", doneErr)
	}
}

func TestScheduler_ValidationFailureEmitsAllDone(t *testing.T) {
	// A graph with a node depending on a missing node fails Validate, and
	// Run must surface that as an AllDone with an error rather than hanging.
	g := New()
	_ = g.AddNode(&Node{ID: "a", JobType: worker.JobTypePlan, Prompt: "x", DependsOn: []NodeID{"missing"}})

	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })
	sched := NewScheduler(g, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var doneErr, doneContent string
	for evt := range sched.Run(ctx, JobOpts{}) {
		if evt.Type == EventAllDone {
			doneErr = evt.Error
			doneContent = evt.Content
		}
	}
	if doneErr == "" {
		t.Error("expected validation error in AllDone event")
	}
	if !strings.Contains(doneContent, "validation failed") {
		t.Errorf("AllDone content = %q, want validation-failed message", doneContent)
	}
}

func TestScheduler_MultipleIndependentFailuresJoined(t *testing.T) {
	// Two independent root nodes both fail -> summaryError joins both messages.
	g := New()
	_ = g.AddNode(subTaskNode("x", "fail:err-x"))
	_ = g.AddNode(subTaskNode("y", "fail:err-y"))
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	exec := func(_ context.Context, cfg SubTaskConfig) (string, error) {
		_, rest, _ := strings.Cut(cfg.Title, ":")

		return "", errors.New(rest)
	}
	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })
	sched := NewScheduler(g, pool, WithSubTaskExecutor(exec))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var doneErr string
	for evt := range sched.Run(ctx, JobOpts{}) {
		if evt.Type == EventAllDone {
			doneErr = evt.Error
		}
	}

	// Both failures must be present, joined with "; ".
	if !strings.Contains(doneErr, "err-x") || !strings.Contains(doneErr, "err-y") {
		t.Errorf("summary error = %q, want both failures joined", doneErr)
	}
	if !strings.Contains(doneErr, ";") {
		t.Errorf("summary error = %q, want '; ' separator from joinErrors", doneErr)
	}
	if !strings.Contains(doneErr, "2 node(s) failed") {
		t.Errorf("summary error = %q, want failure count", doneErr)
	}
}

func TestScheduler_WithMaxParallel(t *testing.T) {
	g := New()
	_ = g.AddNode(subTaskNode("a", "ok:a"))
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })

	// Valid value is applied.
	s := NewScheduler(g, pool, WithMaxParallel(3))
	if s.maxParallel != 3 {
		t.Errorf("maxParallel = %d, want 3", s.maxParallel)
	}
	// Out-of-range values are ignored (default retained).
	s2 := NewScheduler(g, pool, WithMaxParallel(0))
	if s2.maxParallel != DefaultMaxParallel {
		t.Errorf("maxParallel = %d, want default %d", s2.maxParallel, DefaultMaxParallel)
	}
	s3 := NewScheduler(g, pool, WithMaxParallel(9999))
	if s3.maxParallel != DefaultMaxParallel {
		t.Errorf("maxParallel = %d, want default for over-cap", s3.maxParallel)
	}
}

func TestScheduler_ID(t *testing.T) {
	// Single root -> ID is the root node ID.
	g := New()
	_ = g.AddNode(subTaskNode("only", "ok:x"))
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	s := NewScheduler(g, nil)
	if s.ID() != "only" {
		t.Errorf("single-root ID = %q, want only", s.ID())
	}

	// Multiple roots -> joined with '+'.
	g2 := New()
	_ = g2.AddNode(subTaskNode("r1", "ok:1"))
	_ = g2.AddNode(subTaskNode("r2", "ok:2"))
	if err := g2.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	s2 := NewScheduler(g2, nil)
	id := s2.ID()
	if !strings.Contains(id, "+") || !strings.Contains(id, "r1") || !strings.Contains(id, "r2") {
		t.Errorf("multi-root ID = %q, want both roots joined with +", id)
	}
}

func TestStateManager_GetEdgeStateDefaultsUnknown(t *testing.T) {
	g := New()
	_ = g.AddNode(subTaskNode("a", "ok:a"))
	_ = g.AddNode(subTaskNode("b", "ok:b", "a"))
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	sm := NewStateManager(g)
	if sm.GetEdgeState("a", "b") != EdgeUnknown {
		t.Errorf("unresolved edge = %v, want EdgeUnknown", sm.GetEdgeState("a", "b"))
	}
	sm.SetEdgeState("a", "b", EdgeTaken)
	if sm.GetEdgeState("a", "b") != EdgeTaken {
		t.Errorf("edge after set = %v, want EdgeTaken", sm.GetEdgeState("a", "b"))
	}
}
