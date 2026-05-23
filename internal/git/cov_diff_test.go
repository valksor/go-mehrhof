package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeCommit writes a file and commits it, returning the resulting SHA.
func makeCommit(t *testing.T, repo *Repository, dir, name, content, msg string) string {
	t.Helper()
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if err := repo.StageAll(ctx); err != nil {
		t.Fatalf("StageAll: %v", err)
	}
	sha, err := repo.Commit(ctx, msg)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	return sha
}

func TestCommitsBetween_And_Full(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	from, _ := repo.CurrentCommit(ctx)

	makeCommit(t, repo, dir, "one.txt", "1\n", "first change")
	to := makeCommit(t, repo, dir, "two.txt", "2\n", "second change\n\nbody text here")

	entries, err := repo.CommitsBetween(ctx, from, to)
	if err != nil {
		t.Fatalf("CommitsBetween: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("CommitsBetween returned %d, want 2", len(entries))
	}
	if entries[0].Message != "second change" {
		t.Errorf("entries[0].Message = %q, want %q", entries[0].Message, "second change")
	}

	full, err := repo.CommitsBetweenFull(ctx, from, to)
	if err != nil {
		t.Fatalf("CommitsBetweenFull: %v", err)
	}
	if len(full) != 2 {
		t.Fatalf("CommitsBetweenFull returned %d, want 2", len(full))
	}
	if !strings.Contains(full[0].Body, "body text here") {
		t.Errorf("full[0].Body = %q, want it to contain body text", full[0].Body)
	}
}

func TestDiffBetween_And_Stat(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	from, _ := repo.CurrentCommit(ctx)
	to := makeCommit(t, repo, dir, "diffme.txt", "added line\n", "add diffme")

	diff, err := repo.DiffBetween(ctx, from, to)
	if err != nil {
		t.Fatalf("DiffBetween: %v", err)
	}
	if !strings.Contains(diff, "diffme.txt") {
		t.Errorf("DiffBetween output missing filename: %q", diff)
	}

	stat, err := repo.DiffStatBetween(ctx, from, to)
	if err != nil {
		t.Fatalf("DiffStatBetween: %v", err)
	}
	if !strings.Contains(stat, "diffme.txt") {
		t.Errorf("DiffStatBetween output missing filename: %q", stat)
	}
}

func TestDiffAgainst_And_NumStat(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	ref, _ := repo.CurrentCommit(ctx)

	// Uncommitted change relative to ref.
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("line one\nline two\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	diff, err := repo.DiffAgainst(ctx, ref, false)
	if err != nil {
		t.Fatalf("DiffAgainst: %v", err)
	}
	if !strings.Contains(diff, "test.txt") {
		t.Errorf("DiffAgainst output missing filename: %q", diff)
	}

	statDiff, err := repo.DiffAgainst(ctx, ref, true)
	if err != nil {
		t.Fatalf("DiffAgainst(stat): %v", err)
	}
	if !strings.Contains(statDiff, "test.txt") {
		t.Errorf("DiffAgainst(stat) missing filename: %q", statDiff)
	}

	ns, err := repo.DiffNumStatAgainst(ctx, ref)
	if err != nil {
		t.Fatalf("DiffNumStatAgainst: %v", err)
	}
	if ns.Added == 0 {
		t.Errorf("DiffNumStatAgainst Added = 0, want > 0")
	}
	if len(ns.Files) == 0 || ns.Files[0] != "test.txt" {
		t.Errorf("DiffNumStatAgainst Files = %v, want [test.txt]", ns.Files)
	}

	// Empty ref form diffs against the working tree (HEAD).
	nsHead, err := repo.DiffNumStatAgainst(ctx, "")
	if err != nil {
		t.Fatalf("DiffNumStatAgainst(empty): %v", err)
	}
	if nsHead.Added == 0 {
		t.Errorf("DiffNumStatAgainst(empty) Added = 0, want > 0")
	}
}

func TestDiffHunks(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	base, _ := repo.CurrentBranch(ctx)

	// Branch off and add a file with multiple lines.
	if err := repo.CreateBranch(ctx, "work", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	makeCommit(t, repo, dir, "feature.go", "package x\n\nfunc A() {}\nfunc B() {}\n", "add feature")

	hunks, err := repo.DiffHunks(ctx, base)
	if err != nil {
		t.Fatalf("DiffHunks: %v", err)
	}
	ranges, ok := hunks["feature.go"]
	if !ok {
		t.Fatalf("DiffHunks missing feature.go, got %v", hunks)
	}
	if len(ranges) == 0 {
		t.Errorf("DiffHunks feature.go ranges empty")
	}
	if ranges[0][0] != 1 {
		t.Errorf("DiffHunks first range start = %d, want 1", ranges[0][0])
	}
}

func TestDiffHunks_BadBase(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	if _, err := repo.DiffHunks(ctx, "no-such-branch-ref"); err == nil {
		t.Error("DiffHunks(bad base) = nil, want error")
	}
}

func TestPruneWorktrees(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	wtBase := t.TempDir()
	wtPath := filepath.Join(wtBase, "wt")
	if err := repo.AddWorktree(ctx, wtPath, "wt-branch", true, ""); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	before, _ := repo.ListWorktrees(ctx)

	// Remove the worktree directory out from under git, then prune.
	if err := os.RemoveAll(wtPath); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if err := repo.PruneWorktrees(ctx); err != nil {
		t.Fatalf("PruneWorktrees: %v", err)
	}

	after, _ := repo.ListWorktrees(ctx)
	if len(after) >= len(before) {
		t.Errorf("after prune worktree count = %d, want < %d", len(after), len(before))
	}
}

func TestCommit_HookFormatterRetry(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	// Install a pre-commit hook that, on its first run, modifies a tracked file
	// and fails the commit (simulating a formatter). On the retry the marker
	// file already exists, so the hook lets the commit through.
	hooksDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	hook := "#!/bin/sh\n" +
		"if [ ! -f .formatter_ran ]; then\n" +
		"  echo formatted > .formatter_ran\n" +
		"  echo 'formatted, please re-stage' >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(hook), 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	// Stage a real change so there is something to commit.
	if err := os.WriteFile(filepath.Join(dir, "code.txt"), []byte("code\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = repo.StageAll(ctx)

	sha, err := repo.Commit(ctx, "commit with formatting hook")
	if err != nil {
		t.Fatalf("Commit with hook retry: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("commit SHA length = %d, want 40", len(sha))
	}
}

func TestCommit_NoChangesError(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	// Nothing staged and no changes => git commit fails, and there are no
	// uncommitted changes so the hook-formatter retry path is not taken.
	if _, err := repo.Commit(ctx, "nothing to commit"); err == nil {
		t.Error("Commit() with no changes = nil, want error")
	}
}

func TestIsRetryableGitError_Nil(t *testing.T) {
	if isRetryableGitError(nil) {
		t.Error("isRetryableGitError(nil) = true, want false")
	}
}

func TestStash_NoChangesIsHarmless(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	// `git stash` with nothing to stash exits 0 in modern git.
	if err := repo.Stash(ctx); err != nil {
		t.Fatalf("Stash with no changes: %v", err)
	}
}

func TestExec_GitAvailable(t *testing.T) {
	// Guard: these tests require a working git binary.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}
