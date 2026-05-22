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

// TestScheduler_InstantJobNoStreamRace guards against a regression where a job
// that finished before watchNode subscribed to its event stream was reported
// as a node failure ("no stream for job").
//
// The worker's closeStream removes a job's stream from the pool the instant the
// job completes, so when an agent finishes near-instantly (cached results, the
// mock agent) a late watchNode subscription can observe a nil channel. The
// scheduler must fall back to the job's terminal record rather than failing the
// node. The race is purely scheduler-timing dependent — it surfaces readily on
// some platforms (Linux CI) and rarely on others — so the single-node graph is
// run many times to give the race room to trigger if the guard regresses.
func TestScheduler_InstantJobNoStreamRace(t *testing.T) {
	t.Parallel()

	registry := agent.NewRegistry()
	mock := agenttest.NewMockAgent(
		"mock",
		agent.Event{Type: agent.EventStream, Content: "INSTANT DONE"},
		agent.Event{Type: agent.EventComplete},
	)
	if err := registry.Register(mock); err != nil {
		t.Fatalf("register mock: %v", err)
	}

	pool := worker.NewPool(worker.PoolConfig{MaxWorkers: 1, Agents: registry})
	if err := pool.Start(); err != nil {
		t.Fatalf("start pool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Stop() })
	if _, err := pool.AddAgentWorker(context.Background(), "mock", true); err != nil {
		t.Fatalf("add worker: %v", err)
	}

	for i := range 100 {
		g := New()
		_ = g.AddNode(&Node{ID: "only", Label: "plan", JobType: worker.JobTypePlan, Prompt: "go"})
		if err := g.Validate(); err != nil {
			t.Fatalf("iteration %d: validate: %v", i, err)
		}

		sched := NewScheduler(g, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		var (
			output     strings.Builder
			allDoneErr string
			sawDone    bool
		)
		for evt := range sched.Run(ctx, JobOpts{}) {
			switch evt.Type { //nolint:exhaustive // test inspects a subset
			case EventNodeOutput:
				output.WriteString(evt.Content)
			case EventAllDone:
				sawDone = true
				allDoneErr = evt.Error
			}
		}
		cancel()

		if !sawDone {
			t.Fatalf("iteration %d: never saw EventAllDone", i)
		}
		if allDoneErr != "" {
			t.Fatalf("iteration %d: graph finished with error %q", i, allDoneErr)
		}
		if sched.State().HasFailures() {
			t.Fatalf("iteration %d: node failed unexpectedly (no-stream race regression)", i)
		}
		// The node result is the guaranteed contract (it feeds aggregation);
		// the live output event is best-effort but is replayed even when the
		// stream is gone, so both must carry the job's text.
		if got := sched.State().Results()["only"]; !strings.Contains(got, "INSTANT DONE") {
			t.Fatalf("iteration %d: missing node result, got %q", i, got)
		}
		if !strings.Contains(output.String(), "INSTANT DONE") {
			t.Fatalf("iteration %d: missing node output, got %q", i, output.String())
		}
	}
}
