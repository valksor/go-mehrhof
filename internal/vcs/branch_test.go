package vcs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initTestRepo initializes a git repository for testing.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()

	if err := runGit(ctx, dir, "init"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	if err := runGit(ctx, dir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config: %v", err)
	}
	if err := runGit(ctx, dir, "config", "user.name", "Test"); err != nil {
		t.Fatalf("git config: %v", err)
	}

	// Create initial commit
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := runGit(ctx, dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := runGit(ctx, dir, "commit", "-m", "initial"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	return dir
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2020-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2020-01-01T00:00:00Z",
	)
	return cmd.Run()
}

func TestCreateBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = g.CreateBranch("feature/test", "")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	// Should have switched to new branch
	current, err := g.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if current != "feature/test" {
		t.Errorf("CurrentBranch = %q, want %q", current, "feature/test")
	}

	// Branch should exist
	if !g.BranchExists("feature/test") {
		t.Error("feature/test branch should exist")
	}
}

func TestCreateBranchWithBase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Get current branch name first
	baseBranch, _ := g.CurrentBranch()

	err = g.CreateBranch("feature/from-base", baseBranch)
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	if !g.BranchExists("feature/from-base") {
		t.Error("branch should exist")
	}
}

func TestCreateBranchNoCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	originalBranch, _ := g.CurrentBranch()

	err = g.CreateBranchNoCheckout("feature/no-checkout", "")
	if err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}

	// Should NOT have switched branches
	current, _ := g.CurrentBranch()
	if current != originalBranch {
		t.Errorf("should still be on %s, but on %s", originalBranch, current)
	}

	// Branch should exist
	if !g.BranchExists("feature/no-checkout") {
		t.Error("branch should exist")
	}
}

func TestDeleteBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create branch first
	if err := g.CreateBranchNoCheckout("to-delete", ""); err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}
	if !g.BranchExists("to-delete") {
		t.Fatal("branch should exist before delete")
	}

	err = g.DeleteBranch("to-delete", false)
	if err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	if g.BranchExists("to-delete") {
		t.Error("branch should not exist after delete")
	}
}

func TestDeleteBranchForce(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// Create branch with a commit
	if err := g.CreateBranch("force-delete", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(filepath.Join(dir, "new.txt")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("new commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Switch back to base
	if err := g.Checkout(baseBranch); err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	// Force delete should work
	err = g.DeleteBranch("force-delete", true)
	if err != nil {
		t.Errorf("force delete should work: %v", err)
	}
}

func TestBranchExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	if !g.BranchExists(baseBranch) {
		t.Errorf("%s should exist", baseBranch)
	}

	if g.BranchExists("nonexistent-branch") {
		t.Error("nonexistent-branch should not exist")
	}
}

func TestRemoteBranchExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// No remote configured, should return false
	if g.RemoteBranchExists("origin", "main") {
		t.Error("remote branch should not exist without remote")
	}
}

func TestListBranches(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create some branches
	if err := g.CreateBranchNoCheckout("branch-a", ""); err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}
	if err := g.CreateBranchNoCheckout("branch-b", ""); err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}

	branches, err := g.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}

	if len(branches) < 3 {
		t.Errorf("expected at least 3 branches, got %d", len(branches))
	}

	// Check one is marked current
	hasCurrent := false
	for _, b := range branches {
		if b.IsCurrent {
			hasCurrent = true
			break
		}
	}
	if !hasCurrent {
		t.Error("no branch marked as current")
	}
}

func TestGetBaseBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	base, err := g.GetBaseBranch()
	if err != nil {
		t.Fatalf("GetBaseBranch: %v", err)
	}

	// Should be main or master (depends on git version)
	if base != "main" && base != "master" {
		t.Errorf("base branch = %q, expected main or master", base)
	}
}

func TestRenameBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := g.CreateBranchNoCheckout("old-name", ""); err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}
	if !g.BranchExists("old-name") {
		t.Fatal("branch should exist before rename")
	}

	err = g.RenameBranch("old-name", "new-name")
	if err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}

	if g.BranchExists("old-name") {
		t.Error("old name should not exist")
	}
	if !g.BranchExists("new-name") {
		t.Error("new name should exist")
	}
}

func TestGetTrackingBranch_NoRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	_, err = g.GetTrackingBranch(baseBranch)
	if err == nil {
		t.Error("GetTrackingBranch should fail without remote")
	}
}

func TestCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	if err := g.CreateBranchNoCheckout("other", ""); err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}
	err = g.Checkout("other")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	current, _ := g.CurrentBranch()
	if current != "other" {
		t.Errorf("current = %q, want %q", current, "other")
	}

	// Checkout back
	err = g.Checkout(baseBranch)
	if err != nil {
		t.Fatalf("Checkout back: %v", err)
	}
}

func TestMergeBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// Create feature branch with a commit
	if err := g.CreateBranch("feature/merge-test", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(filepath.Join(dir, "feature.txt")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("feature commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Switch back to base and merge
	if err := g.Checkout(baseBranch); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	err = g.MergeBranch("feature/merge-test", false) // noFF=false for fast-forward merge
	if err != nil {
		t.Fatalf("MergeBranch: %v", err)
	}

	// Verify file exists after merge
	if _, err := os.Stat(filepath.Join(dir, "feature.txt")); os.IsNotExist(err) {
		t.Error("feature.txt should exist after merge")
	}
}

func TestMergeSquash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// Create feature branch with multiple commits
	if err := g.CreateBranch("feature/squash-test", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(filepath.Join(dir, "file1.txt")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("commit 1"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(filepath.Join(dir, "file2.txt")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("commit 2"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Switch back to base and squash merge
	if err := g.Checkout(baseBranch); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	err = g.MergeSquash("feature/squash-test")
	if err != nil {
		t.Fatalf("MergeSquash: %v", err)
	}

	// Files should be staged but not committed
	status, _ := g.Status()
	if len(status) == 0 {
		t.Error("should have staged files after squash")
	}
}

func TestSetTrackingBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// This should fail without a remote - SetTrackingBranch(local, remote, branch)
	err = g.SetTrackingBranch(baseBranch, "origin", "main")
	if err == nil {
		t.Error("SetTrackingBranch should fail without remote")
	}
}

func TestGetBranchCommitCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// Create branch with additional commits
	if err := g.CreateBranch("feature/count-test", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f1.txt"), []byte("1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(filepath.Join(dir, "f1.txt")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("commit 1"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f2.txt"), []byte("2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(filepath.Join(dir, "f2.txt")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("commit 2"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	count, err := g.GetBranchCommitCount("feature/count-test", baseBranch)
	if err != nil {
		t.Fatalf("GetBranchCommitCount: %v", err)
	}

	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestGetMergeBase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// Get the current HEAD commit
	baseCommit, _ := g.RevParse("HEAD")

	// Create feature branch
	if err := g.CreateBranch("feature/merge-base-test", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(filepath.Join(dir, "f.txt")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("feature commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	mergeBase, err := g.GetMergeBase("feature/merge-base-test", baseBranch)
	if err != nil {
		t.Fatalf("GetMergeBase: %v", err)
	}

	// Merge base should be the original base commit
	if mergeBase != baseCommit {
		t.Errorf("mergeBase = %q, want %q", mergeBase, baseCommit)
	}
}

func TestIsMerged(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// Create and merge a branch
	if err := g.CreateBranch("feature/merged", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "merged.txt"), []byte("1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(filepath.Join(dir, "merged.txt")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("merged commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := g.Checkout(baseBranch); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if err := g.MergeBranch("feature/merged", false); err != nil {
		t.Fatalf("MergeBranch: %v", err)
	}

	merged, err := g.IsMerged("feature/merged", baseBranch)
	if err != nil {
		t.Fatalf("IsMerged: %v", err)
	}

	if !merged {
		t.Error("branch should be merged")
	}

	// Create an unmerged branch
	if err := g.CreateBranchNoCheckout("feature/unmerged", ""); err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}
	unmerged, err := g.IsMerged("feature/unmerged", baseBranch)
	if err != nil {
		t.Fatalf("IsMerged (unmerged): %v", err)
	}

	// Note: a branch with no additional commits IS considered merged
	// because all its commits are reachable from the target
	if !unmerged {
		t.Log("empty branch is technically merged (all commits reachable)")
	}
}

func TestGetAheadBehind(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// Create feature branch with commits
	if err := g.CreateBranch("feature/ahead-test", ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a1.txt"), []byte("1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(filepath.Join(dir, "a1.txt")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("commit 1"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a2.txt"), []byte("2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(filepath.Join(dir, "a2.txt")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("commit 2"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	ahead, behind, err := g.GetAheadBehind("feature/ahead-test", baseBranch)
	if err != nil {
		t.Fatalf("GetAheadBehind: %v", err)
	}

	if ahead != 2 {
		t.Errorf("ahead = %d, want 2", ahead)
	}
	if behind != 0 {
		t.Errorf("behind = %d, want 0", behind)
	}
}

func TestPushBranch_NoRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// Push should fail without a remote
	err = g.PushBranch(baseBranch, "origin", false)
	if err == nil {
		t.Error("PushBranch should fail without remote")
	}
}

func TestForcePushBranch_NoRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// Force push should fail without a remote
	err = g.ForcePushBranch(baseBranch, "origin")
	if err == nil {
		t.Error("ForcePushBranch should fail without remote")
	}
}
