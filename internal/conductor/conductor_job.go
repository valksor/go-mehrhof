package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/valksor/kvelmo/internal/eventlog"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/graph"
	"github.com/valksor/kvelmo/internal/memory"
	"github.com/valksor/kvelmo/internal/security"
	"github.com/valksor/kvelmo/metrics"
)

// recordPhaseMetrics captures execution metrics for a completed phase.
// Must be called with c.mu held.
func (c *Conductor) recordPhaseMetrics(completionEvent Event, jobID string) {
	phase := phaseFromEvent(completionEvent)
	if phase == "" || c.workUnit == nil {
		return
	}

	if c.workUnit.PhaseMetrics == nil {
		c.workUnit.PhaseMetrics = make(map[string]*PhaseMetrics)
	}

	pm := &PhaseMetrics{}

	// Compute duration from phaseStartedAt or job timing.
	if !c.phaseStartedAt.IsZero() {
		pm.Duration = time.Since(c.phaseStartedAt)
	}

	// Get agent info from the job if available.
	if c.pool != nil && jobID != "" {
		if job := c.pool.GetJob(jobID); job != nil {
			// Use job timing if phaseStartedAt wasn't set.
			if pm.Duration == 0 && job.StartedAt != nil && job.CompletedAt != nil {
				pm.Duration = job.CompletedAt.Sub(*job.StartedAt)
			}
			if agentName, ok := job.Metadata["agent_override"].(string); ok {
				pm.Agent = agentName
			} else if job.WorkerID != "" {
				for _, w := range c.pool.ListWorkers() {
					if w.ID == job.WorkerID {
						pm.Agent = w.AgentName

						break
					}
				}
			}
			if recPath, ok := job.Metadata["recording_path"].(string); ok {
				pm.RecordingPath = recPath
			}
			// Capture token usage from agent.
			if v, ok := toInt64(job.Metadata["input_tokens"]); ok {
				pm.InputTokens = v
			}
			if v, ok := toInt64(job.Metadata["output_tokens"]); ok {
				pm.OutputTokens = v
			}
			if v, ok := toInt64(job.Metadata["total_tokens"]); ok {
				pm.TotalTokens = v
			}
			if pm.TotalTokens > 0 {
				pm.EstCostUSD = estimateCost(pm.Agent, pm.InputTokens, pm.OutputTokens)
			}
		}
	}

	// Capture latest checkpoint SHA if available.
	if len(c.workUnit.Checkpoints) > 0 {
		pm.CheckpointSHA = c.workUnit.Checkpoints[len(c.workUnit.Checkpoints)-1]
	}

	c.workUnit.PhaseMetrics[phase] = pm

	// Record per-agent execution metrics for dashboard breakdown.
	if pm.Agent != "" {
		metrics.Global().RecordAgentExecution(pm.Agent, pm.TotalTokens, pm.Duration, false)
	}

	// Feed completed duration into the calibrator for future progress estimates.
	c.recordProgressCalibration(phase)

	// Clear progress estimator now that the phase is complete.
	c.progressEstimator = nil

	// Emit event log entry for phase completion.
	c.emitEventLog(eventlog.Entry{
		Type:  eventlog.EventPhaseCompleted,
		Phase: phase,
		Data: map[string]any{
			"duration_ms": pm.Duration.Milliseconds(),
			"agent":       pm.Agent,
		},
	})
}

// recordPhaseMetricsFromGraph records metrics for graph-based phase execution.
// Must be called with c.mu held.
func (c *Conductor) recordPhaseMetricsFromGraph(completionEvent Event, _ *graph.Scheduler) {
	phase := phaseFromEvent(completionEvent)
	if phase == "" || c.workUnit == nil {
		return
	}

	if c.workUnit.PhaseMetrics == nil {
		c.workUnit.PhaseMetrics = make(map[string]*PhaseMetrics)
	}

	pm := &PhaseMetrics{}

	// Compute duration from phaseStartedAt.
	if !c.phaseStartedAt.IsZero() {
		pm.Duration = time.Since(c.phaseStartedAt)
	}

	// Resolve agent name from settings.
	if agentName := c.resolveAgent(phase); agentName != "" {
		pm.Agent = agentName
	}

	// Capture latest checkpoint SHA if available.
	if len(c.workUnit.Checkpoints) > 0 {
		pm.CheckpointSHA = c.workUnit.Checkpoints[len(c.workUnit.Checkpoints)-1]
	}

	c.workUnit.PhaseMetrics[phase] = pm

	// Feed completed duration into the calibrator for future progress estimates.
	c.recordProgressCalibration(phase)

	// Clear progress estimator now that the phase is complete.
	c.progressEstimator = nil
}

// setupCanaryHarness creates a canary harness if canary sandboxing is enabled.
// Must be called with c.mu held. Call cleanupCanaryHarness when the job finishes.
func (c *Conductor) setupCanaryHarness() {
	cfg := c.GetEffectiveSettings()
	if !cfg.Security.CanaryEnabled {
		return
	}

	harness, err := security.NewCanaryHarness()
	if err != nil {
		slog.Warn("canary harness setup failed", "error", err)

		return
	}

	c.canaryHarness = harness
	slog.Info("canary harness active", "dir", harness.Dir)
}

// checkCanaryViolations checks canary files for unauthorized access and logs violations.
// Must be called with c.mu held. Cleans up the harness afterward.
func (c *Conductor) checkCanaryViolations(jobOutput string) {
	if c.canaryHarness == nil {
		return
	}

	violations := c.canaryHarness.Check(jobOutput)
	if len(violations) > 0 {
		slog.Warn("canary violations detected", "count", len(violations))
		for _, v := range violations {
			slog.Warn("canary violation", "file", v.File, "method", v.Method)
		}

		if data, err := json.Marshal(violations); err == nil {
			c.emit(ConductorEvent{
				Type:    "canary_violation",
				Message: fmt.Sprintf("%d canary violations detected", len(violations)),
				Data:    data,
			})
		}
	}

	c.canaryHarness.Cleanup()
	c.canaryHarness = nil
}

//nolint:contextcheck // Intentionally accepts lifecycle context, not request context
func (c *Conductor) watchJob(ctx context.Context, jobID string, completionEvent Event) {
	if c.pool == nil {
		return
	}

	stream := c.pool.Stream(jobID)
	if stream == nil {
		return
	}

	// Create pre-job safety checkpoint so user can undo if the job fails or crashes
	c.mu.Lock()
	if c.workUnit != nil {
		workDir := c.getWorkDir()
		if repo, err := git.Open(workDir); err == nil {
			if err := repo.StageAll(ctx); err == nil {
				if hasChanges, _ := repo.HasUncommittedChanges(ctx); hasChanges {
					if sha, commitErr := repo.Commit(ctx, c.formatCheckpointMessage(fmt.Sprintf("pre-%s checkpoint", completionEvent))); commitErr == nil {
						c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, sha)
						c.recordCheckpointMeta(sha, fmt.Sprintf("pre-%s checkpoint", completionEvent), string(c.machine.State()))
						slog.Info("pre-job checkpoint created", "sha", sha, "event", completionEvent)
					}
				}
			}
		}
	}
	c.mu.Unlock()

	for event := range stream {
		// Signal progress on agent activity events (tool calls).
		if event.Type == "tool_use" || event.Type == "tool_result" {
			c.SignalProgress()
		}

		// Forward streaming events
		c.emit(ConductorEvent{
			Type:    "job_output",
			JobID:   jobID,
			Message: event.Content,
		})

		if event.Type == "job_completed" {
			c.mu.Lock()
			c.activeJobID = "" // Clear active job on completion
			var (
				wuSnapshot *WorkUnit
				indexer    *memory.Indexer
			)
			if c.workUnit != nil {
				// Capture pre-job checkpoint ref for commit validation range.
				var preJobCheckpoint string
				if len(c.workUnit.Checkpoints) > 0 {
					preJobCheckpoint = c.workUnit.Checkpoints[len(c.workUnit.Checkpoints)-1]
				}
				// For planning jobs, detect newly written specification files
				// and optionally copy to the repo
				if completionEvent == EventPlanDone {
					c.detectSpecificationFiles()
					if len(c.workUnit.Specifications) == 0 {
						err := errors.New("plan phase produced no specification file (agent did not write required deliverable)")
						c.emitEnrichedError(err, "plan")
						_ = c.machine.Dispatch(ctx, EventError)
						c.persistState()
						c.mu.Unlock()

						return
					}
					c.copySpecsToRepo()
					c.copyPlanToRepo()
					c.commitRepoSpecs(ctx)
				}

				// Create checkpoint after job completion
				// Use work directory (isolated worktree if active, main worktree otherwise)
				workDir := c.getWorkDir()
				if repo, err := git.Open(workDir); err != nil {
					slog.Debug("checkpoint: git open failed", "error", err, "workDir", workDir)
				} else {
					// Stage all changes first
					if stageErr := repo.StageAll(ctx); stageErr != nil {
						slog.Warn("checkpoint: stage failed", "error", stageErr, "workDir", workDir)
					} else if hasChanges, _ := repo.HasUncommittedChanges(ctx); hasChanges {
						sha, commitErr := repo.Commit(ctx, c.formatCheckpointMessage(fmt.Sprintf("%s complete", completionEvent)))
						if commitErr == nil {
							c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, sha)
							c.recordCheckpointMeta(sha, fmt.Sprintf("%s complete", completionEvent), string(c.machine.State()))
							slog.Info("checkpoint created", "sha", sha, "event", completionEvent)
						} else {
							slog.Warn("checkpoint: commit failed", "error", commitErr, "workDir", workDir)
						}
					} else {
						// No uncommitted changes - but Claude may have committed during the job.
						// Capture the current HEAD if it's not already in checkpoints.
						if headSHA, headErr := repo.CurrentCommit(ctx); headErr == nil && headSHA != "" {
							if !slices.Contains(c.workUnit.Checkpoints, headSHA) {
								c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, headSHA)
								c.recordCheckpointMeta(headSHA, fmt.Sprintf("%s complete (agent commit)", completionEvent), string(c.machine.State()))
								slog.Info("checkpoint captured (agent commit)", "sha", headSHA, "event", completionEvent)
							} else {
								slog.Debug("checkpoint: no new commits", "event", completionEvent)
							}
						}
					}
				}

				// Build validation params after checkpoint creation so checkpointSHAs
				// includes newly created checkpoints.
				commitValParams := c.prepareCommitValidation(preJobCheckpoint)

				// Evaluate output via strategy before dispatching completion.
				// If the strategy requests iteration, re-submit and skip normal completion.
				var jobOutput string
				var jobPrompt string
				if c.pool != nil {
					if job := c.pool.GetJob(jobID); job != nil {
						jobOutput = job.Result
						jobPrompt = job.Prompt
					}
				}

				// Store successful response in cache for future deduplication.
				if jobOutput != "" && jobPrompt != "" {
					phase := phaseFromEvent(completionEvent)
					c.storeResponseCache(jobPrompt, jobOutput, phase)
				}

				// Check canary violations using job output
				c.checkCanaryViolations(jobOutput)

				c.mu.Unlock()

				// Validate agent commits without holding the lock (runs git subprocess)
				c.validateAgentCommits(ctx, commitValParams)

				if jobOutput != "" && c.evaluateAndMaybeIterate(ctx, completionEvent, jobOutput) {
					return // Re-submitted; skip normal completion path
				}

				// Router-based adaptive phase progression: evaluate output and decide next action.
				// This runs AFTER strategy evaluation completes, wrapping the normal advance path.
				if c.router != nil {
					phase := phaseFromEvent(completionEvent)
					decision := c.router.Route(ctx, phase, jobOutput, 0)
					if c.applyRouteDecision(ctx, decision, completionEvent) {
						return // Router handled it (retry/skip/rollback)
					}
				}

				c.mu.Lock()

				// Mark implementation as done for guard checks
				if completionEvent == EventImplementDone {
					c.workUnit.HasImplemented = true
				}

				// Clear stale PriorStableState on successful completion so it
				// doesn't affect rollback targets in subsequent operations.
				c.machine.ClearPriorStableState()

				// Record per-phase execution metrics.
				c.recordPhaseMetrics(completionEvent, jobID)

				// Dispatch completion event
				_ = c.machine.Dispatch(ctx, completionEvent)

				// Persist updated state (new checkpoint + new state + metrics)
				c.persistState()

				// Capture snapshot for async memory indexing (only for major phases)
				if completionEvent == EventPlanDone || completionEvent == EventImplementDone {
					wuSnapshot = c.workUnit
					indexer = c.memoryIndexer
				}
			}

			// Detect base branch while we still have context.
			baseBranch, baseBranchErr := c.getBaseBranch(ctx)
			c.mu.Unlock()

			// Trigger async memory indexing so it does not block the workflow.
			// Use detached context so indexing continues even if parent ctx is cancelled.
			// Skip indexing if base branch detection failed (non-critical).
			if baseBranchErr != nil {
				slog.Debug("skipping memory indexing - cannot detect base branch", "error", baseBranchErr)
			} else if indexer != nil && wuSnapshot != nil {
				// Create detached context outside the goroutine so background indexing
				// continues even after the parent request context is cancelled.
				asyncCtx, asyncCancel := context.WithTimeout(context.Background(), 30*time.Second)

				//nolint:contextcheck // Intentionally uses detached context for background indexing
				go func(wu *WorkUnit, idx *memory.Indexer, event Event, base string) {
					defer asyncCancel()
					if err := idx.IndexTask(asyncCtx, wu.ID, wu.Title, wu.Description, wu.Branch, base); err != nil {
						slog.Warn("memory indexing failed", "task_id", wu.ID, "event", event, "error", err)
					}
				}(wuSnapshot, indexer, completionEvent, baseBranch)
			}

			// Auto-advance: trigger next phase if enabled
			c.maybeAutoAdvance(ctx, completionEvent)

			// Grace-period cleanup: keep job metadata queryable for 60s
			// to prevent "job not found" races with UI/CLI polling.
			if c.pool != nil {
				pool := c.pool
				time.AfterFunc(60*time.Second, func() {
					pool.RemoveJob(jobID)
				})
			}

			return
		}

		if event.Type == "job_failed" {
			c.mu.Lock()
			c.activeJobID = "" // Clear active job on failure

			// Clean up canary harness on failure (no output to check)
			c.checkCanaryViolations("")

			// Capture any partial work the agent completed before crashing
			if c.workUnit != nil {
				workDir := c.getWorkDir()
				if repo, err := git.Open(workDir); err == nil {
					if stageErr := repo.StageAll(ctx); stageErr == nil {
						if hasChanges, _ := repo.HasUncommittedChanges(ctx); hasChanges {
							if sha, commitErr := repo.Commit(ctx, c.formatCheckpointMessage(fmt.Sprintf("partial work before %s failure", completionEvent))); commitErr == nil {
								c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, sha)
								slog.Info("partial work checkpoint saved", "sha", sha, "event", completionEvent)
							}
						}
					}
				}
			}
			c.mu.Unlock()

			// Apply per-phase failure policy before default error handling.
			if c.applyFailurePolicy(ctx, completionEvent, event.Content) {
				return // Policy handled it (retry or skip)
			}

			c.mu.Lock()
			_ = c.machine.Dispatch(ctx, EventError)
			c.persistState()
			c.mu.Unlock()

			c.emit(ConductorEvent{
				Type:    "job_failed",
				JobID:   jobID,
				Error:   event.Content,
				Message: "Job failed",
			})

			// Also emit enriched error for user-facing context
			if event.Content != "" {
				c.emitEnrichedError(fmt.Errorf("%s", event.Content), string(completionEvent))
			}

			return
		}
	}
}
