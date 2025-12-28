package conductor

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/events"
	"github.com/valksor/go-mehrhof/internal/storage"
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

func TestTalkOptionsStruct(t *testing.T) {
	opts := TalkOptions{
		Continue:    true,
		SessionFile: "session.yaml",
	}

	if opts.Continue != true {
		t.Errorf("Continue = %v, want true", opts.Continue)
	}
	if opts.SessionFile != "session.yaml" {
		t.Errorf("SessionFile = %q, want %q", opts.SessionFile, "session.yaml")
	}
}

func TestDefaultFinishOptions(t *testing.T) {
	opts := DefaultFinishOptions()

	if opts.SquashMerge != true {
		t.Errorf("SquashMerge = %v, want true", opts.SquashMerge)
	}
	if opts.DeleteBranch != true {
		t.Errorf("DeleteBranch = %v, want true", opts.DeleteBranch)
	}
	if opts.TargetBranch != "" {
		t.Errorf("TargetBranch = %q, want empty (auto-detect)", opts.TargetBranch)
	}
	if opts.PushAfter != false {
		t.Errorf("PushAfter = %v, want false", opts.PushAfter)
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
	ws, err := storage.OpenWorkspace(tmpDir)
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
	opts := DeleteOptions{
		Force:       true,
		KeepBranch:  true,
		KeepWorkDir: true,
	}

	if opts.Force != true {
		t.Errorf("Force = %v, want true", opts.Force)
	}
	if opts.KeepBranch != true {
		t.Errorf("KeepBranch = %v, want true", opts.KeepBranch)
	}
	if opts.KeepWorkDir != true {
		t.Errorf("KeepWorkDir = %v, want true", opts.KeepWorkDir)
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
	if opts.KeepWorkDir != false {
		t.Errorf("KeepWorkDir = %v, want false", opts.KeepWorkDir)
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

// Test buildTalkPrompt
func TestBuildTalkPrompt(t *testing.T) {
	tests := []struct {
		name          string
		title         string
		sourceContent string
		notes         string
		specs         string
		pendingQ      *storage.PendingQuestion
		message       string
		wantContain   []string
	}{
		{
			name:          "basic prompt",
			title:         "Test task",
			sourceContent: "Source",
			notes:         "",
			specs:         "",
			pendingQ:      nil,
			message:       "Hello",
			wantContain: []string{
				"## Task: Test task",
				"## User's Message",
				"Hello",
				"## Instructions",
			},
		},
		{
			name:          "with all context",
			title:         "Task",
			sourceContent: "Requirements",
			notes:         "Previous notes",
			specs:         "# Spec 1",
			pendingQ:      nil,
			message:       "Question",
			wantContain: []string{
				"## Source Content",
				"Requirements",
				"## Current Specifications",
				"# Spec 1",
				"## Previous Notes",
				"Previous notes",
			},
		},
		{
			name:          "with pending question",
			title:         "Task",
			sourceContent: "",
			notes:         "",
			specs:         "",
			pendingQ: &storage.PendingQuestion{
				Question:       "What approach?",
				ContextSummary: "Analysis context",
			},
			message: "Use option A",
			wantContain: []string{
				"## Your Previous Question",
				"What approach?",
				"## Context Before Question",
				"Analysis context",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTalkPrompt(tt.title, tt.sourceContent, tt.notes, tt.specs, tt.pendingQ, tt.message)

			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("buildTalkPrompt() missing %q", want)
				}
			}
		})
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
