package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// FinishOptions configures the finish operation.
type FinishOptions struct {
	// DeleteRemoteBranch deletes the feature branch from remote after cleanup.
	DeleteRemoteBranch bool
	// Force finishes even if PR is not merged.
	Force bool
}

// FinishResult contains the result of the finish operation.
type FinishResult struct {
	// PreviousBranch is the feature branch that was cleaned up.
	PreviousBranch string
	// CurrentBranch is the branch we switched to (usually main/master).
	CurrentBranch string
	// BranchDeleted indicates if the local feature branch was deleted.
	BranchDeleted bool
	// RemoteBranchDeleted indicates if the remote feature branch was deleted.
	RemoteBranchDeleted bool
}

// Finish cleans up after a PR is merged.
// This performs the following steps:
// 1. Optionally check if PR is merged
// 2. Switch to base branch (main/master)
// 3. Pull latest changes
// 4. Delete local feature branch
// 5. Optionally delete remote feature branch
// 6. Clear work unit and reset state to None.
func (c *Conductor) Finish(ctx context.Context, opts FinishOptions) (*FinishResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return nil, errors.New("no task loaded")
	}

	// Check we're in submitted state (or force)
	if c.machine.State() != StateSubmitted && !opts.Force {
		return nil, fmt.Errorf("cannot finish: task is %s (expected submitted). Use --force to override", c.machine.State())
	}

	// Extract and store learnings from this task before cleanup.
	c.extractLearnings(ctx)

	// Link outcome data to memory documents for this task.
	c.linkTaskOutcome(ctx)

	if c.git == nil {
		return nil, errors.New("git not available")
	}

	result := &FinishResult{
		PreviousBranch: c.workUnit.Branch,
	}

	// Get base branch
	baseBranch, err := c.getBaseBranch(ctx)
	if err != nil {
		return nil, fmt.Errorf("get base branch: %w", err)
	}
	result.CurrentBranch = baseBranch

	// Switch to base branch
	c.logVerbosef("Switching to %s...", baseBranch)
	if err := c.git.Checkout(ctx, baseBranch); err != nil {
		return nil, fmt.Errorf("checkout %s: %w", baseBranch, err)
	}

	// Pull latest
	c.logVerbosef("Pulling latest changes...")
	if err := c.git.Pull(ctx); err != nil {
		// Pull might fail if there are local changes; log but continue
		c.logVerbosef("Warning: pull failed: %v", err)
	}

	// Delete local feature branch
	if c.workUnit.Branch != "" && c.workUnit.Branch != baseBranch {
		c.logVerbosef("Deleting local branch %s...", c.workUnit.Branch)
		if err := c.git.DeleteBranch(ctx, c.workUnit.Branch); err != nil {
			c.logVerbosef("Warning: failed to delete local branch: %v", err)
		} else {
			result.BranchDeleted = true
		}
	}

	// Optionally delete remote branch (with same guard as local deletion)
	if opts.DeleteRemoteBranch && c.workUnit.Branch != "" && c.workUnit.Branch != baseBranch {
		c.logVerbosef("Deleting remote branch %s...", c.workUnit.Branch)
		if err := c.git.DeleteRemoteBranch(ctx, c.workUnit.Branch); err != nil {
			c.logVerbosef("Warning: failed to delete remote branch: %v", err)
		} else {
			result.RemoteBranchDeleted = true
		}
	}

	// Transition state to None
	if err := c.machine.Dispatch(ctx, EventFinish); err != nil {
		// Git cleanup done, state dispatch failed - force reset and log
		slog.Warn("state dispatch failed after cleanup, forcing reset", "error", err)
		c.machine.Reset()
	}

	// Archive the completed task before clearing
	c.archiveTask("finished")

	// Emit finish event after state transition (so event reflects new state)
	c.emit(ConductorEvent{
		Type:    "task_finished",
		State:   c.machine.State(),
		Message: "Task finished: " + c.workUnit.Title,
		Data: mustMarshalJSON(map[string]any{
			"task_id":        c.workUnit.ID,
			"branch":         c.workUnit.Branch,
			"branch_deleted": result.BranchDeleted,
		}),
	})

	// Clear work unit
	c.workUnit = nil
	c.machine.SetWorkUnit(nil)
	c.persistState()

	// Auto-advance: if there's a queued task, start it in the background
	if next := c.popNextTask(); next != nil {
		c.emit(ConductorEvent{
			Type:    "queue_advancing",
			State:   c.machine.State(),
			Message: "Loading next queued task: " + next.Source,
		})
		// Start the next task asynchronously using lifecycle context.
		// We can't call Start() here because we already hold c.mu.
		go func(source string) {
			if err := c.Start(c.lifecycleCtx, source); err != nil {
				slog.Warn("auto-advance failed", "source", source, "error", err)
				c.emit(ConductorEvent{
					Type:    "queue_advance_failed",
					State:   c.machine.State(),
					Message: "Failed to load next task: " + err.Error(),
				})
			}
		}(next.Source)
	}

	return result, nil
}

// Refresh checks the PR status and returns information about the current state.
// If the PR is merged, it suggests running finish to clean up.
// If the PR is still open, it reports how many commits behind base the branch is.
func (c *Conductor) Refresh(ctx context.Context) (*RefreshResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return nil, errors.New("no task loaded")
	}

	result := &RefreshResult{
		TaskID: c.workUnit.ID,
		Branch: c.workUnit.Branch,
	}

	// Check PR status if we have a provider
	if c.providers != nil && c.workUnit.Source != nil && c.workUnit.Source.Provider != "" {
		pr, err := c.providers.GetPRStatus(ctx, c.workUnit.Source.Reference)
		if err != nil {
			c.logVerbosef("Warning: could not check PR status: %v", err)
		} else {
			result.PRStatus = pr.State
			result.PRMerged = pr.Merged
			result.PRURL = pr.URL
		}
	}

	// If PR is merged, suggest finishing
	if result.PRMerged {
		result.Action = "merged"
		result.Message = "PR has been merged. Run 'kvelmo finish' to clean up."
		result.RefreshedAt = time.Now()

		return result, nil
	}

	// Pull latest from base branch to keep feature branch up to date
	var baseBranch string
	if c.git != nil {
		var err error
		baseBranch, err = c.getBaseBranch(ctx)
		if err == nil {
			// Fetch latest
			if err := c.git.Fetch(ctx); err != nil {
				c.logVerbosef("Warning: fetch failed: %v", err)
			}

			// Check if base branch has new commits
			behind, err := c.git.CommitsBehind(ctx, "origin/"+baseBranch)
			if err == nil {
				result.CommitsBehindBase = behind
			}
		}
	}

	if result.PRStatus == "open" {
		result.Action = "waiting"
		if result.CommitsBehindBase > 0 {
			if baseBranch != "" {
				result.Message = fmt.Sprintf("PR is open. %d commits behind %s - consider rebasing.",
					result.CommitsBehindBase, baseBranch)
			} else {
				result.Message = fmt.Sprintf("PR is open. %d commits behind base - consider rebasing.",
					result.CommitsBehindBase)
			}
		} else {
			result.Message = "PR is open and up to date."
		}
	} else if result.PRStatus == "closed" && !result.PRMerged {
		result.Action = "closed"
		result.Message = "PR was closed without merging. Use 'kvelmo finish --force' to clean up."
	} else {
		result.Action = "unknown"
		result.Message = "Could not determine PR status."
	}

	result.RefreshedAt = time.Now()

	return result, nil
}

// RefreshResult contains the result of a refresh operation.
type RefreshResult struct {
	TaskID            string    `json:"task_id"`
	Branch            string    `json:"branch"`
	PRStatus          string    `json:"pr_status,omitempty"` // "open", "closed", "merged"
	PRMerged          bool      `json:"pr_merged"`
	PRURL             string    `json:"pr_url,omitempty"`
	CommitsBehindBase int       `json:"commits_behind_base,omitempty"`
	Action            string    `json:"action"` // "merged", "waiting", "closed", "unknown"
	Message           string    `json:"message"`
	RefreshedAt       time.Time `json:"refreshed_at"`
}

// ApprovePR approves the PR/MR for the current work unit.
// This is typically called after reviewing the submitted PR.
func (c *Conductor) ApprovePR(ctx context.Context, comment string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return errors.New("no task loaded")
	}

	if c.machine.State() != StateSubmitted {
		return fmt.Errorf("cannot approve: task is %s (expected submitted)", c.machine.State())
	}

	if c.providers == nil || c.workUnit.Source == nil {
		return errors.New("no provider configured")
	}

	// Get provider and check if it supports merge operations
	provider, err := c.providers.Get(c.workUnit.Source.Provider)
	if err != nil {
		return fmt.Errorf("get provider: %w", err)
	}

	// Check if provider implements MergeProvider
	type mergeProvider interface {
		ApprovePR(ctx context.Context, taskID string, comment string) error
	}

	mp, ok := provider.(mergeProvider)
	if !ok {
		return fmt.Errorf("provider %s does not support PR approval", c.workUnit.Source.Provider)
	}

	// Get PR ID from work unit (should be stored after submit)
	prID := c.workUnit.PRID
	if prID == "" {
		return errors.New("no PR ID found (task may not have been submitted yet)")
	}

	if err := mp.ApprovePR(ctx, prID, comment); err != nil {
		return fmt.Errorf("approve PR: %w", err)
	}

	c.emit(ConductorEvent{
		Type:    "pr_approved",
		State:   c.machine.State(),
		Message: "PR approved: " + prID,
		Data: mustMarshalJSON(map[string]any{
			"pr_id":   prID,
			"comment": comment,
		}),
	})

	return nil
}

// MergePR merges the PR/MR for the current work unit.
// Method should be one of: "merge", "squash", "rebase" (default: "rebase").
func (c *Conductor) MergePR(ctx context.Context, method string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return errors.New("no task loaded")
	}

	if c.machine.State() != StateSubmitted {
		return fmt.Errorf("cannot merge: task is %s (expected submitted)", c.machine.State())
	}

	if c.providers == nil || c.workUnit.Source == nil {
		return errors.New("no provider configured")
	}

	// Get provider and check if it supports merge operations
	provider, err := c.providers.Get(c.workUnit.Source.Provider)
	if err != nil {
		return fmt.Errorf("get provider: %w", err)
	}

	// Check if provider implements MergeProvider
	type mergeProvider interface {
		MergePR(ctx context.Context, taskID string, method string) error
	}

	mp, ok := provider.(mergeProvider)
	if !ok {
		return fmt.Errorf("provider %s does not support PR merging", c.workUnit.Source.Provider)
	}

	// Get PR ID from work unit
	prID := c.workUnit.PRID
	if prID == "" {
		return errors.New("no PR ID found (task may not have been submitted yet)")
	}

	// Default to rebase
	if method == "" {
		method = "rebase"
	}

	if err := mp.MergePR(ctx, prID, method); err != nil {
		return fmt.Errorf("merge PR: %w", err)
	}

	c.emit(ConductorEvent{
		Type:    "pr_merged",
		State:   c.machine.State(),
		Message: "PR merged: " + prID,
		Data: mustMarshalJSON(map[string]any{
			"pr_id":  prID,
			"method": method,
		}),
	})

	return nil
}
