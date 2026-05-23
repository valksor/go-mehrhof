package worker

import (
	"context"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/agenttest"
)

// drainStream consumes a job's event stream until it is closed or the deadline
// elapses, returning the events seen.
func drainStream(t *testing.T, pool *Pool, jobID string) []Event {
	t.Helper()
	stream := pool.Stream(jobID)
	if stream == nil {
		return nil
	}
	var events []Event
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-stream:
			if !ok {
				return events
			}
			events = append(events, ev)
		case <-deadline:
			t.Fatal("timed out draining stream")

			return events
		}
	}
}

func TestSubmitCached_CompletesImmediately(t *testing.T) {
	pool := newTestPool(t)

	job, err := pool.SubmitCached(JobTypePlan, "wt-1", "the prompt", "cached output", &JobOptions{
		WorkDir:  "/tmp/x",
		Agent:    "claude",
		Metadata: map[string]any{"task": "T1"},
	})
	if err != nil {
		t.Fatalf("SubmitCached() error = %v", err)
	}

	if job.Status != JobStatusDone {
		t.Errorf("Status = %q, want done", job.Status)
	}
	if job.Result != "cached output" {
		t.Errorf("Result = %q, want cached output", job.Result)
	}
	if job.StartedAt == nil || job.CompletedAt == nil {
		t.Error("StartedAt and CompletedAt should be set for cached job")
	}
	if cached, _ := job.Metadata["cached"].(bool); !cached {
		t.Error("Metadata[cached] should be true")
	}
	if ov, _ := job.Metadata["agent_override"].(string); ov != "claude" {
		t.Errorf("agent_override = %q, want claude", ov)
	}

	// The stream should carry the cached content then close.
	events := drainStream(t, pool, job.ID)
	var sawStream, sawComplete bool
	for _, ev := range events {
		if ev.Type == "stream" && ev.Content == "cached output" {
			sawStream = true
		}
		if ev.Type == EventJobCompleted {
			sawComplete = true
		}
	}
	if !sawStream {
		t.Error("expected a stream event carrying cached output")
	}
	if !sawComplete {
		t.Error("expected a completion event")
	}
}

func TestSubmitCached_NilOptsStillTagsCached(t *testing.T) {
	pool := newTestPool(t)

	job, err := pool.SubmitCached(JobTypeChat, "wt-2", "p", "r", nil)
	if err != nil {
		t.Fatalf("SubmitCached() error = %v", err)
	}
	if cached, _ := job.Metadata["cached"].(bool); !cached {
		t.Error("Metadata[cached] should be true even with nil opts")
	}
	drainStream(t, pool, job.ID)
}

func TestExecuteDryRunJob(t *testing.T) {
	pool := newTestPool(t)

	job, err := pool.Submit(JobTypeDryRun, "wt-1", "this would run")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	events := drainStream(t, pool, job.ID)
	if len(events) == 0 {
		t.Fatal("expected dry-run events")
	}

	got := pool.GetJob(job.ID)
	if got.Status != JobStatusDone {
		t.Errorf("Status = %q, want done", got.Status)
	}
	if got.Result == "" {
		t.Error("dry-run Result should be non-empty")
	}
	if got.StartedAt == nil || got.CompletedAt == nil {
		t.Error("dry-run should set StartedAt and CompletedAt")
	}
}

func TestInterruptJob(t *testing.T) {
	pool := newTestPool(t)

	// Not found -> nil.
	if err := pool.InterruptJob("missing"); err != nil {
		t.Errorf("InterruptJob(missing) = %v, want nil", err)
	}

	// Queued (not in progress) -> nil.
	pool.mu.Lock()
	pool.jobs["queued"] = &Job{ID: "queued", Status: JobStatusQueued}
	pool.mu.Unlock()
	if err := pool.InterruptJob("queued"); err != nil {
		t.Errorf("InterruptJob(queued) = %v, want nil", err)
	}

	// In progress with a connected agent -> Interrupt() is invoked.
	ag := newTestableAgent("interruptible")
	pool.mu.Lock()
	pool.workers["wi"] = &Worker{ID: "wi", Status: StatusWorking, Agent: ag, CurrentJob: "running"}
	pool.jobs["running"] = &Job{ID: "running", Status: JobStatusInProgress, WorkerID: "wi"}
	pool.mu.Unlock()

	if err := pool.InterruptJob("running"); err != nil {
		t.Errorf("InterruptJob(running) = %v, want nil", err)
	}

	// In progress but no worker assigned -> nil.
	pool.mu.Lock()
	pool.jobs["orphan"] = &Job{ID: "orphan", Status: JobStatusInProgress}
	pool.mu.Unlock()
	if err := pool.InterruptJob("orphan"); err != nil {
		t.Errorf("InterruptJob(orphan) = %v, want nil", err)
	}
}

func TestRemoveJob(t *testing.T) {
	pool := newTestPool(t)

	// Terminal job is removed.
	now := time.Now()
	pool.mu.Lock()
	pool.jobs["done"] = &Job{ID: "done", Status: JobStatusDone, CompletedAt: &now}
	pool.jobCancels["done"] = func() {}
	pool.jobs["active"] = &Job{ID: "active", Status: JobStatusInProgress}
	pool.mu.Unlock()

	pool.RemoveJob("done")
	if pool.GetJob("done") != nil {
		t.Error("RemoveJob(done) should remove the job")
	}

	// Active job is NOT removed.
	pool.RemoveJob("active")
	if pool.GetJob("active") == nil {
		t.Error("RemoveJob(active) should not remove an in-progress job")
	}

	// Missing job is a harmless no-op.
	pool.RemoveJob("never")
}

func TestAssignJob_FallbackWhenPreferredUnavailable(t *testing.T) {
	pool := newTestPool(t)

	// Only a "codex" worker is available; the job prefers "claude". The
	// dispatcher should fall back to codex rather than blocking forever.
	ag := newTestableAgent("codex", withEvents([]agent.Event{
		{Type: agent.EventStream, Content: "fallback result"},
		{Type: agent.EventComplete},
	}))
	pool.mu.Lock()
	pool.workers["codex-w"] = &Worker{ID: "codex-w", Status: StatusAvailable, AgentName: "codex", Agent: ag}
	pool.mu.Unlock()

	job, err := pool.SubmitWithOptions(JobTypePlan, "wt-1", "do the thing", &JobOptions{Agent: "claude"})
	if err != nil {
		t.Fatalf("SubmitWithOptions() error = %v", err)
	}

	drainStream(t, pool, job.ID)

	got := pool.GetJob(job.ID)
	if got.Status != JobStatusDone {
		t.Errorf("Status = %q, want done", got.Status)
	}
	if got.WorkerID != "codex-w" {
		t.Errorf("WorkerID = %q, want codex-w (fallback)", got.WorkerID)
	}
	if got.Result != "fallback result" {
		t.Errorf("Result = %q, want fallback result", got.Result)
	}
}

func TestAddAgentWorker_FromRegistry(t *testing.T) {
	mock := agenttest.NewMockAgent("mockagent")

	reg := agent.NewRegistry()
	if err := reg.Register(mock); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	pool := NewPool(PoolConfig{MaxWorkers: 3, Agents: reg})
	t.Cleanup(func() { _ = pool.Stop() })

	w, err := pool.AddAgentWorker(context.Background(), "mockagent", false)
	if err != nil {
		t.Fatalf("AddAgentWorker() error = %v", err)
	}
	if w.AgentName != "mockagent" {
		t.Errorf("AgentName = %q, want mockagent", w.AgentName)
	}
	if w.Agent == nil {
		t.Error("worker Agent should be set")
	}
	if !w.Agent.Connected() {
		t.Error("agent should be connected after AddAgentWorker")
	}
}

func TestAddAgentWorker_DefaultID(t *testing.T) {
	mock := agenttest.NewMockAgent("mockagent")
	reg := agent.NewRegistry()
	_ = reg.Register(mock)

	pool := NewPool(PoolConfig{MaxWorkers: 3, Agents: reg})
	t.Cleanup(func() { _ = pool.Stop() })

	w, err := pool.AddAgentWorker(context.Background(), "mockagent", true)
	if err != nil {
		t.Fatalf("AddAgentWorker() error = %v", err)
	}
	if w.ID != "default" {
		t.Errorf("default worker ID = %q, want default", w.ID)
	}
	if !w.IsDefault {
		t.Error("worker IsDefault should be true")
	}
}

func TestAddAgentWorker_NotInAllowedList(t *testing.T) {
	mock := agenttest.NewMockAgent("mockagent")
	reg := agent.NewRegistry()
	_ = reg.Register(mock)

	pool := NewPool(PoolConfig{MaxWorkers: 3, Agents: reg, AllowedAgents: []string{"claude"}})
	t.Cleanup(func() { _ = pool.Stop() })

	_, err := pool.AddAgentWorker(context.Background(), "mockagent", false)
	if err == nil {
		t.Error("AddAgentWorker() with disallowed agent should error")
	}
}

func TestAddAgentWorker_ConnectError(t *testing.T) {
	mock := agenttest.NewMockAgent("badconn")
	// Register an agent whose Connect always errors so AddAgentWorker fails at
	// the connect step (after registry lookup succeeds).
	reg := agent.NewRegistry()
	_ = reg.Register(&connFailAgent{MockAgent: mock})

	pool := NewPool(PoolConfig{MaxWorkers: 3, Agents: reg})
	t.Cleanup(func() { _ = pool.Stop() })

	_, err := pool.AddAgentWorker(context.Background(), "badconn", false)
	if err == nil {
		t.Error("AddAgentWorker() should error when agent fails to connect")
	}
}

// connFailAgent wraps a MockAgent but always fails to connect.
type connFailAgent struct {
	*agenttest.MockAgent
}

func (c *connFailAgent) Connect(_ context.Context) error {
	return context.DeadlineExceeded
}
