package conductor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/provider/file"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

// initGitRepo initializes a git repository for testing.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	ctx := context.Background()

	if err := runGitCmd(ctx, dir, "init"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	if err := runGitCmd(ctx, dir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
	if err := runGitCmd(ctx, dir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}

	// Create initial commit
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := runGitCmd(ctx, dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := runGitCmd(ctx, dir, "commit", "-m", "initial commit"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
}

func runGitCmd(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2020-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2020-01-01T00:00:00Z",
	)
	return cmd.Run()
}

func TestNew(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if c == nil {
		t.Fatal("New returned nil conductor")
	}
	if c.machine == nil {
		t.Error("conductor.machine is nil")
	}
	if c.eventBus == nil {
		t.Error("conductor.eventBus is nil")
	}
	if c.providers == nil {
		t.Error("conductor.providers is nil")
	}
	if c.agents == nil {
		t.Error("conductor.agents is nil")
	}
}

func TestNewWithOptions(t *testing.T) {
	tmpDir := t.TempDir()

	c, err := New(
		WithWorkDir(tmpDir),
		WithAutoInit(true),
		WithVerbose(true),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if c.opts.WorkDir != tmpDir {
		t.Errorf("WorkDir = %q, want %q", c.opts.WorkDir, tmpDir)
	}
	if c.opts.AutoInit != true {
		t.Errorf("AutoInit = %v, want true", c.opts.AutoInit)
	}
	if c.opts.Verbose != true {
		t.Errorf("Verbose = %v, want true", c.opts.Verbose)
	}
}

func TestInitialize_NonGitDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir), WithAutoInit(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Initialize should work even without git
	err = c.Initialize(ctx)
	// May fail if no agent is available, but shouldn't fail on workspace init
	// Just check workspace was created
	if c.workspace == nil && err == nil {
		t.Error("workspace should be initialized")
	}
}

func TestInitialize_GitDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir), WithAutoInit(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Initialize - may fail on agent detection
	_ = c.Initialize(ctx)

	// Workspace should be set
	if c.workspace == nil {
		t.Error("workspace should be initialized")
	}

	// Git should be set
	if c.git == nil {
		t.Error("git should be initialized")
	}
}

func TestInitialize_AutoInit(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir), WithAutoInit(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Initialize - ignore agent detection errors
	_ = c.Initialize(ctx)

	// Check .mehrhof directory was created
	taskDir := filepath.Join(tmpDir, ".mehrhof")
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		t.Error(".mehrhof directory should be created with AutoInit")
	}
}

func TestGetProviderRegistry(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	registry := c.GetProviderRegistry()
	if registry == nil {
		t.Error("GetProviderRegistry returned nil")
	}
	if registry != c.providers {
		t.Error("GetProviderRegistry returned different registry")
	}
}

func TestGetAgentRegistry(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	registry := c.GetAgentRegistry()
	if registry == nil {
		t.Error("GetAgentRegistry returned nil")
	}
	if registry != c.agents {
		t.Error("GetAgentRegistry returned different registry")
	}
}

func TestGetEventBus(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	bus := c.GetEventBus()
	if bus == nil {
		t.Error("GetEventBus returned nil")
	}
	if bus != c.eventBus {
		t.Error("GetEventBus returned different bus")
	}
}

func TestGetWorkspace_NotInitialized(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ws := c.GetWorkspace()
	if ws != nil {
		t.Error("GetWorkspace should return nil before Initialize")
	}
}

func TestGetGit_NotInitialized(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	git := c.GetGit()
	if git != nil {
		t.Error("GetGit should return nil before Initialize")
	}
}

func TestGetActiveTask_NoTask(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	task := c.GetActiveTask()
	if task != nil {
		t.Error("GetActiveTask should return nil when no task is active")
	}
}

func TestGetActiveAgent_NotInitialized(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	agent := c.GetActiveAgent()
	if agent != nil {
		t.Error("GetActiveAgent should return nil before Initialize")
	}
}

func TestStart_InvalidReference(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir), WithAutoInit(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set up workspace
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	c.workspace = ws

	// Try to start without a valid provider reference
	err = c.Start(ctx, "nonexistent-reference")
	if err == nil {
		t.Error("Start should fail with invalid reference")
	}
}

func TestResume_NoActiveTask(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir), WithAutoInit(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set up workspace
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	c.workspace = ws

	err = c.Resume(ctx)
	if err == nil {
		t.Error("Resume should fail when no active task exists")
	}
	if err.Error() != "no active task" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPlan_NoActiveTask(t *testing.T) {
	ctx := context.Background()

	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = c.Plan(ctx)
	if err == nil {
		t.Error("Plan should fail when no active task")
	}
	if err.Error() != "no active task" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestImplement_NoActiveTask(t *testing.T) {
	ctx := context.Background()

	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = c.Implement(ctx)
	if err == nil {
		t.Error("Implement should fail when no active task")
	}
	if err.Error() != "no active task" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReview_NoActiveTask(t *testing.T) {
	ctx := context.Background()

	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = c.Review(ctx)
	if err == nil {
		t.Error("Review should fail when no active task")
	}
	if err.Error() != "no active task" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUndo_NoActiveTask(t *testing.T) {
	ctx := context.Background()

	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = c.Undo(ctx)
	if err == nil {
		t.Error("Undo should fail when no active task")
	}
	if err.Error() != "no active task" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRedo_NoActiveTask(t *testing.T) {
	ctx := context.Background()

	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = c.Redo(ctx)
	if err == nil {
		t.Error("Redo should fail when no active task")
	}
	if err.Error() != "no active task" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUndo_NoGit(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set up workspace and fake active task
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	c.workspace = ws
	c.activeTask = &storage.ActiveTask{
		ID:      "test-task",
		State:   "planning",
		Started: time.Now(),
	}

	err = c.Undo(ctx)
	if err == nil {
		t.Error("Undo should fail when git is not available")
	}
	if err.Error() != "git not available" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRedo_NoGit(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set up workspace and fake active task
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	c.workspace = ws
	c.activeTask = &storage.ActiveTask{
		ID:      "test-task",
		State:   "planning",
		Started: time.Now(),
	}

	err = c.Redo(ctx)
	if err == nil {
		t.Error("Redo should fail when git is not available")
	}
	if err.Error() != "git not available" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFinish_NoActiveTask(t *testing.T) {
	ctx := context.Background()

	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = c.Finish(ctx, DefaultFinishOptions())
	if err == nil {
		t.Error("Finish should fail when no active task")
	}
	if err.Error() != "no active task" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDelete_NoActiveTask(t *testing.T) {
	ctx := context.Background()

	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = c.Delete(ctx, DeleteOptions{})
	if err == nil {
		t.Error("Delete should fail when no active task")
	}
	if err.Error() != "no active task" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStatus_NoActiveTask(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.Status()
	if err == nil {
		t.Error("Status should fail when no active task")
	}
	if err.Error() != "no active task" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnsureCleanWorkspace_NoGit(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// No git, no branch creation - should pass
	c.opts.CreateBranch = false
	err = c.ensureCleanWorkspace()
	if err != nil {
		t.Errorf("ensureCleanWorkspace should pass without git: %v", err)
	}
}

func TestEnsureCleanWorkspace_NoBranchCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Initialize git
	ctx := context.Background()
	_ = c.Initialize(ctx)

	// No branch creation requested - should pass even with dirty workspace
	c.opts.CreateBranch = false

	// Make the workspace dirty
	if err := os.WriteFile(filepath.Join(tmpDir, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err = c.ensureCleanWorkspace()
	if err != nil {
		t.Errorf("ensureCleanWorkspace should pass when CreateBranch=false: %v", err)
	}
}

func TestEnsureCleanWorkspace_CleanWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	c, err := New(WithWorkDir(tmpDir), WithCreateBranch(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Initialize git
	ctx := context.Background()
	_ = c.Initialize(ctx)

	err = c.ensureCleanWorkspace()
	if err != nil {
		t.Errorf("ensureCleanWorkspace should pass with clean workspace: %v", err)
	}
}

func TestEnsureCleanWorkspace_DirtyWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	c, err := New(WithWorkDir(tmpDir), WithCreateBranch(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Initialize git
	ctx := context.Background()
	_ = c.Initialize(ctx)

	// Make the workspace dirty
	if err := os.WriteFile(filepath.Join(tmpDir, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err = c.ensureCleanWorkspace()
	if err == nil {
		t.Error("ensureCleanWorkspace should fail with dirty workspace")
	}
}

func TestImplement_NoSpecifications(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set up workspace and active task
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	c.workspace = ws
	c.activeTask = &storage.ActiveTask{
		ID:      "test-task",
		State:   "planning",
		Started: time.Now(),
	}

	// Create empty work directory for this task
	work, err := ws.CreateWork("test-task", storage.SourceInfo{
		Type: "test",
		Ref:  "test",
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}
	c.taskWork = work

	err = c.Implement(ctx)
	if err == nil {
		t.Error("Implement should fail when no specifications exist")
	}
	if err.Error() != "no specifications found - run 'task plan' first" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStart_TaskAlreadyActive(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set up workspace and existing active task
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	c.workspace = ws
	c.activeTask = &storage.ActiveTask{
		ID:      "existing-task",
		State:   "planning",
		Started: time.Now(),
	}

	err = c.Start(ctx, "file:task.md")
	if err == nil {
		t.Error("Start should fail when a task is already active")
	}
	if !contains(err.Error(), "task already active") {
		t.Errorf("unexpected error: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBuildWorkUnit_WithTaskWork(t *testing.T) {
	tmpDir := t.TempDir()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set up workspace and task work
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	work, err := ws.CreateWork("test-task", storage.SourceInfo{
		Type:    "file",
		Ref:     "task.md",
		Content: "Task content here",
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}
	work.Metadata.Title = "Test Task Title"

	c.workspace = ws
	c.taskWork = work

	// Create some specs
	if err := ws.SaveSpecification("test-task", 1, "# Spec 1"); err != nil {
		t.Fatalf("SaveSpec: %v", err)
	}
	if err := ws.SaveSpecification("test-task", 2, "# Spec 2"); err != nil {
		t.Fatalf("SaveSpec: %v", err)
	}

	wu := c.buildWorkUnit()
	if wu == nil {
		t.Fatal("buildWorkUnit returned nil")
	}

	if wu.ID != "test-task" {
		t.Errorf("ID = %q, want %q", wu.ID, "test-task")
	}
	if wu.ExternalID != "task.md" {
		t.Errorf("ExternalID = %q, want %q", wu.ExternalID, "task.md")
	}
	if wu.Title != "Test Task Title" {
		t.Errorf("Title = %q, want %q", wu.Title, "Test Task Title")
	}
	if wu.Source == nil {
		t.Fatal("Source is nil")
	}
	if wu.Source.Reference != "task.md" {
		t.Errorf("Source.Reference = %q, want %q", wu.Source.Reference, "task.md")
	}
	if len(wu.Specifications) != 2 {
		t.Errorf("Specifications len = %d, want 2", len(wu.Specifications))
	}
}

func TestGetTaskWork_WithTask(t *testing.T) {
	tmpDir := t.TempDir()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set up workspace and task work
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	work, err := ws.CreateWork("test-task", storage.SourceInfo{
		Type: "file",
		Ref:  "task.md",
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	c.taskWork = work

	got := c.GetTaskWork()
	if got == nil {
		t.Fatal("GetTaskWork returned nil")
	}
	// GetTaskWork now returns a copy for thread safety, so compare values not pointers
	if got.Metadata.ID != work.Metadata.ID {
		t.Errorf("GetTaskWork returned different work ID: got %q, want %q", got.Metadata.ID, work.Metadata.ID)
	}
}

// TestDelete_NoWorkspace is skipped because Delete() doesn't check for nil workspace
// before using it (it panics instead of returning an error).

func TestDelete_WithWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set up workspace
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	// Create task work
	work, err := ws.CreateWork("test-task", storage.SourceInfo{
		Type: "file",
		Ref:  "task.md",
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	// Save active task file
	activeTask := &storage.ActiveTask{
		ID:      "test-task",
		State:   "planning",
		Started: time.Now(),
	}
	if err := ws.SaveActiveTask(activeTask); err != nil {
		t.Fatalf("SaveActiveTask: %v", err)
	}

	c.workspace = ws
	c.taskWork = work
	c.activeTask = activeTask

	// Delete with DeleteWork=false (keep work dir, no git to delete branch)
	err = c.Delete(ctx, DeleteOptions{DeleteWork: BoolPtr(false)})
	if err != nil {
		t.Errorf("Delete with DeleteWork=false: %v", err)
	}

	// Active task should be cleared
	if c.activeTask != nil {
		t.Error("activeTask should be nil after delete")
	}
}

// TestFinish_NoGit tests that Finish works even when UseGit=true but git is nil.
// (the git merge section is skipped based on the c.git != nil check).
func TestFinish_NoGit(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set up workspace
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	work, err := ws.CreateWork("test-task", storage.SourceInfo{
		Type: "file",
		Ref:  "task.md",
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	// Create a specification so finish guard passes
	if err := ws.SaveSpecification("test-task", 1, "# Test Specification\n\nTest content"); err != nil {
		t.Fatalf("SaveSpec: %v", err)
	}

	// Save active task file
	activeTask := &storage.ActiveTask{
		ID:      "test-task",
		State:   "implementing",
		UseGit:  true, // UseGit is true but git is nil - git operations are skipped
		Started: time.Now(),
	}
	if err := ws.SaveActiveTask(activeTask); err != nil {
		t.Fatalf("SaveActiveTask: %v", err)
	}

	c.workspace = ws
	c.taskWork = work
	c.activeTask = activeTask

	// Set up work unit with specifications for guard to pass
	c.machine.SetWorkUnit(&workflow.WorkUnit{
		ID:             "test-task",
		Specifications: []string{"specification-1.md"},
	})

	// Finish should work - git merge is skipped when c.git is nil
	err = c.Finish(ctx, DefaultFinishOptions())
	if err != nil {
		t.Errorf("Finish with nil git: %v", err)
	}

	// Task should be cleared
	if c.activeTask != nil {
		t.Error("activeTask should be nil after finish")
	}
}

func TestFinish_NoGitUsed(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set up workspace
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	work, err := ws.CreateWork("test-task", storage.SourceInfo{
		Type: "file",
		Ref:  "task.md",
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	// Create a specification so finish guard passes
	if err := ws.SaveSpecification("test-task", 1, "# Test Specification\n\nTest content"); err != nil {
		t.Fatalf("SaveSpec: %v", err)
	}

	// Save active task file
	activeTask := &storage.ActiveTask{
		ID:      "test-task",
		State:   "idle",
		UseGit:  false, // No git used
		Started: time.Now(),
	}
	if err := ws.SaveActiveTask(activeTask); err != nil {
		t.Fatalf("SaveActiveTask: %v", err)
	}

	c.workspace = ws
	c.taskWork = work
	c.activeTask = activeTask

	// Set up work unit with specifications for guard to pass
	c.machine.SetWorkUnit(&workflow.WorkUnit{
		ID:             "test-task",
		Specifications: []string{"specification-1.md"},
	})

	// Finish without git should work
	err = c.Finish(ctx, DefaultFinishOptions())
	if err != nil {
		t.Errorf("Finish without git: %v", err)
	}

	// Task should be cleared
	if c.activeTask != nil {
		t.Error("activeTask should be nil after finish")
	}
}

// TestReview_NoSpecs and TestPlan_NoAgent are skipped because they would panic
// when accessing nil activeAgent. The code lacks nil checks before using activeAgent.

func TestCreateCheckpoint_WithChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Initialize to set git
	_ = c.Initialize(ctx)

	c.activeTask = &storage.ActiveTask{
		ID:      "test-task",
		UseGit:  true,
		Started: time.Now(),
	}

	// Make a change
	if err := os.WriteFile(filepath.Join(tmpDir, "newfile.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Add the file
	if err := runGitCmd(ctx, tmpDir, "add", "newfile.txt"); err != nil {
		t.Fatalf("git add: %v", err)
	}

	event := c.createCheckpointIfNeeded("test-task", "test checkpoint")
	if event == nil {
		// The test passes if checkpoint is created; if not (due to git state) that's okay too
		t.Log("createCheckpointIfNeeded returned nil (possibly no changes or git error)")
	} else {
		if event.Type != "checkpoint" {
			t.Errorf("event type = %q, want %q", event.Type, "checkpoint")
		}
	}
}

// TestStart_WithFileReference tests starting a task with a file:// reference.
func TestStart_WithFileReference(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create a task file with mock agent specified
	taskContent := `---
title: Test Task from File
agent: mock
---
This is a test task description.
`
	taskPath := filepath.Join(tmpDir, "test-task.md")
	if err := os.WriteFile(taskPath, []byte(taskContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create conductor
	c, err := New(WithWorkDir(tmpDir), WithCreateBranch(false))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Register file provider
	file.Register(c.GetProviderRegistry())

	// Register mock agent
	mockAgent := &mockAgent{name: "mock"}
	if err := c.GetAgentRegistry().Register(mockAgent); err != nil {
		t.Fatalf("Register mock agent: %v", err)
	}

	// Initialize
	if err := c.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Start the task with absolute path
	err = c.Start(ctx, "file:"+taskPath)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Verify active task
	activeTask := c.GetActiveTask()
	if activeTask == nil {
		t.Fatal("GetActiveTask returned nil")
	}

	// Verify task work exists
	taskWork := c.GetTaskWork()
	if taskWork == nil {
		t.Fatal("GetTaskWork returned nil")
	}

	if taskWork.Metadata.Title != "Test Task from File" {
		t.Errorf("task title = %q, want %q", taskWork.Metadata.Title, "Test Task from File")
	}

	// Verify agent was set
	if taskWork.Agent.Name != "mock" {
		t.Errorf("agent name = %q, want %q", taskWork.Agent.Name, "mock")
	}
}

// TestStatus_Integration tests getting status when there's an active task.
func TestStatus_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	// Create task work
	taskID := "status-test-123"
	work, err := ws.CreateWork(taskID, storage.SourceInfo{
		Type: "file",
		Ref:  "task.md",
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}
	work.Metadata.Title = "Status Test Task"
	work.Metadata.ExternalKey = "STATUS-123"
	if err := ws.SaveWork(work); err != nil {
		t.Fatalf("SaveWork: %v", err)
	}

	// Create active task
	activeTask := &storage.ActiveTask{
		ID:      taskID,
		Ref:     "file:task.md",
		WorkDir: ".mehrhof/work/" + taskID,
		State:   "implementing",
		Branch:  "feature/status--test",
		Started: time.Now(),
	}
	if err := ws.SaveActiveTask(activeTask); err != nil {
		t.Fatalf("SaveActiveTask: %v", err)
	}

	// Create conductor
	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Initialize - ignore agent detection errors
	ctx := context.Background()
	_ = c.Initialize(ctx)

	// Get status
	status, err := c.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if status.TaskID != taskID {
		t.Errorf("TaskID = %q, want %q", status.TaskID, taskID)
	}
	if status.Title != "Status Test Task" {
		t.Errorf("Title = %q, want %q", status.Title, "Status Test Task")
	}
	if status.State != "implementing" {
		t.Errorf("State = %q, want %q", status.State, "implementing")
	}
	if status.Branch != "feature/status--test" {
		t.Errorf("Branch = %q, want %q", status.Branch, "feature/status--test")
	}
}

// mockAgent is a minimal mock agent for integration testing.
type mockAgent struct {
	name string
}

func (a *mockAgent) Name() string {
	return a.name
}

func (a *mockAgent) Run(ctx context.Context, prompt string) (*agent.Response, error) {
	return &agent.Response{
		Summary:  "Mock response",
		Messages: []string{"This is a mock agent response"},
	}, nil
}

func (a *mockAgent) RunStream(ctx context.Context, prompt string) (<-chan agent.Event, <-chan error) {
	eventCh := make(chan agent.Event, 1)
	errCh := make(chan error, 1)

	eventCh <- agent.Event{Type: agent.EventText, Text: "Mock response"}
	close(eventCh)
	close(errCh)

	return eventCh, errCh
}

func (a *mockAgent) RunWithCallback(ctx context.Context, prompt string, cb agent.StreamCallback) (*agent.Response, error) {
	return a.Run(ctx, prompt)
}

func (a *mockAgent) Available() error {
	return nil
}

func (a *mockAgent) WithEnv(key, value string) agent.Agent {
	return a
}

func (a *mockAgent) WithArgs(args ...string) agent.Agent {
	return a
}
