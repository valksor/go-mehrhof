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

// Merge merges the given branch into the current branch using --no-ff.
func (r *Repository) Merge(ctx context.Context, branch, message string) error {
	slog.Debug("git: merging", "branch", branch)
	args := []string{"merge", "--no-ff", "-m", message, branch}
	_, err := r.run(ctx, args...)

	return err
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

type LogEntry struct {
	SHA     string
	Message string
	Author  string
	Date    string
}
