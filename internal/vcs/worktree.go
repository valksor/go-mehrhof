package vcs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Worktree represents a git worktree.
type Worktree struct {
	Path   string // Absolute path to worktree
	Branch string // Branch checked out in worktree
	Commit string // HEAD commit
	Bare   bool   // Is this the bare repository
	Main   bool   // Is this the main worktree
}

// ListWorktrees returns all worktrees in the repository.
func (g *Git) ListWorktrees(ctx context.Context) ([]Worktree, error) {
	out, err := g.run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	var worktrees []Worktree
	var current Worktree

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = Worktree{}
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "HEAD ") {
			current.Commit = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") {
			// Branch is refs/heads/name
			branch := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
		} else if line == "bare" {
			current.Bare = true
		}
	}

	// Add last worktree if present
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	// Mark main worktree
	if len(worktrees) > 0 {
		worktrees[0].Main = true
	}

	return worktrees, nil
}

// CreateWorktree creates a new worktree for a branch.
func (g *Git) CreateWorktree(ctx context.Context, path, branch string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	// Ensure parent directory exists
	parent := filepath.Dir(absPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	_, err = g.run(ctx, "worktree", "add", absPath, branch)
	if err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	return nil
}

// CreateWorktreeNewBranch creates a worktree with a new branch.
func (g *Git) CreateWorktreeNewBranch(ctx context.Context, path, branch, base string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	// Ensure parent directory exists
	parent := filepath.Dir(absPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	args := []string{"worktree", "add", "-b", branch, absPath}
	if base != "" {
		args = append(args, base)
	}

	_, err = g.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("create worktree with new branch: %w", err)
	}

	return nil
}

// RemoveWorktree removes a worktree.
func (g *Git) RemoveWorktree(ctx context.Context, path string, force bool) error {
	args := []string{"worktree", "remove", path}
	if force {
		args = []string{"worktree", "remove", "--force", path}
	}

	_, err := g.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}

	return nil
}

// PruneWorktrees removes stale worktree information.
func (g *Git) PruneWorktrees(ctx context.Context) error {
	_, err := g.run(ctx, "worktree", "prune")
	return err
}

// GetWorktreeForBranch finds the worktree for a given branch.
func (g *Git) GetWorktreeForBranch(ctx context.Context, branch string) (*Worktree, error) {
	worktrees, err := g.ListWorktrees(ctx)
	if err != nil {
		return nil, err
	}

	for _, wt := range worktrees {
		if wt.Branch == branch {
			return &wt, nil
		}
	}

	return nil, fmt.Errorf("no worktree for branch: %s", branch)
}

// WorktreeExists checks if a worktree exists at the given path.
func (g *Git) WorktreeExists(ctx context.Context, path string) bool {
	worktrees, err := g.ListWorktrees(ctx)
	if err != nil {
		return false
	}

	absPath, _ := filepath.Abs(path)
	for _, wt := range worktrees {
		if wt.Path == absPath {
			return true
		}
	}

	return false
}

// GetWorktreePath returns a standard worktree path for a task
// Worktrees are created as siblings of the main repo: ../repo-worktrees/task-id.
func (g *Git) GetWorktreePath(taskID string) string {
	repoName := filepath.Base(g.repoRoot)
	parent := filepath.Dir(g.repoRoot)
	return filepath.Join(parent, repoName+"-worktrees", taskID)
}

// EnsureWorktreesDir creates the worktrees directory if it doesn't exist.
func (g *Git) EnsureWorktreesDir() error {
	repoName := filepath.Base(g.repoRoot)
	parent := filepath.Dir(g.repoRoot)
	worktreesDir := filepath.Join(parent, repoName+"-worktrees")
	return os.MkdirAll(worktreesDir, 0o755)
}
