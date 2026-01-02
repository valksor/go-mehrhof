package vcs

import (
	"fmt"
	"strings"
)

// Branch represents a git branch.
type Branch struct {
	Name      string
	Remote    string // Empty for local branches
	IsCurrent bool
	Commit    string // HEAD commit hash
}

// CreateBranch creates and optionally checks out a new branch.
func (g *Git) CreateBranch(name string, base string) error {
	args := []string{"checkout", "-b", name}
	if base != "" {
		args = append(args, base)
	}
	_, err := g.run(args...)
	if err != nil {
		return fmt.Errorf("create branch %s: %w", name, err)
	}
	return nil
}

// CreateBranchNoCheckout creates a branch without checking it out.
func (g *Git) CreateBranchNoCheckout(name string, base string) error {
	args := []string{"branch", name}
	if base != "" {
		args = append(args, base)
	}
	_, err := g.run(args...)
	if err != nil {
		return fmt.Errorf("create branch %s: %w", name, err)
	}
	return nil
}

// DeleteBranch deletes a branch.
func (g *Git) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := g.run("branch", flag, name)
	if err != nil {
		return fmt.Errorf("delete branch %s: %w", name, err)
	}
	return nil
}

// BranchExists checks if a branch exists.
func (g *Git) BranchExists(name string) bool {
	_, err := g.run("rev-parse", "--verify", "refs/heads/"+name)
	return err == nil
}

// RemoteBranchExists checks if a remote branch exists.
func (g *Git) RemoteBranchExists(remote, name string) bool {
	_, err := g.run("rev-parse", "--verify", fmt.Sprintf("refs/remotes/%s/%s", remote, name))
	return err == nil
}

// ListBranches returns all local branches.
func (g *Git) ListBranches() ([]Branch, error) {
	out, err := g.run("branch", "-v", "--no-abbrev")
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}

	var branches []Branch
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		b := Branch{}
		if strings.HasPrefix(line, "* ") {
			b.IsCurrent = true
			line = line[2:]
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			b.Name = parts[0]
			b.Commit = parts[1]
		}

		branches = append(branches, b)
	}

	return branches, nil
}

// GetBaseBranch finds the base branch (usually main or master).
func (g *Git) GetBaseBranch() (string, error) {
	// Try common base branch names
	candidates := []string{"main", "master", "develop"}

	for _, name := range candidates {
		if g.BranchExists(name) {
			return name, nil
		}
	}

	// Try to find from remote
	for _, name := range candidates {
		if g.RemoteBranchExists("origin", name) {
			return name, nil
		}
	}

	// Fall back to first branch
	branches, err := g.ListBranches()
	if err != nil {
		return "", err
	}
	if len(branches) > 0 {
		return branches[0].Name, nil
	}

	return "", fmt.Errorf("no base branch found")
}

// GetTrackingBranch returns the remote tracking branch for a local branch.
func (g *Git) GetTrackingBranch(name string) (string, error) {
	out, err := g.run("rev-parse", "--abbrev-ref", name+"@{upstream}")
	if err != nil {
		return "", fmt.Errorf("no tracking branch for %s", name)
	}
	return strings.TrimSpace(out), nil
}

// SetTrackingBranch sets the remote tracking branch.
func (g *Git) SetTrackingBranch(local, remote, branch string) error {
	_, err := g.run("branch", "-u", fmt.Sprintf("%s/%s", remote, branch), local)
	return err
}

// RenameBranch renames a branch.
func (g *Git) RenameBranch(oldName, newName string) error {
	_, err := g.run("branch", "-m", oldName, newName)
	if err != nil {
		return fmt.Errorf("rename branch %s to %s: %w", oldName, newName, err)
	}
	return nil
}

// MergeBranch merges a branch into the current branch.
func (g *Git) MergeBranch(name string, noFF bool) error {
	args := []string{"merge", name}
	if noFF {
		args = append(args, "--no-ff")
	}
	_, err := g.run(args...)
	return err
}

// MergeSquash performs a squash merge.
func (g *Git) MergeSquash(name string) error {
	_, err := g.run("merge", "--squash", name)
	return err
}

// RebaseBranch rebases current branch onto another.
func (g *Git) RebaseBranch(onto string) error {
	_, err := g.run("rebase", onto)
	return err
}

// AbortRebase aborts an in-progress rebase.
func (g *Git) AbortRebase() error {
	_, err := g.run("rebase", "--abort")
	return err
}

// ContinueRebase continues a rebase after resolving conflicts.
func (g *Git) ContinueRebase() error {
	_, err := g.run("rebase", "--continue")
	return err
}

// GetBranchCommitCount returns the number of commits in branch ahead of base.
func (g *Git) GetBranchCommitCount(branch, base string) (int, error) {
	out, err := g.run("rev-list", "--count", fmt.Sprintf("%s..%s", base, branch))
	if err != nil {
		return 0, err
	}

	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(out), "%d", &count)
	return count, err
}

// GetMergeBase returns the common ancestor of two branches.
func (g *Git) GetMergeBase(a, b string) (string, error) {
	out, err := g.run("merge-base", a, b)
	if err != nil {
		return "", fmt.Errorf("merge-base %s %s: %w", a, b, err)
	}
	return strings.TrimSpace(out), nil
}

// IsMerged checks if a branch has been merged into another.
func (g *Git) IsMerged(branch, into string) (bool, error) {
	mergeBase, err := g.GetMergeBase(branch, into)
	if err != nil {
		return false, err
	}

	branchHead, err := g.RevParse(branch)
	if err != nil {
		return false, err
	}

	return mergeBase == branchHead, nil
}

// GetAheadBehind returns commits ahead and behind remote.
func (g *Git) GetAheadBehind(local, remote string) (ahead, behind int, err error) {
	out, err := g.run("rev-list", "--left-right", "--count", fmt.Sprintf("%s...%s", local, remote))
	if err != nil {
		return 0, 0, err
	}

	_, err = fmt.Sscanf(strings.TrimSpace(out), "%d\t%d", &ahead, &behind)
	return ahead, behind, err
}

// PushBranch pushes a branch to remote, optionally setting upstream.
func (g *Git) PushBranch(branch, remote string, setUpstream bool) error {
	args := []string{"push", remote, branch}
	if setUpstream {
		args = []string{"push", "-u", remote, branch}
	}
	_, err := g.run(args...)
	return err
}

// ForcePushBranch force pushes a branch (use with caution).
func (g *Git) ForcePushBranch(branch, remote string) error {
	_, err := g.run("push", "--force-with-lease", remote, branch)
	return err
}
