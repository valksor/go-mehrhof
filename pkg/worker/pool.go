package worker

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	basePort      int      // Starting port for WebSocket workers
	allowedAgents []string // If non-empty, only these agents can be used

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
	AllowedAgents      []string // If non-empty, only these agent names can be used
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
		allowedAgents:      cfg.AllowedAgents,
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
		if opts.Agent != "" {
			if job.Metadata == nil {
				job.Metadata = make(map[string]any)
			}
			job.Metadata["agent_override"] = opts.Agent
		}
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

// SubmitCached creates a job that is immediately completed with a cached result.
// The job goes through the normal stream/event pipeline so downstream watchers
// (checkpoints, metrics, strategy evaluation) work unchanged.
func (p *Pool) SubmitCached(jobType JobType, worktreeID, prompt, cachedResult string, opts *JobOptions) (*Job, error) {
	now := time.Now()
	job := &Job{
		ID:          uuid.New().String()[:8],
		Type:        jobType,
		WorktreeID:  worktreeID,
		Prompt:      prompt,
		Status:      JobStatusDone,
		CreatedAt:   now,
		StartedAt:   &now,
		CompletedAt: &now,
		Result:      cachedResult,
	}

	if opts != nil {
		job.WorkDir = opts.WorkDir
		job.Environment = opts.Environment
		job.Metadata = opts.Metadata
		if opts.Agent != "" {
			if job.Metadata == nil {
				job.Metadata = make(map[string]any)
			}
			job.Metadata["agent_override"] = opts.Agent
		}
	}

	if job.Metadata == nil {
		job.Metadata = make(map[string]any)
	}
	job.Metadata["cached"] = true

	p.mu.Lock()
	p.jobs[job.ID] = job
	p.mu.Unlock()

	metrics.Global().RecordJobSubmitted()

	// Create stream and immediately send completion events.
	// The goroutine closes the channel but does NOT call closeStream (which
	// removes it from the map) — that would race with callers of Stream().
	// Instead, the stream is cleaned up lazily by RemoveJob or pool shutdown.
	stream := make(chan Event, 10)
	p.streamsMu.Lock()
	p.streams[job.ID] = stream
	p.streamsMu.Unlock()

	stream <- Event{Type: "stream", JobID: job.ID, Content: cachedResult, Timestamp: now}
	stream <- Event{Type: "complete", JobID: job.ID, Timestamp: now}
	stream <- Event{Type: "job_completed", JobID: job.ID, Content: "Job completed", Timestamp: now}
	close(stream)
	metrics.Global().RecordJobCompleted()

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
