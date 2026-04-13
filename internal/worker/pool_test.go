package worker

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func newTestPool(t *testing.T) *Pool {
	t.Helper()
	pool := NewPool(DefaultPoolConfig())
	if err := pool.Start(); err != nil {
		t.Fatalf("pool.Start() error = %v", err)
	}
	t.Cleanup(func() { _ = pool.Stop() })

	return pool
}

func TestSubmit_CreatesJob(t *testing.T) {
	pool := newTestPool(t)

	job, err := pool.Submit(JobTypePlan, "wt-1", "write a plan")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if job == nil {
		t.Fatal("Submit() = nil job")
	}
	if job.ID == "" {
		t.Error("job.ID should not be empty")
	}
	if job.Type != JobTypePlan {
		t.Errorf("job.Type = %q, want %q", job.Type, JobTypePlan)
	}
	if job.WorktreeID != "wt-1" {
		t.Errorf("job.WorktreeID = %q, want wt-1", job.WorktreeID)
	}
	if job.Prompt != "write a plan" {
		t.Errorf("job.Prompt = %q, want 'write a plan'", job.Prompt)
	}
}

func TestSubmit_JobQueuedStatus(t *testing.T) {
	// Pool with no workers — job stays queued initially
	pool := NewPool(PoolConfig{MaxWorkers: 5, BasePort: 8765})
	if err := pool.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pool.Stop() }()

	job, err := pool.Submit(JobTypePlan, "wt-1", "test")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	// No workers available, should be queued
	if job.Status != JobStatusQueued {
		t.Errorf("job.Status = %q, want %q", job.Status, JobStatusQueued)
	}
}

func TestGetJob(t *testing.T) {
	pool := newTestPool(t)

	job, err := pool.Submit(JobTypeImplement, "wt-1", "implement X")
	if err != nil {
		t.Fatal(err)
	}

	got := pool.GetJob(job.ID)
	if got == nil {
		t.Fatal("GetJob() = nil, want job")
	}
	if got.ID != job.ID {
		t.Errorf("GetJob().ID = %q, want %q", got.ID, job.ID)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	pool := newTestPool(t)
	if got := pool.GetJob("no-such-id"); got != nil {
		t.Errorf("GetJob(missing) = %v, want nil", got)
	}
}

func TestListJobs_Empty(t *testing.T) {
	pool := newTestPool(t)
	jobs := pool.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("ListJobs() empty = %d, want 0", len(jobs))
	}
}

func TestListJobs_AfterSubmit(t *testing.T) {
	pool := newTestPool(t)

	if _, err := pool.Submit(JobTypePlan, "wt-1", "prompt1"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Submit(JobTypeImplement, "wt-1", "prompt2"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Submit(JobTypeReview, "wt-1", "prompt3"); err != nil {
		t.Fatal(err)
	}

	jobs := pool.ListJobs()
	if len(jobs) != 3 {
		t.Errorf("ListJobs() len = %d, want 3", len(jobs))
	}
}

func TestListQueuedJobs(t *testing.T) {
	// Pool with no workers — all jobs stay queued
	pool := NewPool(PoolConfig{MaxWorkers: 5, BasePort: 8765})
	if err := pool.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pool.Stop() }()

	if _, err := pool.Submit(JobTypePlan, "wt-1", "plan it"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Submit(JobTypeImplement, "wt-1", "implement it"); err != nil {
		t.Fatal(err)
	}

	queued := pool.ListQueuedJobs()
	if len(queued) != 2 {
		t.Errorf("ListQueuedJobs() len = %d, want 2", len(queued))
	}
	for _, j := range queued {
		if j.Status != JobStatusQueued && j.Status != JobStatusInProgress {
			t.Errorf("job %q status = %q, want queued or in_progress", j.ID, j.Status)
		}
	}
}

func TestStats_Empty(t *testing.T) {
	pool := newTestPool(t)
	stats := pool.Stats()

	if stats.TotalWorkers != 0 {
		t.Errorf("TotalWorkers = %d, want 0", stats.TotalWorkers)
	}
	if stats.AvailableWorkers != 0 {
		t.Errorf("AvailableWorkers = %d, want 0", stats.AvailableWorkers)
	}
	if stats.QueuedJobs != 0 {
		t.Errorf("QueuedJobs = %d, want 0", stats.QueuedJobs)
	}
}

func TestStats_WithWorkersAndJobs(t *testing.T) {
	pool := newTestPool(t)

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

func TestAddWorkerWithAgent_Fields(t *testing.T) {
	pool := newTestPool(t)

	w := pool.AddWorkerWithAgent("claude")
	if w == nil {
		t.Fatal("AddWorkerWithAgent() = nil")
	}
	if w.AgentName != "claude" {
		t.Errorf("w.AgentName = %q, want claude", w.AgentName)
	}
	if w.Agent != nil {
		t.Error("AddWorkerWithAgent should not set Agent (not connected)")
	}
}

func TestSimulatedJobExecution(t *testing.T) {
	pool := newTestPool(t)

	// Add a simulated worker (nil agent)
	pool.AddWorker()

	job, err := pool.Submit(JobTypeImplement, "wt-1", "implement the feature")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	stream := pool.Stream(job.ID)

	// Collect events with timeout
	var events []Event
	deadline := time.After(5 * time.Second)
	done := false
	for !done {
		select {
		case ev, ok := <-stream:
			if !ok {
				done = true
			} else {
				events = append(events, ev)
			}
		case <-deadline:
			t.Fatal("timed out waiting for simulated job to complete")
		}
	}

	// Job should be done
	got := pool.GetJob(job.ID)
	if got.Status != JobStatusDone {
		t.Errorf("job.Status = %q, want %q", got.Status, JobStatusDone)
	}
	if got.Result == "" {
		t.Error("job.Result should not be empty after simulated completion")
	}
	if got.CompletedAt == nil {
		t.Error("job.CompletedAt should be set")
	}

	// Should have received at least stream and completed events
	if len(events) == 0 {
		t.Error("expected at least one event from simulated job")
	}
}

func TestStreamClosesOnCompletion(t *testing.T) {
	pool := newTestPool(t)
	pool.AddWorker()

	job, err := pool.Submit(JobTypePlan, "wt-1", "plan the feature")
	if err != nil {
		t.Fatal(err)
	}

	stream := pool.Stream(job.ID)
	if stream == nil {
		t.Fatal("Stream() returned nil for active job")
	}

	// Drain until closed
	deadline := time.After(5 * time.Second)
	closed := false
	for !closed {
		select {
		case _, ok := <-stream:
			if !ok {
				closed = true
			}
		case <-deadline:
			t.Fatal("stream not closed within timeout")
		}
	}

	if !closed {
		t.Error("stream should be closed after job completion")
	}
}

func TestCancelJob_Queued(t *testing.T) {
	// Pool with no workers so job stays queued
	pool := NewPool(PoolConfig{MaxWorkers: 5, BasePort: 8765})
	if err := pool.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pool.Stop() }()

	job, err := pool.Submit(JobTypePlan, "wt-1", "test prompt")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if err := pool.CancelJob(job.ID); err != nil {
		t.Fatalf("CancelJob() error = %v", err)
	}

	got := pool.GetJob(job.ID)
	if got.Status != JobStatusFailed {
		t.Errorf("job.Status = %q, want %q", got.Status, JobStatusFailed)
	}
	if got.Error != "cancelled" {
		t.Errorf("job.Error = %q, want %q", got.Error, "cancelled")
	}
}

func TestCancelJob_NotFound(t *testing.T) {
	pool := newTestPool(t)

	err := pool.CancelJob("nonexistent")
	if err == nil {
		t.Error("CancelJob(nonexistent) should return error")
	}
}

func TestCancelJob_AlreadyDone(t *testing.T) {
	pool := newTestPool(t)
	pool.AddWorker()

	job, err := pool.Submit(JobTypePlan, "wt-1", "test")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for simulated job to complete
	stream := pool.Stream(job.ID)
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-stream:
			if !ok {
				goto done
			}
		case <-deadline:
			t.Fatal("timed out")
		}
	}
done:

	// Cancelling a completed job should not error (no-op)
	if err := pool.CancelJob(job.ID); err != nil {
		t.Errorf("CancelJob(done) error = %v, want nil", err)
	}
}

func TestSubmitWithOptions(t *testing.T) {
	pool := newTestPool(t)

	opts := &JobOptions{
		WorkDir:     "/tmp/test",
		Environment: map[string]string{"KEY": "val"},
		Metadata:    map[string]any{"meta": "data"},
	}

	job, err := pool.SubmitWithOptions(JobTypePlan, "wt-2", "plan it", opts)
	if err != nil {
		t.Fatalf("SubmitWithOptions() error = %v", err)
	}

	if job.WorkDir != "/tmp/test" {
		t.Errorf("job.WorkDir = %q, want /tmp/test", job.WorkDir)
	}
	if job.Environment["KEY"] != "val" {
		t.Errorf("job.Environment[KEY] = %q, want val", job.Environment["KEY"])
	}
	if job.Metadata["meta"] != "data" {
		t.Errorf("job.Metadata[meta] = %q, want data", job.Metadata["meta"])
	}
}

func TestConcurrentJobSubmission(t *testing.T) {
	pool := newTestPool(t)

	// Add 3 simulated workers
	for range 3 {
		pool.AddWorker()
	}

	const numJobs = 10
	var wg sync.WaitGroup
	errs := make(chan error, numJobs)
	jobIDs := make(chan string, numJobs)

	// Submit 10 jobs concurrently across different worktrees
	for i := range numJobs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			worktree := fmt.Sprintf("wt-%d", idx%3) // 3 different worktrees
			job, err := pool.Submit(JobTypeImplement, worktree, fmt.Sprintf("task %d", idx))
			if err != nil {
				errs <- fmt.Errorf("submit job %d: %w", idx, err)

				return
			}
			jobIDs <- job.ID
		}(i)
	}

	wg.Wait()
	close(errs)
	close(jobIDs)

	for err := range errs {
		t.Error(err)
	}

	// Collect all job IDs
	ids := make([]string, 0, numJobs)
	for id := range jobIDs {
		ids = append(ids, id)
	}

	if len(ids) != numJobs {
		t.Fatalf("submitted %d jobs, want %d", len(ids), numJobs)
	}

	// Wait for all jobs to complete by checking stats (avoids accessing job fields directly
	// which would race with the dispatcher goroutine writing to them)
	deadline := time.After(15 * time.Second)
	for {
		stats := pool.Stats()
		if stats.CompletedJobs+stats.FailedJobs >= numJobs {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out: completed=%d failed=%d, want total>=%d",
				stats.CompletedJobs, stats.FailedJobs, numJobs)
		case <-time.After(100 * time.Millisecond):
		}
	}

	stats := pool.Stats()
	if stats.CompletedJobs < numJobs {
		t.Errorf("completed = %d, want >= %d", stats.CompletedJobs, numJobs)
	}
}

func TestConcurrentStreamReaders(t *testing.T) {
	pool := newTestPool(t)
	pool.AddWorker()

	job, err := pool.Submit(JobTypeImplement, "wt-1", "concurrent readers test")
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}

	// 5 concurrent readers of the same job stream
	const numReaders = 5
	var wg sync.WaitGroup
	eventCounts := make([]int, numReaders)

	for i := range numReaders {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			stream := pool.Stream(job.ID)
			count := 0
			deadline := time.After(5 * time.Second)
			for {
				select {
				case _, ok := <-stream:
					if !ok {
						eventCounts[idx] = count

						return
					}
					count++
				case <-deadline:
					eventCounts[idx] = count

					return
				}
			}
		}(i)
	}

	wg.Wait()

	// All readers should have received at least 1 event
	for i, count := range eventCounts {
		if count == 0 {
			t.Errorf("reader %d received 0 events", i)
		}
	}
}
