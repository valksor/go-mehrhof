package conductor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/events"
	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.AgentName != "" {
		t.Errorf("AgentName = %q, want empty (auto-detect)", opts.AgentName)
	}
	if opts.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want %v", opts.Timeout, 30*time.Minute)
	}
	if opts.DryRun != false {
		t.Errorf("DryRun = %v, want false", opts.DryRun)
	}
	if opts.Verbose != false {
		t.Errorf("Verbose = %v, want false", opts.Verbose)
	}
	if opts.Stdout != os.Stdout {
		t.Error("Stdout is not os.Stdout")
	}
	if opts.Stderr != os.Stderr {
		t.Error("Stderr is not os.Stderr")
	}
	if opts.WorkDir != "." {
		t.Errorf("WorkDir = %q, want %q", opts.WorkDir, ".")
	}
}

func TestWithAgent(t *testing.T) {
	opts := DefaultOptions()
	WithAgent("claude")(&opts)

	if opts.AgentName != "claude" {
		t.Errorf("AgentName = %q, want %q", opts.AgentName, "claude")
	}
}

func TestWithTimeout(t *testing.T) {
	opts := DefaultOptions()
	WithTimeout(10 * time.Minute)(&opts)

	if opts.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want %v", opts.Timeout, 10*time.Minute)
	}
}

func TestWithDryRun(t *testing.T) {
	opts := DefaultOptions()
	WithDryRun(true)(&opts)

	if opts.DryRun != true {
		t.Errorf("DryRun = %v, want true", opts.DryRun)
	}
}

func TestWithVerbose(t *testing.T) {
	opts := DefaultOptions()
	WithVerbose(true)(&opts)

	if opts.Verbose != true {
		t.Errorf("Verbose = %v, want true", opts.Verbose)
	}
}

func TestWithCreateBranch(t *testing.T) {
	opts := DefaultOptions()
	WithCreateBranch(true)(&opts)

	if opts.CreateBranch != true {
		t.Errorf("CreateBranch = %v, want true", opts.CreateBranch)
	}
}

func TestWithAutoInit(t *testing.T) {
	opts := DefaultOptions()
	WithAutoInit(true)(&opts)

	if opts.AutoInit != true {
		t.Errorf("AutoInit = %v, want true", opts.AutoInit)
	}
}

func TestWithStdout(t *testing.T) {
	opts := DefaultOptions()
	buf := &bytes.Buffer{}
	WithStdout(buf)(&opts)

	if opts.Stdout != buf {
		t.Error("Stdout was not set correctly")
	}
}

func TestWithStderr(t *testing.T) {
	opts := DefaultOptions()
	buf := &bytes.Buffer{}
	WithStderr(buf)(&opts)

	if opts.Stderr != buf {
		t.Error("Stderr was not set correctly")
	}
}

func TestWithWorkDir(t *testing.T) {
	opts := DefaultOptions()
	WithWorkDir("/tmp/test")(&opts)

	if opts.WorkDir != "/tmp/test" {
		t.Errorf("WorkDir = %q, want %q", opts.WorkDir, "/tmp/test")
	}
}

func TestWithStateChangeCallback(t *testing.T) {
	opts := DefaultOptions()
	called := false
	callback := func(from, to string) {
		called = true
	}
	WithStateChangeCallback(callback)(&opts)

	if opts.OnStateChange == nil {
		t.Fatal("OnStateChange is nil")
	}

	opts.OnStateChange("idle", "planning")
	if !called {
		t.Error("callback was not called")
	}
}

func TestWithProgressCallback(t *testing.T) {
	opts := DefaultOptions()
	called := false
	callback := func(message string, percent int) {
		called = true
	}
	WithProgressCallback(callback)(&opts)

	if opts.OnProgress == nil {
		t.Fatal("OnProgress is nil")
	}

	opts.OnProgress("test", 50)
	if !called {
		t.Error("callback was not called")
	}
}

func TestWithErrorCallback(t *testing.T) {
	opts := DefaultOptions()
	called := false
	callback := func(err error) {
		called = true
	}
	WithErrorCallback(callback)(&opts)

	if opts.OnError == nil {
		t.Fatal("OnError is nil")
	}

	opts.OnError(nil)
	if !called {
		t.Error("callback was not called")
	}
}

func TestOptionsApply(t *testing.T) {
	opts := DefaultOptions()
	opts.Apply(
		WithAgent("claude"),
		WithTimeout(5*time.Minute),
		WithDryRun(true),
		WithVerbose(true),
	)

	if opts.AgentName != "claude" {
		t.Errorf("AgentName = %q, want %q", opts.AgentName, "claude")
	}
	if opts.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want %v", opts.Timeout, 5*time.Minute)
	}
	if opts.DryRun != true {
		t.Errorf("DryRun = %v, want true", opts.DryRun)
	}
	if opts.Verbose != true {
		t.Errorf("Verbose = %v, want true", opts.Verbose)
	}
}

func TestDefaultFinishOptions(t *testing.T) {
	opts := DefaultFinishOptions()

	if opts.SquashMerge != true {
		t.Errorf("SquashMerge = %v, want true", opts.SquashMerge)
	}
	if opts.DeleteBranch != false {
		t.Errorf("DeleteBranch = %v, want false (don't delete by default)", opts.DeleteBranch)
	}
	if opts.TargetBranch != "" {
		t.Errorf("TargetBranch = %q, want empty (auto-detect)", opts.TargetBranch)
	}
	if opts.PushAfter != false {
		t.Errorf("PushAfter = %v, want false", opts.PushAfter)
	}
	if opts.ForceMerge != false {
		t.Errorf("ForceMerge = %v, want false (create PR by default)", opts.ForceMerge)
	}
}

func TestFinishOptionsStruct(t *testing.T) {
	opts := FinishOptions{
		SquashMerge:  false,
		DeleteBranch: false,
		TargetBranch: "main",
		PushAfter:    true,
	}

	if opts.SquashMerge != false {
		t.Errorf("SquashMerge = %v, want false", opts.SquashMerge)
	}
	if opts.DeleteBranch != false {
		t.Errorf("DeleteBranch = %v, want false", opts.DeleteBranch)
	}
	if opts.TargetBranch != "main" {
		t.Errorf("TargetBranch = %q, want %q", opts.TargetBranch, "main")
	}
	if opts.PushAfter != true {
		t.Errorf("PushAfter = %v, want true", opts.PushAfter)
	}
}

func TestTaskStatusStruct(t *testing.T) {
	now := time.Now()
	status := TaskStatus{
		TaskID:         "task123",
		Title:          "Test Task",
		State:          "planning",
		Ref:            "file:task.md",
		Branch:         "task/task123",
		Specifications: 2,
		Checkpoints:    3,
		Started:        now,
	}

	if status.TaskID != "task123" {
		t.Errorf("TaskID = %q, want %q", status.TaskID, "task123")
	}
	if status.Title != "Test Task" {
		t.Errorf("Title = %q, want %q", status.Title, "Test Task")
	}
	if status.State != "planning" {
		t.Errorf("State = %q, want %q", status.State, "planning")
	}
	if status.Ref != "file:task.md" {
		t.Errorf("Ref = %q, want %q", status.Ref, "file:task.md")
	}
	if status.Branch != "task/task123" {
		t.Errorf("Branch = %q, want %q", status.Branch, "task/task123")
	}
	if status.Specifications != 2 {
		t.Errorf("Specifications = %d, want 2", status.Specifications)
	}
	if status.Checkpoints != 3 {
		t.Errorf("Checkpoints = %d, want 3", status.Checkpoints)
	}
	if status.Started != now {
		t.Errorf("Started = %v, want %v", status.Started, now)
	}
}

func TestOptionsStruct(t *testing.T) {
	opts := Options{
		AgentName:    "claude",
		Timeout:      15 * time.Minute,
		DryRun:       true,
		Verbose:      true,
		CreateBranch: true,
		AutoInit:     true,
		WorkDir:      "/tmp/work",
	}

	if opts.AgentName != "claude" {
		t.Errorf("AgentName = %q, want %q", opts.AgentName, "claude")
	}
	if opts.Timeout != 15*time.Minute {
		t.Errorf("Timeout = %v, want %v", opts.Timeout, 15*time.Minute)
	}
	if opts.DryRun != true {
		t.Errorf("DryRun = %v, want true", opts.DryRun)
	}
	if opts.CreateBranch != true {
		t.Errorf("CreateBranch = %v, want true", opts.CreateBranch)
	}
	if opts.AutoInit != true {
		t.Errorf("AutoInit = %v, want true", opts.AutoInit)
	}
}

func TestWithUseWorktree(t *testing.T) {
	opts := DefaultOptions()
	WithUseWorktree(true)(&opts)

	if opts.UseWorktree != true {
		t.Errorf("UseWorktree = %v, want true", opts.UseWorktree)
	}
}

func TestGetTaskWork_Nil(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	work := c.GetTaskWork()
	if work != nil {
		t.Error("GetTaskWork should return nil before task is started")
	}
}

func TestGetMachine(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	machine := c.GetMachine()
	if machine == nil {
		t.Error("GetMachine should return non-nil machine")
	}
	if machine != c.machine {
		t.Error("GetMachine returned different machine")
	}
}

func TestBuildWorkUnit_Nil(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	wu := c.buildWorkUnit()
	if wu != nil {
		t.Error("buildWorkUnit should return nil when taskWork is nil")
	}
}

func TestOnStateChanged_NoCallback(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Should not panic with nil callback
	c.onStateChanged(events.Event{
		Data: map[string]any{
			"from": "idle",
			"to":   "planning",
		},
	})
}

func TestOnStateChanged_WithCallback(t *testing.T) {
	callbackCalled := false
	var fromState, toState string

	c, err := New(WithStateChangeCallback(func(from, to string) {
		callbackCalled = true
		fromState = from
		toState = to
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	c.onStateChanged(events.Event{
		Data: map[string]any{
			"from": "idle",
			"to":   "planning",
		},
	})

	if !callbackCalled {
		t.Error("state change callback was not called")
	}
	if fromState != "idle" {
		t.Errorf("from = %q, want %q", fromState, "idle")
	}
	if toState != "planning" {
		t.Errorf("to = %q, want %q", toState, "planning")
	}
}

func TestOnStateChanged_MissingData(t *testing.T) {
	callbackCalled := false
	var fromState, toState string

	c, err := New(WithStateChangeCallback(func(from, to string) {
		callbackCalled = true
		fromState = from
		toState = to
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Event with no data fields
	c.onStateChanged(events.Event{
		Data: map[string]any{},
	})

	if !callbackCalled {
		t.Error("state change callback was not called")
	}
	if fromState != "" {
		t.Errorf("from = %q, want empty", fromState)
	}
	if toState != "" {
		t.Errorf("to = %q, want empty", toState)
	}
}

func TestLogError_NoCallback(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Should not panic with nil callback
	c.logError(fmt.Errorf("test error"))
}

func TestLogError_WithCallback(t *testing.T) {
	callbackCalled := false
	var capturedErr error

	c, err := New(WithErrorCallback(func(err error) {
		callbackCalled = true
		capturedErr = err
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	testErr := fmt.Errorf("test error")
	c.logError(testErr)

	if !callbackCalled {
		t.Error("error callback was not called")
	}
	if capturedErr != testErr {
		t.Errorf("captured error = %v, want %v", capturedErr, testErr)
	}
}

func TestCountCheckpoints_NoActiveTask(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	count := c.countCheckpoints()
	if count != 0 {
		t.Errorf("countCheckpoints = %d, want 0 when no active task", count)
	}
}

func TestCountCheckpoints_NoGit(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	c.activeTask = &storage.ActiveTask{
		ID:    "test-task",
		State: "planning",
	}

	count := c.countCheckpoints()
	if count != 0 {
		t.Errorf("countCheckpoints = %d, want 0 when git is nil", count)
	}
}

func TestPublishProgress_NoCallback(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Should not panic with nil callback
	c.publishProgress("test message", 50)
}

func TestPublishProgress_WithCallback(t *testing.T) {
	callbackCalled := false
	var capturedMessage string
	var capturedPercent int

	c, err := New(WithProgressCallback(func(message string, percent int) {
		callbackCalled = true
		capturedMessage = message
		capturedPercent = percent
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	c.publishProgress("test message", 75)

	if !callbackCalled {
		t.Error("progress callback was not called")
	}
	if capturedMessage != "test message" {
		t.Errorf("message = %q, want %q", capturedMessage, "test message")
	}
	if capturedPercent != 75 {
		t.Errorf("percent = %d, want 75", capturedPercent)
	}
}

func TestStatus_WithActiveTask(t *testing.T) {
	tmpDir := t.TempDir()

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
	work.Metadata.Title = "Test Title"

	started := time.Now()
	c.workspace = ws
	c.activeTask = &storage.ActiveTask{
		ID:      "test-task",
		State:   "planning",
		Ref:     "file:task.md",
		Branch:  "task/test-task",
		Started: started,
	}
	c.taskWork = work

	// Create some specs
	if err := ws.SaveSpecification("test-task", 1, "# Spec 1"); err != nil {
		t.Fatalf("SaveSpec: %v", err)
	}
	if err := ws.SaveSpecification("test-task", 2, "# Spec 2"); err != nil {
		t.Fatalf("SaveSpec: %v", err)
	}

	status, err := c.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if status.TaskID != "test-task" {
		t.Errorf("TaskID = %q, want %q", status.TaskID, "test-task")
	}
	if status.Title != "Test Title" {
		t.Errorf("Title = %q, want %q", status.Title, "Test Title")
	}
	if status.State != "planning" {
		t.Errorf("State = %q, want %q", status.State, "planning")
	}
	if status.Ref != "file:task.md" {
		t.Errorf("Ref = %q, want %q", status.Ref, "file:task.md")
	}
	if status.Branch != "task/test-task" {
		t.Errorf("Branch = %q, want %q", status.Branch, "task/test-task")
	}
	if status.Specifications != 2 {
		t.Errorf("Specifications = %d, want 2", status.Specifications)
	}
	if status.Started != started {
		t.Errorf("Started = %v, want %v", status.Started, started)
	}
}

func TestDeleteOptionsStruct(t *testing.T) {
	deleteWork := BoolPtr(false) // explicit: keep
	opts := DeleteOptions{
		Force:      true,
		KeepBranch: true,
		DeleteWork: deleteWork,
	}

	if opts.Force != true {
		t.Errorf("Force = %v, want true", opts.Force)
	}
	if opts.KeepBranch != true {
		t.Errorf("KeepBranch = %v, want true", opts.KeepBranch)
	}
	if opts.DeleteWork == nil || *opts.DeleteWork != false {
		t.Errorf("DeleteWork = %v, want false (keep)", opts.DeleteWork)
	}
}

func TestDefaultDeleteOptions(t *testing.T) {
	opts := DefaultDeleteOptions()

	if opts.Force != false {
		t.Errorf("Force = %v, want false", opts.Force)
	}
	if opts.KeepBranch != false {
		t.Errorf("KeepBranch = %v, want false", opts.KeepBranch)
	}
	if opts.DeleteWork != nil {
		t.Errorf("DeleteWork = %v, want nil (defer to config)", opts.DeleteWork)
	}
}

func TestWithStepAgent(t *testing.T) {
	opts := DefaultOptions()
	WithStepAgent("planning", "glm")(&opts)
	WithStepAgent("implementing", "claude")(&opts)

	if opts.StepAgents == nil {
		t.Fatal("StepAgents should be initialized")
	}
	if opts.StepAgents["planning"] != "glm" {
		t.Errorf("StepAgents[planning] = %q, want %q", opts.StepAgents["planning"], "glm")
	}
	if opts.StepAgents["implementing"] != "claude" {
		t.Errorf("StepAgents[implementing] = %q, want %q", opts.StepAgents["implementing"], "claude")
	}
}

func TestWithIncludeFullContext(t *testing.T) {
	opts := DefaultOptions()
	WithIncludeFullContext(true)(&opts)

	if opts.IncludeFullContext != true {
		t.Errorf("IncludeFullContext = %v, want true", opts.IncludeFullContext)
	}
}

func TestWithDefaultProvider(t *testing.T) {
	opts := DefaultOptions()
	WithDefaultProvider("github")(&opts)

	if opts.DefaultProvider != "github" {
		t.Errorf("DefaultProvider = %q, want %q", opts.DefaultProvider, "github")
	}
}

func TestWithExternalKey(t *testing.T) {
	opts := DefaultOptions()
	WithExternalKey("FEATURE-123")(&opts)

	if opts.ExternalKey != "FEATURE-123" {
		t.Errorf("ExternalKey = %q, want %q", opts.ExternalKey, "FEATURE-123")
	}
}

func TestWithCommitPrefixTemplate(t *testing.T) {
	opts := DefaultOptions()
	WithCommitPrefixTemplate("[{key}]")(&opts)

	if opts.CommitPrefixTemplate != "[{key}]" {
		t.Errorf("CommitPrefixTemplate = %q, want %q", opts.CommitPrefixTemplate, "[{key}]")
	}
}

func TestWithBranchPatternTemplate(t *testing.T) {
	opts := DefaultOptions()
	WithBranchPatternTemplate("{type}/{key}--{slug}")(&opts)

	if opts.BranchPatternTemplate != "{type}/{key}--{slug}" {
		t.Errorf("BranchPatternTemplate = %q, want %q", opts.BranchPatternTemplate, "{type}/{key}--{slug}")
	}
}

func TestGetStdout(t *testing.T) {
	buf := &bytes.Buffer{}
	c, err := New(WithStdout(buf))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stdout := c.GetStdout()
	if stdout != buf {
		t.Error("GetStdout did not return the configured stdout")
	}
}

func TestGetStderr(t *testing.T) {
	buf := &bytes.Buffer{}
	c, err := New(WithStderr(buf))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stderr := c.GetStderr()
	if stderr != buf {
		t.Error("GetStderr did not return the configured stderr")
	}
}

func TestGetPluginRegistry(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Initially nil since plugins are loaded during Initialize
	registry := c.GetPluginRegistry()
	if registry != nil {
		t.Error("GetPluginRegistry should return nil before Initialize")
	}
}

func TestLogVerbose_Disabled(t *testing.T) {
	buf := &bytes.Buffer{}
	c, err := New(WithStdout(buf), WithVerbose(false))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	c.logVerbose("test message %s", "arg")

	if buf.Len() != 0 {
		t.Errorf("logVerbose wrote output when verbose is disabled: %q", buf.String())
	}
}

func TestLogVerbose_Enabled(t *testing.T) {
	buf := &bytes.Buffer{}
	c, err := New(WithStdout(buf), WithVerbose(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	c.logVerbose("test message %s", "arg")

	expected := "test message arg\n"
	if buf.String() != expected {
		t.Errorf("logVerbose output = %q, want %q", buf.String(), expected)
	}
}

func TestLogVerbose_NilStdout(t *testing.T) {
	c, err := New(WithVerbose(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Set stdout to nil
	c.opts.Stdout = nil

	// Should not panic
	c.logVerbose("test message")
}

// Test generatePRTitle
func TestGeneratePRTitle(t *testing.T) {
	tests := []struct {
		name     string
		taskWork *storage.TaskWork
		want     string
	}{
		{
			name:     "nil taskWork",
			taskWork: nil,
			want:     "Implementation",
		},
		{
			name: "with external key and title",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					ExternalKey: "123",
					Title:       "Add login feature",
				},
			},
			want: "[#123] Add login feature",
		},
		{
			name: "with external key only",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					ExternalKey: "456",
				},
			},
			want: "[#456] Implementation",
		},
		{
			name: "with title only",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Fix bug",
				},
			},
			want: "Fix bug",
		},
		{
			name: "empty metadata",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{},
			},
			want: "Implementation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New()
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			c.taskWork = tt.taskWork

			got := c.generatePRTitle()
			if got != tt.want {
				t.Errorf("generatePRTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Test generatePRBody
func TestGeneratePRBody(t *testing.T) {
	tests := []struct {
		name        string
		taskWork    *storage.TaskWork
		specs       []*storage.Specification
		diffStat    string
		wantContain []string
	}{
		{
			name:     "nil taskWork",
			taskWork: nil,
			specs:    nil,
			diffStat: "",
			wantContain: []string{
				"## Summary",
				"## Test Plan",
				"*Generated by [Mehrhof]",
			},
		},
		{
			name: "with title",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Add feature",
				},
			},
			wantContain: []string{
				"Implementation for: Add feature",
			},
		},
		{
			name: "github issue closes link",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					ExternalKey: "42",
				},
				Source: storage.SourceInfo{
					Type: "github",
				},
			},
			wantContain: []string{
				"Closes #42",
			},
		},
		{
			name: "with specifications",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Test",
				},
			},
			specs: []*storage.Specification{
				{Number: 1, Title: "Spec 1", Content: "Details"},
			},
			wantContain: []string{
				"## Implementation Details",
				"### Spec 1",
				"Details",
			},
		},
		{
			name: "with diff stats",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Test",
				},
			},
			diffStat: " 2 files changed, 10 insertions(+)",
			wantContain: []string{
				"## Changes",
				"2 files changed",
			},
		},
		{
			name: "spec content truncated",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Test",
				},
			},
			specs: []*storage.Specification{
				{
					Number:  1,
					Content: string(make([]byte, 600)), // > 500 chars
				},
			},
			wantContain: []string{
				"...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New()
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			c.taskWork = tt.taskWork

			got := c.generatePRBody(tt.specs, tt.diffStat)
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("generatePRBody() missing %q in:\n%s", want, got)
				}
			}
		})
	}
}

// Test resolveTargetBranch
func TestResolveTargetBranch(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		taskWork  *storage.TaskWork
		want      string
	}{
		{
			name:      "explicit requested branch",
			requested: "develop",
			taskWork:  nil,
			want:      "develop",
		},
		{
			name:      "from taskWork base branch",
			requested: "",
			taskWork: &storage.TaskWork{
				Git: storage.GitInfo{
					BaseBranch: "main",
				},
			},
			want: "main",
		},
		{
			name:      "requested overrides taskWork",
			requested: "release",
			taskWork: &storage.TaskWork{
				Git: storage.GitInfo{
					BaseBranch: "main",
				},
			},
			want: "release",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New()
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			c.taskWork = tt.taskWork

			got := c.resolveTargetBranch(tt.requested)
			if got != tt.want {
				t.Errorf("resolveTargetBranch(%q) = %q, want %q", tt.requested, got, tt.want)
			}
		})
	}
}

// Test getDiffStats with nil git
func TestGetDiffStats_NilGit(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.git = nil
	c.taskWork = &storage.TaskWork{
		Git: storage.GitInfo{
			BaseBranch: "main",
		},
	}

	got := c.getDiffStats()
	if got != "" {
		t.Errorf("getDiffStats() = %q, want empty string", got)
	}
}

// Test getDiffStats with nil taskWork
func TestGetDiffStats_NilTaskWork(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.taskWork = nil

	got := c.getDiffStats()
	if got != "" {
		t.Errorf("getDiffStats() = %q, want empty string", got)
	}
}

// Test extractContextSummary
func TestExtractContextSummary(t *testing.T) {
	tests := []struct {
		name     string
		response *agent.Response
		want     string
	}{
		{
			name: "with summary",
			response: &agent.Response{
				Summary: "Brief summary",
			},
			want: "Brief summary",
		},
		{
			name: "no summary, has messages",
			response: &agent.Response{
				Messages: []string{"First message", "Second message"},
			},
			want: "First message",
		},
		{
			name: "long message truncated",
			response: &agent.Response{
				Messages: []string{string(make([]byte, 2500))},
			},
			want: "[truncated...]",
		},
		{
			name:     "empty response",
			response: &agent.Response{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractContextSummary(tt.response)

			if tt.want == "" {
				if got != "" {
					t.Errorf("extractContextSummary() = %q, want empty", got)
				}
			} else if !strings.Contains(got, tt.want) {
				t.Errorf("extractContextSummary() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

// Test buildFullContext
func TestBuildFullContext(t *testing.T) {
	tests := []struct {
		name        string
		response    *agent.Response
		wantContain []string
	}{
		{
			name: "with summary and messages",
			response: &agent.Response{
				Summary:  "Summary text",
				Messages: []string{"Msg 1", "Msg 2"},
			},
			wantContain: []string{
				"## Summary",
				"Summary text",
				"## Messages",
				"Msg 1",
				"Msg 2",
			},
		},
		{
			name: "only summary",
			response: &agent.Response{
				Summary: "Just summary",
			},
			wantContain: []string{
				"## Summary",
				"Just summary",
			},
		},
		{
			name:        "empty response",
			response:    &agent.Response{},
			wantContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFullContext(tt.response)

			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("buildFullContext() missing %q in:\n%s", want, got)
				}
			}
		})
	}
}

// Test extractExploredFiles
func TestExtractExploredFiles(t *testing.T) {
	tests := []struct {
		name     string
		response *agent.Response
		want     []string
	}{
		{
			name: "with file changes",
			response: &agent.Response{
				Files: []agent.FileChange{
					{Path: "file1.go"},
					{Path: "file2.go"},
				},
			},
			want: []string{"file1.go", "file2.go"},
		},
		{
			name: "deduplicates paths",
			response: &agent.Response{
				Files: []agent.FileChange{
					{Path: "file1.go"},
					{Path: "file1.go"},
					{Path: "file2.go"},
				},
			},
			want: []string{"file1.go", "file2.go"},
		},
		{
			name:     "no files",
			response: &agent.Response{},
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractExploredFiles(tt.response)

			if len(got) != len(tt.want) {
				t.Errorf("extractExploredFiles() = %v, want %v", got, tt.want)
				return
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("extractExploredFiles()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// Test resolveAgentForTask - agent resolution with 7-level priority
func TestResolveAgentForTask(t *testing.T) {
	tests := []struct {
		name             string
		optsAgentName    string
		taskAgentConfig  *provider.AgentConfig
		workspaceDefault string
		registerAgent    string
		wantAgentName    string
		wantSource       string
		wantError        bool
	}{
		{
			name:          "priority 1: CLI flag",
			optsAgentName: "cli-agent",
			registerAgent: "cli-agent",
			wantAgentName: "cli-agent",
			wantSource:    "cli",
		},
		{
			name:          "priority 2: task frontmatter agent",
			optsAgentName: "",
			taskAgentConfig: &provider.AgentConfig{
				Name: "task-agent",
			},
			registerAgent: "task-agent",
			wantAgentName: "task-agent",
			wantSource:    "task",
		},
		{
			name:             "priority 3: workspace default",
			optsAgentName:    "",
			taskAgentConfig:  nil,
			workspaceDefault: "workspace-agent",
			registerAgent:    "workspace-agent",
			wantAgentName:    "workspace-agent",
			wantSource:       "workspace",
		},
		{
			name:             "priority 4: auto-detect",
			optsAgentName:    "",
			taskAgentConfig:  nil,
			workspaceDefault: "",
			registerAgent:    "auto-agent",
			wantAgentName:    "auto-agent",
			wantSource:       "auto",
		},
		{
			name:          "agent not found",
			optsAgentName: "nonexistent",
			registerAgent: "other-agent",
			wantError:     true,
		},
		{
			name:          "task config with env vars",
			optsAgentName: "",
			taskAgentConfig: &provider.AgentConfig{
				Name: "task-agent",
				Env: map[string]string{
					"TEST_VAR": "test-value",
				},
				Args: []string{"--test-arg"},
			},
			registerAgent: "task-agent",
			wantAgentName: "task-agent",
			wantSource:    "task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create workspace with config
			ws, err := storage.OpenWorkspace(tmpDir, nil)
			if err != nil {
				t.Fatalf("OpenWorkspace: %v", err)
			}
			if err := ws.EnsureInitialized(); err != nil {
				t.Fatalf("EnsureInitialized: %v", err)
			}

			// Save workspace config with default agent
			cfg, _ := ws.LoadConfig()
			cfg.Agent.Default = tt.workspaceDefault
			if err := ws.SaveConfig(cfg); err != nil {
				t.Fatalf("SaveConfig: %v", err)
			}

			// Create conductor
			c, err := New(WithWorkDir(tmpDir), WithAgent(tt.optsAgentName))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			// Set up workspace and agents
			c.workspace = ws
			c.taskAgentConfig = tt.taskAgentConfig

			// Register test agent
			if tt.registerAgent != "" && tt.registerAgent != "auto-agent" {
				mockAgent := &testAgent{name: tt.registerAgent}
				if err := c.agents.Register(mockAgent); err != nil {
					t.Fatalf("Register agent: %v", err)
				}
			}
			if tt.registerAgent == "auto-agent" {
				mockAgent := &testAgent{name: "auto-agent"}
				if err := c.agents.Register(mockAgent); err != nil {
					t.Fatalf("Register agent: %v", err)
				}
			}

			// Call resolveAgentForTask
			gotAgent, gotSource, gotErr := c.resolveAgentForTask()

			if tt.wantError {
				if gotErr == nil {
					t.Error("resolveAgentForTask() expected error, got nil")
				}
				return
			}

			if gotErr != nil {
				t.Fatalf("resolveAgentForTask() unexpected error: %v", gotErr)
			}

			if gotAgent.Name() != tt.wantAgentName {
				t.Errorf("agent name = %q, want %q", gotAgent.Name(), tt.wantAgentName)
			}
			if gotSource != tt.wantSource {
				t.Errorf("source = %q, want %q", gotSource, tt.wantSource)
			}
		})
	}
}

// Test resolveAgentForStep - per-step agent resolution with 7-level priority
func TestResolveAgentForStep(t *testing.T) {
	tests := []struct {
		name            string
		optsAgentName   string
		optsStepAgents  map[string]string
		taskAgentConfig *provider.AgentConfig
		workspaceConfig *storage.WorkspaceConfig
		step            workflow.Step
		wantAgentName   string
		wantSource      string
		wantError       bool
	}{
		{
			name:          "priority 1: CLI step-specific",
			optsAgentName: "global-cli",
			optsStepAgents: map[string]string{
				"planning": "step-cli-agent",
			},
			taskAgentConfig: nil,
			workspaceConfig: nil,
			step:            workflow.StepPlanning,
			wantAgentName:   "step-cli-agent",
			wantSource:      "cli-step",
		},
		{
			name:            "priority 2: CLI global",
			optsAgentName:   "global-cli",
			optsStepAgents:  nil,
			taskAgentConfig: nil,
			workspaceConfig: nil,
			step:            workflow.StepPlanning,
			wantAgentName:   "global-cli",
			wantSource:      "cli",
		},
		{
			name:           "priority 3: task step-specific",
			optsAgentName:  "",
			optsStepAgents: nil,
			taskAgentConfig: &provider.AgentConfig{
				Steps: map[string]provider.StepAgentConfig{
					"planning": {
						Name: "task-step-agent",
						Env:  map[string]string{"STEP_VAR": "step-value"},
						Args: []string{"--step-arg"},
					},
				},
			},
			workspaceConfig: nil,
			step:            workflow.StepPlanning,
			wantAgentName:   "task-step-agent",
			wantSource:      "task-step",
		},
		{
			name:           "priority 4: task default",
			optsAgentName:  "",
			optsStepAgents: nil,
			taskAgentConfig: &provider.AgentConfig{
				Name: "task-default-agent",
				Env:  map[string]string{"TASK_VAR": "task-value"},
				Args: []string{"--task-arg"},
			},
			workspaceConfig: nil,
			step:            workflow.StepPlanning,
			wantAgentName:   "task-default-agent",
			wantSource:      "task",
		},
		{
			name:            "priority 5: workspace step-specific",
			optsAgentName:   "",
			optsStepAgents:  nil,
			taskAgentConfig: nil,
			workspaceConfig: &storage.WorkspaceConfig{
				Agent: storage.AgentSettings{
					Steps: map[string]storage.StepAgentConfig{
						"planning": {
							Name: "workspace-step-agent",
							Env:  map[string]string{"WS_STEP_VAR": "ws-step-value"},
							Args: []string{"--ws-step-arg"},
						},
					},
				},
			},
			step:          workflow.StepPlanning,
			wantAgentName: "workspace-step-agent",
			wantSource:    "workspace-step",
		},
		{
			name:            "priority 6: workspace default",
			optsAgentName:   "",
			optsStepAgents:  nil,
			taskAgentConfig: nil,
			workspaceConfig: &storage.WorkspaceConfig{
				Agent: storage.AgentSettings{
					Default: "workspace-default-agent",
				},
			},
			step:          workflow.StepPlanning,
			wantAgentName: "workspace-default-agent",
			wantSource:    "workspace",
		},
		{
			name:            "agent not found",
			optsAgentName:   "nonexistent",
			optsStepAgents:  nil,
			taskAgentConfig: nil,
			workspaceConfig: nil,
			step:            workflow.StepPlanning,
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create workspace
			ws, err := storage.OpenWorkspace(tmpDir, nil)
			if err != nil {
				t.Fatalf("OpenWorkspace: %v", err)
			}
			if err := ws.EnsureInitialized(); err != nil {
				t.Fatalf("EnsureInitialized: %v", err)
			}

			// Save workspace config if provided
			if tt.workspaceConfig != nil {
				if err := ws.SaveConfig(tt.workspaceConfig); err != nil {
					t.Fatalf("SaveConfig: %v", err)
				}
			}

			// Create conductor with step agents
			opts := []Option{WithWorkDir(tmpDir)}
			if tt.optsAgentName != "" {
				opts = append(opts, WithAgent(tt.optsAgentName))
			}
			for step, agent := range tt.optsStepAgents {
				opts = append(opts, WithStepAgent(step, agent))
			}
			c, err := New(opts...)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			c.workspace = ws
			c.taskAgentConfig = tt.taskAgentConfig

			// Register agents based on test case
			agentsToRegister := []string{
				"step-cli-agent", "global-cli", "task-step-agent",
				"task-default-agent", "workspace-step-agent",
				"workspace-default-agent",
			}
			for _, name := range agentsToRegister {
				mockAgent := &testAgent{name: name}
				if err := c.agents.Register(mockAgent); err != nil {
					t.Fatalf("Register agent %s: %v", name, err)
				}
			}

			// Call resolveAgentForStep
			gotResolution, gotErr := c.resolveAgentForStep(tt.step)

			if tt.wantError {
				if gotErr == nil {
					t.Error("resolveAgentForStep() expected error, got nil")
				}
				return
			}

			if gotErr != nil {
				t.Fatalf("resolveAgentForStep() unexpected error: %v", gotErr)
			}

			if gotResolution.Agent.Name() != tt.wantAgentName {
				t.Errorf("agent name = %q, want %q", gotResolution.Agent.Name(), tt.wantAgentName)
			}
			if gotResolution.Source != tt.wantSource {
				t.Errorf("source = %q, want %q", gotResolution.Source, tt.wantSource)
			}
		})
	}
}

// Test registerAliasAgents - simple alias, chained aliases, circular dependency
func TestRegisterAliasAgents(t *testing.T) {
	tests := []struct {
		name         string
		registerBase []string // Base agents to register first
		aliases      map[string]storage.AgentAliasConfig
		wantError    bool
		errorContain string
		verifyAgents []string // Agents that should be registered
	}{
		{
			name:         "no aliases",
			registerBase: []string{"base"},
			aliases:      map[string]storage.AgentAliasConfig{},
			wantError:    false,
			verifyAgents: []string{"base"},
		},
		{
			name:         "simple alias",
			registerBase: []string{"base"},
			aliases: map[string]storage.AgentAliasConfig{
				"alias1": {
					Extends:     "base",
					Description: "Simple alias",
				},
			},
			wantError:    false,
			verifyAgents: []string{"base", "alias1"},
		},
		{
			name:         "chained aliases",
			registerBase: []string{"base"},
			aliases: map[string]storage.AgentAliasConfig{
				"alias1": {
					Extends: "base",
				},
				"alias2": {
					Extends: "alias1",
				},
				"alias3": {
					Extends: "alias2",
				},
			},
			wantError:    false,
			verifyAgents: []string{"base", "alias1", "alias2", "alias3"},
		},
		{
			name:         "circular dependency",
			registerBase: []string{"base"},
			aliases: map[string]storage.AgentAliasConfig{
				"alias1": {
					Extends: "alias2",
				},
				"alias2": {
					Extends: "alias1",
				},
			},
			wantError:    true,
			errorContain: "circular",
		},
		{
			name:         "unknown base agent",
			registerBase: []string{},
			aliases: map[string]storage.AgentAliasConfig{
				"alias1": {
					Extends: "nonexistent",
				},
			},
			wantError:    true,
			errorContain: "unknown",
		},
		{
			name:         "alias with env and args",
			registerBase: []string{"base"},
			aliases: map[string]storage.AgentAliasConfig{
				"custom": {
					Extends: "base",
					Env: map[string]string{
						"CUSTOM_VAR": "custom-value",
					},
					Args: []string{"--custom-arg"},
				},
			},
			wantError:    false,
			verifyAgents: []string{"base", "custom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New()
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			// Register base agents
			for _, name := range tt.registerBase {
				mockAgent := &testAgent{name: name}
				if err := c.agents.Register(mockAgent); err != nil {
					t.Fatalf("Register base agent %s: %v", name, err)
				}
			}

			// Create workspace config with aliases
			cfg := &storage.WorkspaceConfig{
				Agents: tt.aliases,
			}

			// Call registerAliasAgents
			gotErr := c.registerAliasAgents(cfg)

			if tt.wantError {
				if gotErr == nil {
					t.Error("registerAliasAgents() expected error, got nil")
					return
				}
				if tt.errorContain != "" && !strings.Contains(gotErr.Error(), tt.errorContain) {
					t.Errorf("error = %q, want contain %q", gotErr.Error(), tt.errorContain)
				}
				return
			}

			if gotErr != nil {
				t.Fatalf("registerAliasAgents() unexpected error: %v", gotErr)
			}

			// Verify agents are registered
			for _, name := range tt.verifyAgents {
				agent, err := c.agents.Get(name)
				if err != nil {
					t.Errorf("agent %q not registered: %v", name, err)
				}
				if agent != nil && agent.Name() != name {
					t.Errorf("agent name = %q, want %q", agent.Name(), name)
				}
			}
		})
	}
}

// Test GetAgentForStep - cache hit/miss, persistence
func TestGetAgentForStep(t *testing.T) {
	tests := []struct {
		existingStep   *storage.StepAgentInfo
		name           string
		wantAgentName  string
		registerAgents []string
		wantCached     bool
	}{
		{
			name: "cache hit - returns persisted agent",
			existingStep: &storage.StepAgentInfo{
				Name: "cached-agent",
				InlineEnv: map[string]string{
					"CACHED_VAR": "cached-value",
				},
				Args: []string{"--cached-arg"},
			},
			registerAgents: []string{"cached-agent"},
			wantAgentName:  "cached-agent",
			wantCached:     true,
		},
		{
			name:           "cache miss - resolves fresh",
			existingStep:   nil,
			registerAgents: []string{"fresh-agent"},
			wantAgentName:  "fresh-agent",
			wantCached:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create workspace
			ws, err := storage.OpenWorkspace(tmpDir, nil)
			if err != nil {
				t.Fatalf("OpenWorkspace: %v", err)
			}
			if err := ws.EnsureInitialized(); err != nil {
				t.Fatalf("EnsureInitialized: %v", err)
			}

			// Create task work with step info
			work, err := ws.CreateWork("test-task", storage.SourceInfo{
				Type: "file",
				Ref:  "task.md",
			})
			if err != nil {
				t.Fatalf("CreateWork: %v", err)
			}

			if tt.existingStep != nil {
				work.Agent.Steps = map[string]storage.StepAgentInfo{
					"planning": *tt.existingStep,
				}
				if err := ws.SaveWork(work); err != nil {
					t.Fatalf("SaveWork: %v", err)
				}
			}

			// Create conductor
			c, err := New(WithWorkDir(tmpDir), WithAgent("fresh-agent"))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			c.workspace = ws
			c.taskWork = work

			// Register agents
			for _, name := range tt.registerAgents {
				mockAgent := &testAgent{name: name}
				if err := c.agents.Register(mockAgent); err != nil {
					t.Fatalf("Register agent %s: %v", name, err)
				}
			}

			// Call GetAgentForStep
			gotAgent, gotErr := c.GetAgentForStep(workflow.StepPlanning)

			if gotErr != nil {
				t.Fatalf("GetAgentForStep() unexpected error: %v", gotErr)
			}

			if gotAgent.Name() != tt.wantAgentName {
				t.Errorf("agent name = %q, want %q", gotAgent.Name(), tt.wantAgentName)
			}
		})
	}
}

// Test resolveNaming - external key resolution, template expansion
func TestResolveNaming(t *testing.T) {
	tests := []struct {
		name              string
		workUnit          *provider.WorkUnit
		taskID            string
		optsExternalKey   string
		workspaceConfig   *storage.WorkspaceConfig
		wantBranchContain string
		wantCommitPrefix  string
	}{
		{
			name: "CLI external key override",
			workUnit: &provider.WorkUnit{
				Title:       "Test Feature",
				ExternalKey: "WORKUNIT-123",
				TaskType:    "feature",
				Slug:        "test-feature",
			},
			taskID:          "task-abc",
			optsExternalKey: "CLI-456",
			workspaceConfig: &storage.WorkspaceConfig{
				Git: storage.GitSettings{
					BranchPattern: "feature/{key}--{slug}",
					CommitPrefix:  "[{key}]",
				},
			},
			wantBranchContain: "CLI-456",
			wantCommitPrefix:  "[CLI-456]",
		},
		{
			name: "workUnit external key",
			workUnit: &provider.WorkUnit{
				Title:       "Test Feature",
				ExternalKey: "WORKUNIT-123",
				TaskType:    "feature",
				Slug:        "test-feature",
			},
			taskID: "task-abc",
			workspaceConfig: &storage.WorkspaceConfig{
				Git: storage.GitSettings{
					BranchPattern: "feature/{key}--{slug}",
					CommitPrefix:  "[{key}]",
				},
			},
			wantBranchContain: "WORKUNIT-123",
			wantCommitPrefix:  "[WORKUNIT-123]",
		},
		{
			name: "fallback to taskID",
			workUnit: &provider.WorkUnit{
				Title:    "Test Feature",
				TaskType: "feature",
				Slug:     "test-feature",
			},
			taskID: "task-abc",
			workspaceConfig: &storage.WorkspaceConfig{
				Git: storage.GitSettings{
					BranchPattern: "feature/{key}--{slug}",
					CommitPrefix:  "[{key}]",
				},
			},
			wantBranchContain: "task-abc",
			wantCommitPrefix:  "[task-abc]",
		},
		{
			name: "slug is generated from title",
			workUnit: &provider.WorkUnit{
				Title:       "A Very Long Feature Title That Needs Slugification",
				ExternalKey: "KEY-123",
				TaskType:    "feature",
			},
			taskID: "task-abc",
			workspaceConfig: &storage.WorkspaceConfig{
				Git: storage.GitSettings{
					BranchPattern: "feature/{key}--{slug}",
					CommitPrefix:  "[{key}]",
				},
			},
			wantBranchContain: "KEY-123",
			wantCommitPrefix:  "[KEY-123]",
		},
		{
			name: "task type defaults to 'task'",
			workUnit: &provider.WorkUnit{
				Title:       "Test Feature",
				ExternalKey: "KEY-123",
			},
			taskID: "task-abc",
			workspaceConfig: &storage.WorkspaceConfig{
				Git: storage.GitSettings{
					BranchPattern: "{type}/{key}",
					CommitPrefix:  "[{key}]",
				},
			},
			wantBranchContain: "task/KEY-123",
			wantCommitPrefix:  "[KEY-123]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create workspace
			ws, err := storage.OpenWorkspace(tmpDir, nil)
			if err != nil {
				t.Fatalf("OpenWorkspace: %v", err)
			}
			if err := ws.EnsureInitialized(); err != nil {
				t.Fatalf("EnsureInitialized: %v", err)
			}

			// Save workspace config
			if tt.workspaceConfig != nil {
				if err := ws.SaveConfig(tt.workspaceConfig); err != nil {
					t.Fatalf("SaveConfig: %v", err)
				}
			}

			// Create conductor
			c, err := New(WithWorkDir(tmpDir), WithExternalKey(tt.optsExternalKey))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			c.workspace = ws

			// Call resolveNaming
			gotInfo := c.resolveNaming(tt.workUnit, tt.taskID)

			if gotInfo == nil {
				t.Fatal("resolveNaming() returned nil")
			}

			if !strings.Contains(gotInfo.branchName, tt.wantBranchContain) {
				t.Errorf("branch name = %q, want contain %q", gotInfo.branchName, tt.wantBranchContain)
			}

			if gotInfo.commitPrefix != tt.wantCommitPrefix {
				t.Errorf("commit prefix = %q, want %q", gotInfo.commitPrefix, tt.wantCommitPrefix)
			}
		})
	}
}

// Test buildWorkUnit - WorkUnit construction with specifications
func TestBuildWorkUnit_WithSpecs(t *testing.T) {
	tests := []struct {
		taskWork       *storage.TaskWork
		name           string
		wantID         string
		wantTitle      string
		specifications []int
		wantSpecCount  int
	}{
		{
			name:     "nil taskWork",
			taskWork: nil,
			wantID:   "",
		},
		{
			name: "with specifications",
			taskWork: &storage.TaskWork{
				Version: "1",
				Metadata: storage.WorkMetadata{
					ID:          "task-123",
					Title:       "Test Task",
					ExternalKey: "KEY-123",
				},
				Source: storage.SourceInfo{
					Ref:     "task.md",
					Content: "task content",
				},
			},
			specifications: []int{1, 2, 3},
			wantSpecCount:  3,
			wantID:         "task-123",
			wantTitle:      "Test Task",
		},
		{
			name: "without specifications",
			taskWork: &storage.TaskWork{
				Version: "1",
				Metadata: storage.WorkMetadata{
					ID:          "task-456",
					Title:       "Another Task",
					ExternalKey: "KEY-456",
				},
				Source: storage.SourceInfo{
					Ref:     "another.md",
					Content: "another content",
				},
			},
			specifications: []int{},
			wantSpecCount:  0,
			wantID:         "task-456",
			wantTitle:      "Another Task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create workspace
			ws, err := storage.OpenWorkspace(tmpDir, nil)
			if err != nil {
				t.Fatalf("OpenWorkspace: %v", err)
			}
			if err := ws.EnsureInitialized(); err != nil {
				t.Fatalf("EnsureInitialized: %v", err)
			}

			// Create task work and specifications
			if tt.taskWork != nil {
				work, err := ws.CreateWork(tt.taskWork.Metadata.ID, tt.taskWork.Source)
				if err != nil {
					t.Fatalf("CreateWork: %v", err)
				}
				work.Metadata = tt.taskWork.Metadata

				// Save specifications
				for _, num := range tt.specifications {
					if err := ws.SaveSpecification(tt.taskWork.Metadata.ID, num, fmt.Sprintf("# Specification %d", num)); err != nil {
						t.Fatalf("SaveSpecification: %v", err)
					}
				}

				if err := ws.SaveWork(work); err != nil {
					t.Fatalf("SaveWork: %v", err)
				}
				tt.taskWork = work
			}

			// Create conductor
			c, err := New(WithWorkDir(tmpDir))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			c.workspace = ws
			c.taskWork = tt.taskWork

			// Call buildWorkUnit
			gotWorkUnit := c.buildWorkUnit()

			if tt.wantID == "" {
				if gotWorkUnit != nil {
					t.Error("buildWorkUnit() should return nil when taskWork is nil")
				}
				return
			}

			if gotWorkUnit == nil {
				t.Fatal("buildWorkUnit() returned nil, expected non-nil")
			}

			if gotWorkUnit.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", gotWorkUnit.ID, tt.wantID)
			}

			if gotWorkUnit.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", gotWorkUnit.Title, tt.wantTitle)
			}

			if len(gotWorkUnit.Specifications) != tt.wantSpecCount {
				t.Errorf("specifications count = %d, want %d", len(gotWorkUnit.Specifications), tt.wantSpecCount)
			}
		})
	}
}

// testAgent is a minimal mock agent for testing
type testAgent struct {
	name string
}

func (a *testAgent) Name() string {
	return a.name
}

func (a *testAgent) Run(ctx context.Context, prompt string) (*agent.Response, error) {
	return &agent.Response{}, nil
}

func (a *testAgent) RunStream(ctx context.Context, prompt string) (<-chan agent.Event, <-chan error) {
	return nil, nil
}

func (a *testAgent) RunWithCallback(ctx context.Context, prompt string, cb agent.StreamCallback) (*agent.Response, error) {
	return &agent.Response{}, nil
}

func (a *testAgent) Available() error {
	return nil
}

func (a *testAgent) WithEnv(key, value string) agent.Agent {
	return a
}

func (a *testAgent) WithArgs(args ...string) agent.Agent {
	return a
}

// Tests for DeleteWork tri-state behavior

func TestBoolPtr(t *testing.T) {
	truePtr := BoolPtr(true)
	falsePtr := BoolPtr(false)

	if truePtr == nil || *truePtr != true {
		t.Error("BoolPtr(true) should return pointer to true")
	}
	if falsePtr == nil || *falsePtr != false {
		t.Error("BoolPtr(false) should return pointer to false")
	}
}

func TestFinishOptions_DeleteWork_TriState(t *testing.T) {
	tests := []struct {
		name       string
		deleteWork *bool
		wantNil    bool
		wantValue  bool
	}{
		{
			name:       "nil defers to config",
			deleteWork: nil,
			wantNil:    true,
		},
		{
			name:       "explicit true means delete",
			deleteWork: BoolPtr(true),
			wantNil:    false,
			wantValue:  true,
		},
		{
			name:       "explicit false means keep",
			deleteWork: BoolPtr(false),
			wantNil:    false,
			wantValue:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := FinishOptions{
				DeleteWork: tt.deleteWork,
			}

			if tt.wantNil {
				if opts.DeleteWork != nil {
					t.Errorf("DeleteWork = %v, want nil", opts.DeleteWork)
				}
			} else {
				if opts.DeleteWork == nil {
					t.Fatal("DeleteWork is nil, want non-nil")
				}
				if *opts.DeleteWork != tt.wantValue {
					t.Errorf("*DeleteWork = %v, want %v", *opts.DeleteWork, tt.wantValue)
				}
			}
		})
	}
}

func TestDeleteOptions_DeleteWork_TriState(t *testing.T) {
	tests := []struct {
		name       string
		deleteWork *bool
		wantNil    bool
		wantValue  bool
	}{
		{
			name:       "nil defers to config",
			deleteWork: nil,
			wantNil:    true,
		},
		{
			name:       "explicit true means delete",
			deleteWork: BoolPtr(true),
			wantNil:    false,
			wantValue:  true,
		},
		{
			name:       "explicit false means keep",
			deleteWork: BoolPtr(false),
			wantNil:    false,
			wantValue:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DeleteOptions{
				DeleteWork: tt.deleteWork,
			}

			if tt.wantNil {
				if opts.DeleteWork != nil {
					t.Errorf("DeleteWork = %v, want nil", opts.DeleteWork)
				}
			} else {
				if opts.DeleteWork == nil {
					t.Fatal("DeleteWork is nil, want non-nil")
				}
				if *opts.DeleteWork != tt.wantValue {
					t.Errorf("*DeleteWork = %v, want %v", *opts.DeleteWork, tt.wantValue)
				}
			}
		})
	}
}

// Test precedence logic: CLI flag > config > default
func TestDeleteWorkPrecedence_Documentation(t *testing.T) {
	// This test documents the expected precedence behavior.
	// The actual precedence logic is in Finish() and Delete() methods,
	// but we can verify the expected mapping here:

	// For Finish (default behavior: keep work)
	t.Run("Finish default keeps work", func(t *testing.T) {
		opts := DefaultFinishOptions()
		if opts.DeleteWork != nil {
			t.Error("Default FinishOptions.DeleteWork should be nil (defer to config)")
		}
		// When DeleteWork is nil and config.DeleteWorkOnFinish is false (default),
		// work directory should be kept.
	})

	// For Delete (default behavior: delete work)
	t.Run("Delete default deletes work", func(t *testing.T) {
		opts := DefaultDeleteOptions()
		if opts.DeleteWork != nil {
			t.Error("Default DeleteOptions.DeleteWork should be nil (defer to config)")
		}
		// When DeleteWork is nil and config.DeleteWorkOnAbandon is true (default),
		// work directory should be deleted.
	})

	// CLI --delete-work flag should force deletion
	t.Run("CLI delete-work flag forces deletion", func(t *testing.T) {
		deleteWork := BoolPtr(true)
		opts := FinishOptions{DeleteWork: deleteWork}
		if opts.DeleteWork == nil || *opts.DeleteWork != true {
			t.Error("When CLI flag is set, DeleteWork should be true")
		}
	})

	// CLI --keep-work flag should prevent deletion
	t.Run("CLI keep-work flag prevents deletion", func(t *testing.T) {
		deleteWork := BoolPtr(false) // --keep-work means don't delete
		opts := DeleteOptions{DeleteWork: deleteWork}
		if opts.DeleteWork == nil || *opts.DeleteWork != false {
			t.Error("When CLI flag is set, DeleteWork should be false")
		}
	})
}

// Test WithTitleOverride option
func TestWithTitleOverride(t *testing.T) {
	opts := DefaultOptions()
	WithTitleOverride("Custom Task Title")(&opts)

	if opts.TitleOverride != "Custom Task Title" {
		t.Errorf("TitleOverride = %q, want %q", opts.TitleOverride, "Custom Task Title")
	}
}

// Test WithSlugOverride option
func TestWithSlugOverride(t *testing.T) {
	opts := DefaultOptions()
	WithSlugOverride("custom-task-slug")(&opts)

	if opts.SlugOverride != "custom-task-slug" {
		t.Errorf("SlugOverride = %q, want %q", opts.SlugOverride, "custom-task-slug")
	}
}
