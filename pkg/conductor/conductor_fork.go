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

	"github.com/google/uuid"
)

// ForkInfo describes one fork of a task.
type ForkInfo struct {
	ID            string    `json:"id"`
	Label         string    `json:"label"`
	Branch        string    `json:"branch"`
	WorktreeDir   string    `json:"worktree_dir"`
	CheckpointSHA string    `json:"checkpoint_sha"`
	State         string    `json:"state"` // "active", "completed", "failed"
	TokensUsed    int64     `json:"tokens_used,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Fork creates a new parallel branch from the current checkpoint.
// It creates a new git worktree with a unique branch and returns the fork info.
// Must be called without c.mu held.
func (c *Conductor) Fork(ctx context.Context, label string) (*ForkInfo, error) {
	if err := c.validateForkingEnabled(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	wu := c.workUnit
	repo := c.git
	c.mu.Unlock()

	if wu == nil {
		return nil, errors.New("fork: no active task")
	}

	if repo == nil {
		return nil, errors.New("fork: git not available")
	}

	maxForks := c.maxForks()

	c.mu.RLock()
	currentForkCount := len(wu.Forks)
	c.mu.RUnlock()

	if currentForkCount >= maxForks {
		return nil, fmt.Errorf("fork: maximum forks reached (%d/%d)", currentForkCount, maxForks)
	}

	// Get current checkpoint SHA
	sha, err := repo.CurrentCommit(ctx)
	if err != nil {
		return nil, fmt.Errorf("fork: get current commit: %w", err)
	}

	// Sanitize label for branch name
	safeLabel := sanitizeBranchLabel(label)

	forkID := uuid.New().String()[:8]
	branchName := fmt.Sprintf("kvelmo-fork/%s/%s", wu.ID, safeLabel)
	worktreeDir := filepath.Join(filepath.Dir(repo.Path()), ".kvelmo-worktrees", fmt.Sprintf("%s-fork-%s", wu.ID, safeLabel))

	// Create worktree with new branch from current SHA
	if err := repo.AddWorktree(ctx, worktreeDir, branchName, true, sha); err != nil {
		return nil, fmt.Errorf("fork: create worktree: %w", err)
	}

	info := ForkInfo{
		ID:            forkID,
		Label:         label,
		Branch:        branchName,
		WorktreeDir:   worktreeDir,
		CheckpointSHA: sha,
		State:         "active",
		CreatedAt:     time.Now(),
	}

	c.mu.Lock()
	c.workUnit.Forks = append(c.workUnit.Forks, info)
	c.workUnit.UpdatedAt = time.Now()
	c.persistState()
	c.mu.Unlock()

	slog.Info("fork created", "fork_id", forkID, "label", label, "branch", branchName)

	data, err := json.Marshal(info)
	if err != nil {
		slog.Warn("failed to marshal fork info", "error", err)
	}
	c.emit(ConductorEvent{
		Type:    "fork_created",
		Message: "Fork created: " + label,
		Data:    data,
	})

	return &info, nil
}

// ListForks returns all active forks for the current task.
func (c *Conductor) ListForks() []ForkInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.workUnit == nil {
		return nil
	}

	// Return a copy to avoid race conditions
	forks := make([]ForkInfo, len(c.workUnit.Forks))
	copy(forks, c.workUnit.Forks)

	return forks
}

// SelectFork merges the winning fork back into the main branch and cleans up others.
// Must be called without c.mu held.
func (c *Conductor) SelectFork(ctx context.Context, forkID string) error {
	c.mu.RLock()
	wu := c.workUnit
	repo := c.git
	c.mu.RUnlock()

	if wu == nil {
		return errors.New("select fork: no active task")
	}

	if repo == nil {
		return errors.New("select fork: git not available")
	}

	// Find the winning fork
	var winner *ForkInfo
	for i := range wu.Forks {
		if wu.Forks[i].ID == forkID {
			winner = &wu.Forks[i]

			break
		}
	}

	if winner == nil {
		return fmt.Errorf("select fork: fork %q not found", forkID)
	}

	// Merge the fork branch into the current task branch
	message := fmt.Sprintf("Merge fork %q into task branch", winner.Label)
	if err := repo.Merge(ctx, winner.Branch, message); err != nil {
		return fmt.Errorf("select fork: merge: %w", err)
	}

	// Clean up all fork worktrees and branches except the winner (already merged).
	for _, fork := range wu.Forks {
		if fork.ID == forkID {
			continue
		}
		if removeErr := repo.RemoveWorktree(ctx, fork.WorktreeDir, true); removeErr != nil {
			slog.Warn("failed to remove fork worktree", "fork_id", fork.ID, "error", removeErr)
		}
		if delErr := repo.DeleteBranch(ctx, fork.Branch); delErr != nil {
			slog.Warn("failed to delete fork branch", "fork_id", fork.ID, "error", delErr)
		}
	}

	// Prune stale worktree refs
	if err := repo.PruneWorktrees(ctx); err != nil {
		slog.Warn("failed to prune worktrees", "error", err)
	}

	slog.Info("fork selected", "fork_id", forkID, "label", winner.Label)

	c.mu.Lock()
	c.workUnit.Forks = nil
	c.workUnit.UpdatedAt = time.Now()
	c.persistState()
	c.mu.Unlock()

	data, err := json.Marshal(map[string]string{"fork_id": forkID, "label": winner.Label})
	if err != nil {
		slog.Warn("failed to marshal fork selection", "error", err)
	}
	c.emit(ConductorEvent{
		Type:    "fork_selected",
		Message: "Fork selected: " + winner.Label,
		Data:    data,
	})

	return nil
}

// validateForkingEnabled checks settings to ensure forking is allowed.
func (c *Conductor) validateForkingEnabled() error {
	s := c.getEffectiveSettings()
	if s == nil {
		return errors.New("fork: settings not available")
	}

	if s.Workflow.Forking == nil || !s.Workflow.Forking.Enabled {
		return errors.New("fork: forking is disabled (enable in workflow.forking.enabled)")
	}

	return nil
}

// maxForks returns the configured maximum number of forks, defaulting to 3.
func (c *Conductor) maxForks() int {
	s := c.getEffectiveSettings()
	if s != nil && s.Workflow.Forking != nil && s.Workflow.Forking.MaxForks > 0 {
		return s.Workflow.Forking.MaxForks
	}

	return 3
}

// sanitizeBranchLabel replaces characters not allowed in git branch names.
func sanitizeBranchLabel(label string) string {
	replacer := strings.NewReplacer(
		" ", "-",
		"/", "-",
		"..", "-",
		"~", "-",
		"^", "-",
		":", "-",
		"?", "-",
		"*", "-",
		"[", "-",
		"\\", "-",
	)

	return strings.ToLower(replacer.Replace(label))
}
