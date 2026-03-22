package git

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Worktree struct {
	Path   string
	Branch string
	Bare   bool
}

func (r *Repository) ListWorktrees(ctx context.Context) ([]Worktree, error) {
	out, err := r.run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var worktrees []Worktree
	var current Worktree

	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = Worktree{}
			}

			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		} else if line == "bare" {
			current.Bare = true
		}
	}

	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

func (r *Repository) AddWorktree(ctx context.Context, path, branch string, create bool, startPoint string) error {
	args := []string{"worktree", "add"}
	if create {
		args = append(args, "-b", branch)
	}
	args = append(args, path)
	if !create {
		args = append(args, branch)
	}
	if create && startPoint != "" {
		args = append(args, startPoint)
	}

	_, err := r.run(ctx, args...)

	return err
}

func (r *Repository) RemoveWorktree(ctx context.Context, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)

	_, err := r.run(ctx, args...)

	return err
}

// CreateTaskBranch creates a kvelmo/<taskID> branch from startPoint (or HEAD
// if empty). If the branch already exists, it switches to it and startPoint
// is ignored — the existing branch history is preserved.
func (r *Repository) CreateTaskBranch(ctx context.Context, taskID, startPoint string) (string, error) {
	branchName := "kvelmo/" + taskID

	// Check if branch exists — if so, reattach to it as-is.
	// The startPoint is intentionally ignored here: the branch already has
	// its own history and we don't want to reset it to a different root.
	_, err := r.run(ctx, "rev-parse", "--verify", branchName)
	if err == nil {
		return branchName, r.SwitchBranch(ctx, branchName)
	}

	// Create new branch from start point (or HEAD if empty)
	return branchName, r.CreateBranch(ctx, branchName, startPoint)
}

func (r *Repository) CreateTaskWorktree(ctx context.Context, taskID, basePath, startPoint string) (*Worktree, error) {
	branchName := "kvelmo/" + taskID
	worktreePath := filepath.Join(basePath, taskID)

	// Remove existing worktree if it exists
	if _, err := os.Stat(worktreePath); err == nil {
		_ = r.RemoveWorktree(ctx, worktreePath, true)
	}

	if err := r.AddWorktree(ctx, worktreePath, branchName, true, startPoint); err != nil {
		return nil, fmt.Errorf("add worktree: %w", err)
	}

	// Inject branch guardrails to prevent accidental branch switches
	if err := injectBranchGuardrails(worktreePath, branchName); err != nil {
		// Non-fatal: log and continue
		slog.Warn("failed to inject branch guardrails", "path", worktreePath, "error", err)
	}

	return &Worktree{
		Path:   worktreePath,
		Branch: branchName,
	}, nil
}

// injectBranchGuardrails writes branch protection rules to .claude/CLAUDE.md
// in the worktree to prevent agents from accidentally switching branches.
func injectBranchGuardrails(worktreePath, branch string) error {
	claudeDir := filepath.Join(worktreePath, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("mkdir .claude: %w", err)
	}

	guardrails := fmt.Sprintf(`# Worktree Session — Branch Guardrails

You are working on branch: %s

**Rules:**
1. DO NOT run git checkout, git switch, or any command that changes the current branch
2. All your work MUST stay on the %s branch
3. When committing, commit to %s only
4. If you need to reference code from another branch, use git show other-branch:path/to/file
`, "`"+branch+"`", "`"+branch+"`", "`"+branch+"`")

	claudeMdPath := filepath.Join(claudeDir, "CLAUDE.md")

	// Check if file exists and already has guardrails
	if existing, err := os.ReadFile(claudeMdPath); err == nil {
		if strings.Contains(string(existing), "Branch Guardrails") {
			return nil // Already has guardrails
		}
		// Append to existing file
		guardrails = string(existing) + "\n\n" + guardrails
	}

	if err := os.WriteFile(claudeMdPath, []byte(guardrails), 0o644); err != nil {
		return fmt.Errorf("write CLAUDE.md: %w", err)
	}

	return nil
}
