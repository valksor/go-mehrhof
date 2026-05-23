package graph

import (
	"context"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/worker"
)

func TestScheduler_ApprovalApproved(t *testing.T) {
	g := New()
	a := subTaskNode("gate", "ok:approved-result")
	a.RequiresApproval = true
	a.ApprovalPrompt = "Proceed?"
	_ = g.AddNode(a)
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	exec := func(_ context.Context, cfg SubTaskConfig) (string, error) {
		return "approved-result", nil
	}
	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })
	sched := NewScheduler(g, pool, WithSubTaskExecutor(exec))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events := sched.Run(ctx, JobOpts{})

	// Drain events in a goroutine; approve when the gate asks.
	done := make(chan struct{})
	sawApprovalRequired := false
	go func() {
		defer close(done)
		for evt := range events {
			if evt.Type == EventNodeApprovalRequired && evt.NodeID == "gate" {
				sawApprovalRequired = true
			}
		}
	}()

	// Approve once the gate registers its approval channel.
	approveByPolling(t, sched, "gate", true)

	<-done

	if !sawApprovalRequired {
		t.Error("expected an approval-required event")
	}
	if sched.State().Get("gate") != StateDone {
		t.Errorf("gate state = %v, want Done after approval", sched.State().Get("gate"))
	}
}

func TestScheduler_ApprovalRejected(t *testing.T) {
	g := New()
	a := subTaskNode("gate", "ok:x")
	a.RequiresApproval = true
	_ = g.AddNode(a)
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	exec := func(_ context.Context, _ SubTaskConfig) (string, error) { return "ran", nil }
	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })
	sched := NewScheduler(g, pool, WithSubTaskExecutor(exec))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	events := sched.Run(ctx, JobOpts{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range events {
		}
	}()

	approveByPolling(t, sched, "gate", false) // reject

	<-done

	if sched.State().Get("gate") != StateFailed {
		t.Errorf("gate state = %v, want Failed after rejection", sched.State().Get("gate"))
	}
}

func TestScheduler_ApprovalTimeout(t *testing.T) {
	g := New()
	a := subTaskNode("gate", "ok:x")
	a.RequiresApproval = true
	a.ApprovalTimeout = 50 * time.Millisecond
	_ = g.AddNode(a)
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	exec := func(_ context.Context, _ SubTaskConfig) (string, error) { return "ran", nil }
	pool := worker.NewPool(worker.DefaultPoolConfig())
	t.Cleanup(func() { _ = pool.Stop() })
	sched := NewScheduler(g, pool, WithSubTaskExecutor(exec))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Never approve; the gate must auto-reject on timeout and the graph finish.
	for range sched.Run(ctx, JobOpts{}) {
	}

	if sched.State().Get("gate") != StateFailed {
		t.Errorf("gate state = %v, want Failed after timeout", sched.State().Get("gate"))
	}
}

// approveByPolling waits until the scheduler has registered a pending approval
// for id, then sends the decision. Fails the test if no approval appears.
func approveByPolling(t *testing.T, s *Scheduler, id NodeID, approve bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var ok bool
		if approve {
			ok = s.ApproveNode(id)
		} else {
			ok = s.RejectNode(id)
		}
		if ok {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("approval gate for %q never became pending", id)
}
