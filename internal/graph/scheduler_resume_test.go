package graph

import (
	"context"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/worker"
)

func TestPreloadResults(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "A", JobType: worker.JobTypePlan, Prompt: "a"})
	_ = g.AddNode(&Node{ID: "B", JobType: worker.JobTypePlan, Prompt: "b", DependsOn: []NodeID{"A"}})

	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	sm := NewStateManager(g)

	// Before preload, all nodes should be pending.
	if sm.Get("A") != StatePending {
		t.Errorf("expected A=Pending, got %v", sm.Get("A"))
	}

	if sm.Get("B") != StatePending {
		t.Errorf("expected B=Pending, got %v", sm.Get("B"))
	}

	sm.PreloadResults(map[NodeID]string{
		"A": "result-a",
	})

	if sm.Get("A") != StateDone {
		t.Errorf("expected A=Done after preload, got %v", sm.Get("A"))
	}

	if sm.Get("B") != StatePending {
		t.Errorf("expected B=Pending (not preloaded), got %v", sm.Get("B"))
	}

	results := sm.Results()
	if results["A"] != "result-a" {
		t.Errorf("expected result-a, got %q", results["A"])
	}
}

func TestPreloadResultsIgnoresUnknownNodes(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "A", JobType: worker.JobTypePlan, Prompt: "a"})

	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	sm := NewStateManager(g)
	sm.PreloadResults(map[NodeID]string{
		"A":       "result-a",
		"UNKNOWN": "should-be-ignored",
	})

	if sm.Get("A") != StateDone {
		t.Errorf("expected A=Done, got %v", sm.Get("A"))
	}

	results := sm.Results()
	if _, exists := results["UNKNOWN"]; exists {
		t.Error("unknown node should not appear in results")
	}
}

func TestSchedulerResumeFromAllCached(t *testing.T) {
	// Create a 2-node linear graph: A → B
	// Both nodes are pre-cached, so the scheduler should complete
	// without submitting any jobs to the pool.
	g := New()
	_ = g.AddNode(&Node{ID: "A", Label: "step-a", JobType: worker.JobTypePlan, Prompt: "do A"})
	_ = g.AddNode(&Node{ID: "B", Label: "step-b", JobType: worker.JobTypePlan, Prompt: "do B", DependsOn: []NodeID{"A"}})

	// Pool with no workers — any actual job submission would fail.
	pool := worker.NewPool(worker.DefaultPoolConfig())
	defer func() { _ = pool.Stop() }()

	sched := NewScheduler(g, pool)

	ctx := context.Background()
	events := sched.Run(ctx, JobOpts{
		ResumeFrom: map[NodeID]string{
			"A": "result-a",
			"B": "result-b",
		},
	})

	var (
		cachedNodes  []string
		startedNodes []string
		allDoneCount int
		allDoneError string
	)

	for evt := range events {
		switch evt.Type { //nolint:exhaustive // test only cares about specific event types
		case EventNodeStarted:
			startedNodes = append(startedNodes, string(evt.NodeID))
		case EventNodeSkipped:
			if evt.Content == "cached" {
				cachedNodes = append(cachedNodes, string(evt.NodeID))
			}
		case EventAllDone:
			allDoneCount++
			allDoneError = evt.Error
		}
	}

	if len(cachedNodes) != 2 {
		t.Errorf("expected 2 cached nodes, got %d: %v", len(cachedNodes), cachedNodes)
	}

	if len(startedNodes) != 0 {
		t.Errorf("expected 0 started nodes (all cached), got %d: %v", len(startedNodes), startedNodes)
	}

	if allDoneCount != 1 {
		t.Errorf("expected exactly 1 AllDone event, got %d", allDoneCount)
	}

	if allDoneError != "" {
		t.Errorf("expected no error on AllDone, got %q", allDoneError)
	}

	// Verify state manager has the cached results.
	results := sched.State().Results()
	if results["A"] != "result-a" {
		t.Errorf("expected result-a, got %q", results["A"])
	}

	if results["B"] != "result-b" {
		t.Errorf("expected result-b, got %q", results["B"])
	}
}

func TestSchedulerResumeFromPartial(t *testing.T) {
	// A → B → C: only A and B are cached. C needs execution.
	// Since we have no workers, C will be submitted and hang, so we
	// cancel the context after verifying the cached nodes were skipped.
	g := New()
	_ = g.AddNode(&Node{ID: "A", Label: "step-a", JobType: worker.JobTypePlan, Prompt: "do A"})
	_ = g.AddNode(&Node{ID: "B", Label: "step-b", JobType: worker.JobTypePlan, Prompt: "do B", DependsOn: []NodeID{"A"}})
	_ = g.AddNode(&Node{ID: "C", Label: "step-c", JobType: worker.JobTypePlan, Prompt: "do C", DependsOn: []NodeID{"B"}})

	pool := worker.NewPool(worker.DefaultPoolConfig())
	defer func() { _ = pool.Stop() }()

	sched := NewScheduler(g, pool)

	// Use a short timeout so the test doesn't hang if C blocks on the pool.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events := sched.Run(ctx, JobOpts{
		ResumeFrom: map[NodeID]string{
			"A": "result-a",
			"B": "result-b",
		},
	})

	var (
		cachedNodes  []string
		startedNodes []string
	)

	for evt := range events {
		switch evt.Type { //nolint:exhaustive // test only cares about specific event types
		case EventNodeSkipped:
			if evt.Content == "cached" {
				cachedNodes = append(cachedNodes, string(evt.NodeID))
			}
		case EventNodeStarted:
			startedNodes = append(startedNodes, string(evt.NodeID))
		}
	}

	// A and B should be cached.
	if len(cachedNodes) != 2 {
		t.Errorf("expected 2 cached nodes, got %d: %v", len(cachedNodes), cachedNodes)
	}

	// C should have been started (or at least attempted).
	if len(startedNodes) == 0 {
		// C might have failed to submit or been canceled — check state.
		cState := sched.State().Get("C")
		if cState == StatePending {
			t.Error("expected C to be attempted, but it's still pending")
		}
	}

	// Verify cached results are preserved.
	if sched.State().Result("A") != "result-a" {
		t.Errorf("expected cached result-a, got %q", sched.State().Result("A"))
	}

	if sched.State().Result("B") != "result-b" {
		t.Errorf("expected cached result-b, got %q", sched.State().Result("B"))
	}
}
