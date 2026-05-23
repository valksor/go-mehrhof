package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupRepoWithRemote creates a working repo cloned from a bare "remote" repo,
// returning the working repo dir and the bare remote dir. The clone has an
// origin remote pointing at the bare repo so push/fetch/pull operations work
// without any network access.
func setupRepoWithRemote(t *testing.T) (string, string) {
	t.Helper()

	base := t.TempDir()
	remoteDir := filepath.Join(base, "remote.git")
	seedDir := filepath.Join(base, "seed")

	runGit := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil { //nolint:noctx // test setup
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Create a bare remote.
	runGit("init", "--bare", "-b", "main", remoteDir)

	// Seed it with an initial commit via a throwaway clone.
	runGit("init", "-b", "main", seedDir)
	runGit("-C", seedDir, "config", "user.email", "test@test.com")
	runGit("-C", seedDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte("# seed\n"), 0o644); err != nil {
		t.Fatalf("write seed README: %v", err)
	}
	runGit("-C", seedDir, "add", ".")
	runGit("-C", seedDir, "commit", "-m", "initial commit")
	runGit("-C", seedDir, "remote", "add", "origin", remoteDir)
	runGit("-C", seedDir, "push", "-u", "origin", "main")

	// Clone the remote into the working dir we will actually test against.
	workDir := filepath.Join(base, "work")
	runGit("clone", remoteDir, workDir)
	runGit("-C", workDir, "config", "user.email", "test@test.com")
	runGit("-C", workDir, "config", "user.name", "Test User")

	return workDir, remoteDir
}

func TestCheckoutAlias(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	orig, _ := repo.CurrentBranch(ctx)

	if err := repo.CreateBranch(ctx, "feature", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := repo.Checkout(ctx, orig); err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	cur, _ := repo.CurrentBranch(ctx)
	if cur != orig {
		t.Errorf("after Checkout, branch = %q, want %q", cur, orig)
	}
}

func TestMerge(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	base, _ := repo.CurrentBranch(ctx)

	// Create a feature branch with a new commit.
	if err := repo.CreateBranch(ctx, "feature", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = repo.StageAll(ctx)
	if _, err := repo.Commit(ctx, "feature work"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Back to base, then merge.
	if err := repo.SwitchBranch(ctx, base); err != nil {
		t.Fatalf("SwitchBranch: %v", err)
	}
	if err := repo.Merge(ctx, "feature", "merge feature"); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	// The feature file should now exist on the base branch.
	if _, err := os.Stat(filepath.Join(dir, "feature.txt")); err != nil {
		t.Errorf("expected feature.txt after merge: %v", err)
	}

	// A --no-ff merge creates a merge commit, so there are >=3 commits.
	entries, _ := repo.Log(ctx, 10)
	if len(entries) < 3 {
		t.Errorf("expected >=3 log entries after no-ff merge, got %d", len(entries))
	}
}

func TestMerge_NonexistentBranch(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	if err := repo.Merge(ctx, "does-not-exist", "msg"); err == nil {
		t.Error("Merge() should fail for nonexistent branch")
	}
}

func TestBranchExists(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	cur, _ := repo.CurrentBranch(ctx)

	if !repo.BranchExists(ctx, cur) {
		t.Errorf("BranchExists(%q) = false, want true", cur)
	}
	if repo.BranchExists(ctx, "nope-not-here") {
		t.Error("BranchExists(nope) = true, want false")
	}
}

func TestSigningConfig(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	// Establish a known baseline: explicitly disable repo-local signing so the
	// test is independent of any inherited global git config.
	cmdOff := exec.Command("git", "-C", dir, "config", "commit.gpgsign", "false") //nolint:noctx // test setup
	if out, err := cmdOff.CombinedOutput(); err != nil {
		t.Fatalf("disable gpgsign: %v\n%s", err, out)
	}
	if repo.IsSigningConfigured(ctx) {
		t.Error("IsSigningConfigured() = true when disabled, want false")
	}

	// Enable via git config and re-check.
	cmd := exec.Command("git", "-C", dir, "config", "commit.gpgsign", "true") //nolint:noctx // test setup
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("set gpgsign: %v\n%s", err, out)
	}
	if !repo.IsSigningConfigured(ctx) {
		t.Error("IsSigningConfigured() = false after enabling, want true")
	}

	// SetSignCommits is a pure setter; ensure it does not panic and toggles state.
	repo.SetSignCommits(true)
	repo.SetSignCommits(false)
}

func TestPushDefault_AndCommitsBehind(t *testing.T) {
	ctx := context.Background()
	workDir, _ := setupRepoWithRemote(t)

	repo, err := Open(workDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Initially in sync with origin/main.
	behind, err := repo.CommitsBehind(ctx, "origin/main")
	if err != nil {
		t.Fatalf("CommitsBehind: %v", err)
	}
	if behind != 0 {
		t.Errorf("CommitsBehind = %d, want 0", behind)
	}

	// Make a local commit and push it.
	if err := os.WriteFile(filepath.Join(workDir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = repo.StageAll(ctx)
	if _, err := repo.Commit(ctx, "add a"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := repo.PushDefault(ctx); err != nil {
		t.Fatalf("PushDefault: %v", err)
	}

	// After pushing, fetch then compare again — still not behind.
	if err := repo.Fetch(ctx); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	behind, err = repo.CommitsBehind(ctx, "origin/main")
	if err != nil {
		t.Fatalf("CommitsBehind after push: %v", err)
	}
	if behind != 0 {
		t.Errorf("CommitsBehind after push = %d, want 0", behind)
	}
}

func TestPull(t *testing.T) {
	ctx := context.Background()
	workDir, remoteDir := setupRepoWithRemote(t)

	// Use a second clone to push a new commit to the remote, then pull it in
	// the working repo.
	other := filepath.Join(t.TempDir(), "other")
	runGit := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil { //nolint:noctx // test setup
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("clone", remoteDir, other)
	runGit("-C", other, "config", "user.email", "other@test.com")
	runGit("-C", other, "config", "user.name", "Other")
	if err := os.WriteFile(filepath.Join(other, "remote.txt"), []byte("remote\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGit("-C", other, "add", ".")
	runGit("-C", other, "commit", "-m", "remote commit")
	runGit("-C", other, "push", "origin", "main")

	repo, _ := Open(workDir)
	if err := repo.Pull(ctx); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "remote.txt")); err != nil {
		t.Errorf("expected remote.txt after pull: %v", err)
	}
}

func TestDeleteRemoteBranch(t *testing.T) {
	ctx := context.Background()
	workDir, _ := setupRepoWithRemote(t)

	repo, _ := Open(workDir)

	// Push a new branch to origin.
	if err := repo.CreateBranch(ctx, "throwaway", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := repo.Push(ctx, "origin", "throwaway"); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Now delete it from the remote.
	if err := repo.DeleteRemoteBranch(ctx, "throwaway"); err != nil {
		t.Fatalf("DeleteRemoteBranch: %v", err)
	}

	// Deleting a branch that no longer exists on the remote should error.
	if err := repo.DeleteRemoteBranch(ctx, "never-existed"); err == nil {
		t.Error("DeleteRemoteBranch(never-existed) = nil, want error")
	}
}

func TestDefaultBranch_RemoteHEAD(t *testing.T) {
	ctx := context.Background()
	workDir, _ := setupRepoWithRemote(t)

	repo, _ := Open(workDir)

	// A clone has origin/HEAD set to the remote's default branch (main).
	branch, err := repo.DefaultBranch(ctx)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("DefaultBranch = %q, want main", branch)
	}
}

func TestFetch_NoRemote(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	// `git fetch` with no configured remote is a successful no-op; it should
	// not surface an error. (The error path is exercised elsewhere by Pull.)
	if err := repo.Fetch(ctx); err != nil {
		t.Errorf("Fetch() with no remote = %v, want nil (no-op)", err)
	}
}

func TestPull_NoRemote(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	if err := repo.Pull(ctx); err == nil {
		t.Error("Pull() with no remote = nil, want error")
	}
}

func TestCommitsBehind_BadRef(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	if _, err := repo.CommitsBehind(ctx, "origin/nonexistent"); err == nil {
		t.Error("CommitsBehind(bad ref) = nil error, want error")
	}
}
