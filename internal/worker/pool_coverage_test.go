package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
)

// --- CancelJob edge cases ---

func TestCancelJob_AlreadyFailed(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	now := time.Now()
	pool.mu.Lock()
	pool.jobs["j-fail"] = &Job{
		ID:          "j-fail",
		Status:      JobStatusFailed,
		Error:       "original error",
		CompletedAt: &now,
	}
	pool.mu.Unlock()

	// Cancelling an already-failed job should be a no-op (no error)
	if err := pool.CancelJob("j-fail"); err != nil {
		t.Errorf("CancelJob(failed) error = %v, want nil", err)
	}

	// Original error should be preserved
	got := pool.GetJob("j-fail")
	if got.Error != "original error" {
		t.Errorf("Error = %q, want original error", got.Error)
	}
}

func TestCancelJob_InProgressReleasesWorker(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	if err := pool.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pool.Stop() }()

	w := pool.AddWorker()

	pool.mu.Lock()
	w.Status = StatusWorking
	w.CurrentJob = "j-running"
	cancelCalled := false
	pool.jobs["j-running"] = &Job{
		ID:       "j-running",
		Status:   JobStatusInProgress,
		WorkerID: w.ID,
	}
	pool.jobCancels["j-running"] = func() { cancelCalled = true }
	pool.mu.Unlock()

	if err := pool.CancelJob("j-running"); err != nil {
		t.Fatalf("CancelJob() error = %v", err)
	}

	if !cancelCalled {
		t.Error("cancel func should have been called")
	}

	got := pool.GetJob("j-running")
	if got.Status != JobStatusFailed {
		t.Errorf("Status = %q, want %q", got.Status, JobStatusFailed)
	}
	if got.Error != "cancelled" {
		t.Errorf("Error = %q, want cancelled", got.Error)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}

	// Worker should be released
	workers := pool.ListWorkers()
	for _, ww := range workers {
		if ww.ID == w.ID {
			if ww.Status != StatusAvailable {
				t.Errorf("worker Status = %q, want %q", ww.Status, StatusAvailable)
			}
			if ww.CurrentJob != "" {
				t.Errorf("worker CurrentJob = %q, want empty", ww.CurrentJob)
			}
		}
	}
}

func TestCancelJob_QueuedNoCancel(t *testing.T) {
	// Cancel a queued job that has no cancel func (never assigned to worker)
	pool := NewPool(DefaultPoolConfig())
	pool.mu.Lock()
	pool.jobs["j-q"] = &Job{
		ID:     "j-q",
		Status: JobStatusQueued,
	}
	pool.mu.Unlock()

	if err := pool.CancelJob("j-q"); err != nil {
		t.Errorf("CancelJob(queued) error = %v", err)
	}

	got := pool.GetJob("j-q")
	if got.Status != JobStatusFailed {
		t.Errorf("Status = %q, want %q", got.Status, JobStatusFailed)
	}
}

// --- AddAgentWorker edge cases ---

func TestAddAgentWorker_MaxCapacity(t *testing.T) {
	cfg := DefaultPoolConfig()
	cfg.MaxWorkers = 1
	pool := NewPool(cfg)
	pool.AddWorker() // fills capacity

	_, err := pool.AddAgentWorker(context.Background(), "claude", false)
	if err == nil {
		t.Error("AddAgentWorker() at max capacity should return error")
	}
	if !strings.Contains(err.Error(), "max workers") {
		t.Errorf("error = %q, want to contain 'max workers'", err.Error())
	}
}

func TestAddAgentWorker_AutoDetectFails(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	// Empty agent name triggers auto-detect, which fails when no agents registered
	_, err := pool.AddAgentWorker(context.Background(), "", false)
	if err == nil {
		t.Error("AddAgentWorker() with auto-detect should fail when no agents available")
	}
	if !strings.Contains(err.Error(), "detect agent") {
		t.Errorf("error = %q, want to contain 'detect agent'", err.Error())
	}
}

// --- RemoveWorker with agent closes it ---

func TestRemoveWorker_WithAgent(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	mock := &mockAgent{connected: true}
	pool.mu.Lock()
	pool.workers["ag-test"] = &Worker{
		ID:    "ag-test",
		Agent: mock,
	}
	pool.mu.Unlock()

	if err := pool.RemoveWorker("ag-test"); err != nil {
		t.Fatalf("RemoveWorker() error = %v", err)
	}
	if !mock.closed {
		t.Error("RemoveWorker() should call Close() on agent")
	}

	if len(pool.ListWorkers()) != 0 {
		t.Error("worker should be removed from pool")
	}
}

// --- SimulatedJob cancellation path ---

func TestSimulatedJob_Cancellation(t *testing.T) {
	pool := newTestPool(t)
	pool.AddWorker()

	job, err := pool.Submit(JobTypePlan, "wt-1", "a task that will be cancelled mid-execution")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	stream := pool.Stream(job.ID)

	// Wait for at least one stream event (job has started)
	deadline := time.After(5 * time.Second)
	select {
	case <-stream:
		// Got first event, job is running
	case <-deadline:
		t.Fatal("timed out waiting for first event")
	}

	// Cancel the job
	if err := pool.CancelJob(job.ID); err != nil {
		t.Fatalf("CancelJob() error = %v", err)
	}

	// Drain remaining events
	for {
		select {
		case _, ok := <-stream:
			if !ok {
				goto drained
			}
		case <-time.After(3 * time.Second):
			goto drained
		}
	}
drained:

	got := pool.GetJob(job.ID)
	if got.Status != JobStatusFailed {
		t.Errorf("Status = %q, want %q", got.Status, JobStatusFailed)
	}
}

// --- executeWithAgent via mock ---

// testableAgent implements agent.Agent with controllable behavior.
type testableAgent struct {
	name        string
	connectedV  atomic.Bool
	closedV     atomic.Bool
	connectErr  error
	sendErr     error
	events      []agent.Event
	workDirCopy *testableAgent // returned by WithWorkDir
}

// newTestableAgent creates a testableAgent with the given options pre-connected.
func newTestableAgent(name string, opts ...func(*testableAgent)) *testableAgent {
	a := &testableAgent{name: name}
	a.connectedV.Store(true)
	for _, opt := range opts {
		opt(a)
	}

	return a
}

// withEvents sets the events sequence for a testableAgent.
func withEvents(events []agent.Event) func(*testableAgent) {
	return func(a *testableAgent) { a.events = events }
}

// withSendErr sets the sendErr for a testableAgent.
func withSendErr(err error) func(*testableAgent) {
	return func(a *testableAgent) { a.sendErr = err }
}

// withConnectErr sets the connectErr and disconnects a testableAgent.
func withConnectErr(err error) func(*testableAgent) {
	return func(a *testableAgent) {
		a.connectErr = err
		a.connectedV.Store(false)
	}
}

// withWorkDirCopy sets the workDirCopy agent.
func withWorkDirCopy(wdCopy *testableAgent) func(*testableAgent) {
	return func(a *testableAgent) { a.workDirCopy = wdCopy }
}

// withDisconnected marks the agent as not connected.
func withDisconnected() func(*testableAgent) {
	return func(a *testableAgent) { a.connectedV.Store(false) }
}

func (a *testableAgent) Name() string                            { return a.name }
func (a *testableAgent) Available() error                        { return nil }
func (a *testableAgent) Connected() bool                         { return a.connectedV.Load() }
func (a *testableAgent) HandlePermission(_ string, _ bool) error { return nil }
func (a *testableAgent) WithEnv(_, _ string) agent.Agent         { return a }
func (a *testableAgent) WithArgs(_ ...string) agent.Agent        { return a }
func (a *testableAgent) WithTimeout(_ time.Duration) agent.Agent { return a }
func (a *testableAgent) Interrupt() error                        { return nil }

func (a *testableAgent) Connect(_ context.Context) error {
	if a.connectErr != nil {
		return a.connectErr
	}
	a.connectedV.Store(true)

	return nil
}

func (a *testableAgent) Close() error {
	a.closedV.Store(true)

	return nil
}

func (a *testableAgent) WithWorkDir(_ string) agent.Agent {
	if a.workDirCopy != nil {
		return a.workDirCopy
	}

	return a
}

func (a *testableAgent) SendPrompt(_ context.Context, _ string) (<-chan agent.Event, error) {
	if a.sendErr != nil {
		return nil, a.sendErr
	}
	ch := make(chan agent.Event, len(a.events))
	for _, ev := range a.events {
		ch <- ev
	}
	close(ch)

	return ch, nil
}

func TestExecuteWithAgent_Success(t *testing.T) {
	pool := newTestPool(t)

	ag := newTestableAgent("test-agent", withEvents([]agent.Event{
		{Type: agent.EventStream, Content: "hello "},
		{Type: agent.EventStream, Content: "world"},
		{Type: agent.EventComplete, Content: "done"},
	}))

	pool.mu.Lock()
	pool.workers["ag-w"] = &Worker{
		ID:        "ag-w",
		Status:    StatusAvailable,
		AgentName: "test-agent",
		Agent:     ag,
	}
	pool.mu.Unlock()

	job, err := pool.Submit(JobTypePlan, "wt-1", "test prompt for agent")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	stream := pool.Stream(job.ID)

	// Drain events
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-stream:
			if !ok {
				goto complete
			}
		case <-deadline:
			t.Fatal("timed out waiting for agent job completion")
		}
	}
complete:

	got := pool.GetJob(job.ID)
	if got.Status != JobStatusDone {
		t.Errorf("Status = %q, want %q", got.Status, JobStatusDone)
	}
	if got.Result != "hello world" {
		t.Errorf("Result = %q, want 'hello world'", got.Result)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}

	// Worker should be released
	workers := pool.ListWorkers()
	for _, w := range workers {
		if w.ID == "ag-w" {
			if w.Status != StatusAvailable {
				t.Errorf("worker Status = %q, want %q", w.Status, StatusAvailable)
			}
		}
	}
}

func TestExecuteWithAgent_SendPromptError(t *testing.T) {
	pool := newTestPool(t)

	ag := newTestableAgent("err-agent", withSendErr(errors.New("prompt send failed")))

	pool.mu.Lock()
	pool.workers["ag-err"] = &Worker{
		ID:        "ag-err",
		Status:    StatusAvailable,
		AgentName: "err-agent",
		Agent:     ag,
	}
	pool.mu.Unlock()

	job, err := pool.Submit(JobTypeImplement, "wt-1", "test failing prompt")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

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

	got := pool.GetJob(job.ID)
	if got.Status != JobStatusFailed {
		t.Errorf("Status = %q, want %q", got.Status, JobStatusFailed)
	}
	if got.Error != "prompt send failed" {
		t.Errorf("Error = %q, want 'prompt send failed'", got.Error)
	}
}

func TestExecuteWithAgent_ErrorEvent(t *testing.T) {
	pool := newTestPool(t)

	ag := newTestableAgent("err-event-agent", withEvents([]agent.Event{
		{Type: agent.EventStream, Content: "starting..."},
		{Type: agent.EventError, Error: "something broke"},
	}))

	pool.mu.Lock()
	pool.workers["ag-ev-err"] = &Worker{
		ID:        "ag-ev-err",
		Status:    StatusAvailable,
		AgentName: "err-event-agent",
		Agent:     ag,
	}
	pool.mu.Unlock()

	job, err := pool.Submit(JobTypeReview, "wt-1", "test error event")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

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

	got := pool.GetJob(job.ID)
	if got.Status != JobStatusFailed {
		t.Errorf("Status = %q, want %q", got.Status, JobStatusFailed)
	}
	if got.Error != "something broke" {
		t.Errorf("Error = %q, want 'something broke'", got.Error)
	}
}

func TestExecuteWithAgent_ErrorEventFallbackContent(t *testing.T) {
	pool := newTestPool(t)

	ag := newTestableAgent("fallback-agent", withEvents([]agent.Event{
		{Type: agent.EventError, Content: "fallback error msg"},
	}))

	pool.mu.Lock()
	pool.workers["ag-fb"] = &Worker{
		ID:        "ag-fb",
		Status:    StatusAvailable,
		AgentName: "fallback-agent",
		Agent:     ag,
	}
	pool.mu.Unlock()

	job, err := pool.Submit(JobTypePlan, "wt-1", "test error fallback")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

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

	got := pool.GetJob(job.ID)
	if got.Error != "fallback error msg" {
		t.Errorf("Error = %q, want 'fallback error msg'", got.Error)
	}
}

func TestExecuteWithAgent_WithWorkDir(t *testing.T) {
	pool := newTestPool(t)

	workDirAgent := newTestableAgent("workdir-agent", withDisconnected(), withEvents([]agent.Event{
		{Type: agent.EventComplete, Content: "done in workdir"},
	}))

	ag := newTestableAgent("main-agent", withWorkDirCopy(workDirAgent))

	pool.mu.Lock()
	pool.workers["ag-wd"] = &Worker{
		ID:        "ag-wd",
		Status:    StatusAvailable,
		AgentName: "main-agent",
		Agent:     ag,
	}
	pool.mu.Unlock()

	opts := &JobOptions{WorkDir: "/tmp/workdir-test"}
	job, err := pool.SubmitWithOptions(JobTypeImplement, "wt-1", "implement in workdir", opts)
	if err != nil {
		t.Fatalf("SubmitWithOptions() error = %v", err)
	}

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

	got := pool.GetJob(job.ID)
	if got.Status != JobStatusDone {
		t.Errorf("Status = %q, want %q", got.Status, JobStatusDone)
	}
	// workDirAgent should have been connected and then closed
	if !workDirAgent.connectedV.Load() {
		t.Error("workDir agent should have been connected")
	}
	if !workDirAgent.closedV.Load() {
		t.Error("workDir agent should have been closed")
	}
}

func TestExecuteWithAgent_WorkDirConnectFails(t *testing.T) {
	pool := newTestPool(t)

	workDirAgent := newTestableAgent("fail-workdir", withConnectErr(errors.New("connection refused")))

	ag := newTestableAgent("main-agent", withWorkDirCopy(workDirAgent))

	pool.mu.Lock()
	pool.workers["ag-wf"] = &Worker{
		ID:        "ag-wf",
		Status:    StatusAvailable,
		AgentName: "main-agent",
		Agent:     ag,
	}
	pool.mu.Unlock()

	opts := &JobOptions{WorkDir: "/tmp/fail-connect"}
	job, err := pool.SubmitWithOptions(JobTypeImplement, "wt-1", "test connect fail", opts)
	if err != nil {
		t.Fatalf("SubmitWithOptions() error = %v", err)
	}

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

	got := pool.GetJob(job.ID)
	if got.Status != JobStatusFailed {
		t.Errorf("Status = %q, want %q", got.Status, JobStatusFailed)
	}
	if !strings.Contains(got.Error, "connection refused") {
		t.Errorf("Error = %q, want to contain 'connection refused'", got.Error)
	}
	// workDirAgent should have been closed on failure
	if !workDirAgent.closedV.Load() {
		t.Error("workDir agent should have been closed after connect failure")
	}
}

func TestExecuteWithAgent_ChannelClosedWithoutCompletion(t *testing.T) {
	pool := newTestPool(t)

	// Agent sends a stream event and a redundant assistant event (mirroring
	// how the Claude websocket adapter delivers the same text twice: as
	// streaming deltas and again as a final assistant message). The pool
	// must not double-count — streams are the canonical form and the
	// assistant fallback is ignored whenever any stream content arrived.
	ag := newTestableAgent("no-complete", withEvents([]agent.Event{
		{Type: agent.EventStream, Content: "partial output"},
		{Type: agent.EventAssistant, Content: "partial output"},
	}))

	pool.mu.Lock()
	pool.workers["ag-nc"] = &Worker{
		ID:        "ag-nc",
		Status:    StatusAvailable,
		AgentName: "no-complete",
		Agent:     ag,
	}
	pool.mu.Unlock()

	job, err := pool.Submit(JobTypePlan, "wt-1", "test channel close without explicit completion")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

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

	got := pool.GetJob(job.ID)
	if got.Status != JobStatusDone {
		t.Errorf("Status = %q, want %q (should complete when channel closes)", got.Status, JobStatusDone)
	}
	if got.Result != "partial output" {
		t.Errorf("Result = %q, want 'partial output'", got.Result)
	}
}

// --- Stats edge cases ---

func TestStats_DisconnectedAgentNotCounted(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	mock := &mockAgent{connected: false}
	pool.mu.Lock()
	pool.workers["disc"] = &Worker{
		ID:     "disc",
		Status: StatusAvailable, // Status says available, but agent is disconnected
		Agent:  mock,
	}
	pool.mu.Unlock()

	stats := pool.Stats()
	// Worker with disconnected agent should not be counted as available
	if stats.AvailableWorkers != 0 {
		t.Errorf("AvailableWorkers = %d, want 0 (agent disconnected)", stats.AvailableWorkers)
	}
	if stats.TotalWorkers != 1 {
		t.Errorf("TotalWorkers = %d, want 1", stats.TotalWorkers)
	}
}

// --- Stream for nonexistent job ---

func TestStream_NonexistentJob(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	ch := pool.Stream("no-such-job")
	if ch != nil {
		t.Errorf("Stream(nonexistent) = %v, want nil", ch)
	}
}

// --- ListQueuedJobs excludes completed/failed ---

func TestListQueuedJobs_ExcludesTerminal(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	now := time.Now()
	pool.mu.Lock()
	pool.jobs["j-q"] = &Job{ID: "j-q", Status: JobStatusQueued, CreatedAt: now}
	pool.jobs["j-ip"] = &Job{ID: "j-ip", Status: JobStatusInProgress, CreatedAt: now.Add(time.Second)}
	pool.jobs["j-d"] = &Job{ID: "j-d", Status: JobStatusDone, CreatedAt: now.Add(2 * time.Second)}
	pool.jobs["j-f"] = &Job{ID: "j-f", Status: JobStatusFailed, CreatedAt: now.Add(3 * time.Second)}
	pool.mu.Unlock()

	queued := pool.ListQueuedJobs()
	if len(queued) != 2 {
		t.Errorf("ListQueuedJobs() len = %d, want 2", len(queued))
	}
	for _, j := range queued {
		if j.Status != JobStatusQueued && j.Status != JobStatusInProgress {
			t.Errorf("unexpected status %q in queued jobs", j.Status)
		}
	}
}

// --- emitEvent to nonexistent stream ---

func TestEmitEvent_NoStream(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	// Should not panic when emitting to a job with no stream
	pool.emitEvent("nonexistent", Event{
		Type:    "stream",
		JobID:   "nonexistent",
		Content: "test",
	})
}

// --- Multiple workers, jobs assigned round-robin ---

func TestMultipleSimulatedWorkers_AllGetJobs(t *testing.T) {
	pool := newTestPool(t)

	// Add 3 workers
	for range 3 {
		pool.AddWorker()
	}

	// Submit 3 jobs
	jobs := make([]*Job, 3)
	for i := range 3 {
		var err error
		jobs[i], err = pool.Submit(JobTypePlan, fmt.Sprintf("wt-%d", i), fmt.Sprintf("task %d", i))
		if err != nil {
			t.Fatalf("Submit(%d) error = %v", i, err)
		}
	}

	// Wait for all to complete
	deadline := time.After(10 * time.Second)
	for {
		stats := pool.Stats()
		if stats.CompletedJobs+stats.FailedJobs >= 3 {
			break
		}
		select {
		case <-deadline:
			stats := pool.Stats()
			t.Fatalf("timed out: completed=%d failed=%d", stats.CompletedJobs, stats.FailedJobs)
		case <-time.After(50 * time.Millisecond):
		}
	}

	stats := pool.Stats()
	if stats.CompletedJobs != 3 {
		t.Errorf("CompletedJobs = %d, want 3", stats.CompletedJobs)
	}
}

// --- NewPool with nil Agents in config ---

func TestNewPool_NilAgents(t *testing.T) {
	cfg := PoolConfig{
		MaxWorkers: 3,
		BasePort:   9000,
		Agents:     nil,
	}
	pool := NewPool(cfg)
	if pool.agents == nil {
		t.Error("Pool should create default agents registry when nil")
	}
}

// --- AddDefaultWorker bypasses max workers ---

func TestAddDefaultWorker_BypassesMaxWorkers(t *testing.T) {
	cfg := PoolConfig{MaxWorkers: 1, BasePort: 8765}
	pool := NewPool(cfg)
	pool.AddWorker() // fills capacity

	// AddDefaultWorker does not check capacity
	w := pool.AddDefaultWorker("claude")
	if w == nil {
		t.Error("AddDefaultWorker() should succeed even at max capacity")
	}

	stats := pool.Stats()
	if stats.TotalWorkers != 2 {
		t.Errorf("TotalWorkers = %d, want 2", stats.TotalWorkers)
	}
}

// --- ListJobs sorting ---

func TestListJobs_SortedByCreation(t *testing.T) {
	pool := NewPool(DefaultPoolConfig())
	now := time.Now()
	pool.mu.Lock()
	pool.jobs["j3"] = &Job{ID: "j3", Status: JobStatusQueued, CreatedAt: now.Add(2 * time.Second)}
	pool.jobs["j1"] = &Job{ID: "j1", Status: JobStatusQueued, CreatedAt: now}
	pool.jobs["j2"] = &Job{ID: "j2", Status: JobStatusQueued, CreatedAt: now.Add(time.Second)}
	pool.mu.Unlock()

	jobs := pool.ListJobs()
	if len(jobs) != 3 {
		t.Fatalf("ListJobs() len = %d, want 3", len(jobs))
	}
	if jobs[0].ID != "j1" || jobs[1].ID != "j2" || jobs[2].ID != "j3" {
		t.Errorf("jobs not sorted by creation: %s, %s, %s", jobs[0].ID, jobs[1].ID, jobs[2].ID)
	}
}
