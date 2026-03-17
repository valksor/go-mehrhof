package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/changelog"
	"github.com/valksor/kvelmo/pkg/changeset"
	"github.com/valksor/kvelmo/pkg/memory"
	"github.com/valksor/kvelmo/pkg/provider"
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

	// Check approval requirement
	if err := c.checkApproval(EventSubmit); err != nil {
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

	// Validate PR template required sections (checked early to fail fast)
	prRequiredSections := settings.Git.PRRequiredSections

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
	} else {
		// No cached result - run synchronously (Review was skipped or old state)
		slog.Info("submit: running quality gate synchronously")
		if err := c.runQualityGate(ctx); err != nil {
			c.mu.Unlock()

			return fmt.Errorf("quality gate failed: %w", err)
		}
		slog.Info("submit: quality gate passed (sync)")
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
	specCount := len(c.workUnit.Specifications)
	checkpointCount := len(c.workUnit.Checkpoints)
	var sourceProvider, sourceURL string
	if c.workUnit.Source != nil {
		sourceProvider = c.workUnit.Source.Provider
		sourceURL = c.workUnit.Source.URL
	}
	changelogPath := settings.Storage.ChangelogPath
	git := c.git
	providers := c.providers
	memoryIndexer := c.memoryIndexer
	lifecycleCtx := c.lifecycleCtx
	shouldComment := c.shouldPostTicketComment()
	c.mu.Unlock()

	// Phase 2.5: Auto-generate changelog entry if configured (before push)
	if changelogPath != "" && git != nil {
		workDir := worktreePath
		if workDir == "" {
			workDir = git.Path()
		}
		fullPath := filepath.Join(workDir, changelogPath)
		if err := changelog.AppendEntry(fullPath, changelog.Entry{
			Date:   time.Now(),
			Title:  title,
			TaskID: externalID,
		}); err != nil {
			slog.Warn("changelog update failed", "error", err)
		} else {
			if err := git.StageAll(ctx); err == nil {
				if _, err := git.Commit(ctx, "Update changelog for "+title); err != nil {
					slog.Warn("changelog commit failed", "error", err)
				}
			}
		}
	}

	// Phase 3: Network operations (no lock held)
	slog.Info("submit: starting network operations", "branch", branch, "has_git", git != nil)
	var prURL, prID string
	if git != nil && branch != "" {
		slog.Info("submit: pushing branch", "branch", branch)
		if err := git.Push(ctx, "origin", branch); err != nil {
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
					repoPath := git.Path()
					if tmpl := detectPRTemplate(repoPath); tmpl != "" {
						prBody = fillPRTemplate(tmpl, workUnitDescription, checkpointCount, sourceURL)
					} else {
						prBody = buildPRDescriptionWithDecisions(workUnitDescription, specCount, checkpointCount, "", nil)
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
					if result, err := sp.CreatePR(ctx, prOpts); err == nil {
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
			if err := git.DeleteBranch(ctx, branch); err != nil {
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
	if worktreePath != "" && git != nil {
		if err := git.RemoveWorktree(ctx, worktreePath, false); err != nil {
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
	hashIdx := strings.Index(externalID, "#")
	if hashIdx < 0 {
		return "", ""
	}
	repoPath := externalID[:hashIdx]
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

// buildPRDescription constructs the PR body from task metadata.
// Takes explicit parameters so it can be called outside the lock with copied values.
func buildPRDescription(description string, specCount, checkpointCount int) string {
	desc := fmt.Sprintf("## Summary\n\n%s\n", description)

	if specCount > 0 {
		desc += "\n## Implementation\n\nImplemented according to kvelmo specifications.\n"
	}

	if checkpointCount > 0 {
		desc += fmt.Sprintf("\n## Checkpoints\n\n%d checkpoint(s) created during development.\n", checkpointCount)
	}

	desc += "\n---\n*Generated by [kvelmo](https://github.com/valksor/kvelmo)*"

	return desc
}

// buildPRDescriptionWithDecisions extends the PR body with AI decision context.
func buildPRDescriptionWithDecisions(description string, specCount, checkpointCount int, diffStat string, recordings []map[string]any) string {
	base := buildPRDescription(description, specCount, checkpointCount)

	decisions := changeset.ExtractDecisions(recordings)
	if len(decisions) == 0 {
		return base
	}

	aiSection := changeset.FormatMarkdown(decisions, diffStat)

	return base + "\n" + aiSection
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
