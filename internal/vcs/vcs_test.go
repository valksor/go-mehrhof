package vcs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBranchStruct(t *testing.T) {
	b := Branch{
		Name:      "feature/test",
		Remote:    "origin",
		IsCurrent: true,
		Commit:    "abc123def456",
	}

	if b.Name != "feature/test" {
		t.Errorf("Branch.Name = %q, want %q", b.Name, "feature/test")
	}
	if b.Remote != "origin" {
		t.Errorf("Branch.Remote = %q, want %q", b.Remote, "origin")
	}
	if !b.IsCurrent {
		t.Error("Branch.IsCurrent = false, want true")
	}
	if b.Commit != "abc123def456" {
		t.Errorf("Branch.Commit = %q, want %q", b.Commit, "abc123def456")
	}
}

func TestCheckpointStruct(t *testing.T) {
	now := time.Now()
	cp := Checkpoint{
		ID:        "abc123",
		TaskID:    "task-001",
		Number:    5,
		Message:   "Test checkpoint",
		Timestamp: now,
	}

	if cp.ID != "abc123" {
		t.Errorf("Checkpoint.ID = %q, want %q", cp.ID, "abc123")
	}
	if cp.TaskID != "task-001" {
		t.Errorf("Checkpoint.TaskID = %q, want %q", cp.TaskID, "task-001")
	}
	if cp.Number != 5 {
		t.Errorf("Checkpoint.Number = %d, want 5", cp.Number)
	}
	if cp.Message != "Test checkpoint" {
		t.Errorf("Checkpoint.Message = %q, want %q", cp.Message, "Test checkpoint")
	}
}

func TestCheckpointPrefix(t *testing.T) {
	if CheckpointPrefix != "task-checkpoint" {
		t.Errorf("CheckpointPrefix = %q, want %q", CheckpointPrefix, "task-checkpoint")
	}
}

func TestFileStatusStruct(t *testing.T) {
	fs := FileStatus{
		Path:    "test.go",
		Index:   'M',
		WorkDir: ' ',
	}

	if fs.Path != "test.go" {
		t.Errorf("FileStatus.Path = %q, want %q", fs.Path, "test.go")
	}
	if fs.Index != 'M' {
		t.Errorf("FileStatus.Index = %c, want %c", fs.Index, 'M')
	}
}

func TestFileStatusIsStaged(t *testing.T) {
	tests := []struct {
		name   string
		status FileStatus
		want   bool
	}{
		{
			name:   "staged modified",
			status: FileStatus{Index: 'M', WorkDir: ' '},
			want:   true,
		},
		{
			name:   "staged added",
			status: FileStatus{Index: 'A', WorkDir: ' '},
			want:   true,
		},
		{
			name:   "not staged",
			status: FileStatus{Index: ' ', WorkDir: 'M'},
			want:   false,
		},
		{
			name:   "untracked",
			status: FileStatus{Index: '?', WorkDir: '?'},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsStaged()
			if got != tt.want {
				t.Errorf("IsStaged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileStatusIsModified(t *testing.T) {
	tests := []struct {
		name   string
		status FileStatus
		want   bool
	}{
		{
			name:   "modified in workdir",
			status: FileStatus{Index: ' ', WorkDir: 'M'},
			want:   true,
		},
		{
			name:   "deleted in workdir",
			status: FileStatus{Index: ' ', WorkDir: 'D'},
			want:   true,
		},
		{
			name:   "not modified",
			status: FileStatus{Index: ' ', WorkDir: ' '},
			want:   false,
		},
		{
			name:   "staged only",
			status: FileStatus{Index: 'M', WorkDir: ' '},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsModified()
			if got != tt.want {
				t.Errorf("IsModified() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Key:   "user.email",
		Value: "test@example.com",
	}

	if cfg.Key != "user.email" {
		t.Errorf("Config.Key = %q, want %q", cfg.Key, "user.email")
	}
	if cfg.Value != "test@example.com" {
		t.Errorf("Config.Value = %q, want %q", cfg.Value, "test@example.com")
	}
}

func TestIsRepo(t *testing.T) {
	// Test with non-repo directory
	tmpDir := t.TempDir()
	if IsRepo(tmpDir) {
		t.Error("IsRepo should return false for non-git directory")
	}

	// Test with non-existent directory
	if IsRepo("/nonexistent/path") {
		t.Error("IsRepo should return false for non-existent path")
	}
}

func TestNewGitNonRepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := New(tmpDir)
	if err == nil {
		t.Error("New should fail for non-git directory")
	}
}

func TestNewCheckpointTracker(t *testing.T) {
	tracker := NewCheckpointTracker(nil, "test-task")

	if tracker == nil {
		t.Fatal("NewCheckpointTracker returned nil")
	}
	if tracker.taskID != "test-task" {
		t.Errorf("tracker.taskID = %q, want %q", tracker.taskID, "test-task")
	}
}

// Integration tests that require a real git repo
func TestGitIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary directory and initialize git repo
	tmpDir := t.TempDir()

	// Initialize git repo
	if err := runGitInit(tmpDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Configure git user for commits
	ctx := context.Background()
	if _, err := runGitCommandContext(ctx, tmpDir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
	if _, err := runGitCommandContext(ctx, tmpDir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", testFile, err)
	}
	if _, err := runGitCommandContext(ctx, tmpDir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := runGitCommandContext(ctx, tmpDir, "commit", "-m", "initial commit"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	// Now test Git operations
	g, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	t.Run("Root", func(t *testing.T) {
		root := g.Root()
		// Resolve symlinks for comparison (macOS uses /private/var symlink)
		expectedRoot, _ := filepath.EvalSymlinks(tmpDir)
		actualRoot, _ := filepath.EvalSymlinks(root)
		if actualRoot != expectedRoot {
			t.Errorf("Root() = %q, want %q", actualRoot, expectedRoot)
		}
	})

	t.Run("CurrentBranch", func(t *testing.T) {
		branch, err := g.CurrentBranch()
		if err != nil {
			t.Fatalf("CurrentBranch failed: %v", err)
		}
		// Could be "main" or "master" depending on git config
		if branch != "main" && branch != "master" {
			t.Errorf("CurrentBranch() = %q, want main or master", branch)
		}
	})

	t.Run("HasChanges_NoChanges", func(t *testing.T) {
		hasChanges, err := g.HasChanges()
		if err != nil {
			t.Fatalf("HasChanges failed: %v", err)
		}
		if hasChanges {
			t.Error("HasChanges() = true, want false (no changes)")
		}
	})

	t.Run("HasChanges_WithChanges", func(t *testing.T) {
		// Make a change
		if err := os.WriteFile(testFile, []byte("modified content"), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", testFile, err)
		}

		hasChanges, err := g.HasChanges()
		if err != nil {
			t.Fatalf("HasChanges failed: %v", err)
		}
		if !hasChanges {
			t.Error("HasChanges() = false, want true (has changes)")
		}

		// Reset for other tests
		_, resetErr := g.run("checkout", "--", testFile)
		if resetErr != nil {
			t.Fatalf("git checkout %s: %v", testFile, resetErr)
		}
	})

	t.Run("Status", func(t *testing.T) {
		// Make a change
		if err := os.WriteFile(testFile, []byte("modified content"), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", testFile, err)
		}

		status, err := g.Status()
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}
		if len(status) == 0 {
			t.Error("Status returned empty, expected changes")
		}

		// Reset
		_, err = g.run("checkout", "--", testFile)
		if err != nil {
			t.Fatalf("git checkout %s: %v", testFile, err)
		}
	})

	t.Run("BranchExists", func(t *testing.T) {
		branch, _ := g.CurrentBranch()
		if !g.BranchExists(branch) {
			t.Errorf("BranchExists(%q) = false, want true", branch)
		}
		if g.BranchExists("nonexistent-branch") {
			t.Error("BranchExists(nonexistent) = true, want false")
		}
	})

	t.Run("CreateAndDeleteBranch", func(t *testing.T) {
		err := g.CreateBranchNoCheckout("test-branch", "")
		if err != nil {
			t.Fatalf("CreateBranchNoCheckout failed: %v", err)
		}

		if !g.BranchExists("test-branch") {
			t.Error("Branch was not created")
		}

		err = g.DeleteBranch("test-branch", false)
		if err != nil {
			t.Fatalf("DeleteBranch failed: %v", err)
		}

		if g.BranchExists("test-branch") {
			t.Error("Branch was not deleted")
		}
	})

	t.Run("ListBranches", func(t *testing.T) {
		branches, err := g.ListBranches()
		if err != nil {
			t.Fatalf("ListBranches failed: %v", err)
		}
		if len(branches) == 0 {
			t.Error("ListBranches returned empty")
		}

		// Check that current branch is marked
		hasCurrent := false
		for _, b := range branches {
			if b.IsCurrent {
				hasCurrent = true
				break
			}
		}
		if !hasCurrent {
			t.Error("No branch marked as current")
		}
	})

	t.Run("RevParse", func(t *testing.T) {
		hash, err := g.RevParse("HEAD")
		if err != nil {
			t.Fatalf("RevParse failed: %v", err)
		}
		if len(hash) < 7 {
			t.Errorf("RevParse returned short hash: %q", hash)
		}
	})

	t.Run("GetCommitMessage", func(t *testing.T) {
		msg, err := g.GetCommitMessage("HEAD")
		if err != nil {
			t.Fatalf("GetCommitMessage failed: %v", err)
		}
		if msg == "" {
			t.Error("GetCommitMessage returned empty")
		}
	})

	t.Run("AddAndCommit", func(t *testing.T) {
		// Create new file
		newFile := filepath.Join(tmpDir, "new.txt")
		if err := os.WriteFile(newFile, []byte("new content"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", newFile, err)
		}

		err := g.Add(newFile)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		hash, err := g.Commit("test commit")
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
		if hash == "" {
			t.Error("Commit returned empty hash")
		}
	})

	t.Run("GetConfig", func(t *testing.T) {
		email, err := g.GetConfig("user.email")
		if err != nil {
			t.Fatalf("GetConfig failed: %v", err)
		}
		if email != "test@example.com" {
			t.Errorf("GetConfig(user.email) = %q, want %q", email, "test@example.com")
		}
	})

	t.Run("SetConfig", func(t *testing.T) {
		err := g.SetConfig("test.key", "test-value")
		if err != nil {
			t.Fatalf("SetConfig failed: %v", err)
		}

		val, err := g.GetConfig("test.key")
		if err != nil {
			t.Fatalf("GetConfig failed: %v", err)
		}
		if val != "test-value" {
			t.Errorf("GetConfig(test.key) = %q, want %q", val, "test-value")
		}
	})

	t.Run("Diff", func(t *testing.T) {
		// Make a change
		if err := os.WriteFile(testFile, []byte("diff content"), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", testFile, err)
		}

		diff, err := g.Diff()
		if err != nil {
			t.Fatalf("Diff failed: %v", err)
		}
		if diff == "" {
			t.Error("Diff returned empty, expected changes")
		}

		// Reset
		_, err = g.run("checkout", "--", testFile)
		if err != nil {
			t.Fatalf("git checkout %s: %v", testFile, err)
		}
	})

	t.Run("Log", func(t *testing.T) {
		log, err := g.Log("-1")
		if err != nil {
			t.Fatalf("Log failed: %v", err)
		}
		if log == "" {
			t.Error("Log returned empty")
		}
	})
}

// Helper to initialize git repo
func runGitInit(dir string) error {
	_, err := runGitCommandContext(context.Background(), dir, "init")
	return err
}

// TestGitStatusConstants verifies our status parsing constants match git porcelain format
func TestGitStatusConstants(t *testing.T) {
	// Git porcelain v1 format: "XY PATH" where X=index, Y=worktree
	testEntry := "M  modified-file.go"

	if len(testEntry) < gitStatusMinLength {
		t.Errorf("test entry should be >= %d chars", gitStatusMinLength)
	}

	// Verify constants extract correct positions
	if testEntry[gitStatusIndexPos] != 'M' {
		t.Errorf("gitStatusIndexPos: got %c, want 'M'", testEntry[gitStatusIndexPos])
	}
	if testEntry[gitStatusWorkDirPos] != ' ' {
		t.Errorf("gitStatusWorkDirPos: got %c, want ' '", testEntry[gitStatusWorkDirPos])
	}
	expectedPath := "modified-file.go"
	actualPath := testEntry[gitStatusPathStart:]
	if actualPath != expectedPath {
		t.Errorf("gitStatusPathStart: got %q, want %q", actualPath, expectedPath)
	}
}

// TestCommitOptions tests the CommitOptions struct and functionality
func TestCommitOptions(t *testing.T) {
	// Test default CommitOptions
	opts := CommitOptions{}
	if opts.AllowEmpty {
		t.Error("CommitOptions.AllowEmpty should default to false")
	}

	// Test with AllowEmpty set
	opts = CommitOptions{AllowEmpty: true}
	if !opts.AllowEmpty {
		t.Error("CommitOptions.AllowEmpty should be true when set")
	}
}

// TestCommitWithOptions tests the Commit function with options (integration test)
func TestCommitWithOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Initialize git repo
	if err := runGitInit(tmpDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	ctx := context.Background()
	if _, err := runGitCommandContext(ctx, tmpDir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
	if _, err := runGitCommandContext(ctx, tmpDir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := runGitCommandContext(ctx, tmpDir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := runGitCommandContext(ctx, tmpDir, "commit", "-m", "initial"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	g, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Run("CommitWithoutOptions", func(t *testing.T) {
		// Make a change first
		if err := os.WriteFile(testFile, []byte("changed"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := g.Add(testFile); err != nil {
			t.Fatalf("Add: %v", err)
		}

		hash, err := g.Commit("regular commit")
		if err != nil {
			t.Fatalf("Commit: %v", err)
		}
		if hash == "" {
			t.Error("Commit returned empty hash")
		}
	})

	t.Run("CommitAllowEmpty", func(t *testing.T) {
		// No changes staged, but should succeed with AllowEmpty
		hash, err := g.Commit("empty commit", CommitOptions{AllowEmpty: true})
		if err != nil {
			t.Fatalf("Commit with AllowEmpty: %v", err)
		}
		if hash == "" {
			t.Error("Commit returned empty hash")
		}
	})

	t.Run("CommitAllowEmptyViaDeprecated", func(t *testing.T) {
		// Test backward compatibility
		hash, err := g.CommitAllowEmpty("deprecated empty commit")
		if err != nil {
			t.Fatalf("CommitAllowEmpty: %v", err)
		}
		if hash == "" {
			t.Error("CommitAllowEmpty returned empty hash")
		}
	})
}

func TestGetCommitAuthor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	author, err := g.GetCommitAuthor("HEAD")
	if err != nil {
		t.Fatalf("GetCommitAuthor: %v", err)
	}

	// Should contain the configured test email
	if author == "" {
		t.Error("GetCommitAuthor returned empty")
	}
}

func TestResetSoft(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Make a commit
	testFile := filepath.Join(dir, "soft.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(testFile); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("commit to reset"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Get current HEAD
	originalHead, _ := g.RevParse("HEAD")

	// Make another commit
	if err := os.WriteFile(testFile, []byte("changed"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(testFile); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("another commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Soft reset back
	err = g.ResetSoft(originalHead)
	if err != nil {
		t.Fatalf("ResetSoft: %v", err)
	}

	// HEAD should be back to original
	newHead, _ := g.RevParse("HEAD")
	if newHead != originalHead {
		t.Errorf("HEAD = %q, want %q", newHead, originalHead)
	}

	// But file should still be staged (soft reset keeps changes)
	status, _ := g.Status()
	if len(status) == 0 {
		t.Error("soft reset should keep changes staged")
	}
}

func TestClean(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create untracked file
	untrackedFile := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("untracked"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Clean without force (should work but may not remove files depending on git version)
	err = g.Clean(false)
	// Error expected on some git versions without -f
	_ = err

	// Create another untracked file
	untrackedFile2 := filepath.Join(dir, "untracked2.txt")
	if err := os.WriteFile(untrackedFile2, []byte("untracked2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Clean with force should remove it
	err = g.Clean(true)
	if err != nil {
		t.Fatalf("Clean(force): %v", err)
	}

	// File should be gone
	if _, err := os.Stat(untrackedFile2); !os.IsNotExist(err) {
		t.Error("untracked file should be removed after clean")
	}
}

func TestStashAndPop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Make a change
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Modified content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Stash with message
	err = g.Stash("test stash")
	if err != nil {
		t.Fatalf("Stash: %v", err)
	}

	// File should be reverted
	content, _ := os.ReadFile(testFile)
	if string(content) != "# Test\n" {
		t.Error("file should be reverted after stash")
	}

	// Pop the stash
	err = g.StashPop()
	if err != nil {
		t.Fatalf("StashPop: %v", err)
	}

	// File should be restored
	content, _ = os.ReadFile(testFile)
	if string(content) != "# Modified content\n" {
		t.Errorf("file content = %q, want %q", string(content), "# Modified content\n")
	}
}

func TestStash_NoMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Make a change
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# No message stash\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Stash without message
	err = g.Stash("")
	if err != nil {
		t.Fatalf("Stash (no message): %v", err)
	}

	// Pop it back
	if err := g.StashPop(); err != nil {
		t.Fatalf("StashPop: %v", err)
	}
}

func TestRemoteURL_NoRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Should fail without a remote
	_, err = g.RemoteURL("origin")
	if err == nil {
		t.Error("RemoteURL should fail without remote")
	}
}

func TestFetch_NoRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Should fail without a remote
	err = g.Fetch("origin")
	if err == nil {
		t.Error("Fetch should fail without remote")
	}
}

func TestPull_NoRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Should fail without a remote
	err = g.Pull("origin", "main")
	if err == nil {
		t.Error("Pull should fail without remote")
	}
}

func TestPush_NoRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// Should fail without a remote
	err = g.Push("origin", baseBranch)
	if err == nil {
		t.Error("Push should fail without remote")
	}
}

func TestRebaseBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	baseBranch, _ := g.CurrentBranch()

	// Create a feature branch with a commit
	if err := g.CreateBranch("feature/rebase", ""); err != nil {
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

	// Go back to base and make a commit
	if err := g.Checkout(baseBranch); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := g.Add(filepath.Join(dir, "base.txt")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := g.Commit("base commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Switch to feature and rebase onto base
	if err := g.Checkout("feature/rebase"); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	err = g.RebaseBranch(baseBranch)
	if err != nil {
		t.Fatalf("RebaseBranch: %v", err)
	}

	// Both files should exist after rebase
	if _, err := os.Stat(filepath.Join(dir, "feature.txt")); os.IsNotExist(err) {
		t.Error("feature.txt should exist after rebase")
	}
	if _, err := os.Stat(filepath.Join(dir, "base.txt")); os.IsNotExist(err) {
		t.Error("base.txt should exist after rebase")
	}
}

func TestAbortRebase_NoRebaseInProgress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Should fail when no rebase is in progress
	err = g.AbortRebase()
	if err == nil {
		t.Error("AbortRebase should fail when no rebase in progress")
	}
}

func TestContinueRebase_NoRebaseInProgress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Should fail when no rebase is in progress
	err = g.ContinueRebase()
	if err == nil {
		t.Error("ContinueRebase should fail when no rebase in progress")
	}
}

func TestRunContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	out, err := g.RunContext(ctx, "status", "--porcelain")
	if err != nil {
		t.Fatalf("RunContext: %v", err)
	}
	// Output could be empty if no changes - that's fine
	_ = out
}

func TestChangeSummaryStruct(t *testing.T) {
	summary := ChangeSummary{
		Added:    []string{"new.txt"},
		Modified: []string{"changed.txt"},
		Deleted:  []string{"removed.txt"},
		Total:    3,
	}

	if len(summary.Added) != 1 || summary.Added[0] != "new.txt" {
		t.Errorf("Added = %v, want [new.txt]", summary.Added)
	}
	if len(summary.Modified) != 1 || summary.Modified[0] != "changed.txt" {
		t.Errorf("Modified = %v, want [changed.txt]", summary.Modified)
	}
	if len(summary.Deleted) != 1 || summary.Deleted[0] != "removed.txt" {
		t.Errorf("Deleted = %v, want [removed.txt]", summary.Deleted)
	}
	if summary.Total != 3 {
		t.Errorf("Total = %d, want 3", summary.Total)
	}
}

func TestGetChangeSummary_WithDeletions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := initTestRepo(t)
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Delete the README
	if err := os.Remove(filepath.Join(dir, "README.md")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	summary, err := g.GetChangeSummary()
	if err != nil {
		t.Fatalf("GetChangeSummary: %v", err)
	}

	if len(summary.Deleted) == 0 {
		t.Error("expected deleted files in summary")
	}
}
