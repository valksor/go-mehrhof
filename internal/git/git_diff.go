package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// logFormat is the git log format used by Log, CommitsSince, and CommitInfo.
const logFormat = "%H|%s|%an|%ai"

// parseLogOutput parses git log output produced with logFormat into LogEntry slices.
func parseLogOutput(out string) []LogEntry {
	var entries []LogEntry
	for line := range strings.SplitSeq(out, "\n") {
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

// CommitsBetween returns log entries for commits in the range from..to (exclusive..inclusive).
func (r *Repository) CommitsBetween(ctx context.Context, from, to string) ([]LogEntry, error) {
	out, err := r.run(ctx, "log", "--format="+logFormat, from+".."+to)
	if err != nil {
		return nil, err
	}

	return parseLogOutput(out), nil
}

// logFormatFull includes the commit body, using NUL as record separator.
const logFormatFull = "%H|%s|%an|%ai%x00%b%x00"

// LogEntryFull extends LogEntry with the commit body.
type LogEntryFull struct {
	LogEntry

	Body string
}

// parseLogOutputFull parses git log output produced with logFormatFull.
func parseLogOutputFull(out string) []LogEntryFull {
	var entries []LogEntryFull
	records := strings.Split(out, "\x00")
	// Records come in pairs: header, body, header, body, ...
	for i := 0; i+1 < len(records); i += 2 {
		header := strings.TrimSpace(records[i])
		body := strings.TrimSpace(records[i+1])
		if header == "" {
			continue
		}
		parts := strings.SplitN(header, "|", 4)
		if len(parts) < 4 {
			continue
		}
		entries = append(entries, LogEntryFull{
			LogEntry: LogEntry{
				SHA:     parts[0],
				Message: parts[1],
				Author:  parts[2],
				Date:    parts[3],
			},
			Body: body,
		})
	}

	return entries
}

// CommitsBetweenFull returns log entries with body text for commits in the range from..to.
func (r *Repository) CommitsBetweenFull(ctx context.Context, from, to string) ([]LogEntryFull, error) {
	out, err := r.run(ctx, "log", "--format="+logFormatFull, from+".."+to)
	if err != nil {
		return nil, err
	}

	return parseLogOutputFull(out), nil
}

// DiffBetween returns the diff between two refs (from..to).
func (r *Repository) DiffBetween(ctx context.Context, from, to string) (string, error) {
	return r.run(ctx, "diff", from+".."+to)
}

// DiffStatBetween returns the --stat summary for the diff between two refs.
func (r *Repository) DiffStatBetween(ctx context.Context, from, to string) (string, error) {
	return r.run(ctx, "diff", "--stat", from+".."+to)
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
	for line := range strings.SplitSeq(out, "\n") {
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
	for line := range strings.SplitSeq(out, "\n") {
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
		if after, ok := strings.CutPrefix(out, "refs/remotes/origin/"); ok {
			name = after
		} else if after, ok := strings.CutPrefix(out, "refs/heads/"); ok {
			name = after
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
