package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/agent/recorder"
	"github.com/valksor/kvelmo/pkg/changelog"
	"github.com/valksor/kvelmo/pkg/changeset"
	"github.com/valksor/kvelmo/pkg/git"
	"github.com/valksor/kvelmo/pkg/memory"
	"github.com/valksor/kvelmo/pkg/policy"
	"github.com/valksor/kvelmo/pkg/provider"
	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/storage"
)

// Submit submits the task to the provider (creates PR, updates issue, etc).
// The lock is released during network operations to avoid blocking other callers.
// State transition happens AFTER successful operations to avoid terminal state on failure.
func (c *Conductor) Submit(ctx context.Context, deleteBranch bool) error {
	slog.Info("submit: called", "delete_branch", deleteBranch)
	// Phase 1: Pre-flight checks and validate state transition is possible
	slog.Info("submit: acquiring lock")
	c.mu.Lock()
	slog.Info("submit: lock acquired")
	if c.workUnit == nil {
		err := errors.New("no task loaded")
		c.mu.Unlock()
		c.emitEnrichedError(err, "submit")

		return err
	}

	// Check review checklist completion
	settings := c.getEffectiveSettings()
	checklist := settings.Workflow.Policy.ReviewChecklist
	if len(checklist) > 0 {
		for _, item := range checklist {
			if !slices.Contains(c.workUnit.ChecklistChecked, item) {
				c.mu.Unlock()

				return fmt.Errorf("cannot submit: review checklist item %q not checked. Run: kvelmo review --check %q", item, item)
			}
		}
	}

	// Check required phases policy.
	// On re-submit, only check phases completed since the last submit (not cumulative history).
	if requiredPhases := settings.Workflow.Policy.RequiredPhases; len(requiredPhases) > 0 {
		history := c.machine.History()
		// Find the most recent EventSubmit to scope the check to the current cycle
		sinceIdx := 0
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Event == EventSubmit {
				sinceIdx = i + 1

				break
			}
		}
		recentHistory := history[sinceIdx:]
		phaseEventMap := map[string]Event{
			"plan":      EventPlanDone,
			"implement": EventImplementDone,
			"review":    EventReviewDone,
			"simplify":  EventSimplifyDone,
			"optimize":  EventOptimizeDone,
		}
		for _, phase := range requiredPhases {
			doneEvent, ok := phaseEventMap[phase]
			if !ok {
				continue
			}
			found := false
			for _, h := range recentHistory {
				if h.Event == doneEvent {
					found = true

					break
				}
			}
			if !found {
				c.mu.Unlock()

				return fmt.Errorf("cannot submit: required phase %q was not completed", phase)
			}
		}
	}

	// Check task group readiness (cross-repo synchronized submit)
	if c.taskGroupChecker != nil {
		syncSubmit := true
		if tg := settings.Workflow.TaskGroups; tg != nil {
			syncSubmit = tg.SyncSubmit
		}
		taskID := c.workUnit.ID
		c.mu.Unlock()
		ok, groupErr := c.taskGroupChecker.CanSubmit(taskID, syncSubmit)
		c.mu.Lock()
		if !ok {
			c.mu.Unlock()

			return groupErr
		}
	}

	// Check approval requirement
	if err := c.checkApproval(ctx, EventSubmit); err != nil {
		c.mu.Unlock()

		return err
	}

	// Run pre-transition hooks (release lock during shell execution)
	c.mu.Unlock()
	if err := c.RunTransitionHooks(ctx, EventSubmit); err != nil {
		c.emitEnrichedError(err, "submit")

		return err
	}
	c.mu.Lock()

	// Hoist DiffFilesWithStatus so both the policy block and the sensitive-paths
	// block can reuse the result without calling git twice.
	var changedFiles []git.FileStatus
	var changedPaths []string
	if c.git != nil {
		c.mu.Unlock()
		files, diffErr := c.git.DiffFilesWithStatus(ctx)
		c.mu.Lock()
		if diffErr != nil {
			slog.Warn("failed to diff files for policy/sensitive-path checks", "error", diffErr)
		} else {
			changedFiles = files
			changedPaths = make([]string, len(files))
			for i, f := range files {
				changedPaths[i] = f.Path
			}
		}
	}

	// Enforce policy violations — error-severity violations block submit.
	if policyCfg := settings.Workflow.Policy; policyCfg.RequiredPhases != nil || policyCfg.RequireSecurityScan || len(policyCfg.SensitivePaths) > 0 || len(policyCfg.DocRequirements) > 0 {
		specs := c.workUnit.Specifications
		state := string(c.machine.State())
		var docReqs []policy.DocRequirement
		for _, d := range policyCfg.DocRequirements {
			docReqs = append(docReqs, policy.DocRequirement{Trigger: d.Trigger, Requires: d.Requires})
		}
		violations := policy.Evaluate(policy.Settings{
			RequiredPhases:      policyCfg.RequiredPhases,
			SensitivePaths:      policyCfg.SensitivePaths,
			MinSpecSections:     policyCfg.MinSpecSections,
			RequireSecurityScan: policyCfg.RequireSecurityScan,
			DocRequirements:     docReqs,
		}, "submit", state, specs, changedPaths)
		if policy.HasBlockingViolation(violations) {
			var msgs []string
			for _, v := range violations {
				if v.Severity == policy.SeverityError {
					msgs = append(msgs, v.Message)
				}
			}
			c.mu.Unlock()

			return fmt.Errorf("policy violations block submit: %s", strings.Join(msgs, "; "))
		}
	}

	// Validate PR template required sections (checked early to fail fast)
	prRequiredSections := settings.Git.PRRequiredSections
	prCustomSections := settings.Git.PRCustomSections

	// Check quality gate - use cached result from Review() if available,
	// otherwise run synchronously (when Review was skipped)
	slog.Info("submit: checking quality gate", "cached", c.workUnit.QualityGatePassed != nil)
	if c.workUnit.QualityGatePassed != nil {
		// Use cached result from async quality gate
		if !*c.workUnit.QualityGatePassed {
			errMsg := c.workUnit.QualityGateError
			c.mu.Unlock()

			return fmt.Errorf("quality gate failed: %s", errMsg)
		}
		slog.Info("submit: quality gate passed (cached)")
	} else if c.qualityGateRunning {
		// Another goroutine is already running the quality gate; wait for its result.
		c.mu.Unlock()
		c.qualityGateDone.Wait()
		c.mu.Lock()
		if c.workUnit.QualityGatePassed == nil || !*c.workUnit.QualityGatePassed {
			errMsg := c.workUnit.QualityGateError
			c.mu.Unlock()

			return fmt.Errorf("quality gate failed: %s", errMsg)
		}
		slog.Info("submit: quality gate passed (waited)")
	} else {
		// No cached result - run synchronously (Review was skipped or old state).
		// Unlock before calling runQualityGate — it acquires its own locks internally.
		// Set guard flag to prevent concurrent runs.
		c.qualityGateRunning = true
		c.qualityGateDone.Add(1)
		slog.Info("submit: running quality gate synchronously")
		c.mu.Unlock()
		err := c.runQualityGate(ctx)
		c.mu.Lock()
		c.qualityGateRunning = false
		c.qualityGateDone.Done() // Must be called under mu so waiters see the state write above
		if err != nil {
			c.mu.Unlock()

			return fmt.Errorf("quality gate failed: %w", err)
		}
		// Cache the result so downstream checks (e.g. RequireSecurityScan) see it
		passed := true
		c.workUnit.QualityGatePassed = &passed
		slog.Info("submit: quality gate passed (sync)")
	}

	// Check sensitive paths policy — changes to sensitive files require review.
	// Uses the hoisted changedFiles computed above.
	if sensitivePaths := settings.Workflow.Policy.SensitivePaths; len(sensitivePaths) > 0 && c.git != nil {
		if changedFiles != nil {
			// Scope review check to current cycle (since last EventSubmit),
			// matching the required-phases check above.
			history := c.machine.History()
			sinceIdx := 0
			for i := len(history) - 1; i >= 0; i-- {
				if history[i].Event == EventSubmit {
					sinceIdx = i + 1

					break
				}
			}
			reviewDone := false
			for _, h := range history[sinceIdx:] {
				if h.Event == EventReviewDone {
					reviewDone = true

					break
				}
			}
			if !reviewDone {
				var matchedPatterns []string
				for _, pattern := range sensitivePaths {
					for _, fs := range changedFiles {
						// Use filepath.Match for simple patterns and fall back to
						// prefix matching for directory patterns (filepath.Match
						// does not match across path separators with *).
						matched, _ := filepath.Match(pattern, fs.Path)
						if !matched {
							// Check if pattern is a directory prefix (e.g., "config/" matches "config/prod/secrets.yaml")
							dir := strings.TrimRight(pattern, "*")
							if strings.HasSuffix(dir, "/") || strings.HasSuffix(dir, string(filepath.Separator)) {
								matched = strings.HasPrefix(fs.Path, dir)
							}
						}
						if matched {
							matchedPatterns = append(matchedPatterns, pattern)

							break
						}
					}
				}
				if len(matchedPatterns) > 0 {
					c.mu.Unlock()

					return fmt.Errorf("changes to sensitive files require review: %s", strings.Join(matchedPatterns, ", "))
				}
			}
		}
	}

	// Check security scan requirement.
	// Note: Uses QualityGatePassed as a proxy — a dedicated SecurityScanPassed field
	// would make this check independent of the general quality gate result.
	if settings.Workflow.Policy.RequireSecurityScan {
		if c.workUnit.QualityGatePassed == nil {
			c.mu.Unlock()

			return errors.New("security scan required before submission")
		}
	}

	// Verify state transition is possible before starting network operations.
	// Don't dispatch yet - we dispatch after success to avoid terminal state on failure.
	slog.Info("submit: checking state transition")
	if can, reason := c.machine.CanDispatch(ctx, EventSubmit); !can {
		c.mu.Unlock()
		slog.Info("submit: cannot dispatch", "reason", reason)

		return fmt.Errorf("cannot submit: %s", reason)
	}
	slog.Info("submit: state transition ok")

	// Phase 2: Copy state needed for network operations
	branch := c.workUnit.Branch
	title := c.workUnit.Title
	externalID := c.workUnit.ExternalID
	worktreePath := c.workUnit.WorktreePath
	workUnitDescription := c.workUnit.Description
	existingPRID := c.workUnit.PRID
	specCount := len(c.workUnit.Specifications)
	checkpointCount := len(c.workUnit.Checkpoints)
	var sourceProvider, sourceURL string
	if c.workUnit.Source != nil {
		sourceProvider = c.workUnit.Source.Provider
		sourceURL = c.workUnit.Source.URL
	}
	changelogPath := settings.Storage.ChangelogPath
	repo := c.git
	providers := c.providers
	memoryIndexer := c.memoryIndexer
	lifecycleCtx := c.lifecycleCtx
	shouldComment := c.shouldPostTicketComment()

	// Copy data needed for PR context (spec, review, recordings)
	phaseMetrics := c.workUnit.PhaseMetrics
	taskID := c.workUnit.ID
	checklistChecked := c.workUnit.ChecklistChecked
	qualityGatePassed := c.workUnit.QualityGatePassed
	store := c.store
	c.mu.Unlock()

	// Get diff stats for PR body (best-effort, outside lock)
	var diffStat string
	var fileStatuses []git.FileStatus
	if repo != nil {
		if base, bErr := c.getBaseBranch(ctx); bErr == nil && base != "" {
			if stat, sErr := repo.DiffAgainst(ctx, "origin/"+base, true); sErr == nil {
				diffStat = stat
			}
		}
		if statuses, fErr := repo.DiffFilesWithStatus(ctx); fErr == nil {
			fileStatuses = statuses
		}
	}

	// Build PR context: spec content, review summary, recordings
	prc := &prContext{
		Recordings: loadRecordingsFlat(phaseMetrics),
	}
	if store != nil && taskID != "" {
		specStore := storage.NewSpecStore(store)
		if content, _, err := specStore.GetLatestSpecificationContent(taskID); err == nil {
			prc.SpecContent = content
		}
		prc.ReviewSummary = buildReviewSummary(store, taskID, checklistChecked, qualityGatePassed)
	}

	// Phase 2.5: Auto-generate changelog entry if configured (before push)
	if changelogPath != "" && repo != nil {
		workDir := worktreePath
		if workDir == "" {
			workDir = repo.Path()
		}
		fullPath := filepath.Join(workDir, changelogPath)
		if err := changelog.AppendEntry(fullPath, changelog.Entry{
			Date:   time.Now(),
			Title:  title,
			TaskID: externalID,
		}); err != nil {
			slog.Warn("changelog update failed", "error", err)
		} else {
			if err := repo.StageAll(ctx); err == nil {
				if _, err := repo.Commit(ctx, "Update changelog for "+title); err != nil {
					slog.Warn("changelog commit failed", "error", err)
				}
			}
		}
	}

	// Phase 3: Network operations (no lock held)
	slog.Info("submit: starting network operations", "branch", branch, "has_git", repo != nil)
	var prURL, prID string
	if repo != nil && branch != "" {
		slog.Info("submit: pushing branch", "branch", branch)
		if err := repo.Push(ctx, "origin", branch); err != nil {
			wrapped := fmt.Errorf("push branch %s: %w", branch, err)
			c.emitEnrichedError(wrapped, "submit")

			return wrapped
		}
		slog.Info("submit: push completed")

		// Create PR via provider if supported
		if sourceProvider != "" && providers != nil {
			if p, err := providers.Get(sourceProvider); err == nil {
				if sp, ok := p.(provider.SubmitProvider); ok {
					// Get base branch (configured or auto-detected)
					baseBranch, err := c.getBaseBranch(ctx)
					if err != nil {
						return fmt.Errorf("determine base branch for PR: %w", err)
					}

					// Check branch protection rules if available (best-effort)
					if bpp, ok := p.(provider.BranchProtectionProvider); ok {
						bpOwner, bpRepo := parseOwnerRepo(externalID)
						if bpOwner != "" && bpRepo != "" {
							protection, bpErr := bpp.GetBranchProtection(ctx, bpOwner, bpRepo, baseBranch)
							if bpErr != nil {
								slog.Warn("could not check branch protection", "error", bpErr)
							} else if protection != nil {
								if protection.RequireReviews && protection.MinReviewers > 0 {
									c.emit(ConductorEvent{
										Type:    "warning",
										Message: fmt.Sprintf("Branch %q requires %d reviewer(s) before merge", baseBranch, protection.MinReviewers),
									})
								}
								if len(protection.RequiredChecks) > 0 {
									c.emit(ConductorEvent{
										Type:    "warning",
										Message: fmt.Sprintf("Branch %q has %d required status check(s)", baseBranch, len(protection.RequiredChecks)),
									})
								}
							}
						}
					}

					// Check for PR template in the repo
					var prBody string
					repoPath := repo.Path()
					if tmpl := detectPRTemplate(repoPath); tmpl != "" {
						prBody = fillPRTemplate(tmpl, workUnitDescription, checkpointCount, sourceURL)
					} else {
						prBody = buildPRDescriptionWithDecisions(workUnitDescription, specCount, checkpointCount, diffStat, fileStatuses, prCustomSections, prc)
					}
					// Validate required PR template sections are filled
					if len(prRequiredSections) > 0 {
						if missing := validatePRSections(prBody, prRequiredSections); len(missing) > 0 {
							return fmt.Errorf("PR template has unfilled required sections: %s", strings.Join(missing, ", "))
						}
					}

					prOpts := provider.PROptions{
						Title:   c.interpolatePRTitle(title),
						Body:    prBody,
						Head:    branch,
						Base:    baseBranch,
						TaskID:  externalID,
						TaskURL: sourceURL,
					}
					// If PR already exists (re-submit after re-entry), push new
					// commits instead of creating a duplicate PR.
					if existingPRID != "" {
						c.logVerbosef("PR already exists (%s), pushing updates to existing branch", existingPRID)
						prID = existingPRID
					} else if result, err := sp.CreatePR(ctx, prOpts); err == nil {
						prURL = result.URL
						prID = result.ID // Store PR ID for approve/merge operations
						c.logVerbosef("Created PR: %s", prURL)
						// Add comment linking to PR on original task (if enabled)
						if shouldComment {
							if err := sp.AddComment(ctx, externalID,
								"Pull request created: "+prURL); err != nil {
								c.logVerbosef("Warning: could not add comment: %v", err)
							}
						}
					} else {
						// PR creation failed - state remains in StateReviewing (not terminal)
						slog.Error("failed to create PR", "error", err, "branch", branch)
						wrapped := fmt.Errorf("create PR: %w", err)
						c.emitEnrichedError(wrapped, "submit")

						return wrapped
					}
				}
			}
		}

		// Delete local branch after successful submission if requested
		if deleteBranch {
			if err := repo.DeleteBranch(ctx, branch); err != nil {
				c.logVerbosef("Warning: could not delete branch: %v", err)
			}
		}
	}

	// Phase 4: State transition - only after all critical operations succeed.
	// This ensures we don't end up in terminal StateSubmitted on failure.
	if err := c.machine.Dispatch(ctx, EventSubmit); err != nil {
		// This shouldn't fail since we checked CanDispatch earlier, but handle it
		return fmt.Errorf("state transition failed: %w", err)
	}

	// Remove git worktree if isolation was used (branch has the changes now)
	if worktreePath != "" && repo != nil {
		if err := repo.RemoveWorktree(ctx, worktreePath, false); err != nil {
			c.logVerbosef("Warning: could not remove worktree %s: %v", worktreePath, err)
		}
	}

	// Phase 5: Re-acquire lock to persist state
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear worktree path so we don't try again
	if c.workUnit != nil && c.workUnit.WorktreePath != "" {
		c.workUnit.WorktreePath = ""
	}

	// Store PR ID for approve/merge operations
	if c.workUnit != nil && prID != "" {
		c.workUnit.PRID = prID
	}

	// Build event data
	eventData, err := json.Marshal(map[string]any{
		"pr_url": prURL,
	})
	if err != nil {
		slog.Warn("marshal event data failed", "error", err)
	}

	c.persistState()

	c.emit(ConductorEvent{
		Type:    "task_submitted",
		State:   c.machine.State(),
		Message: "Task submitted",
		Data:    eventData,
	})

	// Start CI fix loop if auto-fix is enabled, or CI watcher if watch-only.
	// CI watching requires providers to implement ciwatch.StatusFetcher.
	if s := c.getEffectiveSettings(); prID != "" && s.Workflow.CI.WatchEnabled {
		if s.Workflow.CI.AutoFix {
			//nolint:contextcheck // intentionally uses lifecycle context for background CI fix loop
			go c.startCIFixLoop(lifecycleCtx)
		} else {
			slog.Debug("CI watching enabled but auto-fix disabled", "pr_id", prID)
		}
	}

	// Trigger async memory indexing for submitted task.
	// Use lifecycle context (not request ctx which may be cancelled when handler returns).
	// Get base branch BEFORE goroutine (ctx may be cancelled when handler returns).
	if memoryIndexer != nil && c.workUnit != nil {
		baseBranch, err := c.getBaseBranch(ctx)
		if err != nil {
			c.logVerbosef("Warning: cannot index task - %v", err)
		} else {
			//nolint:contextcheck // intentionally uses lifecycle context for background indexing
			go func(wu *WorkUnit, idx *memory.Indexer, lctx context.Context, base string) {
				if err := idx.IndexTask(lctx, wu.ID, wu.Title, wu.Description, wu.Branch, base); err != nil {
					slog.Warn("memory indexing failed after submit", "task_id", wu.ID, "error", err)
				}
			}(c.workUnit, memoryIndexer, lifecycleCtx, baseBranch)
		}
	}

	return nil
}

// SubmitPreview holds the PR preview data for dry-run submission.
type SubmitPreview struct {
	Title          string               `json:"title"`
	Body           string               `json:"body"`
	Branch         string               `json:"branch"`
	BaseBranch     string               `json:"base_branch"`
	DiffStat       string               `json:"diff_stat,omitempty"`
	FileChanges    []git.FileStatus     `json:"file_changes,omitempty"`
	Checkpoints    int                  `json:"checkpoints"`
	Specifications int                  `json:"specifications"`
	CustomSections []settings.PRSection `json:"custom_sections,omitempty"`
}

// PreviewSubmit runs the pre-flight checks and PR body generation without
// actually pushing or creating a PR. Returns the preview of what would be submitted.
func (c *Conductor) PreviewSubmit(ctx context.Context) (*SubmitPreview, error) {
	c.mu.Lock()
	if c.workUnit == nil {
		c.mu.Unlock()

		return nil, errors.New("no task loaded")
	}

	// Copy state needed for preview generation
	branch := c.workUnit.Branch
	title := c.workUnit.Title
	workUnitDescription := c.workUnit.Description
	specCount := len(c.workUnit.Specifications)
	checkpointCount := len(c.workUnit.Checkpoints)
	var sourceURL string
	if c.workUnit.Source != nil {
		sourceURL = c.workUnit.Source.URL
	}
	repo := c.git
	effectiveSettings := c.getEffectiveSettings()
	var prCustomSections []settings.PRSection
	if effectiveSettings != nil {
		prCustomSections = effectiveSettings.Git.PRCustomSections
	}

	// Copy PR context data
	phaseMetrics := c.workUnit.PhaseMetrics
	taskID := c.workUnit.ID
	checklistChecked := c.workUnit.ChecklistChecked
	qualityGatePassed := c.workUnit.QualityGatePassed
	store := c.store
	c.mu.Unlock()

	// Get diff stats (best-effort, outside lock)
	var diffStat string
	var fileStatuses []git.FileStatus
	var baseBranch string
	if repo != nil {
		if base, bErr := c.getBaseBranch(ctx); bErr == nil && base != "" {
			baseBranch = base
			if stat, sErr := repo.DiffAgainst(ctx, "origin/"+base, true); sErr == nil {
				diffStat = stat
			}
		}
		if statuses, fErr := repo.DiffFilesWithStatus(ctx); fErr == nil {
			fileStatuses = statuses
		}
	}

	// Build PR context
	prc := &prContext{
		Recordings: loadRecordingsFlat(phaseMetrics),
	}
	if store != nil && taskID != "" {
		specStore := storage.NewSpecStore(store)
		if content, _, err := specStore.GetLatestSpecificationContent(taskID); err == nil {
			prc.SpecContent = content
		}
		prc.ReviewSummary = buildReviewSummary(store, taskID, checklistChecked, qualityGatePassed)
	}

	// Build PR body using same logic as Submit
	var prBody string
	if repo != nil {
		repoPath := repo.Path()
		if tmpl := detectPRTemplate(repoPath); tmpl != "" {
			prBody = fillPRTemplate(tmpl, workUnitDescription, checkpointCount, sourceURL)
		} else {
			prBody = buildPRDescriptionWithDecisions(workUnitDescription, specCount, checkpointCount, diffStat, fileStatuses, prCustomSections, prc)
		}
	} else {
		prBody = buildPRDescriptionWithDecisions(workUnitDescription, specCount, checkpointCount, diffStat, fileStatuses, prCustomSections, prc)
	}

	return &SubmitPreview{
		Title:          c.interpolatePRTitle(title),
		Body:           prBody,
		Branch:         branch,
		BaseBranch:     baseBranch,
		DiffStat:       diffStat,
		FileChanges:    fileStatuses,
		Checkpoints:    checkpointCount,
		Specifications: specCount,
		CustomSections: prCustomSections,
	}, nil
}

// Abandon stops any running jobs, optionally deletes the branch, and resets state.
func (c *Conductor) Abandon(ctx context.Context, keepBranch bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Cancel any running or queued jobs for this worktree
	if c.pool != nil && c.workUnit != nil {
		for _, jobID := range c.workUnit.Jobs {
			if err := c.pool.CancelJob(jobID); err != nil {
				c.logVerbosef("Could not cancel job %s: %v", jobID, err)
			}
		}
	}

	// Delete branch unless keep_branch is set
	if !keepBranch && c.git != nil && c.workUnit != nil && c.workUnit.Branch != "" {
		if err := c.git.DeleteBranch(ctx, c.workUnit.Branch); err != nil {
			c.logVerbosef("Warning: could not delete branch %s: %v", c.workUnit.Branch, err)
		}
	}

	// Remove git worktree if isolation was used
	if c.workUnit != nil && c.workUnit.WorktreePath != "" && c.git != nil {
		if err := c.git.RemoveWorktree(ctx, c.workUnit.WorktreePath, true); err != nil {
			c.logVerbosef("Warning: could not remove worktree %s: %v", c.workUnit.WorktreePath, err)
		}
	}

	// Delete persisted task state so it is not restored on next socket start.
	if c.store != nil && c.workUnit != nil {
		if err := c.store.DeleteTaskState(c.workUnit.ID); err != nil {
			slog.Warn("delete task state failed", "task_id", c.workUnit.ID, "error", err)
		}
	}

	// Reset state and clear work unit
	c.machine.Reset()
	c.workUnit = nil

	c.emit(ConductorEvent{
		Type:    "task_abandoned",
		State:   StateNone,
		Message: "Task abandoned",
	})

	return nil
}

// Delete clears the work unit when in a terminal or none state.
func (c *Conductor) Delete(ctx context.Context, deleteBranch bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	state := c.machine.State()
	if state != StateSubmitted && state != StateFailed && state != StateNone {
		return fmt.Errorf("delete only allowed in terminal states (submitted, failed, none); current state: %s", state)
	}

	// Optionally delete the branch
	if deleteBranch && c.git != nil && c.workUnit != nil && c.workUnit.Branch != "" {
		if err := c.git.DeleteBranch(ctx, c.workUnit.Branch); err != nil {
			c.logVerbosef("Warning: could not delete branch %s: %v", c.workUnit.Branch, err)
		}
	}

	// Remove git worktree if still present (cleanup safety net)
	if c.workUnit != nil && c.workUnit.WorktreePath != "" && c.git != nil {
		if err := c.git.RemoveWorktree(ctx, c.workUnit.WorktreePath, true); err != nil {
			c.logVerbosef("Warning: could not remove worktree %s: %v", c.workUnit.WorktreePath, err)
		}
	}

	// Delete persisted task state so it is not restored on next socket start.
	if c.store != nil && c.workUnit != nil {
		if err := c.store.DeleteTaskState(c.workUnit.ID); err != nil {
			slog.Warn("delete task state failed", "task_id", c.workUnit.ID, "error", err)
		}
	}

	// Reset state and clear work unit
	c.machine.Reset()
	c.workUnit = nil

	c.emit(ConductorEvent{
		Type:    "task_deleted",
		State:   StateNone,
		Message: "Task deleted",
	})

	return nil
}

// parseOwnerRepo extracts owner and repo from an external ID like "owner/repo#123".
// Returns empty strings if the format is not recognized.
func parseOwnerRepo(externalID string) (string, string) {
	// Format: owner/repo#number
	repoPath, _, ok := strings.Cut(externalID, "#")
	if !ok {
		return "", ""
	}
	parts := strings.SplitN(repoPath, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}

	return parts[0], parts[1]
}

// detectPRTemplate searches for PR templates in well-known locations within the repo.
// Returns empty string when no template is found.
func detectPRTemplate(repoPath string) string {
	candidates := []string{
		".github/PULL_REQUEST_TEMPLATE.md",
		".github/pull_request_template.md",
		".github/PULL_REQUEST_TEMPLATE",
		".gitlab/merge_request_templates/Default.md",
		"PULL_REQUEST_TEMPLATE.md",
		"pull_request_template.md",
		"docs/pull_request_template.md",
	}
	for _, candidate := range candidates {
		path := filepath.Join(repoPath, candidate)
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
	}

	return ""
}

// fillPRTemplate attempts to auto-fill sections in a PR template using task metadata.
// Returns the filled template. Sections that can't be filled are left with placeholder markers.
func fillPRTemplate(template, description string, checkpointCount int, sourceURL string) string {
	lines := strings.Split(template, "\n")
	var result []string

	for i, line := range lines {
		result = append(result, line)
		trimmed := strings.TrimSpace(strings.ToLower(line))

		// Auto-fill known sections
		switch {
		case strings.Contains(trimmed, "summary") || strings.Contains(trimmed, "what changed") || strings.Contains(trimmed, "description"):
			// Insert summary after header if next line is empty or a placeholder
			if i+1 < len(lines) && (strings.TrimSpace(lines[i+1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[i+1]), "<!--")) {
				result = append(result, "", description)
			}
		case strings.Contains(trimmed, "test") || strings.Contains(trimmed, "how to test"):
			if i+1 < len(lines) && (strings.TrimSpace(lines[i+1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[i+1]), "<!--")) {
				result = append(result, "", "- [ ] Verify implementation matches specification", fmt.Sprintf("- [ ] %d checkpoint(s) reviewed", checkpointCount))
			}
		case strings.Contains(trimmed, "related") || strings.Contains(trimmed, "ticket") || strings.Contains(trimmed, "issue"):
			if sourceURL != "" && i+1 < len(lines) && (strings.TrimSpace(lines[i+1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[i+1]), "<!--")) {
				result = append(result, "", sourceURL)
			}
		}
	}

	return strings.Join(result, "\n")
}

// prContext holds spec, review, and recording data for PR body generation.
type prContext struct {
	SpecContent   string
	ReviewSummary string
	Recordings    []map[string]any
}

// buildPRDescription constructs the PR body from task metadata.
// Takes explicit parameters so it can be called outside the lock with copied values.
func buildPRDescription(description string, specCount, checkpointCount int, diffStat string, fileStatuses []git.FileStatus, customSections []settings.PRSection, ctx *prContext) string {
	desc := fmt.Sprintf("## Summary\n\n%s\n", description)

	if len(fileStatuses) > 0 {
		desc += "\n## Changes\n\n"
		var descSb498 strings.Builder
		for _, fs := range fileStatuses {
			var icon string
			switch fs.Status {
			case "added":
				icon = "+"
			case "deleted":
				icon = "-"
			case "modified":
				icon = "~"
			case "renamed":
				icon = ">"
			default:
				icon = "?"
			}
			fmt.Fprintf(&descSb498, "- `%s` %s\n", icon, fs.Path)
		}
		desc += descSb498.String()
	}

	if diffStat != "" {
		desc += "\n<details>\n<summary>Diff stats</summary>\n\n```\n" + strings.TrimSpace(diffStat) + "\n```\n</details>\n"
	}

	if specCount > 0 {
		specContent := ""
		if ctx != nil {
			specContent = ctx.SpecContent
		}
		if specContent != "" {
			// Truncate long specs to keep PR readable (rune-safe)
			const maxSpecLen = 2000
			runes := []rune(specContent)
			truncated := specContent
			if len(runes) > maxSpecLen {
				truncated = string(runes[:maxSpecLen]) + "\n\n…(truncated)"
			}
			// Sanitize closing tags that would break the <details> wrapper
			truncated = strings.ReplaceAll(truncated, "</details>", "&lt;/details&gt;")
			desc += "\n## Specification\n\n<details>\n<summary>Implementation specification</summary>\n\n" + truncated + "\n\n</details>\n"
		} else {
			desc += "\n## Implementation\n\nImplemented according to kvelmo specifications.\n"
		}
	}

	if ctx != nil && ctx.ReviewSummary != "" {
		desc += "\n## Review\n\n" + ctx.ReviewSummary + "\n"
	}

	if checkpointCount > 0 {
		desc += fmt.Sprintf("\n## Checkpoints\n\n%d checkpoint(s) created during development.\n", checkpointCount)
	}

	var descSb532 strings.Builder
	for _, section := range customSections {
		fmt.Fprintf(&descSb532, "\n## %s\n\n%s\n", section.Title, section.Content)
	}
	desc += descSb532.String()

	desc += "\n---\n*Generated by [kvelmo](https://github.com/valksor/kvelmo)*"

	return desc
}

// buildPRDescriptionWithDecisions extends the PR body with AI decision context.
func buildPRDescriptionWithDecisions(description string, specCount, checkpointCount int, diffStat string, fileStatuses []git.FileStatus, customSections []settings.PRSection, ctx *prContext) string {
	base := buildPRDescription(description, specCount, checkpointCount, diffStat, fileStatuses, customSections, ctx)

	var recordings []map[string]any
	if ctx != nil {
		recordings = ctx.Recordings
	}
	decisions := changeset.ExtractDecisions(recordings)
	if len(decisions) == 0 {
		return base
	}

	aiSection := changeset.FormatMarkdown(decisions, diffStat)

	return base + "\n" + aiSection
}

// loadRecordingsFlat reads recording JSONL files from phase metrics and flattens
// them into the format expected by changeset.ExtractDecisions.
// Uses streaming reader to avoid loading entire files into memory.
// Caps at maxRecordsPerFile records per file and maxTotalRecords total.
func loadRecordingsFlat(phaseMetrics map[string]*PhaseMetrics) []map[string]any {
	const maxRecordsPerFile = 5000
	const maxTotalRecords = 10000

	var flat []map[string]any

	for _, pm := range phaseMetrics {
		if pm == nil || pm.RecordingPath == "" {
			continue
		}

		r, err := recorder.OpenReader(pm.RecordingPath)
		if err != nil {
			slog.Debug("could not open recording for PR", "path", pm.RecordingPath, "error", err)

			continue
		}

		count := 0
		for count < maxRecordsPerFile && len(flat) < maxTotalRecords {
			rec, err := r.Next()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					slog.Debug("error reading recording", "path", pm.RecordingPath, "error", err)
				}

				break
			}
			if rec == nil {
				break
			}

			entry := map[string]any{
				"type": rec.Type,
			}

			// Unmarshal Event to extract tool name and file info
			var event map[string]any
			if err := json.Unmarshal(rec.Event, &event); err == nil {
				if content, ok := event["content"].(string); ok {
					entry["tool"] = content
				}
				if data, ok := event["data"].(map[string]any); ok {
					if fp, ok := data["file_path"].(string); ok {
						entry["file"] = fp
					}
				}
			}

			flat = append(flat, entry)
			count++
		}
		if err := r.Close(); err != nil {
			slog.Debug("could not close recording reader", "path", pm.RecordingPath, "error", err)
		}
	}

	return flat
}

// buildReviewSummary constructs a review summary string from work unit state.
func buildReviewSummary(store *storage.Store, taskID string, checklistChecked []string, qualityGatePassed *bool) string {
	var parts []string

	// Quality gate status
	if qualityGatePassed != nil {
		if *qualityGatePassed {
			parts = append(parts, "Quality gate: passed")
		} else {
			parts = append(parts, "Quality gate: failed")
		}
	}

	// Checklist items
	if len(checklistChecked) > 0 {
		parts = append(parts, fmt.Sprintf("Checklist: %d item(s) checked", len(checklistChecked)))
	}

	// Latest review status
	if store != nil && taskID != "" {
		reviStore := storage.NewReviewStore(store)
		if review, err := reviStore.GetLatestReview(taskID); err == nil && review != nil {
			status := review.Status
			if status == "" {
				status = "pending"
			}
			parts = append(parts, "Review status: "+status)
		}
	}

	return strings.Join(parts, "\n")
}

// validatePRSections checks that all required sections in the PR body contain
// meaningful content (not just placeholder text or empty lines).
// Returns the list of section keywords that are missing content.
func validatePRSections(body string, requiredSections []string) []string {
	lines := strings.Split(body, "\n")
	var missing []string

	for _, section := range requiredSections {
		sectionLower := strings.ToLower(section)
		found := false
		hasContent := false

		for i, line := range lines {
			trimmed := strings.TrimSpace(strings.ToLower(line))
			// Check if this line is a header containing the section keyword
			if !strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, sectionLower) {
				continue
			}
			found = true
			// Check lines after header for real content
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				// Stop at next header
				if strings.HasPrefix(nextLine, "#") {
					break
				}
				// Skip empty lines and HTML comments (placeholders)
				if nextLine == "" || strings.HasPrefix(nextLine, "<!--") {
					continue
				}
				hasContent = true

				break
			}

			break
		}

		if !found || !hasContent {
			missing = append(missing, section)
		}
	}

	return missing
}
