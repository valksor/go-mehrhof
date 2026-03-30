package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const testStatusModified = "modified"

func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init", dir) //nolint:noctx // Test setup, no context needed
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "-C", dir, "config", "user.email", "test@test.com") //nolint:noctx // Test setup
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", dir, "config", "user.name", "Test User") //nolint:noctx // Test setup
	_ = cmd.Run()

	// Create initial file and commit
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	cmd = exec.Command("git", "-C", dir, "add", ".") //nolint:noctx // Test setup
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", dir, "commit", "-m", "initial commit") //nolint:noctx // Test setup
	_ = cmd.Run()

	return dir, func() {
		// Cleanup is handled by t.TempDir()
	}
}

func TestOpen(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if repo.Path() != dir {
		t.Errorf("repo.Path() = %s, want %s", repo.Path(), dir)
	}
}

func TestOpenNotGitRepo(t *testing.T) {
	dir := t.TempDir()

	_, err := Open(dir)
	if err == nil {
		t.Error("Open() should fail for non-git directory")
	}
}

func TestCurrentBranch(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}

	// Could be "main" or "master" depending on git config
	if branch != "main" && branch != "master" {
		t.Errorf("CurrentBranch() = %s, want main or master", branch)
	}
}

func TestCurrentCommit(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	commit, err := repo.CurrentCommit(ctx)
	if err != nil {
		t.Fatalf("CurrentCommit() error = %v", err)
	}

	if len(commit) != 40 {
		t.Errorf("commit SHA length = %d, want 40", len(commit))
	}
}

func TestCreateBranch(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	err := repo.CreateBranch(ctx, "test-branch", "")
	if err != nil {
		t.Fatalf("CreateBranch() error = %v", err)
	}

	branch, _ := repo.CurrentBranch(ctx)
	if branch != "test-branch" {
		t.Errorf("current branch = %s, want test-branch", branch)
	}
}

func TestSwitchBranch(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	originalBranch, _ := repo.CurrentBranch(ctx)

	_ = repo.CreateBranch(ctx, "other-branch", "")
	_ = repo.SwitchBranch(ctx, originalBranch)

	current, _ := repo.CurrentBranch(ctx)
	if current != originalBranch {
		t.Errorf("after switch, branch = %s, want %s", current, originalBranch)
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	// Initially no changes
	hasChanges, err := repo.HasUncommittedChanges(ctx)
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error = %v", err)
	}
	if hasChanges {
		t.Error("should not have uncommitted changes initially")
	}

	// Make a change
	testFile := filepath.Join(dir, "test.txt")
	_ = os.WriteFile(testFile, []byte("modified content"), 0o644)

	hasChanges, _ = repo.HasUncommittedChanges(ctx)
	if !hasChanges {
		t.Error("should have uncommitted changes after modifying file")
	}
}

func TestStageAndCommit(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	// Make a change
	testFile := filepath.Join(dir, "new_file.txt")
	_ = os.WriteFile(testFile, []byte("new content"), 0o644)

	// Stage
	if err := repo.StageAll(ctx); err != nil {
		t.Fatalf("StageAll() error = %v", err)
	}

	// Commit
	sha, err := repo.Commit(ctx, "test commit")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	if len(sha) != 40 {
		t.Errorf("commit SHA length = %d, want 40", len(sha))
	}

	// Verify no uncommitted changes
	hasChanges, _ := repo.HasUncommittedChanges(ctx)
	if hasChanges {
		t.Error("should not have uncommitted changes after commit")
	}
}

func TestReset(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	originalCommit, _ := repo.CurrentCommit(ctx)

	// Make a new commit
	testFile := filepath.Join(dir, "new_file.txt")
	_ = os.WriteFile(testFile, []byte("content"), 0o644)
	_ = repo.StageAll(ctx)
	_, _ = repo.Commit(ctx, "new commit")

	// Reset to original
	err := repo.Reset(ctx, originalCommit, true)
	if err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	currentCommit, _ := repo.CurrentCommit(ctx)
	if currentCommit != originalCommit {
		t.Errorf("after reset, commit = %s, want %s", currentCommit, originalCommit)
	}
}

func TestLog(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	entries, err := repo.Log(ctx, 10)
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	if len(entries) == 0 {
		t.Error("Log() should return at least one entry")
	}

	if len(entries[0].SHA) != 40 {
		t.Errorf("log entry SHA length = %d, want 40", len(entries[0].SHA))
	}
}

func TestDiff(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	// Make a change
	testFile := filepath.Join(dir, "test.txt")
	_ = os.WriteFile(testFile, []byte("modified content\n"), 0o644)

	diff, err := repo.Diff(ctx, false)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if diff == "" {
		t.Error("Diff() should return non-empty diff")
	}
}

func TestCommitInfo(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	// The setupTestRepo already created one commit — look it up
	sha, err := repo.CurrentCommit(ctx)
	if err != nil {
		t.Fatalf("CurrentCommit: %v", err)
	}

	entry, err := repo.CommitInfo(ctx, sha)
	if err != nil {
		t.Fatalf("CommitInfo: %v", err)
	}

	if entry.SHA != sha {
		t.Errorf("SHA: got %q want %q", entry.SHA, sha)
	}
	if entry.Message == "" {
		t.Error("Message should not be empty")
	}
	if entry.Author == "" {
		t.Error("Author should not be empty")
	}
	if entry.Date == "" {
		t.Error("Date should not be empty")
	}
}

func TestCommitInfo_InvalidSHA(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	_, err := repo.CommitInfo(ctx, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Error("CommitInfo should return error for nonexistent SHA")
	}
}

func TestParseNameStatusLine(t *testing.T) {
	cases := []struct {
		line   string
		path   string
		status string
	}{
		{"M\tpkg/foo.go", "pkg/foo.go", testStatusModified},
		{"A\tweb/new.ts", "web/new.ts", "added"},
		{"D\told.go", "old.go", "deleted"},
		{"R100\told.go\tnew.go", "new.go", "renamed"},
		{"C100\torig.go\tcopy.go", "copy.go", "renamed"},
		{"M\tsimple.txt", "simple.txt", testStatusModified},
	}
	for _, tc := range cases {
		path, status := parseNameStatusLine(tc.line)
		if path != tc.path {
			t.Errorf("path: got %q want %q (line %q)", path, tc.path, tc.line)
		}
		if status != tc.status {
			t.Errorf("status: got %q want %q (line %q)", status, tc.status, tc.line)
		}
	}
}

func TestDiffFilesWithStatus(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	// Modify existing file
	testFile := filepath.Join(dir, "test.txt")
	_ = os.WriteFile(testFile, []byte("modified content\n"), 0o644)

	files, err := repo.DiffFilesWithStatus(ctx)
	if err != nil {
		t.Fatalf("DiffFilesWithStatus() error = %v", err)
	}

	if len(files) != 1 {
		t.Errorf("DiffFilesWithStatus() returned %d files, want 1", len(files))
	}
	if files[0].Path != "test.txt" {
		t.Errorf("files[0].Path = %s, want test.txt", files[0].Path)
	}
	if files[0].Status != testStatusModified {
		t.Errorf("files[0].Status = %s, want %s", files[0].Status, testStatusModified)
	}
}

func TestValidateCommitMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		pattern string
		wantErr bool
	}{
		{"empty pattern allows any", "anything goes", "", false},
		{"matching pattern", "feat(auth): add login", `^(feat|fix)\(.*\):`, false},
		{"non-matching pattern", "added login", `^(feat|fix)\(.*\):`, true},
		{"multiline checks subject only", "feat(auth): add login\n\ndetails here", `^(feat|fix)\(.*\):`, false},
		{"invalid regex", "anything", `[invalid`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommitMessage(tt.message, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCommitMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDiffFiles(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)

	// Make a change
	testFile := filepath.Join(dir, "test.txt")
	_ = os.WriteFile(testFile, []byte("modified content\n"), 0o644)

	files, err := repo.DiffFiles(ctx)
	if err != nil {
		t.Fatalf("DiffFiles() error = %v", err)
	}

	if len(files) != 1 {
		t.Errorf("DiffFiles() returned %d files, want 1", len(files))
	}

	if files[0] != "test.txt" {
		t.Errorf("DiffFiles()[0] = %s, want test.txt", files[0])
	}
}

func TestCommitsSince(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	ctx := context.Background()

	// Get initial commit as base ref
	initialSHA, _ := repo.CurrentCommit(ctx)

	// Create two more commits
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	_ = repo.StageAll(ctx)
	_, _ = repo.Commit(ctx, "add file a")

	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644)
	_ = repo.StageAll(ctx)
	_, _ = repo.Commit(ctx, "add file b")

	entries, err := repo.CommitsSince(ctx, initialSHA)
	if err != nil {
		t.Fatalf("CommitsSince() error = %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("CommitsSince() returned %d entries, want 2", len(entries))
	}

	// Most recent first
	if entries[0].Message != "add file b" {
		t.Errorf("entries[0].Message = %q, want %q", entries[0].Message, "add file b")
	}
	if entries[1].Message != "add file a" {
		t.Errorf("entries[1].Message = %q, want %q", entries[1].Message, "add file a")
	}
}

func TestCommitsSince_NoNewCommits(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	ctx := context.Background()

	headSHA, _ := repo.CurrentCommit(ctx)
	entries, err := repo.CommitsSince(ctx, headSHA)
	if err != nil {
		t.Fatalf("CommitsSince() error = %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("CommitsSince() returned %d entries, want 0", len(entries))
	}
}

func TestStageFiles(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Open(dir)
	ctx := context.Background()

	// Create two files but only stage one
	_ = os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "unstaged.txt"), []byte("unstaged"), 0o644)

	err := repo.StageFiles(ctx, filepath.Join(dir, "staged.txt"))
	if err != nil {
		t.Fatalf("StageFiles() error = %v", err)
	}

	// Commit only staged file
	sha, err := repo.Commit(ctx, "add staged file")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if sha == "" {
		t.Error("Commit() returned empty SHA")
	}

	// Unstaged file should still show as untracked
	has, _ := repo.HasUncommittedChanges(ctx)
	if !has {
		t.Error("expected uncommitted changes (unstaged.txt), got none")
	}
}
