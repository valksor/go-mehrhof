package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/valksor/kvelmo/internal/changelog"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/memory"
	"github.com/valksor/kvelmo/internal/policy"
	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/internal/storage"
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
	if checklist := settings.Workflow.Policy.ReviewChecklist; len(checklist) > 0 {
		if err := c.checkReviewChecklist(checklist); err != nil {
			c.mu.Unlock()

			return err
		}
	}

	// Check required phases policy.
	// On re-submit, only check phases completed since the last submit (not cumulative history).
	if requiredPhases := settings.Workflow.Policy.RequiredPhases; len(requiredPhases) > 0 {
		if err := c.checkRequiredPhases(requiredPhases); err != nil {
			c.mu.Unlock()

			return err
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
	} else if c.qualityGateCh != nil {
		// Another goroutine is already running the quality gate; wait for its result.
		ch := c.qualityGateCh
		c.mu.Unlock()
		<-ch
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
		// Create a channel so concurrent callers can wait on this run.
		c.qualityGateCh = make(chan struct{})
		slog.Info("submit: running quality gate synchronously")
		c.mu.Unlock()
		err := c.runQualityGate(ctx)
		c.mu.Lock()
		close(c.qualityGateCh)
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
		if err := c.checkSensitivePaths(sensitivePaths, changedFiles); err != nil {
			c.mu.Unlock()

			return err
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
