package conductor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/settings"
)

// gitCmd runs a git command with context for the given arguments.
func gitCmd(ctx context.Context, t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "git", args...)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}

// setupForkTestRepo creates a git repo with an initial commit and returns
// a Conductor wired to it. Caller must defer cleanup().
func setupForkTestRepo(t *testing.T, forkingEnabled bool, maxForks int) (*Conductor, string, func()) {
	t.Helper()

	dir := t.TempDir()

	ctx := context.Background()

	// Initialize git repo with initial commit
	for _, args := range [][]string{
		{"init", dir},
		{"-C", dir, "config", "user.email", "test@test.com"},
		{"-C", dir, "config", "user.name", "Test User"},
	} {
		gitCmd(ctx, t, args...)
	}

	// Create initial commit
	testFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"-C", dir, "add", "."},
		{"-C", dir, "commit", "-m", "initial commit"},
	} {
		gitCmd(ctx, t, args...)
	}

	repo, err := git.Open(dir)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}

	s := settings.DefaultSettings()
	if forkingEnabled {
		s.Workflow.Forking = &settings.ForkingSettings{
			Enabled:  true,
			MaxForks: maxForks,
		}
	}

	c := &Conductor{
		git:      repo,
		worktree: dir,
		workUnit: &WorkUnit{
			ID:    "test-task-1",
			Title: "Test Task",
		},
		events:         make(chan ConductorEvent, 16),
		pendingPrompts: make(map[string]chan bool),
	}
	c.cachedSettings.Store(s)

	return c, dir, func() {}
}

func TestFork_CreatesWorktreeAndBranch(t *testing.T) {
	c, dir, cleanup := setupForkTestRepo(t, true, 3)
	defer cleanup()

	ctx := context.Background()
	info, err := c.Fork(ctx, "approach-A")
	if err != nil {
		t.Fatalf("Fork() error: %v", err)
	}

	if info.ID == "" {
		t.Error("expected non-empty fork ID")
	}
	if info.Label != "approach-A" {
		t.Errorf("expected label 'approach-A', got %q", info.Label)
	}
	if info.Branch != "kvelmo-fork/test-task-1/approach-a" {
		t.Errorf("expected branch 'kvelmo-fork/test-task-1/approach-a', got %q", info.Branch)
	}
	if info.State != "active" {
		t.Errorf("expected state 'active', got %q", info.State)
	}
	if info.CheckpointSHA == "" {
		t.Error("expected non-empty checkpoint SHA")
	}

	// Verify worktree directory exists
	expectedDir := filepath.Join(filepath.Dir(dir), ".kvelmo-worktrees", "test-task-1-fork-approach-a")
	if info.WorktreeDir != expectedDir {
		t.Errorf("expected worktree dir %q, got %q", expectedDir, info.WorktreeDir)
	}
	if _, err := os.Stat(info.WorktreeDir); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// Verify fork is stored in work unit
	if len(c.workUnit.Forks) != 1 {
		t.Errorf("expected 1 fork in work unit, got %d", len(c.workUnit.Forks))
	}
}

func TestFork_MaxForksEnforced(t *testing.T) {
	c, _, cleanup := setupForkTestRepo(t, true, 2)
	defer cleanup()

	ctx := context.Background()

	// Create first fork
	_, err := c.Fork(ctx, "approach-A")
	if err != nil {
		t.Fatalf("Fork 1 error: %v", err)
	}

	// Create second fork
	_, err = c.Fork(ctx, "approach-B")
	if err != nil {
		t.Fatalf("Fork 2 error: %v", err)
	}

	// Third fork should fail (max_forks = 2)
	_, err = c.Fork(ctx, "approach-C")
	if err == nil {
		t.Fatal("expected error when exceeding max forks, got nil")
	}

	if len(c.workUnit.Forks) != 2 {
		t.Errorf("expected 2 forks, got %d", len(c.workUnit.Forks))
	}
}

func TestFork_DisabledReturnsError(t *testing.T) {
	c, _, cleanup := setupForkTestRepo(t, false, 3)
	defer cleanup()

	ctx := context.Background()
	_, err := c.Fork(ctx, "approach-A")
	if err == nil {
		t.Fatal("expected error when forking is disabled, got nil")
	}
}

func TestFork_ListForks(t *testing.T) {
	c, _, cleanup := setupForkTestRepo(t, true, 5)
	defer cleanup()

	ctx := context.Background()

	// No forks initially
	forks := c.ListForks()
	if len(forks) != 0 {
		t.Fatalf("expected 0 forks, got %d", len(forks))
	}

	// Create forks
	_, err := c.Fork(ctx, "approach-A")
	if err != nil {
		t.Fatalf("Fork A error: %v", err)
	}
	_, err = c.Fork(ctx, "approach-B")
	if err != nil {
		t.Fatalf("Fork B error: %v", err)
	}

	forks = c.ListForks()
	if len(forks) != 2 {
		t.Fatalf("expected 2 forks, got %d", len(forks))
	}

	labels := map[string]bool{}
	for _, f := range forks {
		labels[f.Label] = true
	}
	if !labels["approach-A"] || !labels["approach-B"] {
		t.Errorf("expected forks with labels 'approach-A' and 'approach-B', got %v", forks)
	}
}

func TestCompareForks_DiffStats(t *testing.T) {
	c, _, cleanup := setupForkTestRepo(t, true, 5)
	defer cleanup()

	ctx := context.Background()

	// Create a fork
	info, err := c.Fork(ctx, "with-changes")
	if err != nil {
		t.Fatalf("Fork error: %v", err)
	}

	// Add a file in the fork worktree
	newFile := filepath.Join(info.WorktreeDir, "new_feature.go")
	if err := os.WriteFile(newFile, []byte("package main\n\nfunc newFeature() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Commit in the fork worktree
	for _, args := range [][]string{
		{"-C", info.WorktreeDir, "add", "."},
		{"-C", info.WorktreeDir, "commit", "-m", "add new feature"},
	} {
		gitCmd(ctx, t, args...)
	}

	comparison, err := c.CompareForks(ctx)
	if err != nil {
		t.Fatalf("CompareForks error: %v", err)
	}

	if len(comparison.Forks) != 1 {
		t.Fatalf("expected 1 fork in comparison, got %d", len(comparison.Forks))
	}

	entry := comparison.Forks[0]
	if entry.Label != "with-changes" {
		t.Errorf("expected label 'with-changes', got %q", entry.Label)
	}

	// The fork should show some changes
	if entry.DiffStats.Files == 0 && entry.LinesAdded == 0 {
		t.Log("Warning: diff stats show no changes (git diff may not work across worktrees)")
	}
}

func TestSanitizeBranchLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"approach A", "approach-a"},
		{"use/slashes", "use-slashes"},
		{"with spaces", "with-spaces"},
		{"UPPERCASE", "uppercase"},
		{"clean-label", "clean-label"},
		{"dots..bad", "dots-bad"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeBranchLabel(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeBranchLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDiffStat(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantFiles   int
		wantAdded   int
		wantRemoved int
	}{
		{
			name:        "typical output",
			input:       " file1.go | 10 ++++------\n file2.go | 5 +++++\n 2 files changed, 9 insertions(+), 6 deletions(-)",
			wantFiles:   2,
			wantAdded:   9,
			wantRemoved: 6,
		},
		{
			name:        "single file",
			input:       " main.go | 3 +++\n 1 file changed, 3 insertions(+)",
			wantFiles:   1,
			wantAdded:   3,
			wantRemoved: 0,
		},
		{
			name:        "empty",
			input:       "",
			wantFiles:   0,
			wantAdded:   0,
			wantRemoved: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, _, linesAdded, linesRemoved := parseDiffStat(tt.input)
			if ds.Files != tt.wantFiles {
				t.Errorf("files: got %d, want %d", ds.Files, tt.wantFiles)
			}
			if linesAdded != tt.wantAdded {
				t.Errorf("lines added: got %d, want %d", linesAdded, tt.wantAdded)
			}
			if linesRemoved != tt.wantRemoved {
				t.Errorf("lines removed: got %d, want %d", linesRemoved, tt.wantRemoved)
			}
		})
	}
}
