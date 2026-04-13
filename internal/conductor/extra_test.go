package conductor

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/storage"
)

// ─── Options & New ───────────────────────────────────────────────────────────

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.WorkDir != "." {
		t.Errorf("DefaultOptions().WorkDir = %q, want %q", opts.WorkDir, ".")
	}
	if opts.Stdout == nil {
		t.Error("DefaultOptions().Stdout should not be nil")
	}
	if opts.Stderr == nil {
		t.Error("DefaultOptions().Stderr should not be nil")
	}
}

func TestWithVerbose(t *testing.T) {
	opts := DefaultOptions()
	WithVerbose(true)(&opts)
	if !opts.Verbose {
		t.Error("WithVerbose(true) did not set Verbose to true")
	}
}

func TestWithPool(t *testing.T) {
	opts := DefaultOptions()
	WithPool(nil)(&opts) // nil is valid for testing
	if opts.Pool != nil {
		t.Error("WithPool(nil) should set Pool to nil")
	}
}

func TestWithStdout(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	WithStdout(&buf)(&opts)
	if opts.Stdout != &buf {
		t.Error("WithStdout() did not set Stdout")
	}
}

func TestWithStderr(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	WithStderr(&buf)(&opts)
	if opts.Stderr != &buf {
		t.Error("WithStderr() did not set Stderr")
	}
}

func TestNew_WithOptions(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	c, err := New(
		WithVerbose(true),
		WithStdout(&outBuf),
		WithStderr(&errBuf),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if !c.opts.Verbose {
		t.Error("New with WithVerbose: opts.Verbose should be true")
	}
}

// ─── Conductor accessors ─────────────────────────────────────────────────────

func TestConductorGetWorkUnit_Nil(t *testing.T) {
	c, _ := New()
	if c.GetWorkUnit() != nil {
		t.Error("GetWorkUnit() should return nil when no task is loaded")
	}
}

func TestConductorGetWorkUnit_NonNil(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "test-1", Title: "Test"}
	c.ForceWorkUnit(wu)
	got := c.GetWorkUnit()
	if got == nil {
		t.Fatal("GetWorkUnit() returned nil after ForceWorkUnit")
	}
	if got.ID != "test-1" {
		t.Errorf("GetWorkUnit().ID = %q, want test-1", got.ID)
	}
}

func TestConductorMachine(t *testing.T) {
	c, _ := New()
	if c.Machine() == nil {
		t.Error("Machine() should not return nil")
	}
	if c.Machine() != c.machine {
		t.Error("Machine() should return the internal machine")
	}
}

func TestConductorGetWorkDir_NoWorktree(t *testing.T) {
	c, _ := New()
	dir := c.getWorkDir()
	if dir == "" {
		t.Error("getWorkDir() should return non-empty string when no worktree set")
	}
}

func TestConductorGetWorkDir_WithWorktree(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "x", Title: "x", WorktreePath: "/tmp/test-worktree"}
	c.ForceWorkUnit(wu)
	dir := c.getWorkDir()
	if dir != "/tmp/test-worktree" {
		t.Errorf("getWorkDir() = %q, want /tmp/test-worktree", dir)
	}
}

func TestConductorEvents(t *testing.T) {
	c, _ := New()
	ch := c.Events()
	if ch == nil {
		t.Error("Events() should return non-nil channel")
	}
}

func TestConductorAddListener(t *testing.T) {
	c, _ := New()
	called := false
	c.AddListener(func(_ ConductorEvent) {
		called = true
	})
	if len(c.listeners) != 1 {
		t.Errorf("AddListener: expected 1 listener, got %d", len(c.listeners))
	}
	_ = called // listener would be called on events
}

func TestConductorOnEvent(t *testing.T) {
	c, _ := New()
	c.OnEvent(func(_ ConductorEvent) {})
	if len(c.listeners) != 1 {
		t.Errorf("OnEvent: expected 1 listener, got %d", len(c.listeners))
	}
}

func TestConductorClose(t *testing.T) {
	c, _ := New()
	if err := c.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
	// Channel should be closed after Close()
	_, ok := <-c.events
	if ok {
		t.Error("events channel should be closed after Close()")
	}
}

func TestConductorSetMemoryIndexer(t *testing.T) {
	c, _ := New()
	c.SetMemoryIndexer(nil)
	if c.memoryIndexer != nil {
		t.Error("SetMemoryIndexer(nil) should set memoryIndexer to nil")
	}
}

func TestConductorSetStore(t *testing.T) {
	c, _ := New()
	c.SetStore(nil)
	if c.store != nil {
		t.Error("SetStore(nil) should set store to nil")
	}
}

func TestConductorStatus_NoTask(t *testing.T) {
	c, _ := New()
	status := c.Status()
	if status == nil {
		t.Fatal("Status() returned nil")
	}
	if _, ok := status["state"]; !ok {
		t.Error("Status() missing 'state' key")
	}
	if _, ok := status["worktree"]; !ok {
		t.Error("Status() missing 'worktree' key")
	}
	if _, ok := status["task"]; ok {
		t.Error("Status() should not have 'task' key when no task loaded")
	}
}

func TestConductorStatus_WithTask(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{
		ID:          "t1",
		Title:       "My Task",
		Branch:      "kvelmo/test-t1",
		Checkpoints: []string{"abc"},
		Jobs:        []string{"j1", "j2"},
	}
	c.ForceWorkUnit(wu)
	status := c.Status()
	task, ok := status["task"]
	if !ok {
		t.Fatal("Status() missing 'task' key when task is loaded")
	}
	taskMap, ok := task.(map[string]any)
	if !ok {
		t.Fatal("Status()['task'] is not a map")
	}
	if taskMap["id"] != "t1" {
		t.Errorf("task id = %v, want t1", taskMap["id"])
	}
	if taskMap["checkpoints"] != 1 {
		t.Errorf("task checkpoints = %v, want 1", taskMap["checkpoints"])
	}
	if taskMap["jobs"] != 2 {
		t.Errorf("task jobs = %v, want 2", taskMap["jobs"])
	}
}

func TestConductorForceWorkUnit(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "forced", Title: "Forced"}
	c.ForceWorkUnit(wu)
	if c.workUnit == nil {
		t.Fatal("ForceWorkUnit did not set workUnit")
	}
	if c.workUnit.ID != "forced" {
		t.Errorf("ForceWorkUnit: workUnit.ID = %q, want forced", c.workUnit.ID)
	}
	if c.machine.WorkUnit() == nil {
		t.Error("ForceWorkUnit should also set workUnit on the machine")
	}
}

// ─── Plan/Implement error paths ───────────────────────────────────────────────

func TestConductorPlan_NoTask(t *testing.T) {
	c, _ := New()
	_, err := c.Plan(context.Background())
	if err == nil {
		t.Error("Plan() with no task should return an error")
	}
}

func TestConductorPlan_NilPool(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{
		ID:          "plan-1",
		Title:       "Plan Test",
		Description: "Test description",
		Source:      &Source{Provider: "test", Reference: "test:1", Content: "content"},
	}
	c.ForceWorkUnit(wu)
	c.machine.ForceState(StateLoaded)

	_, err := c.Plan(context.Background())
	if err == nil {
		t.Error("Plan() with nil pool should return an error")
	}
	if !strings.Contains(err.Error(), "worker pool") {
		t.Errorf("Plan() error = %q, want to contain 'worker pool'", err.Error())
	}
}

func TestConductorImplement_NoTask(t *testing.T) {
	c, _ := New()
	_, err := c.Implement(context.Background())
	if err == nil {
		t.Error("Implement() with no task should return an error")
	}
}

func TestConductorImplement_NilPool(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{
		ID:             "impl-1",
		Title:          "Impl Test",
		Description:    "Test description",
		Specifications: []string{"spec1.md"},
		Source:         &Source{Provider: "test", Reference: "test:1", Content: "content"},
	}
	c.ForceWorkUnit(wu)
	c.machine.ForceState(StatePlanned)

	_, err := c.Implement(context.Background())
	if err == nil {
		t.Error("Implement() with nil pool should return an error")
	}
	if !strings.Contains(err.Error(), "worker pool") {
		t.Errorf("Implement() error = %q, want to contain 'worker pool'", err.Error())
	}
}

// ─── Abort / Reset ───────────────────────────────────────────────────────────

func TestConductorAbort_FromLoaded(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "a1", Title: "Abort", Source: &Source{Provider: "test", Reference: "ref"}}
	c.ForceWorkUnit(wu)
	c.machine.ForceState(StateLoaded)

	if err := c.Abort(context.Background()); err != nil {
		t.Errorf("Abort() error = %v", err)
	}
	if c.machine.State() != StateFailed {
		t.Errorf("after Abort: state = %s, want failed", c.machine.State())
	}
}

func TestConductorReset_NotFailed(t *testing.T) {
	c, _ := New()
	// Machine starts in StateNone - not StateFailed
	err := c.Reset(context.Background())
	if err == nil {
		t.Error("Reset() from non-failed state should return an error")
	}
}

// ─── CreateCheckpoint ────────────────────────────────────────────────────────

func TestConductorCreateCheckpoint_NoGit(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "cp1", Title: "Checkpoint Test"}
	c.ForceWorkUnit(wu)

	_, err := c.CreateCheckpoint(context.Background(), "test checkpoint")
	if err == nil {
		t.Error("CreateCheckpoint() with no git should return an error")
	}
}

func TestConductorCreateCheckpoint_NoTask(t *testing.T) {
	c, _ := New()
	_, err := c.CreateCheckpoint(context.Background(), "test")
	if err == nil {
		t.Error("CreateCheckpoint() with no task should return an error")
	}
}

// ─── LoadState ───────────────────────────────────────────────────────────────

func TestConductorLoadState_NilStore(t *testing.T) {
	c, _ := New()
	// store is nil by default → LoadState is a no-op
	if err := c.LoadState(context.Background()); err != nil {
		t.Errorf("LoadState() with nil store returned error: %v", err)
	}
}

// ─── Initialize ──────────────────────────────────────────────────────────────

func TestConductorInitialize_NonGitDir(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	// tmpDir is not a git repo — Initialize should return nil (non-fatal warning)
	if err := c.Initialize(context.Background()); err != nil {
		t.Errorf("Initialize() on non-git dir returned error: %v", err)
	}
}

// ─── generateBranchName ──────────────────────────────────────────────────────

func TestGenerateBranchName_WithSource(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{
		ID:         "local-id",
		ExternalID: "EXT-123",
		Title:      "Add user authentication",
		Source:     &Source{Provider: "github"},
	}
	branch := c.generateBranchName(wu)
	// Default pattern: feature/{key}-{slug}
	want := "feature/EXT-123-add-user-authentication"
	if branch != want {
		t.Errorf("generateBranchName with source = %q, want %q", branch, want)
	}
}

func TestGenerateBranchName_NoSource(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "local-id-789", Title: "Fix bug", Source: nil}
	branch := c.generateBranchName(wu)
	// Default pattern: feature/{key}-{slug}, no source so key=ID
	want := "feature/local-id-789-fix-bug"
	if branch != want {
		t.Errorf("generateBranchName without source = %q, want %q", branch, want)
	}
}

func TestGenerateBranchName_EmptyTitle(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "task-456", Title: "", Source: nil}
	branch := c.generateBranchName(wu)
	// Empty title results in no slug, trailing dashes cleaned up
	want := "feature/task-456"
	if branch != want {
		t.Errorf("generateBranchName with empty title = %q, want %q", branch, want)
	}
}

// ─── buildHierarchySection ───────────────────────────────────────────────────

func TestBuildHierarchySection_Nil(t *testing.T) {
	result := buildHierarchySection(nil)
	if result != "" {
		t.Errorf("buildHierarchySection(nil) = %q, want empty string", result)
	}
}

func TestBuildHierarchySection_EmptyHierarchy(t *testing.T) {
	result := buildHierarchySection(&HierarchyContext{})
	if result != "" {
		t.Errorf("buildHierarchySection(empty) = %q, want empty string", result)
	}
}

func TestBuildHierarchySection_WithParent(t *testing.T) {
	h := &HierarchyContext{
		Parent: &TaskSummary{
			ID:     "parent-1",
			Title:  "Parent Task",
			Status: "in-progress",
		},
	}
	result := buildHierarchySection(h)
	if !strings.Contains(result, "Parent Task") {
		t.Errorf("buildHierarchySection with parent missing title, got: %q", result)
	}
	if !strings.Contains(result, "Parent Task Context") {
		t.Errorf("buildHierarchySection with parent missing section header, got: %q", result)
	}
	if !strings.Contains(result, "in-progress") {
		t.Errorf("buildHierarchySection with parent missing status, got: %q", result)
	}
}

func TestBuildHierarchySection_WithParentDescription(t *testing.T) {
	h := &HierarchyContext{
		Parent: &TaskSummary{
			Title:       "Parent",
			Description: "A parent description",
		},
	}
	result := buildHierarchySection(h)
	if !strings.Contains(result, "A parent description") {
		t.Errorf("buildHierarchySection missing parent description, got: %q", result)
	}
}

func TestBuildHierarchySection_WithSiblings(t *testing.T) {
	h := &HierarchyContext{
		Siblings: []TaskSummary{
			{ID: "s1", Title: "Sibling One", Status: "done"},
			{ID: "s2", Title: "Sibling Two"},
		},
	}
	result := buildHierarchySection(h)
	if !strings.Contains(result, "Sibling One") {
		t.Errorf("buildHierarchySection missing sibling one, got: %q", result)
	}
	if !strings.Contains(result, "Sibling Two") {
		t.Errorf("buildHierarchySection missing sibling two, got: %q", result)
	}
	if !strings.Contains(result, "Related Subtasks") {
		t.Errorf("buildHierarchySection missing siblings header, got: %q", result)
	}
	if !strings.Contains(result, "done") {
		t.Errorf("buildHierarchySection missing sibling status, got: %q", result)
	}
}

func TestBuildHierarchySection_LongParentDescription(t *testing.T) {
	longDesc := strings.Repeat("x", 600)
	h := &HierarchyContext{
		Parent: &TaskSummary{Title: "P", Description: longDesc},
	}
	result := buildHierarchySection(h)
	// Should be truncated at 500 chars + "..."
	if !strings.Contains(result, "...") {
		t.Error("long parent description should be truncated with '...'")
	}
}

// ─── buildPlanPromptForComplexity ─────────────────────────────────────────────

func TestBuildPlanPromptForComplexity_Simple(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{Title: "Fix typo", Description: "Change 'teh' to 'the'"}
	c.ForceWorkUnit(wu)
	prompt := c.buildPlanPromptForComplexity(ComplexitySimple, "")
	if !strings.Contains(prompt, "straightforward task") {
		t.Errorf("simple prompt missing 'straightforward task', got: %q", prompt[:100])
	}
	if !strings.Contains(prompt, "Fix typo") {
		t.Error("simple prompt missing task title")
	}
}

func TestBuildPlanPromptForComplexity_Complex(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{Title: "Refactor auth", Description: "Redesign the entire auth system"}
	c.ForceWorkUnit(wu)
	prompt := c.buildPlanPromptForComplexity(ComplexityComplex, "")
	if !strings.Contains(prompt, "expert software engineer") {
		t.Errorf("complex prompt missing 'expert software engineer', got: %q", prompt[:100])
	}
	if !strings.Contains(prompt, "Refactor auth") {
		t.Error("complex prompt missing task title")
	}
}

func TestBuildPlanPromptForComplexity_WithHierarchy(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{
		Title:       "Subtask",
		Description: "Part of a parent",
		Hierarchy: &HierarchyContext{
			Parent: &TaskSummary{Title: "Epic Parent", Status: "active"},
		},
	}
	c.ForceWorkUnit(wu)
	prompt := c.buildPlanPromptForComplexity(ComplexityComplex, "")
	if !strings.Contains(prompt, "Epic Parent") {
		t.Error("prompt with hierarchy missing parent title")
	}
}

// ─── buildImplementPrompt ─────────────────────────────────────────────────────

func TestBuildImplementPrompt(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{
		Title:       "Add feature",
		Description: "Implement the new feature",
	}
	c.ForceWorkUnit(wu)
	prompt := c.buildImplementPrompt()
	if !strings.Contains(prompt, "Add feature") {
		t.Error("implement prompt missing task title")
	}
	if !strings.Contains(prompt, "Implement the new feature") {
		t.Error("implement prompt missing task description")
	}
}

func TestBuildImplementPrompt_WithSpecs(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{
		Title:          "Add feature",
		Description:    "Do it",
		Specifications: []string{"spec-1.md", "spec-2.md"},
	}
	c.ForceWorkUnit(wu)
	prompt := c.buildImplementPrompt()
	if !strings.Contains(prompt, "spec-1.md") {
		t.Error("implement prompt missing specification")
	}
}

// ─── buildJobOptions ─────────────────────────────────────────────────────────

func TestBuildJobOptions_WithWorkUnit(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{
		ID:         "j1",
		Title:      "Job",
		ExternalID: "ext-1",
		Source:     &Source{Provider: "github", Reference: "github:owner/repo#1"},
	}
	c.ForceWorkUnit(wu)
	opts := c.buildJobOptions()
	if opts == nil {
		t.Fatal("buildJobOptions() returned nil")
	}
	if opts.Metadata["task_id"] != "j1" {
		t.Errorf("task_id = %q, want j1", opts.Metadata["task_id"])
	}
	if opts.Metadata["provider"] != "github" {
		t.Errorf("provider = %q, want github", opts.Metadata["provider"])
	}
}

func TestBuildJobOptions_NoWorkUnit(t *testing.T) {
	c, _ := New()
	opts := c.buildJobOptions()
	if opts == nil {
		t.Fatal("buildJobOptions() returned nil")
	}
	if len(opts.Metadata) != 0 {
		t.Errorf("buildJobOptions with no workunit: metadata should be empty, got %v", opts.Metadata)
	}
}

// ─── workUnitToTaskState / taskStateToWorkUnit ───────────────────────────────

func TestWorkUnitToTaskState_Basic(t *testing.T) {
	now := time.Now()
	wu := &WorkUnit{
		ID:          "wu-1",
		ExternalID:  "ext-1",
		Title:       "Test",
		Description: "Desc",
		Branch:      "kvelmo/test",
		Source: &Source{
			Provider:  "github",
			Reference: "github:owner/repo#1",
			URL:       "https://example.com",
			Content:   "content",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	ts := workUnitToTaskState(StatePlanned, wu, nil)
	if ts.State != string(StatePlanned) {
		t.Errorf("State = %q, want %q", ts.State, StatePlanned)
	}
	if ts.ID != "wu-1" {
		t.Errorf("ID = %q, want wu-1", ts.ID)
	}
	if ts.Source == nil {
		t.Fatal("Source should not be nil")
	}
	if ts.Source.Provider != "github" {
		t.Errorf("Source.Provider = %q, want github", ts.Source.Provider)
	}
}

func TestWorkUnitToTaskState_WithHierarchy(t *testing.T) {
	wu := &WorkUnit{
		ID:    "wu-2",
		Title: "Sub",
		Hierarchy: &HierarchyContext{
			Parent: &TaskSummary{ID: "p1", Title: "Parent"},
			Siblings: []TaskSummary{
				{ID: "s1", Title: "Sibling"},
			},
		},
	}
	ts := workUnitToTaskState(StateLoaded, wu, nil)
	if ts.Hierarchy == nil {
		t.Fatal("Hierarchy should not be nil")
	}
	if ts.Hierarchy.Parent == nil {
		t.Fatal("Hierarchy.Parent should not be nil")
	}
	if ts.Hierarchy.Parent.ID != "p1" {
		t.Errorf("Hierarchy.Parent.ID = %q, want p1", ts.Hierarchy.Parent.ID)
	}
	if len(ts.Hierarchy.Siblings) != 1 {
		t.Errorf("Hierarchy.Siblings length = %d, want 1", len(ts.Hierarchy.Siblings))
	}
}

func TestTaskStateToWorkUnit_Basic(t *testing.T) {
	now := time.Now()
	ts := &storage.TaskState{
		State:       "planned",
		ID:          "ts-1",
		Title:       "Task",
		Description: "Desc",
		Branch:      "kvelmo/test",
		Source: &storage.TaskSource{
			Provider:  "file",
			Reference: "file:task.md",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	state, wu, _ := taskStateToWorkUnit(ts)
	if state != StatePlanned {
		t.Errorf("state = %q, want planned", state)
	}
	if wu.ID != "ts-1" {
		t.Errorf("wu.ID = %q, want ts-1", wu.ID)
	}
	if wu.Source == nil {
		t.Fatal("wu.Source should not be nil")
	}
	if wu.Source.Provider != "file" {
		t.Errorf("wu.Source.Provider = %q, want file", wu.Source.Provider)
	}
}

func TestTaskStateToWorkUnit_NilMetadata(t *testing.T) {
	ts := &storage.TaskState{
		State:    "none",
		ID:       "ts-2",
		Title:    "T",
		Metadata: nil, // nil should be initialized to empty map
	}
	_, wu, _ := taskStateToWorkUnit(ts)
	if wu.Metadata == nil {
		t.Error("wu.Metadata should be initialized (not nil) when ts.Metadata is nil")
	}
}

func TestTaskStateToWorkUnit_WithHierarchy(t *testing.T) {
	ts := &storage.TaskState{
		State: "loaded",
		ID:    "ts-3",
		Title: "Sub",
		Hierarchy: &storage.TaskHierarchy{
			Parent: &storage.TaskHierarchySummary{
				ID:    "p1",
				Title: "Parent",
			},
			Siblings: []storage.TaskHierarchySummary{
				{ID: "s1", Title: "Sibling"},
			},
		},
	}
	_, wu, _ := taskStateToWorkUnit(ts)
	if wu.Hierarchy == nil {
		t.Fatal("wu.Hierarchy should not be nil")
	}
	if wu.Hierarchy.Parent == nil {
		t.Fatal("wu.Hierarchy.Parent should not be nil")
	}
	if wu.Hierarchy.Parent.ID != "p1" {
		t.Errorf("wu.Hierarchy.Parent.ID = %q, want p1", wu.Hierarchy.Parent.ID)
	}
	if len(wu.Hierarchy.Siblings) != 1 {
		t.Errorf("wu.Hierarchy.Siblings length = %d, want 1", len(wu.Hierarchy.Siblings))
	}
}

func TestWorkUnitToTaskState_IncludesHistory(t *testing.T) {
	history := []HistoryEntry{
		{From: StateNone, To: StateLoaded, Event: EventStart, Timestamp: time.Now()},
		{From: StateLoaded, To: StatePlanning, Event: EventPlan, Timestamp: time.Now()},
	}
	wu := &WorkUnit{
		ID:        "test-1",
		Title:     "Test Task",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  map[string]string{},
	}
	ts := workUnitToTaskState(StateLoaded, wu, history)
	if len(ts.History) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(ts.History))
	}
	if ts.History[0].Event != "start" {
		t.Errorf("expected first event 'start', got %q", ts.History[0].Event)
	}
}

func TestTaskStateToWorkUnit_RestoresHistory(t *testing.T) {
	now := time.Now()
	ts := &storage.TaskState{
		State: "loaded",
		ID:    "ts-hist",
		Title: "History Test",
		History: []storage.TaskHistoryEntry{
			{From: "none", To: "loaded", Event: "start", Timestamp: now},
			{From: "loaded", To: "planning", Event: "plan", Timestamp: now},
		},
	}
	state, wu, history := taskStateToWorkUnit(ts)
	if state != StateLoaded {
		t.Errorf("state = %q, want loaded", state)
	}
	if wu.ID != "ts-hist" {
		t.Errorf("wu.ID = %q, want ts-hist", wu.ID)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	if history[0].Event != EventStart {
		t.Errorf("history[0].Event = %q, want start", history[0].Event)
	}
	if history[1].From != StateLoaded {
		t.Errorf("history[1].From = %q, want loaded", history[1].From)
	}
}

// ─── State machine extra ─────────────────────────────────────────────────────

func TestMachineIsTerminal_False(t *testing.T) {
	m := NewMachine()
	if m.IsTerminal() {
		t.Error("IsTerminal() should be false for initial StateNone")
	}
}

func TestMachineIsTerminal_Submitted(t *testing.T) {
	// StateSubmitted is no longer terminal - it can transition to StateNone via EventFinish
	m := NewMachine()
	m.ForceState(StateSubmitted)
	if m.IsTerminal() {
		t.Error("IsTerminal() should be false for StateSubmitted (can finish)")
	}
}

func TestMachineIsPhase_MainPhase(t *testing.T) {
	m := NewMachine()
	// StateNone is a Phase state
	if !m.IsPhase() {
		t.Error("IsPhase() should be true for StateNone")
	}
}

func TestMachineIsPhase_AuxiliaryState(t *testing.T) {
	m := NewMachine()
	m.ForceState(StateFailed)
	if m.IsPhase() {
		t.Error("IsPhase() should be false for StateFailed (auxiliary state)")
	}
}

func TestMachineForceState(t *testing.T) {
	m := NewMachine()
	m.ForceState(StateImplemented)
	if m.State() != StateImplemented {
		t.Errorf("ForceState: State() = %s, want %s", m.State(), StateImplemented)
	}
}

func TestMachineDispatchWithResume_Answer(t *testing.T) {
	m := NewMachine()
	ctx := context.Background()

	wu := &WorkUnit{
		ID:          "r1",
		Title:       "Resume Test",
		Source:      &Source{Provider: "test", Reference: "ref", Content: "c"},
		Description: "desc",
	}
	m.SetWorkUnit(wu)

	// Transition to Planning then to Waiting
	_ = m.Dispatch(ctx, EventStart) // None -> Loaded
	_ = m.Dispatch(ctx, EventPlan)  // Loaded -> Planning
	_ = m.Dispatch(ctx, EventWait)  // Planning -> Waiting

	if m.State() != StateWaiting {
		t.Fatalf("expected StateWaiting, got %s", m.State())
	}

	// DispatchWithResume(Answer) should return to Planning
	if err := m.DispatchWithResume(ctx, EventAnswer); err != nil {
		t.Errorf("DispatchWithResume(EventAnswer) error = %v", err)
	}
	if m.State() != StatePlanning {
		t.Errorf("after DispatchWithResume(Answer): state = %s, want StatePlanning", m.State())
	}
}

func TestMachineDispatchWithResume_Pause(t *testing.T) {
	m := NewMachine()
	ctx := context.Background()

	wu := &WorkUnit{
		ID:          "p1",
		Title:       "Pause Test",
		Source:      &Source{Provider: "test", Reference: "ref", Content: "c"},
		Description: "desc",
	}
	m.SetWorkUnit(wu)

	_ = m.Dispatch(ctx, EventStart)
	_ = m.Dispatch(ctx, EventPlan)
	_ = m.Dispatch(ctx, EventPause)

	if m.State() != StatePaused {
		t.Fatalf("expected StatePaused, got %s", m.State())
	}

	// DispatchWithResume(Resume) should return to Planning
	if err := m.DispatchWithResume(ctx, EventResume); err != nil {
		t.Errorf("DispatchWithResume(EventResume) error = %v", err)
	}
	if m.State() != StatePlanning {
		t.Errorf("after DispatchWithResume(Resume): state = %s, want StatePlanning", m.State())
	}
}

func TestMachineAddListener(t *testing.T) {
	m := NewMachine()
	ctx := context.Background()

	wu := &WorkUnit{
		ID:     "l1",
		Title:  "Listener Test",
		Source: &Source{Provider: "test", Reference: "ref"},
	}
	m.SetWorkUnit(wu)

	done := make(chan struct{})
	var fromState, toState State
	m.AddListener(func(from, to State, _ Event, _ *WorkUnit) {
		fromState = from
		toState = to
		close(done)
	})

	_ = m.Dispatch(ctx, EventStart)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("listener was not called within 1s")
	}

	if fromState != StateNone {
		t.Errorf("listener: from = %s, want StateNone", fromState)
	}
	if toState != StateLoaded {
		t.Errorf("listener: to = %s, want StateLoaded", toState)
	}
}

// ─── Package-level functions ─────────────────────────────────────────────────

func TestCanTransition_Valid(t *testing.T) {
	if !CanTransition(StateNone, StateLoaded) {
		t.Error("CanTransition(None, Loaded) should be true")
	}
}

func TestCanTransition_Invalid(t *testing.T) {
	if CanTransition(StateNone, StateSubmitted) {
		t.Error("CanTransition(None, Submitted) should be false")
	}
}

func TestNextStates_FromNone(t *testing.T) {
	states := NextStates(StateNone)
	if len(states) == 0 {
		t.Error("NextStates(None) should return at least one state")
	}
	if !slices.Contains(states, StateLoaded) {
		t.Error("NextStates(None) should include StateLoaded")
	}
}

func TestNextStates_FromSubmitted(t *testing.T) {
	states := NextStates(StateSubmitted)
	// Submitted can transition to many states via re-entry transitions + finish + abort
	if len(states) < 2 {
		t.Errorf("NextStates(Submitted) should return multiple states (re-entry + finish), got %v", states)
	}
	// Must include None (via Finish)
	hasNone := false
	for _, s := range states {
		if s == StateNone {
			hasNone = true
		}
	}
	if !hasNone {
		t.Errorf("NextStates(Submitted) should include none (via Finish), got %v", states)
	}
}

func TestEvaluateGuards_NoGuards(t *testing.T) {
	result := EvaluateGuards(context.Background(), nil, nil)
	if !result {
		t.Error("EvaluateGuards with no guards should return true")
	}
}

func TestEvaluateGuards_PassingGuard(t *testing.T) {
	wu := &WorkUnit{Source: &Source{Reference: "ref"}}
	guards := []Guard{{Check: guardHasSource, Message: "no source"}}
	result := EvaluateGuards(context.Background(), wu, guards)
	if !result {
		t.Error("EvaluateGuards with passing guard should return true")
	}
}

func TestEvaluateGuards_FailingGuard(t *testing.T) {
	wu := &WorkUnit{Source: &Source{Reference: ""}} // empty reference
	guards := []Guard{{Check: guardHasSource, Message: "no source"}}
	result := EvaluateGuards(context.Background(), wu, guards)
	if result {
		t.Error("EvaluateGuards with failing guard should return false")
	}
}

// ─── Guard functions ─────────────────────────────────────────────────────────

func TestGuardHasSource(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{"nil wu", nil, false},
		{"nil source", &WorkUnit{}, false},
		{"empty reference", &WorkUnit{Source: &Source{}}, false},
		{"valid source", &WorkUnit{Source: &Source{Reference: "ref"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := guardHasSource(ctx, tt.wu); got != tt.want {
				t.Errorf("guardHasSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardHasDescription(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{"nil wu", nil, false},
		{"empty description", &WorkUnit{}, false},
		{"has description", &WorkUnit{Description: "do something"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := guardHasDescription(ctx, tt.wu); got != tt.want {
				t.Errorf("guardHasDescription() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardHasSpecifications(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{"nil wu", nil, false},
		{"no specs", &WorkUnit{}, false},
		{"has specs", &WorkUnit{Specifications: []string{"spec.md"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := guardHasSpecifications(ctx, tt.wu); got != tt.want {
				t.Errorf("guardHasSpecifications() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardCanUndo(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{"nil wu", nil, false},
		{"no checkpoints", &WorkUnit{}, false},
		{"has checkpoints", &WorkUnit{Checkpoints: []string{"abc"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := guardCanUndo(ctx, tt.wu); got != tt.want {
				t.Errorf("guardCanUndo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardCanRedo(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{"nil wu", nil, false},
		{"no redo stack", &WorkUnit{}, false},
		{"has redo stack", &WorkUnit{RedoStack: []string{"def"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := guardCanRedo(ctx, tt.wu); got != tt.want {
				t.Errorf("guardCanRedo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardCanSubmit(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{"nil wu", nil, false},
		{"nil source", &WorkUnit{}, false},
		{"empty provider", &WorkUnit{Source: &Source{}}, false},
		{"has provider", &WorkUnit{Source: &Source{Provider: "github"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := guardCanSubmit(ctx, tt.wu); got != tt.want {
				t.Errorf("guardCanSubmit() = %v, want %v", got, tt.want)
			}
		})
	}
}
