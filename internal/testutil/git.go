// Package testutil provides shared testing utilities for go-mehrhof tests.
package testutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// CreateCheckpoint creates a git checkpoint for testing.
func CreateCheckpoint(t *testing.T, repoDir, taskID, message string) string {
	t.Helper()

	// Make a change first
	testFile := filepath.Join(repoDir, "checkpoint-test.txt")
	if err := os.WriteFile(testFile, []byte(message), 0o644); err != nil {
		t.Fatalf("Write checkpoint test file: %v", err)
	}

	// Stage and commit with task ID prefix
	mustRunGit(t, repoDir, "add", ".")
	commitMsg := "[" + taskID + "] " + message
	mustRunGit(t, repoDir, "commit", "-m", commitMsg)

	// Get commit hash
	output := RunGit(t, repoDir, "rev-parse", "HEAD")

	return strings.TrimSpace(output)
}

// GetCurrentBranch returns the current branch name.
func GetCurrentBranch(t *testing.T, repoDir string) string {
	t.Helper()

	return strings.TrimSpace(RunGit(t, repoDir, "branch", "--show-current"))
}

// GetCheckpointCount returns the number of checkpoints (commits) for a task.
func GetCheckpointCount(t *testing.T, repoDir, taskID string) int {
	t.Helper()

	output := RunGit(t, repoDir, "log", "--oneline", "--grep=^\\["+taskID+"\\]")
	lines := strings.Split(strings.TrimSpace(output), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	return count
}

// AssertBranchExists fails the test if the branch doesn't exist.
func AssertBranchExists(t *testing.T, repoDir, branch string) {
	t.Helper()

	branches := RunGit(t, repoDir, "branch", "--list", branch)
	if strings.TrimSpace(branches) == "" {
		t.Errorf("branch %q does not exist", branch)
	}
}

// AssertBranchNotExists fails the test if the branch exists.
func AssertBranchNotExists(t *testing.T, repoDir, branch string) {
	t.Helper()

	branches := RunGit(t, repoDir, "branch", "--list", branch)
	if strings.TrimSpace(branches) != "" {
		t.Errorf("branch %q exists but should not", branch)
	}
}

// AssertWorktreeExists fails the test if the worktree doesn't exist.
func AssertWorktreeExists(t *testing.T, repoDir, worktreePath string) {
	t.Helper()

	worktrees := RunGit(t, repoDir, "worktree", "list", "--porcelain")
	if !strings.Contains(worktrees, worktreePath) {
		t.Errorf("worktree %q does not exist", worktreePath)
	}
}

// AssertWorktreeNotExists fails the test if the worktree exists.
func AssertWorktreeNotExists(t *testing.T, repoDir, worktreePath string) {
	t.Helper()

	worktrees := RunGit(t, repoDir, "worktree", "list", "--porcelain")
	if strings.Contains(worktrees, worktreePath) {
		t.Errorf("worktree %q exists but should not", worktreePath)
	}
}

// AssertCurrentBranch fails the test if current branch doesn't match.
func AssertCurrentBranch(t *testing.T, repoDir, expected string) {
	t.Helper()

	current := GetCurrentBranch(t, repoDir)
	if current != expected {
		t.Errorf("current branch = %q, want %q", current, expected)
	}
}

// AssertFileInCommit fails the test if the file is not in the given commit.
func AssertFileInCommit(t *testing.T, repoDir, commit, filePath string) {
	t.Helper()

	// Check if file exists in commit
	output := RunGit(t, repoDir, "ls-tree", commit, filePath)
	if strings.TrimSpace(output) == "" {
		t.Errorf("file %q not found in commit %s", filePath, commit)
	}
}

// CreateGitWithCommit creates a git repo with an initial commit.
func CreateGitWithCommit(t *testing.T, dir string) string {
	t.Helper()

	initGitRepo(t, dir)

	// Create a file and commit
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
		t.Fatalf("Write test file: %v", err)
	}

	mustRunGit(t, dir, "add", ".")
	mustRunGit(t, dir, "commit", "-m", "initial commit")

	// Return commit hash
	return strings.TrimSpace(RunGit(t, dir, "rev-parse", "HEAD"))
}

// GetCommitCount returns the total number of commits in the repo.
func GetCommitCount(t *testing.T, repoDir string) int {
	t.Helper()

	output := RunGit(t, repoDir, "rev-list", "--count", "HEAD")
	count := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(output), "%d", &count); err == nil {
		return count
	}

	return 0
}

// GitCommit creates a new commit with the given message.
func GitCommit(t *testing.T, repoDir, message string) string {
	t.Helper()

	mustRunGit(t, repoDir, "add", ".")
	mustRunGit(t, repoDir, "commit", "-m", message)

	return strings.TrimSpace(RunGit(t, repoDir, "rev-parse", "HEAD"))
}

// GitCheckout switches to the given branch.
func GitCheckout(t *testing.T, repoDir, branch string) {
	t.Helper()
	mustRunGit(t, repoDir, "checkout", branch)
}

// GitCreateBranch creates a new branch at the current HEAD.
func GitCreateBranch(t *testing.T, repoDir, branch string) {
	t.Helper()
	mustRunGit(t, repoDir, "branch", branch)
}

// GitCreateAndCheckoutBranch creates and switches to a new branch.
func GitCreateAndCheckoutBranch(t *testing.T, repoDir, branch string) {
	t.Helper()
	mustRunGit(t, repoDir, "checkout", "-b", branch)
}

// initGitRepo initializes a git repository in the given directory.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	ctx := context.Background()

	if err := runGitCmd(ctx, dir, "init"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	if err := runGitCmd(ctx, dir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
	if err := runGitCmd(ctx, dir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}

	// Create initial commit
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := runGitCmd(ctx, dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := runGitCmd(ctx, dir, "commit", "-m", "initial commit"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
}

// runGitCmd runs a git command and returns any error.
func runGitCmd(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2020-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2020-01-01T00:00:00Z",
	)

	return cmd.Run()
}

// GitGetBaseBranch returns the base branch of the repo.
func GitGetBaseBranch(t *testing.T, repoDir string) string {
	t.Helper()

	// Try to get the default branch name
	branches := RunGit(t, repoDir, "branch", "--format=%(refname:short)")
	lines := strings.Split(strings.TrimSpace(branches), "\n")

	// Common default branch names
	defaults := []string{"main", "master", "develop"}
	for _, def := range defaults {
		for _, line := range lines {
			if strings.TrimSpace(line) == def {
				return def
			}
		}
	}

	// Fall back to first branch
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}

	return "main"
}
