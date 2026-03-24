package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/retry"
)

type Repository struct {
	path        string
	signCommits bool
}

func Open(path string) (*Repository, error) {
	// Verify it's a git repo
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir") //nolint:noctx // Quick one-shot existence check, no meaningful context to propagate
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("not a git repository: %s", path)
	}

	return &Repository{path: path}, nil
}

func (r *Repository) Path() string {
	return r.path
}

// gitRetryDelay is the base delay between retries for lock-contended git operations.
const gitRetryDelay = 100 * time.Millisecond

// gitMaxRetries is the number of retry attempts for retryable git operations.
const gitMaxRetries = 3

func (r *Repository) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", r.path}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		slog.Debug("git: command failed", "args", args, "error", err, "stderr", stderr.String())

		return "", formatGitError(args, stderr.String(), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// runRetryable executes a git command with automatic retry on lock-file
// conflicts and other transient errors.
func (r *Repository) runRetryable(ctx context.Context, args ...string) error {
	return retry.RetryableOp(ctx, gitMaxRetries, gitRetryDelay, func() error {
		_, runErr := r.run(ctx, args...)

		return runErr
	}, retry.WithRetryCheck(isRetryableGitError))
}

// isRetryableGitError checks for lock file conflicts and transient git errors
// that may resolve on retry.
func isRetryableGitError(err error) bool {
	if err == nil {
		return false
	}

	return retry.IsRetryable(err)
}

// formatGitError converts git command errors to user-friendly messages.
func formatGitError(args []string, stderr string, err error) error {
	stderr = strings.TrimSpace(stderr)

	// Common error patterns with user-friendly messages
	switch {
	case strings.Contains(stderr, "not a git repository"):
		return errors.New("not a git repository\nRun 'git init' to initialize one, or navigate to a project directory")

	case strings.Contains(stderr, "already exists"):
		if len(args) > 0 && args[0] == "checkout" {
			return fmt.Errorf("branch already exists: %s", stderr)
		}

		return fmt.Errorf("already exists: %s", stderr)

	case strings.Contains(stderr, "did not match any"):
		return fmt.Errorf("branch or commit not found: %s", stderr)

	case strings.Contains(stderr, "Your local changes"):
		return errors.New("uncommitted changes would be overwritten\nCommit or stash your changes first")

	case strings.Contains(stderr, "CONFLICT"):
		return errors.New("merge conflict detected\nResolve conflicts manually, then run 'git add' and 'git commit'")

	case strings.Contains(stderr, "Permission denied"):
		return fmt.Errorf("permission denied: %s", stderr)

	case strings.Contains(stderr, "Could not resolve host"):
		return errors.New("cannot reach remote server\nCheck your network connection")

	case strings.Contains(stderr, "Authentication failed"):
		return errors.New("authentication failed\nCheck your credentials or token")

	case strings.Contains(stderr, "No space left on device") || strings.Contains(stderr, "ENOSPC"):
		return errors.New("disk full — free up space and retry")
	}

	// Default: include stderr if present
	if stderr != "" {
		return fmt.Errorf("%w: %s", err, stderr)
	}

	return err
}

func (r *Repository) CurrentBranch(ctx context.Context) (string, error) {
	return r.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
}

func (r *Repository) CurrentCommit(ctx context.Context) (string, error) {
	return r.run(ctx, "rev-parse", "HEAD")
}

func (r *Repository) CreateBranch(ctx context.Context, name, startPoint string) error {
	slog.Debug("git: creating branch", "name", name, "startPoint", startPoint)

	args := []string{"checkout", "-b", name}
	if startPoint != "" {
		args = append(args, startPoint)
	}

	_, err := r.run(ctx, args...)

	return err
}

func (r *Repository) SwitchBranch(ctx context.Context, name string) error {
	_, err := r.run(ctx, "checkout", name)

	return err
}

// Checkout is an alias for SwitchBranch.
func (r *Repository) Checkout(ctx context.Context, name string) error {
	return r.SwitchBranch(ctx, name)
}

func (r *Repository) DeleteBranch(ctx context.Context, name string) error {
	slog.Debug("git: deleting branch", "name", name)
	_, err := r.run(ctx, "branch", "-D", name)

	return err
}

// DeleteRemoteBranch deletes a branch from the remote (origin).
func (r *Repository) DeleteRemoteBranch(ctx context.Context, name string) error {
	slog.Debug("git: deleting remote branch", "name", name)
	_, err := r.run(ctx, "push", "origin", "--delete", name)

	return err
}

// BranchExists checks if a branch exists (local or remote).
func (r *Repository) BranchExists(ctx context.Context, name string) bool {
	_, err := r.run(ctx, "rev-parse", "--verify", name)

	return err == nil
}

// LocalBranchExists checks if a local branch exists (refs/heads/ only).
// Unlike BranchExists, this won't match remote tracking branches.
func (r *Repository) LocalBranchExists(ctx context.Context, name string) bool {
	_, err := r.run(ctx, "rev-parse", "--verify", "refs/heads/"+name)

	return err == nil
}

func (r *Repository) HasUncommittedChanges(ctx context.Context) (bool, error) {
	out, err := r.run(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}

	return len(out) > 0, nil
}

func (r *Repository) StageAll(ctx context.Context) error {
	err := r.runRetryable(ctx, "add", "-A")

	return err
}

// StageFiles stages specific files for commit.
func (r *Repository) StageFiles(ctx context.Context, paths ...string) error {
	args := append([]string{"add", "--"}, paths...)

	return r.runRetryable(ctx, args...)
}

// ValidateCommitMessage checks if a commit message subject line matches the required pattern.
// Returns nil if pattern is empty (no validation) or if the message matches.
func ValidateCommitMessage(message, pattern string) error {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid commit pattern %q: %w", pattern, err)
	}
	subject := strings.SplitN(message, "\n", 2)[0]
	if !re.MatchString(subject) {
		return fmt.Errorf("commit message %q does not match required pattern %s", subject, pattern)
	}

	return nil
}

// SetSignCommits enables or disables GPG commit signing.
func (r *Repository) SetSignCommits(sign bool) {
	r.signCommits = sign
}

// IsSigningConfigured checks whether git commit signing is configured in the repository.
func (r *Repository) IsSigningConfigured(ctx context.Context) bool {
	out, err := r.run(ctx, "config", "commit.gpgsign")
	if err != nil {
		return false
	}

	return strings.TrimSpace(out) == "true"
}

func (r *Repository) Commit(ctx context.Context, message string) (string, error) {
	slog.Debug("git: committing", "message", message)
	args := []string{"commit"}
	if r.signCommits {
		args = append(args, "--gpg-sign")
	}
	args = append(args, "-m", message)
	err := r.runRetryable(ctx, args...)
	if err != nil {
		// Check if this looks like a pre-commit hook failure where files were modified
		// (formatters that fix files but reject the commit)
		if r.isHookFormatterFailure(ctx) {
			slog.Info("git: pre-commit hook modified files, re-staging and retrying")
			if stageErr := r.StageAll(ctx); stageErr != nil {
				return "", fmt.Errorf("re-stage after hook: %w", stageErr)
			}
			retryErr := r.runRetryable(ctx, args...)
			if retryErr != nil {
				return "", fmt.Errorf("commit after hook retry: %w", retryErr)
			}
		} else {
			return "", err
		}
	}

	sha, err := r.CurrentCommit(ctx)
	if err != nil {
		slog.Warn("git: committed but failed to get SHA", "error", err)

		return "", fmt.Errorf("commit succeeded but failed to get SHA: %w", err)
	}
	if sha == "" {
		slog.Error("git: committed but empty SHA")

		return "", errors.New("commit succeeded but SHA is empty")
	}
	slog.Debug("git: committed", "sha", sha)

	return sha, nil
}

// isHookFormatterFailure checks if a commit failure was caused by a pre-commit hook
// that modified files (common with formatters like prettier, black, gofmt).
func (r *Repository) isHookFormatterFailure(ctx context.Context) bool {
	// After a failed commit, check if there are now unstaged changes
	// (which would indicate a formatter modified files)
	has, err := r.HasUncommittedChanges(ctx)
	if err != nil {
		return false
	}

	return has
}

func (r *Repository) Reset(ctx context.Context, commit string, hard bool) error {
	slog.Debug("git: resetting", "commit", commit, "hard", hard)
	args := []string{"reset"}
	if hard {
		args = append(args, "--hard")
	}
	args = append(args, commit)
	err := r.runRetryable(ctx, args...)

	return err
}

func (r *Repository) Stash(ctx context.Context) error {
	_, err := r.run(ctx, "stash")

	return err
}

func (r *Repository) StashPop(ctx context.Context) error {
	_, err := r.run(ctx, "stash", "pop")

	return err
}

// Push pushes to the remote repository.
func (r *Repository) Push(ctx context.Context, remote, branch string) error {
	slog.Debug("git: pushing", "remote", remote, "branch", branch)
	err := r.runRetryable(ctx, "push", remote, branch)

	return err
}

// PushDefault pushes to origin with the current branch.
func (r *Repository) PushDefault(ctx context.Context) error {
	branch, err := r.CurrentBranch(ctx)
	if err != nil {
		return err
	}

	return r.Push(ctx, "origin", branch)
}

// Pull pulls from the remote repository.
func (r *Repository) Pull(ctx context.Context) error {
	slog.Debug("git: pulling")
	err := r.runRetryable(ctx, "pull")

	return err
}

// Fetch fetches from the remote repository.
func (r *Repository) Fetch(ctx context.Context) error {
	slog.Debug("git: fetching")
	err := r.runRetryable(ctx, "fetch")

	return err
}

// CommitsBehind returns how many commits the current branch is behind the given ref.
// ref should be a full remote ref like "origin/main" or "upstream/develop".
func (r *Repository) CommitsBehind(ctx context.Context, ref string) (int, error) {
	current, err := r.CurrentBranch(ctx)
	if err != nil {
		return 0, err
	}

	// Count commits that are in ref but not in current
	out, err := r.run(ctx, "rev-list", "--count", fmt.Sprintf("%s..%s", current, ref))
	if err != nil {
		return 0, err
	}

	var count int
	if _, err := fmt.Sscanf(out, "%d", &count); err != nil {
		return 0, fmt.Errorf("parse count: %w", err)
	}

	return count, nil
}

// logFormat is the git log format used by Log, CommitsSince, and CommitInfo.
const logFormat = "%H|%s|%an|%ai"

// parseLogOutput parses git log output produced with logFormat into LogEntry slices.
func parseLogOutput(out string) []LogEntry {
	var entries []LogEntry
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		entries = append(entries, LogEntry{
			SHA:     parts[0],
			Message: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}

	return entries
}

func (r *Repository) Log(ctx context.Context, n int) ([]LogEntry, error) {
	out, err := r.run(ctx, "log", fmt.Sprintf("-n%d", n), "--format="+logFormat)
	if err != nil {
		return nil, err
	}

	return parseLogOutput(out), nil
}

// CommitsSince returns log entries for all commits between sinceRef (exclusive)
// and HEAD (inclusive). Useful for inspecting agent commits since the last checkpoint.
func (r *Repository) CommitsSince(ctx context.Context, sinceRef string) ([]LogEntry, error) {
	out, err := r.run(ctx, "log", "--format="+logFormat, sinceRef+"..HEAD")
	if err != nil {
		return nil, err
	}

	return parseLogOutput(out), nil
}

// CommitInfo returns metadata for a single commit SHA.
func (r *Repository) CommitInfo(ctx context.Context, sha string) (LogEntry, error) {
	out, err := r.run(ctx, "log", "-1", "--format="+logFormat, sha)
	if err != nil {
		return LogEntry{}, fmt.Errorf("commit %s not found: %w", sha, err)
	}
	if out == "" {
		return LogEntry{}, fmt.Errorf("commit %s not found", sha)
	}
	parts := strings.SplitN(out, "|", 4)
	if len(parts) < 4 {
		return LogEntry{}, fmt.Errorf("unexpected log format: %q", out)
	}

	return LogEntry{
		SHA:     parts[0],
		Message: parts[1],
		Author:  parts[2],
		Date:    parts[3],
	}, nil
}

func (r *Repository) Diff(ctx context.Context, cached bool) (string, error) {
	args := []string{"diff"}
	if cached {
		args = append(args, "--cached")
	}

	return r.run(ctx, args...)
}

// DiffAgainst shows the diff between a given commit and the current working tree (including
// uncommitted changes). When stat is true only the --stat summary is returned.
func (r *Repository) DiffAgainst(ctx context.Context, ref string, stat bool) (string, error) {
	args := []string{"diff", ref}
	if stat {
		args = append(args, "--stat")
	}

	return r.run(ctx, args...)
}

func (r *Repository) DiffFiles(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, "diff", "--name-only")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	return strings.Split(out, "\n"), nil
}

// DiffNumStat holds line-level diff statistics.
type DiffNumStat struct {
	Added   int
	Removed int
	Files   []string
}

// DiffNumStatAgainst returns line-level diff stats against a reference commit.
// If ref is empty, diffs against HEAD.
func (r *Repository) DiffNumStatAgainst(ctx context.Context, ref string) (DiffNumStat, error) {
	args := []string{"diff", "--numstat"}
	if ref != "" {
		args = []string{"diff", ref, "--numstat"}
	}

	out, err := r.run(ctx, args...)
	if err != nil {
		return DiffNumStat{}, err
	}

	return parseNumStat(out), nil
}

func parseNumStat(out string) DiffNumStat {
	var result DiffNumStat
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		// Binary files show "-" for added/removed
		if fields[0] == "-" || fields[1] == "-" {
			result.Files = append(result.Files, fields[2])
			continue
		}
		added := 0
		removed := 0
		if _, err := fmt.Sscanf(fields[0], "%d", &added); err == nil {
			result.Added += added
		}
		if _, err := fmt.Sscanf(fields[1], "%d", &removed); err == nil {
			result.Removed += removed
		}
		result.Files = append(result.Files, fields[2])
	}
	return result
}

type LogEntry struct {
	SHA     string
	Message string
	Author  string
	Date    string
}

// FileStatus holds a path and its change status from git.
type FileStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"` // "added", "modified", "deleted", "renamed"
}

// parseNameStatusLine parses one line of `git diff --name-status` output.
//
//nolint:nonamedreturns // Named returns document the return values
func parseNameStatusLine(line string) (path, status string) {
	parts := strings.SplitN(line, "\t", 3)
	if len(parts) < 2 {
		return line, "modified"
	}
	code := parts[0]
	// Renames/copies have destination path in parts[2]
	if len(parts) == 3 {
		path = parts[2]
	} else {
		path = parts[1]
	}
	switch {
	case strings.HasPrefix(code, "A"):
		status = "added"
	case strings.HasPrefix(code, "D"):
		status = "deleted"
	case strings.HasPrefix(code, "R"), strings.HasPrefix(code, "C"):
		status = "renamed"
	default:
		status = "modified"
	}

	return path, status
}

// DiffFilesWithStatus returns changed files with their git change status.
func (r *Repository) DiffFilesWithStatus(ctx context.Context) ([]FileStatus, error) {
	out, err := r.run(ctx, "diff", "--name-status")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var result []FileStatus
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		path, status := parseNameStatusLine(line)
		result = append(result, FileStatus{Path: path, Status: status})
	}

	return result, nil
}

// DefaultBranch returns the default branch name.
// Detection order: remote HEAD symbolic ref (authoritative), then local
// main/master existence as fallback for offline or no-remote scenarios.
// Returns error if detection fails — callers should configure git.base_branch.
func (r *Repository) DefaultBranch(ctx context.Context) (string, error) {
	// 1. Try remote HEAD symbolic ref — this is authoritative when available.
	out, err := r.run(ctx, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil && out != "" {
		var name string
		if strings.HasPrefix(out, "refs/remotes/origin/") {
			name = strings.TrimPrefix(out, "refs/remotes/origin/")
		} else if strings.HasPrefix(out, "refs/heads/") {
			name = strings.TrimPrefix(out, "refs/heads/")
		}
		if name != "" {
			return name, nil
		}
	}

	// 2. Fall back to local main/master for offline or no-remote repos.
	for _, name := range []string{"main", "master"} {
		if r.LocalBranchExists(ctx, name) {
			return name, nil
		}
	}

	return "", errors.New("cannot detect default branch: origin/HEAD not set and no local main/master branch found; run 'kvelmo config set git.base_branch <branch>' to configure")
}
