package worker

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/valksor/kvelmo/pkg/agent"
	"github.com/valksor/kvelmo/pkg/agent/recorder"
	"github.com/valksor/kvelmo/pkg/metrics"
)

// Pool manages a shared pool of workers and a global job queue.
// Per flow_v2.md: "Max 5-6 workers total across all projects".
type Pool struct {
	mu sync.RWMutex

	// Workers with their agents
	workers    map[string]*Worker
	maxWorkers int

	// Agent registry for creating new agent instances
	agents *agent.Registry

	// Job queue and tracking
	jobs       map[string]*Job
	queue      chan *Job
	streams    map[string]chan Event
	streamsMu  sync.RWMutex
	jobCancels map[string]context.CancelFunc

	// Lifecycle
	ctx    context.Context //nolint:containedctx // Pool owns its lifecycle context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Configuration
	basePort int // Starting port for WebSocket workers

	// Recording
	recordingEnabled   bool
	recordingDir       string
	recordingSanitizer *recorder.Sanitizer
}

// PoolConfig configures the worker pool.
type PoolConfig struct {
	MaxWorkers         int
	BasePort           int
	Agents             *agent.Registry
	RecordingEnabled   bool
	RecordingDir       string
	RecordingSanitizer *recorder.Sanitizer
}

// DefaultPoolConfig returns sensible defaults.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxWorkers: 5, // Per flow_v2.md: "Max 5-6 workers total"
		BasePort:   8765,
		Agents:     agent.NewRegistry(),
	}
}

// NewPool creates a new worker pool.
func NewPool(cfg PoolConfig) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	agents := cfg.Agents
	if agents == nil {
		agents = agent.NewRegistry()
	}

	return &Pool{
		workers:            make(map[string]*Worker),
		agents:             agents,
		jobs:               make(map[string]*Job),
		queue:              make(chan *Job, 100),
		streams:            make(map[string]chan Event),
		jobCancels:         make(map[string]context.CancelFunc),
		maxWorkers:         cfg.MaxWorkers,
		basePort:           cfg.BasePort,
		recordingEnabled:   cfg.RecordingEnabled,
		recordingDir:       cfg.RecordingDir,
		recordingSanitizer: cfg.RecordingSanitizer,
		ctx:                ctx,
		cancel:             cancel,
	}
}

// Start starts the pool dispatcher.
func (p *Pool) Start() error {
	p.wg.Add(1)
	go p.dispatcher()

	return nil
}

// Stop stops all workers and the dispatcher.
func (p *Pool) Stop() error {
	p.cancel()

	// Close all agent connections
	p.mu.Lock()
	for _, w := range p.workers {
		if w.Agent != nil {
			_ = w.Agent.Close()
		}
	}
	p.mu.Unlock()

	p.wg.Wait()

	return nil
}

// dispatcher assigns jobs to available workers.
func (p *Pool) dispatcher() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("worker dispatcher panic", "panic", r)
		}
		p.wg.Done()
	}()

	for {
		select {
		case <-p.ctx.Done():
			return
		case job := <-p.queue:
			p.assignJob(job)
		}
	}
}

// assignJob finds an available worker and assigns the job.
func (p *Pool) assignJob(job *Job) {
	p.mu.Lock()

	// Find an available worker with a connected agent
	for _, w := range p.workers {
		if w.Status == StatusAvailable {
			if w.Agent != nil && w.Agent.Connected() {
				// Agent-based worker
				jobCtx, jobCancel := context.WithCancel(p.ctx)
				w.Status = StatusWorking
				w.CurrentJob = job.ID
				job.Status = JobStatusInProgress
				job.WorkerID = w.ID
				now := time.Now()
				job.StartedAt = &now
				p.jobCancels[job.ID] = jobCancel
				p.mu.Unlock()

				p.emitEvent(job.ID, Event{
					Type:    "job_started",
					JobID:   job.ID,
					Content: fmt.Sprintf("Job assigned to worker %s (%s)", w.ID, w.AgentName),
				})

				p.wg.Add(1)
				go p.executeWithAgent(jobCtx, job, w)

				return
			} else if w.Agent == nil {
				// Simulated worker (no agent)
				jobCtx, jobCancel := context.WithCancel(p.ctx)
				w.Status = StatusWorking
				w.CurrentJob = job.ID
				job.Status = JobStatusInProgress
				job.WorkerID = w.ID
				now := time.Now()
				job.StartedAt = &now
				p.jobCancels[job.ID] = jobCancel
				p.mu.Unlock()

				p.emitEvent(job.ID, Event{
					Type:    "job_started",
					JobID:   job.ID,
					Content: "Job assigned to worker " + w.ID,
				})

				p.wg.Add(1)
				go p.executeSimulatedJob(jobCtx, job, w)

				return
			}
		}
	}

	p.mu.Unlock()

	// No worker available, re-queue after delay
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("job re-queue panic", "panic", r)
			}
		}()
		select {
		case <-time.After(500 * time.Millisecond):
			select {
			case p.queue <- job:
			case <-p.ctx.Done():
			}
		case <-p.ctx.Done():
		}
	}()
}

// executeWithAgent executes a job using an agent.
func (p *Pool) executeWithAgent(ctx context.Context, job *Job, w *Worker) {
	defer p.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("executeWithAgent panic", "panic", r, "job_id", job.ID, "worker_id", w.ID)
			// Ensure worker is marked available on panic
			p.mu.Lock()
			w.Status = StatusAvailable
			w.CurrentJob = ""
			job.Status = JobStatusFailed
			now := time.Now()
			job.CompletedAt = &now
			job.Error = fmt.Sprintf("panic: %v", r)
			p.mu.Unlock()
			metrics.Global().RecordJobFailed()
			p.closeStream(job.ID)
		}
	}()

	// Debug: log job details
	slog.Info("executeWithAgent starting", "job_id", job.ID, "type", job.Type, "work_dir", job.WorkDir, "prompt_len", len(job.Prompt))

	// Use job-specific agent if WorkDir is set
	ag := w.Agent
	var jobAgent agent.Agent
	if job.WorkDir != "" {
		// Create a new agent configured for this job's working directory
		jobAgent = w.Agent.WithWorkDir(job.WorkDir)
		if err := jobAgent.Connect(ctx); err != nil {
			slog.Error("failed to connect job-specific agent", "error", err, "work_dir", job.WorkDir)
			// Clean up agent resources allocated by WithWorkDir
			_ = jobAgent.Close()
			p.emitEvent(job.ID, Event{
				Type:    "job_failed",
				JobID:   job.ID,
				Content: fmt.Sprintf("agent connect failed: %v", err),
			})
			p.mu.Lock()
			w.Status = StatusAvailable
			w.CurrentJob = ""
			job.Status = JobStatusFailed
			now := time.Now()
			job.CompletedAt = &now
			job.Error = err.Error()
			p.mu.Unlock()
			metrics.Global().RecordJobFailed()
			p.closeStream(job.ID)

			return
		}
		ag = jobAgent
		defer func() { _ = jobAgent.Close() }()
	}

	// Set up recording if enabled
	var rec *recorder.Recorder
	if p.recordingEnabled {
		recCfg := recorder.Config{
			Dir:       p.recordingDir,
			JobID:     job.ID,
			Agent:     ag.Name(),
			WorkDir:   job.WorkDir,
			Sanitizer: p.recordingSanitizer,
		}
		if r, recErr := recorder.New(recCfg); recErr == nil {
			rec = r
			defer func() { _ = rec.Close() }()
			_ = rec.RecordInbound(job.Prompt)
		} else {
			slog.Warn("failed to create recorder", "error", recErr, "job_id", job.ID)
		}
	}

	// Send prompt to agent
	eventCh, err := ag.SendPrompt(ctx, job.Prompt)
	if err != nil {
		p.emitEvent(job.ID, Event{
			Type:    "job_failed",
			JobID:   job.ID,
			Content: err.Error(),
		})

		// Mark worker available again
		p.mu.Lock()
		w.Status = StatusAvailable
		w.CurrentJob = ""
		job.Status = JobStatusFailed
		now := time.Now()
		job.CompletedAt = &now
		job.Error = err.Error()
		p.mu.Unlock()

		metrics.Global().RecordJobFailed()
		p.closeStream(job.ID)

		return
	}

	// Forward agent events to job stream
	var result strings.Builder
	for agentEvent := range eventCh {
		// Record the event if recording is active
		if rec != nil {
			_ = rec.RecordOutbound(string(agentEvent.Type), agentEvent)
		}

		// Convert agent.Event to worker.Event
		workerEvent := Event{
			Type:      string(agentEvent.Type),
			JobID:     job.ID,
			Content:   agentEvent.Content,
			Data:      agentEvent.Data,
			Timestamp: agentEvent.Timestamp,
		}
		p.emitEvent(job.ID, workerEvent)

		// Accumulate content for result
		if agentEvent.Type == agent.EventStream || agentEvent.Type == agent.EventAssistant {
			result.WriteString(agentEvent.Content)
		}

		// Handle completion
		if agentEvent.Type == agent.EventComplete {
			p.mu.Lock()
			job.Status = JobStatusDone
			now := time.Now()
			job.CompletedAt = &now
			job.Result = result.String()
			w.Status = StatusAvailable
			w.CurrentJob = ""
			p.mu.Unlock()

			metrics.Global().RecordJobCompleted()
			p.emitEvent(job.ID, Event{
				Type:    "job_completed",
				JobID:   job.ID,
				Content: "Job completed",
			})
			p.closeStream(job.ID)

			return
		}

		// Handle error
		if agentEvent.Type == agent.EventError {
			p.mu.Lock()
			job.Status = JobStatusFailed
			now := time.Now()
			job.CompletedAt = &now
			job.Error = agentEvent.Error
			if job.Error == "" {
				job.Error = agentEvent.Content
			}
			w.Status = StatusAvailable
			w.CurrentJob = ""
			p.mu.Unlock()

			metrics.Global().RecordJobFailed()
			p.emitEvent(job.ID, Event{
				Type:    "job_failed",
				JobID:   job.ID,
				Content: job.Error,
			})
			p.closeStream(job.ID)

			return
		}
	}

	// Channel closed without explicit completion - mark as complete
	p.mu.Lock()
	if job.Status == JobStatusInProgress {
		job.Status = JobStatusDone
		now := time.Now()
		job.CompletedAt = &now
		job.Result = result.String()
	}
	w.Status = StatusAvailable
	w.CurrentJob = ""
	p.mu.Unlock()

	p.emitEvent(job.ID, Event{
		Type:    "job_completed",
		JobID:   job.ID,
		Content: "Job completed",
	})
	p.closeStream(job.ID)
}

// executeSimulatedJob simulates job execution for testing.
func (p *Pool) executeSimulatedJob(ctx context.Context, job *Job, worker *Worker) {
	defer p.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("executeSimulatedJob panic", "panic", r, "job_id", job.ID, "worker_id", worker.ID)
			p.mu.Lock()
			worker.Status = StatusAvailable
			worker.CurrentJob = ""
			job.Status = JobStatusFailed
			now := time.Now()
			job.CompletedAt = &now
			job.Error = fmt.Sprintf("panic: %v", r)
			p.mu.Unlock()
			metrics.Global().RecordJobFailed()
			p.closeStream(job.ID)
		}
	}()

	// Simulate streaming output
	messages := []string{
		fmt.Sprintf("Starting %s job...", job.Type),
		"Analyzing task requirements...",
		"Processing...",
		"Generating output...",
	}

	for _, msg := range messages {
		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
		p.emitEvent(job.ID, Event{
			Type:    "stream",
			JobID:   job.ID,
			Content: msg,
		})
	}

	// Mark complete
	p.mu.Lock()
	job.Status = JobStatusDone
	now := time.Now()
	job.CompletedAt = &now
	job.Result = fmt.Sprintf("Simulated %s result for: %s", job.Type, job.Prompt[:minInt(50, len(job.Prompt))])
	worker.Status = StatusAvailable
	worker.CurrentJob = ""
	p.mu.Unlock()

	metrics.Global().RecordJobCompleted()
	p.emitEvent(job.ID, Event{
		Type:    "job_completed",
		JobID:   job.ID,
		Content: "Job completed",
	})

	p.closeStream(job.ID)
}

func (p *Pool) emitEvent(jobID string, event Event) {
	event.Timestamp = time.Now()

	p.streamsMu.RLock()
	ch, ok := p.streams[jobID]
	p.streamsMu.RUnlock()

	if ok {
		select {
		case ch <- event:
		default:
			metrics.Global().RecordEventDropped()
			slog.Warn("worker event channel full, dropping event", "job_id", jobID, "type", event.Type)
		}
	}
}

func (p *Pool) closeStream(jobID string) {
	p.streamsMu.Lock()
	if ch, ok := p.streams[jobID]; ok {
		close(ch)
		delete(p.streams, jobID)
	}
	p.streamsMu.Unlock()

	// Clean up per-job cancel func
	p.mu.Lock()
	delete(p.jobCancels, jobID)
	p.mu.Unlock()
}

// Submit adds a job to the queue.
func (p *Pool) Submit(jobType JobType, worktreeID, prompt string) (*Job, error) {
	return p.SubmitWithOptions(jobType, worktreeID, prompt, nil)
}

// SubmitWithOptions adds a job to the queue with additional execution context.
// This enables multi-project support where jobs carry full context.
func (p *Pool) SubmitWithOptions(jobType JobType, worktreeID, prompt string, opts *JobOptions) (*Job, error) {
	job := &Job{
		ID:         uuid.New().String()[:8],
		Type:       jobType,
		WorktreeID: worktreeID,
		Prompt:     prompt,
		Status:     JobStatusQueued,
		CreatedAt:  time.Now(),
	}

	// Apply options if provided
	if opts != nil {
		job.WorkDir = opts.WorkDir
		job.Environment = opts.Environment
		job.Metadata = opts.Metadata
	}

	p.mu.Lock()
	p.jobs[job.ID] = job
	p.mu.Unlock()

	// Create stream channel
	p.streamsMu.Lock()
	p.streams[job.ID] = make(chan Event, 100)
	p.streamsMu.Unlock()

	select {
	case p.queue <- job:
		metrics.Global().RecordJobSubmitted()
	case <-p.ctx.Done():
		return nil, errors.New("pool stopped")
	}

	return job, nil
}

// Stream returns the event stream for a job.
func (p *Pool) Stream(jobID string) <-chan Event {
	p.streamsMu.RLock()
	defer p.streamsMu.RUnlock()

	return p.streams[jobID]
}

// GetJob returns a job by ID.
func (p *Pool) GetJob(jobID string) *Job {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.jobs[jobID]
}

// CancelJob cancels a running or queued job by ID.
// If the job is already completed or failed, this is a no-op.
// Returns an error if the job is not found.
func (p *Pool) CancelJob(jobID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	job, ok := p.jobs[jobID]
	if !ok {
		return fmt.Errorf("cancel job: job %s not found", jobID)
	}

	// Already terminal — no-op
	if job.Status == JobStatusDone || job.Status == JobStatusFailed {
		return nil
	}

	// Cancel the job context if one exists
	if cancel, exists := p.jobCancels[jobID]; exists {
		cancel()
	}

	// Mark as failed
	job.Status = JobStatusFailed
	now := time.Now()
	job.CompletedAt = &now
	job.Error = "cancelled"

	// Release the worker
	if job.WorkerID != "" {
		if w, wok := p.workers[job.WorkerID]; wok {
			w.Status = StatusAvailable
			w.CurrentJob = ""
		}
	}

	metrics.Global().RecordJobFailed()

	return nil
}

// InterruptJob sends an interrupt signal to the agent running a job.
// Unlike CancelJob, this does not mark the job as failed - it just signals
// the agent to abort its current turn. The job context remains valid.
// Returns nil if the job is not found or not currently running.
func (p *Pool) InterruptJob(jobID string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	job, ok := p.jobs[jobID]
	if !ok {
		return nil // Job not found - nothing to interrupt
	}

	if job.Status != JobStatusInProgress {
		return nil // Not running - nothing to interrupt
	}

	// Find the worker running this job
	if job.WorkerID == "" {
		return nil
	}

	w, ok := p.workers[job.WorkerID]
	if !ok || w.Agent == nil {
		return nil
	}

	// Send interrupt to the agent
	return w.Agent.Interrupt()
}

// AddWorker adds a simulated worker to the pool (for testing).
func (p *Pool) AddWorker() *Worker {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.workers) >= p.maxWorkers {
		return nil
	}

	w := &Worker{
		ID:        "w-" + uuid.New().String()[:6],
		Status:    StatusAvailable,
		StartedAt: time.Now(),
	}
	p.workers[w.ID] = w

	return w
}

// AddWorkerWithAgent adds a worker with specified agent name (without connecting).
func (p *Pool) AddWorkerWithAgent(agentName string) *Worker {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.workers) >= p.maxWorkers {
		return nil
	}

	w := &Worker{
		ID:        "w-" + uuid.New().String()[:6],
		AgentName: agentName,
		Status:    StatusAvailable,
		StartedAt: time.Now(),
	}
	p.workers[w.ID] = w

	return w
}

// AddDefaultWorker adds the default worker (cannot be removed).
func (p *Pool) AddDefaultWorker(agentName string) *Worker {
	p.mu.Lock()
	defer p.mu.Unlock()

	w := &Worker{
		ID:        "default",
		AgentName: agentName,
		Status:    StatusAvailable,
		StartedAt: time.Now(),
		IsDefault: true,
	}
	p.workers[w.ID] = w

	return w
}

// AddAgentWorker creates a worker backed by an agent from the registry.
// If agentName is empty, auto-detects the first available agent.
// If isDefault is true, the worker cannot be removed.
func (p *Pool) AddAgentWorker(ctx context.Context, agentName string, isDefault bool) (*Worker, error) {
	p.mu.Lock()
	if len(p.workers) >= p.maxWorkers {
		p.mu.Unlock()

		return nil, fmt.Errorf("max workers (%d) reached", p.maxWorkers)
	}
	p.mu.Unlock()

	// Get agent from registry
	var ag agent.Agent
	var err error
	if agentName != "" {
		ag, err = p.agents.Get(agentName)
		if err != nil {
			return nil, fmt.Errorf("get agent %q: %w", agentName, err)
		}
	} else {
		ag, err = p.agents.Detect()
		if err != nil {
			return nil, fmt.Errorf("detect agent: %w", err)
		}
	}

	// Connect agent
	if err := ag.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connect agent %q: %w", ag.Name(), err)
	}

	id := "ag-" + uuid.New().String()[:6]
	if isDefault {
		id = "default"
	}
	w := &Worker{
		ID:        id,
		Status:    StatusAvailable,
		StartedAt: time.Now(),
		AgentName: ag.Name(),
		Agent:     ag,
		IsDefault: isDefault,
	}

	p.mu.Lock()
	p.workers[w.ID] = w
	p.mu.Unlock()

	return w, nil
}

// Agents returns the agent registry.
func (p *Pool) Agents() *agent.Registry {
	return p.agents
}

// RemoveWorker removes a worker from the pool.
// Returns error if worker is default or not found.
func (p *Pool) RemoveWorker(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	w, ok := p.workers[id]
	if !ok {
		return fmt.Errorf("worker %s not found", id)
	}
	if w.IsDefault {
		return errors.New("cannot remove default worker")
	}
	if w.Agent != nil {
		_ = w.Agent.Close()
	}
	delete(p.workers, id)

	return nil
}

// ListWorkers returns all workers.
func (p *Pool) ListWorkers() []*Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	workers := make([]*Worker, 0, len(p.workers))

	for _, w := range p.workers {
		// Update status from agent if connected
		if w.Agent != nil {
			if w.Agent.Connected() && w.Status == StatusDisconnected {
				w.Status = StatusAvailable
			} else if !w.Agent.Connected() && w.Status != StatusDisconnected {
				w.Status = StatusDisconnected
			}
		}
		workers = append(workers, w)
	}

	// Sort by ID for consistent ordering
	slices.SortFunc(workers, func(a, b *Worker) int {
		return cmp.Compare(a.ID, b.ID)
	})

	return workers
}

// ListJobs returns all jobs.
func (p *Pool) ListJobs() []*Job {
	p.mu.RLock()
	defer p.mu.RUnlock()

	jobs := make([]*Job, 0, len(p.jobs))
	for _, j := range p.jobs {
		jobs = append(jobs, j)
	}

	// Sort by creation time
	slices.SortFunc(jobs, func(a, b *Job) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})

	return jobs
}

// RemoveJob removes a completed or failed job from the pool.
// Active jobs cannot be removed — cancel them first.
func (p *Pool) RemoveJob(jobID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	job, ok := p.jobs[jobID]
	if !ok {
		return
	}

	// Only remove terminal jobs.
	if job.Status != JobStatusDone && job.Status != JobStatusFailed {
		return
	}

	delete(p.jobs, jobID)
	delete(p.jobCancels, jobID)
}

// ListQueuedJobs returns jobs that are queued or in progress.
func (p *Pool) ListQueuedJobs() []*Job {
	p.mu.RLock()
	defer p.mu.RUnlock()

	jobs := make([]*Job, 0)
	for _, j := range p.jobs {
		if j.Status == JobStatusQueued || j.Status == JobStatusInProgress {
			jobs = append(jobs, j)
		}
	}

	slices.SortFunc(jobs, func(a, b *Job) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})

	return jobs
}

// Stats returns pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := PoolStats{
		TotalWorkers: len(p.workers),
	}

	for _, w := range p.workers {
		// Update status from agent connection state
		status := w.Status
		if w.Agent != nil && !w.Agent.Connected() {
			status = StatusDisconnected
		}

		switch status { //nolint:exhaustive // Only counting Available and Working
		case StatusAvailable:
			stats.AvailableWorkers++
		case StatusWorking:
			stats.WorkingWorkers++
		}
	}

	for _, j := range p.jobs {
		switch j.Status {
		case JobStatusQueued:
			stats.QueuedJobs++
		case JobStatusInProgress:
			stats.InProgressJobs++
		case JobStatusDone:
			stats.CompletedJobs++
		case JobStatusFailed:
			stats.FailedJobs++
		}
	}

	return stats
}

// PoolStats contains pool statistics.
type PoolStats struct {
	TotalWorkers     int `json:"total_workers"`
	AvailableWorkers int `json:"available_workers"`
	WorkingWorkers   int `json:"working_workers"`
	QueuedJobs       int `json:"queued_jobs"`
	InProgressJobs   int `json:"in_progress_jobs"`
	CompletedJobs    int `json:"completed_jobs"`
	FailedJobs       int `json:"failed_jobs"`
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
