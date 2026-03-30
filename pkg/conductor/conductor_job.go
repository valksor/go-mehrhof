package conductor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/eventlog"
	"github.com/valksor/kvelmo/pkg/git"
	"github.com/valksor/kvelmo/pkg/graph"
	"github.com/valksor/kvelmo/pkg/memory"
	"github.com/valksor/kvelmo/pkg/metrics"
	"github.com/valksor/kvelmo/pkg/provider"
	"github.com/valksor/kvelmo/pkg/security"
	"github.com/valksor/kvelmo/pkg/storage"
	"github.com/valksor/kvelmo/pkg/worker"
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

// saveJobSession persists a session entry for the given job so that it can be
// resumed later.  This is a best-effort operation; errors are logged and ignored.
func (c *Conductor) saveJobSession(jobID, phase, agentType string) {
	if c.store == nil || c.workUnit == nil {
		return
	}
	sessStore := storage.NewSessionStore(c.store)
	entry := storage.SessionEntry{
		SessionID: jobID,
		AgentType: agentType,
		TaskID:    c.workUnit.ID,
		Phase:     phase,
	}
	if err := sessStore.SaveSession(entry); err != nil {
		slog.Warn("persist session failed", "task_id", c.workUnit.ID, "phase", phase, "error", err)
	}
}

func (c *Conductor) generateBranchName(wu *WorkUnit) string {
	effectiveSettings := c.getEffectiveSettings()
	pattern := effectiveSettings.Git.BranchPattern
	if pattern == "" {
		pattern = "feature/{key}--{slug}"
	}

	// Determine key
	key := wu.ID
	if wu.ExternalID != "" {
		key = wu.ExternalID
	}

	// Generate slug from title
	slug := slugify(wu.Title)

	// Determine type (provider name or "local")
	taskType := "local"
	if wu.Source != nil {
		taskType = wu.Source.Provider
	}

	// Interpolate variables
	result := pattern
	if key == "" {
		// Remove {key}-- or {key}- patterns when key is empty to avoid leading dashes
		result = strings.ReplaceAll(result, "{key}--", "")
		result = strings.ReplaceAll(result, "{key}-", "")
	}
	result = strings.ReplaceAll(result, "{key}", key)
	result = strings.ReplaceAll(result, "{slug}", slug)
	result = strings.ReplaceAll(result, "{type}", taskType)

	// Clean up: collapse multiple dashes and remove trailing dashes
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	result = strings.TrimRight(result, "-")

	return result
}

func validateBranchName(name, pattern string) error {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid branch validation pattern %q: %w", pattern, err)
	}
	if !re.MatchString(name) {
		return fmt.Errorf("generated branch name %q does not match required pattern %s", name, pattern)
	}

	return nil
}

// formatCheckpointMessage formats a checkpoint commit message using the same
// CommitPrefix as agent commits, keeping git history style-consistent.
func (c *Conductor) formatCheckpointMessage(message string) string {
	prefix := c.interpolatedCommitPrefix()

	return prefix + " " + message
}

// recordCheckpointMeta stores rich metadata for a checkpoint SHA.
// Caller must hold c.mu.
func (c *Conductor) recordCheckpointMeta(sha, message, state string) {
	if c.workUnit == nil {
		return
	}
	if c.workUnit.CheckpointMeta == nil {
		c.workUnit.CheckpointMeta = make(map[string]CheckpointMeta)
	}
	c.workUnit.CheckpointMeta[sha] = CheckpointMeta{
		Message:   message,
		State:     state,
		CreatedAt: time.Now(),
	}
}

func (c *Conductor) interpolatedCommitPrefix() string {
	settings := c.getEffectiveSettings()
	prefix := settings.Git.CommitPrefix
	if prefix == "" {
		return "[kvelmo]"
	}
	if c.workUnit != nil && c.workUnit.ExternalID != "" {
		prefix = strings.ReplaceAll(prefix, "{key}", c.workUnit.ExternalID)
	} else {
		prefix = strings.ReplaceAll(prefix, "{key}", "")
		prefix = strings.ReplaceAll(prefix, "[]", "[kvelmo]")
	}

	return prefix
}

func (c *Conductor) interpolatePRTitle(title string) string {
	settings := c.getEffectiveSettings()
	pattern := settings.Git.PRTitlePattern
	if pattern == "" {
		return "[kvelmo] " + title
	}

	key := ""
	taskType := "local"
	if c.workUnit != nil {
		if c.workUnit.ExternalID != "" {
			key = c.workUnit.ExternalID
		}
		if c.workUnit.Source != nil {
			taskType = c.workUnit.Source.Provider
		}
	}

	slug := ""
	if c.workUnit != nil {
		slug = slugify(c.workUnit.Title)
	}

	result := pattern
	if key == "" {
		result = strings.ReplaceAll(result, "[{key}] ", "")
		result = strings.ReplaceAll(result, "{key} ", "")
	}
	result = strings.ReplaceAll(result, "{key}", key)
	result = strings.ReplaceAll(result, "{title}", title)
	result = strings.ReplaceAll(result, "{type}", taskType)
	result = strings.ReplaceAll(result, "{slug}", slug)

	return result
}

func (c *Conductor) buildGitConventionInstructions() string {
	settings := c.getEffectiveSettings()
	var instructions []string

	prefix := c.interpolatedCommitPrefix()
	if prefix != "[kvelmo]" && prefix != "" {
		instructions = append(instructions, "- Prefix all commit messages with: "+prefix)
	}

	if settings.Git.CommitPattern != "" {
		instructions = append(instructions, "- All commit messages must match this pattern: "+settings.Git.CommitPattern)
	}

	if len(instructions) == 0 {
		return ""
	}

	return "\n\n## Git Conventions\n\n" + strings.Join(instructions, "\n") + "\n"
}

// commitValidationParams holds the data needed to validate agent commits,
// extracted from conductor state under the lock so the git subprocess can
// run without holding c.mu.
type commitValidationParams struct {
	pattern        string
	workDir        string
	lastCheckpoint string
	checkpointSHAs map[string]struct{}
}

// prepareCommitValidation extracts commit validation parameters from conductor state.
// Must be called with c.mu held. Returns nil if validation is not configured.
func (c *Conductor) prepareCommitValidation(lastCheckpoint string) *commitValidationParams {
	if lastCheckpoint == "" {
		return nil
	}

	settings := c.getEffectiveSettings()
	pattern := settings.Git.CommitPattern
	if pattern == "" {
		return nil
	}

	// Collect checkpoint SHAs so validateAgentCommits can skip kvelmo's own commits.
	cpSHAs := make(map[string]struct{}, len(c.workUnit.Checkpoints))
	for _, sha := range c.workUnit.Checkpoints {
		cpSHAs[sha] = struct{}{}
	}

	return &commitValidationParams{
		pattern:        pattern,
		workDir:        c.getWorkDir(),
		lastCheckpoint: lastCheckpoint,
		checkpointSHAs: cpSHAs,
	}
}

// validateAgentCommits checks commits made by the agent against CommitPattern.
// Emits a degraded warning if any agent commits don't match the configured pattern.
// Safe to call without c.mu held — runs git subprocess without holding the lock.
func (c *Conductor) validateAgentCommits(ctx context.Context, params *commitValidationParams) {
	if params == nil {
		return
	}

	repo, err := git.Open(params.workDir)
	if err != nil {
		slog.Debug("commit validation: could not open repo", "error", err)

		return
	}

	commits, err := repo.CommitsSince(ctx, params.lastCheckpoint)
	if err != nil {
		slog.Debug("commit validation: could not list commits", "error", err)

		return
	}

	var invalid []string
	for _, entry := range commits {
		// Skip kvelmo's own checkpoint commits (identified by SHA, not prefix).
		if _, isCheckpoint := params.checkpointSHAs[entry.SHA]; isCheckpoint {
			continue
		}
		if err := git.ValidateCommitMessage(entry.Message, params.pattern); err != nil {
			shortSHA := entry.SHA
			if len(shortSHA) > 8 {
				shortSHA = shortSHA[:8]
			}
			invalid = append(invalid, fmt.Sprintf("%s: %s", shortSHA, entry.Message))
		}
	}

	if len(invalid) == 0 {
		return
	}

	msg := fmt.Sprintf("%d commit(s) do not match pattern %s: %s",
		len(invalid), params.pattern, strings.Join(invalid, "; "))
	slog.Warn("commit validation", "violations", len(invalid), "pattern", params.pattern)

	c.emit(ConductorEvent{
		Type:           "commit_validation_warning",
		Message:        msg,
		FailureClass:   FailureClassDegraded,
		FailureMessage: msg,
	})
}

// slugify converts a string to a URL-safe slug.
func slugify(s string) string {
	// Lowercase
	s = strings.ToLower(s)
	// Replace spaces and underscores with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	// Remove non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	// Collapse multiple hyphens
	slug := result.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")
	// Limit length (use runes for UTF-8 safety)
	runes := []rune(slug)
	if len(runes) > 50 {
		slug = string(runes[:50])
		// Don't end with hyphen
		slug = strings.TrimRight(slug, "-")
	}

	return slug
}

// shouldPostTicketComment checks if ticket comments are enabled for the current provider.
func (c *Conductor) shouldPostTicketComment() bool {
	if c.workUnit == nil || c.workUnit.Source == nil {
		return false
	}

	effectiveSettings := c.getEffectiveSettings()

	switch c.workUnit.Source.Provider {
	case provider.NameGitHub:
		return effectiveSettings.Providers.GitHub.AllowTicketComment
	case provider.NameGitLab:
		return effectiveSettings.Providers.GitLab.AllowTicketComment
	case provider.NameWrike:
		return effectiveSettings.Providers.Wrike.AllowTicketComment
	case provider.NameLinear:
		return effectiveSettings.Providers.Linear.AllowTicketComment
	case provider.NameJira:
		return effectiveSettings.Providers.Jira.AllowTicketComment
	default:
		return false
	}
}

// buildJobOptions creates JobOptions with execution context for multi-project support.
// This ensures jobs carry full context (WorkDir, metadata) so any worker can execute them.
// If canary sandboxing is enabled, injects fake HOME environment variables.
// buildJobOptionsForPhase creates JobOptions with the per-phase agent override set.
func (c *Conductor) buildJobOptionsForPhase(phase string) *worker.JobOptions {
	opts := c.buildJobOptions()
	if agentName := c.resolveAgent(phase); agentName != "" {
		opts.Agent = agentName
	}

	return opts
}

func (c *Conductor) buildJobOptions() *worker.JobOptions {
	opts := &worker.JobOptions{
		WorkDir:  c.getWorkDir(), // Use isolated worktree if available
		Metadata: make(map[string]any),
	}

	// Add task metadata
	if c.workUnit != nil {
		opts.Metadata["task_id"] = c.workUnit.ID
		opts.Metadata["task_title"] = c.workUnit.Title
		if c.workUnit.ExternalID != "" {
			opts.Metadata["external_id"] = c.workUnit.ExternalID
		}
		if c.workUnit.Source != nil {
			opts.Metadata["provider"] = c.workUnit.Source.Provider
			opts.Metadata["reference"] = c.workUnit.Source.Reference
		}
	}

	// Inject canary harness environment if active
	if c.canaryHarness != nil {
		if opts.Environment == nil {
			opts.Environment = make(map[string]string)
		}
		maps.Copy(opts.Environment, c.canaryHarness.Env())
	}

	return opts
}

// detectSpecificationFiles scans for specification files and adds any new ones
// to the work unit's Specifications list. Uses the storage layer path which
// respects the saveInProject config setting.
// Must be called with c.mu held.
func (c *Conductor) detectSpecificationFiles() {
	if c.workUnit == nil || c.store == nil {
		return
	}

	// Build set of known specs for quick lookup (normalized for deduplication)
	known := make(map[string]bool)
	for _, sp := range c.workUnit.Specifications {
		known[filepath.Clean(sp)] = true
	}

	specDir := c.store.SpecificationsDir(c.workUnit.ID)
	entries, err := os.ReadDir(specDir)
	if err != nil {
		// Directory may not exist yet
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "specification-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		fullPath := filepath.Join(specDir, name)
		if !known[filepath.Clean(fullPath)] {
			c.workUnit.Specifications = append(c.workUnit.Specifications, fullPath)
			slog.Info("detected new specification file", "path", fullPath)
		}
	}
}

// copySpecsToRepo copies specification files to an in-repo path if configured.
// Must be called with c.mu held.
func (c *Conductor) copySpecsToRepo() {
	if c.workUnit == nil {
		return
	}

	settings := c.getEffectiveSettings()
	outputPath := settings.Storage.SpecOutputPath
	if outputPath == "" {
		return
	}

	// Interpolate variables
	key := ""
	if c.workUnit.ExternalID != "" {
		key = c.workUnit.ExternalID
	}
	slug := slugify(c.workUnit.Title)

	workDir := c.getWorkDir()

	for _, specPath := range c.workUnit.Specifications {
		data, err := os.ReadFile(specPath)
		if err != nil {
			c.logVerbosef("Warning: could not read spec for repo copy: %v", err)

			continue
		}

		// Interpolate output path per spec
		resolved := outputPath
		resolved = strings.ReplaceAll(resolved, "{key}", key)
		resolved = strings.ReplaceAll(resolved, "{slug}", slug)

		fullPath := filepath.Join(workDir, resolved)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			c.logVerbosef("Warning: could not create spec output dir: %v", err)

			continue
		}

		if err := os.WriteFile(fullPath, data, 0o644); err != nil {
			c.logVerbosef("Warning: could not write spec to repo: %v", err)
		} else {
			slog.Info("spec copied to repo", "path", fullPath)
		}
	}
}

// copyPlanToRepo copies the latest plan to an in-repo path if configured.
// Must be called with c.mu held.
func (c *Conductor) copyPlanToRepo() {
	if c.workUnit == nil || c.store == nil {
		return
	}

	s := c.getEffectiveSettings()
	outputPath := s.Storage.PlanOutputPath
	if outputPath == "" {
		return
	}

	planStore := storage.NewPlanStore(c.store)
	plan, err := planStore.GetLatestPlan(c.workUnit.ID)
	if err != nil || plan == nil {
		return
	}

	// Load plan history markdown
	history, err := planStore.LoadPlanHistory(c.workUnit.ID, plan.ID)
	if err != nil {
		c.logVerbosef("Warning: could not read plan history for repo copy: %v", err)

		return
	}

	// Interpolate variables
	key := ""
	if c.workUnit.ExternalID != "" {
		key = c.workUnit.ExternalID
	}
	slug := slugify(c.workUnit.Title)

	resolved := outputPath
	resolved = strings.ReplaceAll(resolved, "{key}", key)
	resolved = strings.ReplaceAll(resolved, "{slug}", slug)

	workDir := c.getWorkDir()
	fullPath := filepath.Join(workDir, resolved)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		c.logVerbosef("Warning: could not create plan output dir: %v", err)

		return
	}

	if err := os.WriteFile(fullPath, []byte(history), 0o644); err != nil {
		c.logVerbosef("Warning: could not write plan to repo: %v", err)
	} else {
		slog.Info("plan copied to repo", "path", fullPath)
	}
}

// commitRepoSpecs commits spec and plan files that were copied to the repo.
// Must be called after copySpecsToRepo/copyPlanToRepo and with c.mu held.
func (c *Conductor) commitRepoSpecs(ctx context.Context) {
	settings := c.getEffectiveSettings()
	if settings.Storage.CommitSpecs == nil || !*settings.Storage.CommitSpecs {
		return
	}
	if settings.Storage.SpecOutputPath == "" && settings.Storage.PlanOutputPath == "" {
		return
	}

	workDir := c.getWorkDir()
	repo, err := git.Open(workDir)
	if err != nil {
		return
	}

	// Stage only spec/plan output files
	key := ""
	slug := ""
	if c.workUnit != nil {
		if c.workUnit.ExternalID != "" {
			key = c.workUnit.ExternalID
		}
		slug = slugify(c.workUnit.Title)
	}

	var filesToStage []string
	for _, outputPath := range []string{settings.Storage.SpecOutputPath, settings.Storage.PlanOutputPath} {
		if outputPath == "" {
			continue
		}
		resolved := outputPath
		resolved = strings.ReplaceAll(resolved, "{key}", key)
		resolved = strings.ReplaceAll(resolved, "{slug}", slug)
		fullPath := filepath.Join(workDir, resolved)
		if _, err := os.Stat(fullPath); err == nil {
			filesToStage = append(filesToStage, fullPath)
		} else {
			slog.Warn("spec commit: output file not found", "path", fullPath)
		}
	}

	if len(filesToStage) == 0 {
		return
	}

	if err := repo.StageFiles(ctx, filesToStage...); err != nil {
		slog.Warn("spec stage failed", "error", err)

		return
	}

	hasChanges, _ := repo.HasUncommittedChanges(ctx)
	if !hasChanges {
		return
	}

	label := "specification"
	if key != "" {
		label += " for " + key
	}
	commitMsg := c.formatCheckpointMessage("Add " + label)

	sha, err := repo.Commit(ctx, commitMsg)
	if err != nil {
		slog.Warn("spec commit failed", "error", err)

		return
	}

	c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, sha)
	slog.Info("spec committed to repo", "sha", sha)
}

// toInt64 extracts an int64 from an any value (handles int64 and float64 from JSON).
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

// estimateCost returns a rough USD cost estimate based on provider pricing.
// Prices are approximate and should be updated as provider pricing changes.
func estimateCost(agentName string, inputTokens, outputTokens int64) float64 {
	var inputPricePer1M, outputPricePer1M float64

	switch agentName {
	case "anthropic":
		// Claude Sonnet 4 pricing (approximate)
		inputPricePer1M = 3.0
		outputPricePer1M = 15.0
	case "openai":
		// GPT-4o pricing (approximate)
		inputPricePer1M = 2.5
		outputPricePer1M = 10.0
	case "ollama":
		// Local models: no cost
		return 0
	default:
		// Conservative default estimate
		inputPricePer1M = 3.0
		outputPricePer1M = 15.0
	}

	return float64(inputTokens)/1_000_000*inputPricePer1M +
		float64(outputTokens)/1_000_000*outputPricePer1M
}
