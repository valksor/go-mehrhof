package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/recorder"
	"github.com/valksor/kvelmo/metrics"
)

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
	// Dry-run: bypass worker assignment, complete immediately
	if job.Type == JobTypeDryRun {
		p.executeDryRunJob(job)

		return
	}

	p.mu.Lock()

	// Determine preferred agent name from job metadata.
	var preferredAgent string
	if job.Metadata != nil {
		if v, ok := job.Metadata["agent_override"].(string); ok {
			preferredAgent = v
		}
	}

	// Find an available worker with a connected agent.
	// When a preferred agent is specified, try matching workers first.
	var fallback *Worker
	for _, w := range p.workers {
		if w.Status == StatusAvailable {
			if w.Agent != nil && w.Agent.Connected() {
				if preferredAgent != "" && w.AgentName != preferredAgent {
					if fallback == nil {
						fallback = w
					}

					continue
				}
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

	// No exact agent match found — use fallback worker if available.
	if fallback != nil {
		w := fallback
		if w.Agent != nil && w.Agent.Connected() {
			jobCtx, jobCancel := context.WithCancel(p.ctx)
			w.Status = StatusWorking
			w.CurrentJob = job.ID
			job.Status = JobStatusInProgress
			job.WorkerID = w.ID
			now := time.Now()
			job.StartedAt = &now
			p.jobCancels[job.ID] = jobCancel
			p.mu.Unlock()

			slog.Info("preferred agent not available, using fallback", "preferred", preferredAgent, "fallback", w.AgentName)
			p.emitEvent(job.ID, Event{
				Type:    "job_started",
				JobID:   job.ID,
				Content: fmt.Sprintf("Job assigned to worker %s (%s, preferred %s unavailable)", w.ID, w.AgentName, preferredAgent),
			})

			p.wg.Add(1)
			go p.executeWithAgent(jobCtx, job, w)

			return
		}
	}

	p.mu.Unlock()

	// No worker available, re-queue after delay
	p.wg.Go(func() {
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
	})
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
			// Store recording path in job metadata for phase metrics.
			p.mu.Lock()
			if job.Metadata == nil {
				job.Metadata = make(map[string]any)
			}
			job.Metadata["recording_path"] = rec.Path()
			p.mu.Unlock()
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

	// Forward agent events to job stream.
	//
	// Result accumulation strategy: prefer streaming deltas, since they are
	// the canonical token-by-token form of the agent's output. Only fall
	// back to EventAssistant when no stream content has arrived — this
	// handles adapters that deliver the full message in one event without
	// streaming. The two forms must never both be accumulated for the same
	// turn: Claude's websocket adapter, for example, emits both a series of
	// stream deltas AND a final EventAssistant carrying the same text, so
	// writing both would double every character and put heavy allocation
	// pressure on strings.Builder for long responses.
	var result strings.Builder
	var assistantFallback string
	var streamReceived bool
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

		switch agentEvent.Type {
		case agent.EventStream:
			if agentEvent.Content != "" {
				streamReceived = true
				result.WriteString(agentEvent.Content)
			}
		case agent.EventAssistant:
			if agentEvent.Content != "" {
				assistantFallback = agentEvent.Content
			}
		case agent.EventToolUse, agent.EventToolResult, agent.EventPermission,
			agent.EventComplete, agent.EventError, agent.EventInit,
			agent.EventKeepAlive, agent.EventSubagent, agent.EventProgress,
			agent.EventToolProgress, agent.EventInterrupted,
			agent.EventPromptSuggestion:
			// Other event types are forwarded to the job stream above but
			// do not contribute to the accumulated result text.
		}

		// Handle completion
		if agentEvent.Type == agent.EventComplete {
			if !streamReceived && assistantFallback != "" {
				result.WriteString(assistantFallback)
			}
			p.mu.Lock()
			job.Status = JobStatusDone
			now := time.Now()
			job.CompletedAt = &now
			job.Result = result.String()
			// Capture token usage from agent event data.
			if agentEvent.Data != nil {
				if job.Metadata == nil {
					job.Metadata = make(map[string]any)
				}
				if v, ok := agentEvent.Data["input_tokens"]; ok {
					job.Metadata["input_tokens"] = v
				}
				if v, ok := agentEvent.Data["output_tokens"]; ok {
					job.Metadata["output_tokens"] = v
				}
				if v, ok := agentEvent.Data["total_tokens"]; ok {
					job.Metadata["total_tokens"] = v
				}
			}
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
	if !streamReceived && assistantFallback != "" {
		result.WriteString(assistantFallback)
	}
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
	job.Result = fmt.Sprintf("Simulated %s result for: %s", job.Type, job.Prompt[:min(50, len(job.Prompt))])
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

// executeDryRunJob completes a job immediately without spawning an agent.
func (p *Pool) executeDryRunJob(job *Job) {
	p.mu.Lock()
	job.Status = JobStatusInProgress
	now := time.Now()
	job.StartedAt = &now
	p.mu.Unlock()

	result := fmt.Sprintf("[dry-run] Phase would execute. Prompt: %d chars", len(job.Prompt))

	p.emitEvent(job.ID, Event{
		Type:    "job_output",
		JobID:   job.ID,
		Content: result,
	})

	p.mu.Lock()
	job.Status = JobStatusDone
	end := time.Now()
	job.CompletedAt = &end
	job.Result = result
	p.mu.Unlock()

	p.emitEvent(job.ID, Event{
		Type:    "job_completed",
		JobID:   job.ID,
		Content: result,
	})

	metrics.Global().RecordJobCompleted()
	p.closeStream(job.ID)
}

// emitEvent sends an event to the job's stream channel.
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

// closeStream closes and removes the event stream for a job.
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
