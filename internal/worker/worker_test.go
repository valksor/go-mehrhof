package worker

import (
	"testing"
	"time"
)

func TestPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()

	if cfg.MaxWorkers != 5 {
		t.Errorf("DefaultPoolConfig().MaxWorkers = %d, want 5", cfg.MaxWorkers)
	}

	if cfg.BasePort != 8765 {
		t.Errorf("DefaultPoolConfig().BasePort = %d, want 8765", cfg.BasePort)
	}
}

func TestNewPool(t *testing.T) {
	cfg := DefaultPoolConfig()
	pool := NewPool(cfg)

	if pool == nil {
		t.Fatal("NewPool returned nil")
	}

	stats := pool.Stats()
	if stats.TotalWorkers != 0 {
		t.Errorf("new pool TotalWorkers = %d, want 0", stats.TotalWorkers)
	}
}

func TestPoolAddWorker(t *testing.T) {
	cfg := DefaultPoolConfig()
	pool := NewPool(cfg)

	w := pool.AddWorker()
	if w == nil {
		t.Fatal("AddWorker returned nil")
	}

	if w.Status != StatusAvailable {
		t.Errorf("worker.Status = %s, want %s", w.Status, StatusAvailable)
	}

	stats := pool.Stats()
	if stats.TotalWorkers != 1 {
		t.Errorf("pool TotalWorkers = %d, want 1", stats.TotalWorkers)
	}
	if stats.AvailableWorkers != 1 {
		t.Errorf("pool AvailableWorkers = %d, want 1", stats.AvailableWorkers)
	}
}

func TestPoolMaxWorkers(t *testing.T) {
	cfg := PoolConfig{MaxWorkers: 2, BasePort: 8765}
	pool := NewPool(cfg)

	w1 := pool.AddWorker()
	w2 := pool.AddWorker()
	w3 := pool.AddWorker()

	if w1 == nil {
		t.Error("first worker should be added")
	}
	if w2 == nil {
		t.Error("second worker should be added")
	}
	if w3 != nil {
		t.Error("third worker should be rejected (max reached)")
	}

	stats := pool.Stats()
	if stats.TotalWorkers != 2 {
		t.Errorf("pool TotalWorkers = %d, want 2", stats.TotalWorkers)
	}
}

func TestPoolRemoveWorker(t *testing.T) {
	cfg := DefaultPoolConfig()
	pool := NewPool(cfg)

	w := pool.AddWorker()
	_ = pool.RemoveWorker(w.ID)

	stats := pool.Stats()
	if stats.TotalWorkers != 0 {
		t.Errorf("pool TotalWorkers after remove = %d, want 0", stats.TotalWorkers)
	}
}

func TestPoolListWorkers(t *testing.T) {
	cfg := DefaultPoolConfig()
	pool := NewPool(cfg)

	pool.AddWorker()
	pool.AddWorker()

	workers := pool.ListWorkers()
	if len(workers) != 2 {
		t.Errorf("ListWorkers length = %d, want 2", len(workers))
	}
}

func TestPoolStartStop(t *testing.T) {
	cfg := DefaultPoolConfig()
	pool := NewPool(cfg)

	if err := pool.Start(); err != nil {
		t.Errorf("Start() error = %v", err)
	}

	// Give dispatcher goroutine time to start
	time.Sleep(10 * time.Millisecond)

	if err := pool.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestJobTypes(t *testing.T) {
	types := []JobType{JobTypePlan, JobTypeImplement, JobTypeReview, JobTypeOptimize}

	for _, jt := range types {
		if jt == "" {
			t.Error("job type should not be empty")
		}
	}
}

func TestJobStatus(t *testing.T) {
	statuses := []JobStatus{JobStatusQueued, JobStatusInProgress, JobStatusDone, JobStatusFailed}

	for _, s := range statuses {
		if s == "" {
			t.Error("job status should not be empty")
		}
	}
}

func TestWorkerStatus(t *testing.T) {
	statuses := []WorkerStatus{StatusAvailable, StatusWorking, StatusDisconnected}

	for _, s := range statuses {
		if s == "" {
			t.Error("worker status should not be empty")
		}
	}
}

func TestEvent(t *testing.T) {
	e := Event{
		Type:      "stream",
		JobID:     "job-123",
		Content:   "test content",
		Timestamp: time.Now(),
	}

	if e.Type != "stream" {
		t.Errorf("Event.Type = %s, want stream", e.Type)
	}

	if e.JobID != "job-123" {
		t.Errorf("Event.JobID = %s, want job-123", e.JobID)
	}
}

func TestJob(t *testing.T) {
	now := time.Now()
	j := &Job{
		ID:         "job-1",
		Type:       JobTypeImplement,
		WorktreeID: "wt-1",
		Prompt:     "test prompt",
		Status:     JobStatusQueued,
		CreatedAt:  now,
	}

	if j.ID != "job-1" {
		t.Errorf("Job.ID = %s, want job-1", j.ID)
	}

	if j.Type != JobTypeImplement {
		t.Errorf("Job.Type = %s, want %s", j.Type, JobTypeImplement)
	}

	if j.Status != JobStatusQueued {
		t.Errorf("Job.Status = %s, want %s", j.Status, JobStatusQueued)
	}
}
