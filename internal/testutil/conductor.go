// Package testutil provides shared testing utilities for go-mehrhof tests.
package testutil

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/storage"
)

// NewTestConductor creates a conductor for testing with minimal setup.
// It creates a temporary directory, initializes a workspace, and provides
// a mock agent for testing.
func NewTestConductor(t *testing.T, opts ...conductor.Option) *conductor.Conductor {
	t.Helper()

	tmpDir := t.TempDir()

	// Set default options for testing
	defaultOpts := []conductor.Option{
		conductor.WithWorkDir(tmpDir),
		conductor.WithAutoInit(true),
		conductor.WithDryRun(true), // Don't make actual changes during tests
		conductor.WithStdout(io.Discard),
		conductor.WithStderr(io.Discard),
		conductor.WithStateChangeCallback(func(from, to string) {}),
		conductor.WithProgressCallback(func(msg string, pct int) {}),
		conductor.WithErrorCallback(func(err error) {}),
	}

	// Apply user options after defaults
	defaultOpts = append(defaultOpts, opts...)

	c, err := conductor.New(defaultOpts...)
	if err != nil {
		t.Fatalf("New conductor: %v", err)
	}

	// Initialize (ignore agent detection errors for testing)
	ctx := context.Background()
	_ = c.Initialize(ctx)

	return c
}

// SetupTestTask creates and registers a test task in the conductor.
// It creates a task file, sets up the workspace, and returns the active task.
func SetupTestTask(t *testing.T, c *conductor.Conductor, title string) *storage.ActiveTask {
	t.Helper()

	tmpDir := t.TempDir()

	// Create a simple task file
	taskContent := `---` + "\n" + `title: ` + title + "\n" + `---` + "\n\n" + `Test task description`
	taskPath := filepath.Join(tmpDir, "task.md")
	if err := os.WriteFile(taskPath, []byte(taskContent), 0o644); err != nil {
		t.Fatalf("Write task file: %v", err)
	}

	// Get workspace
	ws := c.GetWorkspace()
	if ws == nil {
		// Create workspace if not initialized
		var err error
		ws, err = storage.OpenWorkspace(tmpDir, nil)
		if err != nil {
			t.Fatalf("Open workspace: %v", err)
		}
		if err := ws.EnsureInitialized(); err != nil {
			t.Fatalf("Ensure initialized: %v", err)
		}
	}

	// Create task work
	taskID := "test-task-" + title
	work, err := ws.CreateWork(taskID, storage.SourceInfo{
		Type:    "file",
		Ref:     "task.md",
		Content: taskContent,
	})
	if err != nil {
		t.Fatalf("Create work: %v", err)
	}

	work.Metadata.Title = title
	if err := ws.SaveWork(work); err != nil {
		t.Fatalf("Save work: %v", err)
	}

	// Create active task
	activeTask := storage.NewActiveTask(taskID, "file:task.md", ws.WorkPath(taskID))
	activeTask.Started = time.Now()

	if err := ws.SaveActiveTask(activeTask); err != nil {
		t.Fatalf("Save active task: %v", err)
	}

	return activeTask
}

// AssertState asserts that the conductor's state machine is in the expected state.
func AssertState(t *testing.T, c *conductor.Conductor, expected string) {
	t.Helper()

	machine := c.GetMachine()
	if machine == nil {
		t.Error("conductor has no state machine")

		return
	}

	// Check if current state matches expected
	// workflow.State is a string type, so we can directly compare
	currentState := string(machine.State())
	if currentState != expected {
		t.Errorf("state = %q, want %q", currentState, expected)
	}
}

// WithMockAgent configures a conductor to use a mock agent for testing.
func WithMockAgent(c *conductor.Conductor, mockAgent agent.Agent) {
	// This is a helper that can be used to inject a mock agent
	// into the conductor's agent registry
	registry := c.GetAgentRegistry()
	if registry != nil {
		_ = registry.Register(mockAgent)
	}
}

// TestConductorOptions returns common conductor options for testing.
func TestConductorOptions(tmpDir string) []conductor.Option {
	return []conductor.Option{
		conductor.WithWorkDir(tmpDir),
		conductor.WithAutoInit(true),
		conductor.WithDryRun(true),
		conductor.WithStdout(io.Discard),
		conductor.WithStderr(io.Discard),
		conductor.WithStateChangeCallback(func(from, to string) {}),
		conductor.WithProgressCallback(func(msg string, pct int) {}),
		conductor.WithErrorCallback(func(err error) {}),
	}
}
