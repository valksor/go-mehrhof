package worker

import (
	"context"
	"testing"
	"time"

	"github.com/valksor/kvelmo/pkg/agent"
)

func TestAddDefaultWorker(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())

	w := pool.AddDefaultWorker("claude")
	if w == nil {
		t.Fatal("AddDefaultWorker() returned nil")
	}
	if w.ID != "default" {
		t.Errorf("ID = %q, want default", w.ID)
	}
	if w.AgentName != "claude" {
		t.Errorf("AgentName = %q, want claude", w.AgentName)
	}
	if !w.IsDefault {
		t.Error("IsDefault should be true")
	}
	if w.Status != StatusAvailable {
		t.Errorf("Status = %q, want %q", w.Status, StatusAvailable)
	}
}

func TestPoolAgents(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	if pool.Agents() == nil {
		t.Error("Agents() should not return nil")
	}
}

func TestAddAgentWorker_UnknownAgent(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	_, err := pool.AddAgentWorker(context.Background(), "nonexistent-agent-xyz", false)
	if err == nil {
		t.Error("AddAgentWorker() with unknown agent should return error")
	}
}

func TestStats_WithAvailableWorkers(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	pool.AddWorker()
	pool.AddWorker()

	stats := pool.Stats()
	if stats.TotalWorkers != 2 {
		t.Errorf("TotalWorkers = %d, want 2", stats.TotalWorkers)
	}
	if stats.AvailableWorkers != 2 {
		t.Errorf("AvailableWorkers = %d, want 2", stats.AvailableWorkers)
	}
}

func TestStats_WithQueuedJob(t *testing.T) {
	// Pool with no workers so job stays queued
	pool := NewPool(DefaultPoolConfig())
	if err := pool.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pool.Stop() }()

	_, err := pool.Submit(JobTypePlan, "wt-1", "plan")
	if err != nil {
		t.Fatal(err)
	}

	stats := pool.Stats()
	if stats.QueuedJobs != 1 {
		t.Errorf("QueuedJobs = %d, want 1", stats.QueuedJobs)
	}
}

func TestMinBuiltin(t *testing.T) {
	tests := []struct{ a, b, want int }{
		{3, 5, 3},
		{5, 3, 3},
		{4, 4, 4},
	}
	for _, tt := range tests {
		if got := min(tt.a, tt.b); got != tt.want {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestRemoveWorker_NotFound(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	if err := pool.RemoveWorker("nonexistent-id"); err == nil {
		t.Error("RemoveWorker() should return error for non-existent worker")
	}
}

func TestRemoveWorker_Default(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	pool.AddDefaultWorker("claude")
	if err := pool.RemoveWorker("default"); err == nil {
		t.Error("RemoveWorker() should return error for default worker")
	}
}

func TestRemoveWorker_NonDefault(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	w := pool.AddWorker()
	if w == nil {
		t.Fatal("AddWorker() returned nil")
	}
	if err := pool.RemoveWorker(w.ID); err != nil {
		t.Errorf("RemoveWorker() error = %v, want nil", err)
	}
	for _, ww := range pool.ListWorkers() {
		if ww.ID == w.ID {
			t.Error("worker still present after removal")
		}
	}
}

func TestListWorkers_Empty(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	if got := pool.ListWorkers(); len(got) != 0 {
		t.Errorf("ListWorkers() = %d workers, want 0", len(got))
	}
}

func TestListWorkers_Sorted(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	pool.AddWorker()
	pool.AddWorker()

	workers := pool.ListWorkers()
	if len(workers) != 2 {
		t.Fatalf("ListWorkers() = %d workers, want 2", len(workers))
	}
	if workers[0].ID > workers[1].ID {
		t.Error("ListWorkers() not sorted by ID")
	}
}

func TestAddWorkerWithAgent_FieldsSet(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	w := pool.AddWorkerWithAgent("claude")
	if w == nil {
		t.Fatal("AddWorkerWithAgent() returned nil")
	}
	if w.AgentName != "claude" {
		t.Errorf("AgentName = %q, want claude", w.AgentName)
	}
}

func TestAddWorkerWithAgent_AtMaxCapacity(t *testing.T) {
	cfg := DefaultPoolConfig()
	cfg.MaxWorkers = 1
	pool := NewPool(cfg)
	pool.AddWorker() // fills capacity

	if w := pool.AddWorkerWithAgent("claude"); w != nil {
		t.Error("AddWorkerWithAgent() should return nil when at max capacity")
	}
}

// mockAgent implements agent.Agent for testing purposes.
type mockAgent struct {
	connected bool
	closed    bool
}

func (m *mockAgent) Name() string     { return "mock" }
func (m *mockAgent) Available() error { return nil }
func (m *mockAgent) Connect(_ context.Context) error {
	m.connected = true

	return nil
}
func (m *mockAgent) Connected() bool { return m.connected }
func (m *mockAgent) SendPrompt(_ context.Context, _ string) (<-chan agent.Event, error) {
	return nil, nil //nolint:nilnil // mock only
}
func (m *mockAgent) HandlePermission(_ string, _ bool) error { return nil }
func (m *mockAgent) Close() error {
	m.closed = true

	return nil
}
func (m *mockAgent) Interrupt() error                        { return nil }
func (m *mockAgent) WithEnv(_, _ string) agent.Agent         { return m }
func (m *mockAgent) WithArgs(_ ...string) agent.Agent        { return m }
func (m *mockAgent) WithWorkDir(_ string) agent.Agent        { return m }
func (m *mockAgent) WithTimeout(_ time.Duration) agent.Agent { return m }

func TestStats_WorkingWorker(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	pool.mu.Lock()
	pool.workers["working-w"] = &Worker{
		ID:     "working-w",
		Status: StatusWorking,
	}
	pool.mu.Unlock()

	stats := pool.Stats()
	if stats.WorkingWorkers != 1 {
		t.Errorf("WorkingWorkers = %d, want 1", stats.WorkingWorkers)
	}
}

func TestStats_JobStatusCounts(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	now := time.Now()
	pool.mu.Lock()
	pool.jobs["j-done"] = &Job{ID: "j-done", Status: JobStatusDone, CreatedAt: now}
	pool.jobs["j-fail"] = &Job{ID: "j-fail", Status: JobStatusFailed, CreatedAt: now}
	pool.jobs["j-prog"] = &Job{ID: "j-prog", Status: JobStatusInProgress, CreatedAt: now}
	pool.mu.Unlock()

	stats := pool.Stats()
	if stats.CompletedJobs != 1 {
		t.Errorf("CompletedJobs = %d, want 1", stats.CompletedJobs)
	}
	if stats.FailedJobs != 1 {
		t.Errorf("FailedJobs = %d, want 1", stats.FailedJobs)
	}
	if stats.InProgressJobs != 1 {
		t.Errorf("InProgressJobs = %d, want 1", stats.InProgressJobs)
	}
}

func TestStop_WithAgentWorker(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	if err := pool.Start(); err != nil {
		t.Fatal(err)
	}

	mock := &mockAgent{connected: true}
	pool.mu.Lock()
	pool.workers["ag-w"] = &Worker{
		ID:    "ag-w",
		Agent: mock,
	}
	pool.mu.Unlock()

	if err := pool.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if !mock.closed {
		t.Error("Stop() should call Close() on agent workers")
	}
}

func TestListWorkers_AgentDisconnected(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	mock := &mockAgent{connected: false}
	pool.mu.Lock()
	pool.workers["disc-w"] = &Worker{
		ID:     "disc-w",
		Status: StatusAvailable,
		Agent:  mock,
	}
	pool.mu.Unlock()

	workers := pool.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("ListWorkers() = %d workers, want 1", len(workers))
	}
	if workers[0].Status != StatusDisconnected {
		t.Errorf("Status = %q, want %q", workers[0].Status, StatusDisconnected)
	}
}

func TestListWorkers_AgentReconnected(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	mock := &mockAgent{connected: true}
	pool.mu.Lock()
	pool.workers["conn-w"] = &Worker{
		ID:     "conn-w",
		Status: StatusDisconnected,
		Agent:  mock,
	}
	pool.mu.Unlock()

	workers := pool.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("ListWorkers() = %d workers, want 1", len(workers))
	}
	if workers[0].Status != StatusAvailable {
		t.Errorf("Status = %q, want %q", workers[0].Status, StatusAvailable)
	}
}

func TestSubmitWithOptions_NilOpts(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	if err := pool.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pool.Stop() }()

	job, err := pool.SubmitWithOptions(JobTypePlan, "wt-1", "do something", nil)
	if err != nil {
		t.Fatalf("SubmitWithOptions() error = %v", err)
	}
	if job == nil {
		t.Fatal("SubmitWithOptions() returned nil")
	}
}

func TestSubmitWithOptions_WithOpts(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	if err := pool.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pool.Stop() }()

	opts := &JobOptions{
		WorkDir:     "/tmp",
		Environment: map[string]string{"FOO": "bar"},
		Metadata:    map[string]any{"key": "val"},
	}
	job, err := pool.SubmitWithOptions(JobTypePlan, "wt-2", "prompt", opts)
	if err != nil {
		t.Fatalf("SubmitWithOptions() error = %v", err)
	}
	if job.WorkDir != "/tmp" {
		t.Errorf("WorkDir = %q, want /tmp", job.WorkDir)
	}
}
