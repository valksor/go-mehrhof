package graph

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/agenttest"
	"github.com/valksor/kvelmo/internal/worker"
)

// poolWithMockAgent builds a started worker pool backed by a single mock agent
// emitting the given events, with one worker attached.
func poolWithMockAgent(t *testing.T, events ...agent.Event) *worker.Pool {
	t.Helper()

	registry := agent.NewRegistry()
	mock := agenttest.NewMockAgent("mock", events...)
	if err := registry.Register(mock); err != nil {
		t.Fatalf("register mock: %v", err)
	}
	pool := worker.NewPool(worker.PoolConfig{MaxWorkers: 2, Agents: registry})
	if err := pool.Start(); err != nil {
		t.Fatalf("start pool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Stop() })
	if _, err := pool.AddAgentWorker(context.Background(), "mock", true); err != nil {
		t.Fatalf("add worker: %v", err)
	}

	return pool
}

func TestScheduler_AgentJob_StreamsOutputAndCompletes(t *testing.T) {
	pool := poolWithMockAgent(
		t,
		agent.Event{Type: agent.EventStream, Content: "hello "},
		agent.Event{Type: agent.EventStream, Content: "world"},
		agent.Event{Type: agent.EventComplete},
	)

	g := New()
	_ = g.AddNode(&Node{ID: "n", Label: "plan", JobType: worker.JobTypePlan, Prompt: "do the plan"})
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	sched := NewScheduler(g, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var output strings.Builder
	var started, completed bool
	for evt := range sched.Run(ctx, JobOpts{WorktreeID: "wt-1"}) {
		switch evt.Type { //nolint:exhaustive // test inspects a subset
		case EventNodeStarted:
			started = true
		case EventNodeOutput:
			output.WriteString(evt.Content)
		case EventNodeCompleted:
			completed = true
		}
	}

	if !started {
		t.Error("expected a node-started event")
	}
	if !completed {
		t.Error("expected a node-completed event")
	}
	if sched.State().Get("n") != StateDone {
		t.Errorf("node state = %v, want Done", sched.State().Get("n"))
	}
	// Streamed deltas should have been forwarded as output events.
	if !strings.Contains(output.String(), "hello") {
		t.Errorf("expected streamed output, got %q", output.String())
	}
}

func TestScheduler_AgentJob_FailurePropagates(t *testing.T) {
	pool := poolWithMockAgent(
		t,
		agent.Event{Type: agent.EventStream, Content: "partial work"},
		agent.Event{Type: agent.EventError, Error: "agent exploded"},
	)

	g := New()
	_ = g.AddNode(&Node{ID: "n", Label: "impl", JobType: worker.JobTypeImplement, Prompt: "do it"})
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	sched := NewScheduler(g, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var failed bool
	var doneErr string
	for evt := range sched.Run(ctx, JobOpts{WorktreeID: "wt-1"}) {
		switch evt.Type { //nolint:exhaustive // test inspects a subset
		case EventNodeFailed:
			failed = true
		case EventAllDone:
			doneErr = evt.Error
		}
	}

	if !failed {
		t.Error("expected a node-failed event")
	}
	if sched.State().Get("n") != StateFailed {
		t.Errorf("node state = %v, want Failed", sched.State().Get("n"))
	}
	if doneErr == "" {
		t.Error("expected non-empty AllDone summary error")
	}
}

func TestScheduler_AgentJob_ParallelFanout(t *testing.T) {
	pool := poolWithMockAgent(
		t,
		agent.Event{Type: agent.EventStream, Content: "ok"},
		agent.Event{Type: agent.EventComplete},
	)

	// root -> {a, b} both depend on root; a and b can run in parallel.
	g := New()
	_ = g.AddNode(&Node{ID: "root", Label: "root", JobType: worker.JobTypePlan, Prompt: "root"})
	_ = g.AddNode(&Node{ID: "a", Label: "a", JobType: worker.JobTypePlan, Prompt: "a", DependsOn: []NodeID{"root"}})
	_ = g.AddNode(&Node{ID: "b", Label: "b", JobType: worker.JobTypePlan, Prompt: "b", DependsOn: []NodeID{"root"}})
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	sched := NewScheduler(g, pool, WithMaxParallel(2))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	completed := 0
	for evt := range sched.Run(ctx, JobOpts{WorktreeID: "wt-1"}) {
		if evt.Type == EventNodeCompleted {
			completed++
		}
	}

	if completed != 3 {
		t.Errorf("completed = %d, want 3 (root, a, b)", completed)
	}
	for _, id := range []NodeID{"root", "a", "b"} {
		if sched.State().Get(id) != StateDone {
			t.Errorf("node %s = %v, want Done", id, sched.State().Get(id))
		}
	}
}

func TestScheduler_ContextCancelledDrainsCleanly(t *testing.T) {
	pool := poolWithMockAgent(
		t,
		agent.Event{Type: agent.EventComplete},
	)

	g := New()
	_ = g.AddNode(&Node{ID: "n", Label: "n", JobType: worker.JobTypePlan, Prompt: "x"})
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	sched := NewScheduler(g, pool)

	// Pre-cancel so the run loop exits via its ctx.Done() branch. The blocking
	// AllDone emit respects ctx cancellation and may drop the event, so we only
	// assert the event channel is drained and closed without hanging.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for range sched.Run(ctx, JobOpts{WorktreeID: "wt-1"}) {
		// Drain; the loop terminates when run() closes the channel.
	}
	// Reaching here means the channel closed cleanly under cancellation.
}
