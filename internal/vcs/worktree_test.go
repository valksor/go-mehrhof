package vcs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreeStruct(t *testing.T) {
	wt := Worktree{
		Path:   "/path/to/worktree",
		Branch: "feature/test",
		Commit: "abc123",
		Bare:   false,
		Main:   true,
	}

	if wt.Path != "/path/to/worktree" {
		t.Errorf("Path = %q, want %q", wt.Path, "/path/to/worktree")
	}
	if wt.Branch != "feature/test" {
		t.Errorf("Branch = %q, want %q", wt.Branch, "feature/test")
	}
	if wt.Commit != "abc123" {
		t.Errorf("Commit = %q, want %q", wt.Commit, "abc123")
	}
	if wt.Bare {
		t.Error("Bare should be false")
	}
	if !wt.Main {
		t.Error("Main should be true")
	}
}

func TestListWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	worktrees, err := g.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}

	// Should have at least the main worktree
	if len(worktrees) < 1 {
		t.Fatal("expected at least 1 worktree")
	}

	// First worktree should be marked as main
	if !worktrees[0].Main {
		t.Error("first worktree should be marked as main")
	}

	// Main worktree should have the repo path (resolve symlinks for comparison on macOS)
	expectedPath, _ := filepath.EvalSymlinks(dir)
	actualPath, _ := filepath.EvalSymlinks(worktrees[0].Path)
	if actualPath != expectedPath {
		t.Errorf("main worktree path = %q, want %q", actualPath, expectedPath)
	}
}

func TestCreateWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create a branch for the worktree
	if err := g.CreateBranchNoCheckout(ctx, "worktree-branch", ""); err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}

	// Create worktree
	wtPath := filepath.Join(t.TempDir(), "worktree1")
	err = g.CreateWorktree(ctx, wtPath, "worktree-branch")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory should exist")
	}

	// Verify it's listed (resolve symlinks for comparison on macOS)
	worktrees, _ := g.ListWorktrees(ctx)
	expectedPath, _ := filepath.EvalSymlinks(wtPath)
	found := false
	for _, wt := range worktrees {
		actualPath, _ := filepath.EvalSymlinks(wt.Path)
		if actualPath == expectedPath {
			found = true
			if wt.Branch != "worktree-branch" {
				t.Errorf("worktree branch = %q, want %q", wt.Branch, "worktree-branch")
			}
			break
		}
	}
	if !found {
		t.Error("created worktree not found in list")
	}
}

func TestCreateWorktreeNewBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch(ctx)

	// Create worktree with new branch
	wtPath := filepath.Join(t.TempDir(), "worktree-new")
	err = g.CreateWorktreeNewBranch(ctx, wtPath, "new-branch", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktreeNewBranch: %v", err)
	}

	// Verify branch exists
	if !g.BranchExists(ctx, "new-branch") {
		t.Error("new-branch should exist")
	}

	// Verify worktree exists with branch
	worktrees, _ := g.ListWorktrees(ctx)
	found := false
	for _, wt := range worktrees {
		if wt.Branch == "new-branch" {
			found = true
			break
		}
	}
	if !found {
		t.Error("worktree with new-branch not found")
	}
}

func TestCreateWorktreeNewBranch_NoBase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create worktree with new branch, no explicit base
	wtPath := filepath.Join(t.TempDir(), "worktree-nobase")
	err = g.CreateWorktreeNewBranch(ctx, wtPath, "nobase-branch", "")
	if err != nil {
		t.Fatalf("CreateWorktreeNewBranch: %v", err)
	}

	// Verify branch exists
	if !g.BranchExists(ctx, "nobase-branch") {
		t.Error("nobase-branch should exist")
	}
}

func TestRemoveWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create a worktree first
	if err := g.CreateBranchNoCheckout(ctx, "to-remove-branch", ""); err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}
	wtPath := filepath.Join(t.TempDir(), "to-remove")
	if err := g.CreateWorktree(ctx, wtPath, "to-remove-branch"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Remove it
	err = g.RemoveWorktree(ctx, wtPath, false)
	if err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	// Verify it's no longer listed
	worktrees, _ := g.ListWorktrees(ctx)
	for _, wt := range worktrees {
		if wt.Path == wtPath {
			t.Error("removed worktree should not be in list")
		}
	}
}

func TestRemoveWorktree_Force(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create a worktree
	if err := g.CreateBranchNoCheckout(ctx, "force-remove-branch", ""); err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}
	wtPath := filepath.Join(t.TempDir(), "force-remove")
	if err := g.CreateWorktree(ctx, wtPath, "force-remove-branch"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Make some uncommitted changes in the worktree
	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Force remove should work even with dirty files
	err = g.RemoveWorktree(ctx, wtPath, true)
	if err != nil {
		t.Fatalf("RemoveWorktree (force): %v", err)
	}
}

func TestPruneWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Prune should work even with no stale worktrees
	err = g.PruneWorktrees(ctx)
	if err != nil {
		t.Fatalf("PruneWorktrees: %v", err)
	}
}

func TestGetWorktreeForBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch(ctx)

	// Main worktree should be found for current branch
	wt, err := g.GetWorktreeForBranch(ctx, baseBranch)
	if err != nil {
		t.Fatalf("GetWorktreeForBranch: %v", err)
	}

	// Resolve symlinks for comparison on macOS
	expectedPath, _ := filepath.EvalSymlinks(dir)
	actualPath, _ := filepath.EvalSymlinks(wt.Path)
	if actualPath != expectedPath {
		t.Errorf("worktree path = %q, want %q", actualPath, expectedPath)
	}
}

func TestGetWorktreeForBranch_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Non-existent branch should return error
	_, err = g.GetWorktreeForBranch(ctx, "nonexistent-branch-xyz")
	if err == nil {
		t.Error("GetWorktreeForBranch should fail for non-existent branch")
	}
}

func TestWorktreeExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Get the actual worktree path from listing (avoids symlink issues)
	worktrees, err := g.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(worktrees) == 0 {
		t.Fatal("expected at least one worktree")
	}

	// Main repo path should exist as worktree (use the path from ListWorktrees)
	if !g.WorktreeExists(ctx, worktrees[0].Path) {
		t.Error("main repo should exist as worktree")
	}

	// Random path should not exist
	if g.WorktreeExists(ctx, "/nonexistent/path/xyz") {
		t.Error("nonexistent path should not be a worktree")
	}
}

func TestGetWorktreePath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	path := g.GetWorktreePath("task-123")

	// Should contain the task ID
	if filepath.Base(path) != "task-123" {
		t.Errorf("path basename = %q, want %q", filepath.Base(path), "task-123")
	}

	// Should be in a worktrees directory
	parentDir := filepath.Dir(path)
	if !filepath.IsAbs(path) {
		t.Error("path should be absolute")
	}
	_ = parentDir // Parent should be repo-worktrees
}

func TestEnsureWorktreesDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = g.EnsureWorktreesDir()
	if err != nil {
		t.Fatalf("EnsureWorktreesDir: %v", err)
	}

	// Worktrees directory should exist
	repoName := filepath.Base(dir)
	parent := filepath.Dir(dir)
	worktreesDir := filepath.Join(parent, repoName+"-worktrees")

	if _, err := os.Stat(worktreesDir); os.IsNotExist(err) {
		t.Error("worktrees directory should exist")
	}
}

func TestIsWorktree_MainRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Main repo should NOT be a worktree
	if g.IsWorktree() {
		t.Error("main repo should not be identified as worktree")
	}
}

func TestIsWorktree_ActualWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create a worktree
	if err := g.CreateBranchNoCheckout(ctx, "wt-test-branch", ""); err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}
	wtPath := filepath.Join(t.TempDir(), "wt-test")
	if err := g.CreateWorktree(ctx, wtPath, "wt-test-branch"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Create a Git instance from the worktree
	wtGit, err := New(ctx, wtPath)
	if err != nil {
		t.Fatalf("New for worktree: %v", err)
	}

	// Worktree SHOULD be identified as worktree
	if !wtGit.IsWorktree() {
		t.Error("worktree should be identified as worktree")
	}
}

func TestGetMainWorktreePath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create a worktree
	if err := g.CreateBranchNoCheckout(ctx, "main-path-test-branch", ""); err != nil {
		t.Fatalf("CreateBranchNoCheckout: %v", err)
	}
	wtPath := filepath.Join(t.TempDir(), "main-path-test")
	if err := g.CreateWorktree(ctx, wtPath, "main-path-test-branch"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Create a Git instance from the worktree
	wtGit, err := New(ctx, wtPath)
	if err != nil {
		t.Fatalf("New for worktree: %v", err)
	}

	// Get main repo path from worktree
	mainPath, err := wtGit.GetMainWorktreePath(ctx)
	if err != nil {
		t.Fatalf("GetMainWorktreePath: %v", err)
	}

	// Resolve symlinks for comparison on macOS
	expectedPath, _ := filepath.EvalSymlinks(dir)
	actualPath, _ := filepath.EvalSymlinks(mainPath)

	if actualPath != expectedPath {
		t.Errorf("main path = %q, want %q", actualPath, expectedPath)
	}
}

func TestGetMainWorktreePath_NotWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Should error when called on main repo
	_, err = g.GetMainWorktreePath(ctx)
	if err == nil {
		t.Error("GetMainWorktreePath should fail on main repo")
	}
}
