// Package vcs provides git operations for the mehrhof task automation tool.
//
// The Git type wraps common git operations needed for task management:
//   - Branch creation and switching
//   - Checkpoint creation for undo/redo functionality
//   - Worktree management for parallel task execution
//   - Status and diff operations
//
// Thread safety:
//   - Git methods are safe for concurrent use as they don't maintain mutable state.
//   - The Git value itself should not be copied after creation.
//
// Usage:
//
//	g, err := vcs.New("/path/to/repo")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	branch, _ := g.CurrentBranch()
package vcs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Git porcelain v1 format constants
// Format: XY PATH where X=index status, Y=worktree status
// See: https://git-scm.com/docs/git-status#_short_format
const (
	gitStatusIndexPos   = 0 // Position of index (staged) status character
	gitStatusWorkDirPos = 1 // Position of working directory status character
	gitStatusPathStart  = 3 // Position where file path begins (after "XY ")
	gitStatusMinLength  = 4 // Minimum valid entry length (XY + space + at least 1 char)
)

// Git provides git operations for a repository
type Git struct {
	repoRoot string
}

// New creates a Git instance for the given path
func New(path string) (*Git, error) {
	root, err := findRepoRoot(path)
	if err != nil {
		return nil, err
	}
	return &Git{repoRoot: root}, nil
}

// Root returns the repository root path
func (g *Git) Root() string {
	return g.repoRoot
}

// IsRepo checks if the path is inside a git repository
func IsRepo(path string) bool {
	_, err := findRepoRoot(path)
	return err == nil
}

// findRepoRoot locates the git repository root
func findRepoRoot(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// Use background context for repo discovery (this is a fast local operation)
	out, err := runGitCommandContext(context.Background(), absPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}

	return strings.TrimSpace(out), nil
}

// CurrentBranch returns the current branch name
func (g *Git) CurrentBranch() (string, error) {
	out, err := g.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// Status returns uncommitted changes
func (g *Git) Status() ([]FileStatus, error) {
	out, err := g.run("status", "--porcelain", "-z")
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}

	if out == "" {
		return nil, nil
	}

	var files []FileStatus
	entries := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	for _, entry := range entries {
		if len(entry) < gitStatusMinLength {
			continue
		}
		fs := FileStatus{
			Index:   entry[gitStatusIndexPos],
			WorkDir: entry[gitStatusWorkDirPos],
			Path:    strings.TrimSpace(entry[gitStatusPathStart:]),
		}
		files = append(files, fs)
	}

	return files, nil
}

// FileStatus represents a file's git status
type FileStatus struct {
	Index   byte   // Status in index
	WorkDir byte   // Status in working directory
	Path    string // File path
}

// IsStaged returns true if the file is staged
func (f FileStatus) IsStaged() bool {
	return f.Index != ' ' && f.Index != '?'
}

// IsModified returns true if the file is modified in working directory
func (f FileStatus) IsModified() bool {
	return f.WorkDir == 'M' || f.WorkDir == 'D'
}

// HasChanges returns true if there are uncommitted changes
func (g *Git) HasChanges() (bool, error) {
	files, err := g.Status()
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}

// Add stages files for commit
func (g *Git) Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	_, err := g.run(args...)
	if err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	return nil
}

// AddAll stages all changes
func (g *Git) AddAll() error {
	return g.Add("-A")
}

// CommitOptions configures commit behavior
type CommitOptions struct {
	AllowEmpty bool // Create commit even with no changes
}

// Commit creates a commit with the given message.
// Optional CommitOptions can be provided to modify behavior.
func (g *Git) Commit(message string, opts ...CommitOptions) (string, error) {
	args := []string{"commit"}

	// Apply options if provided
	if len(opts) > 0 && opts[0].AllowEmpty {
		args = append(args, "--allow-empty")
	}

	args = append(args, "-m", message)

	if _, err := g.run(args...); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}

	// Get the commit hash
	out, err := g.run("rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get commit hash: %w", err)
	}

	return strings.TrimSpace(out), nil
}

// CommitAllowEmpty creates a commit even if there are no changes.
// Deprecated: Use Commit(message, CommitOptions{AllowEmpty: true}) instead.
func (g *Git) CommitAllowEmpty(message string) (string, error) {
	return g.Commit(message, CommitOptions{AllowEmpty: true})
}

// Checkout switches to a branch
func (g *Git) Checkout(ref string) error {
	_, err := g.run("checkout", ref)
	if err != nil {
		return fmt.Errorf("git checkout %s: %w", ref, err)
	}
	return nil
}

// Diff returns diff output
func (g *Git) Diff(args ...string) (string, error) {
	cmdArgs := append([]string{"diff"}, args...)
	return g.run(cmdArgs...)
}

// Log returns commit logs
func (g *Git) Log(args ...string) (string, error) {
	cmdArgs := append([]string{"log"}, args...)
	return g.run(cmdArgs...)
}

// RevParse resolves a git reference
func (g *Git) RevParse(ref string) (string, error) {
	out, err := g.run("rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %w", ref, err)
	}
	return strings.TrimSpace(out), nil
}

// GetCommitMessage returns the message for a commit
func (g *Git) GetCommitMessage(ref string) (string, error) {
	out, err := g.run("log", "-1", "--format=%B", ref)
	if err != nil {
		return "", fmt.Errorf("get commit message: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// GetCommitAuthor returns the author of a commit
func (g *Git) GetCommitAuthor(ref string) (string, error) {
	out, err := g.run("log", "-1", "--format=%an <%ae>", ref)
	if err != nil {
		return "", fmt.Errorf("get commit author: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// ResetHard resets to a ref, discarding all changes
func (g *Git) ResetHard(ref string) error {
	_, err := g.run("reset", "--hard", ref)
	if err != nil {
		return fmt.Errorf("reset hard to %s: %w", ref, err)
	}
	return nil
}

// ResetSoft resets to a ref, keeping changes staged
func (g *Git) ResetSoft(ref string) error {
	_, err := g.run("reset", "--soft", ref)
	if err != nil {
		return fmt.Errorf("reset soft to %s: %w", ref, err)
	}
	return nil
}

// Clean removes untracked files
func (g *Git) Clean(force bool) error {
	args := []string{"clean", "-d"}
	if force {
		args = append(args, "-f")
	}
	_, err := g.run(args...)
	return err
}

// Stash saves changes to stash
func (g *Git) Stash(message string) error {
	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := g.run(args...)
	return err
}

// StashPop applies and removes the top stash entry
func (g *Git) StashPop() error {
	_, err := g.run("stash", "pop")
	return err
}

// run executes a git command in the repo root.
// Note: This uses a background context and does not propagate cancellation.
// For operations that need context cancellation or deadlines, use RunContext().
func (g *Git) run(args ...string) (string, error) {
	return runGitCommandContext(context.Background(), g.repoRoot, args...)
}

// RunContext executes a git command with context
func (g *Git) RunContext(ctx context.Context, args ...string) (string, error) {
	return runGitCommandContext(ctx, g.repoRoot, args...)
}

// runGitCommandContext executes a git command with context
func runGitCommandContext(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("%s", strings.TrimSpace(errMsg))
	}

	return stdout.String(), nil
}

// Config represents git configuration
type Config struct {
	Key   string
	Value string
}

// GetConfig reads a git config value
func (g *Git) GetConfig(key string) (string, error) {
	out, err := g.run("config", "--get", key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// SetConfig sets a git config value
func (g *Git) SetConfig(key, value string) error {
	_, err := g.run("config", key, value)
	return err
}

// RemoteURL returns the URL for a remote
func (g *Git) RemoteURL(name string) (string, error) {
	out, err := g.run("remote", "get-url", name)
	if err != nil {
		return "", fmt.Errorf("get remote URL %s: %w", name, err)
	}
	return strings.TrimSpace(out), nil
}

// Fetch fetches from a remote
func (g *Git) Fetch(remote string, args ...string) error {
	cmdArgs := append([]string{"fetch", remote}, args...)
	_, err := g.run(cmdArgs...)
	return err
}

// Pull pulls from a remote
func (g *Git) Pull(remote, branch string) error {
	_, err := g.run("pull", remote, branch)
	return err
}

// Push pushes to a remote
func (g *Git) Push(remote, branch string, args ...string) error {
	cmdArgs := append([]string{"push", remote, branch}, args...)
	_, err := g.run(cmdArgs...)
	return err
}

// IsWorktree returns true if the current repository is a git worktree
// (as opposed to the main repository). Worktrees have a .git file
// pointing to the main repo, while main repos have a .git directory.
func (g *Git) IsWorktree() bool {
	gitPath := filepath.Join(g.repoRoot, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	// Worktrees have .git as a file, main repos have it as a directory
	return !info.IsDir()
}

// GetMainWorktreePath returns the path to the main repository when called
// from within a worktree. Returns an error if not in a worktree.
func (g *Git) GetMainWorktreePath() (string, error) {
	if !g.IsWorktree() {
		return "", fmt.Errorf("not in a worktree")
	}

	// git rev-parse --git-common-dir returns the shared .git directory
	// e.g., /path/to/main-repo/.git
	out, err := g.run("rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("get git common dir: %w", err)
	}

	gitCommonDir := strings.TrimSpace(out)

	// Handle relative paths (git may return relative path)
	if !filepath.IsAbs(gitCommonDir) {
		gitCommonDir = filepath.Join(g.repoRoot, gitCommonDir)
	}

	// Clean up the path (resolve .., symlinks, etc.)
	gitCommonDir, err = filepath.Abs(gitCommonDir)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}

	// The main repo root is the parent of the .git directory
	mainRepoRoot := filepath.Dir(gitCommonDir)

	return mainRepoRoot, nil
}
