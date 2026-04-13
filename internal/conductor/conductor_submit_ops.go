package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/settings"
)

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
