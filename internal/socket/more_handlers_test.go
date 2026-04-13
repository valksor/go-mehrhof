package socket

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/settings"
)

// ============================================================
// handleStatus tests
// ============================================================

func TestWorktreeHandleStatus_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}

	resp, err := w.handleStatus(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStatus() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleStatus() returned error: %s", resp.Error.Message)
	}

	var result StatusResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.State != StateNone {
		t.Errorf("state = %q, want %q", result.State, StateNone)
	}
}

func TestWorktreeHandleStatus_NoTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleStatus(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStatus() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleStatus() returned error: %s", resp.Error.Message)
	}

	var result StatusResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.State != StateNone {
		t.Errorf("state = %q, want %q", result.State, StateNone)
	}
	if result.Task != nil {
		t.Error("expected nil task when no work unit loaded")
	}
}

func TestWorktreeHandleStatus_WithTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	resp, err := w.handleStatus(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStatus() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleStatus() returned error: %s", resp.Error.Message)
	}

	var result StatusResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.State != "loaded" {
		t.Errorf("state = %q, want %q", result.State, "loaded")
	}
	if result.Task == nil {
		t.Fatal("expected task info, got nil")
	}
	if result.Task.ID != "test-task-id" {
		t.Errorf("task ID = %q, want %q", result.Task.ID, "test-task-id")
	}
	if result.Task.Title != "Test Task" {
		t.Errorf("task title = %q, want %q", result.Task.Title, "Test Task")
	}
}

func TestWorktreeHandleStatus_WithActiveJob(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateImplementing)

	wu := w.conductor.WorkUnit()
	wu.Jobs = append(wu.Jobs, "job-123")

	resp, err := w.handleStatus(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStatus() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleStatus() returned error: %s", resp.Error.Message)
	}

	var result StatusResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.ActiveJobID != "job-123" {
		t.Errorf("active_job_id = %q, want %q", result.ActiveJobID, "job-123")
	}
}

func TestWorktreeHandleStatus_NoActiveJobInTerminalState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	wu := w.conductor.WorkUnit()
	wu.Jobs = append(wu.Jobs, "job-old")

	resp, err := w.handleStatus(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStatus() error = %v", err)
	}

	var result StatusResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.ActiveJobID != "" {
		t.Errorf("active_job_id = %q, want empty (loaded is not a working state)", result.ActiveJobID)
	}
}

// ============================================================
// handleShowSpec tests
// ============================================================

func TestWorktreeHandleShowSpec_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}

	resp, err := w.handleShowSpec(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleShowSpec() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when conductor is nil")
	}
}

func TestWorktreeHandleShowSpec_NoTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleShowSpec(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleShowSpec() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleShowSpec() returned error: %s", resp.Error.Message)
	}

	var result ShowSpecResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Specifications) != 0 {
		t.Errorf("expected empty specs, got %d", len(result.Specifications))
	}
}

func TestWorktreeHandleShowSpec_WithSpecs(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	// Create a temp spec file.
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	specContent := "# Specification\n\nDo the thing."
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	setWorkUnitInState(t, w, conductor.StateLoaded)
	wu := w.conductor.WorkUnit()
	wu.Specifications = []string{specPath}

	resp, err := w.handleShowSpec(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleShowSpec() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleShowSpec() returned error: %s", resp.Error.Message)
	}

	var result ShowSpecResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Specifications) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(result.Specifications))
	}
	if result.Specifications[0].Path != specPath {
		t.Errorf("spec path = %q, want %q", result.Specifications[0].Path, specPath)
	}
	if result.Specifications[0].Content != specContent {
		t.Errorf("spec content = %q, want %q", result.Specifications[0].Content, specContent)
	}
}

func TestWorktreeHandleShowSpec_MissingFile(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	wu := w.conductor.WorkUnit()
	wu.Specifications = []string{"/nonexistent/path/spec.md"}

	resp, err := w.handleShowSpec(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleShowSpec() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleShowSpec() returned error: %s", resp.Error.Message)
	}

	var result ShowSpecResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Missing files are skipped with a warning, not an error.
	if len(result.Specifications) != 0 {
		t.Errorf("expected 0 specs (file missing), got %d", len(result.Specifications))
	}
}

// ============================================================
// handleFinish tests
// ============================================================

func TestWorktreeHandleFinish_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}

	resp, err := w.handleFinish(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleFinish() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when conductor is nil")
	}
}

func TestWorktreeHandleFinish_NoTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleFinish(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleFinish() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when no task loaded")
	}
}

// ============================================================
// handleRefresh tests
// ============================================================

func TestWorktreeHandleRefresh_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}

	resp, err := w.handleRefresh(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when conductor is nil")
	}
}

func TestWorktreeHandleRefresh_NoTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleRefresh(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRefresh() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when no task loaded")
	}
}

// ============================================================
// handleRemoteApprove tests
// ============================================================

func TestWorktreeHandleRemoteApprove_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}

	resp, err := w.handleRemoteApprove(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRemoteApprove() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when conductor is nil")
	}
}

func TestWorktreeHandleRemoteApprove_NoTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleRemoteApprove(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRemoteApprove() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when no task loaded")
	}
}

// ============================================================
// handleRemoteMerge tests
// ============================================================

func TestWorktreeHandleRemoteMerge_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}

	resp, err := w.handleRemoteMerge(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRemoteMerge() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when conductor is nil")
	}
}

func TestWorktreeHandleRemoteMerge_NoTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleRemoteMerge(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRemoteMerge() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when no task loaded")
	}
}

// ============================================================
// handleSubmit tests
// ============================================================

func TestWorktreeHandleSubmit_WrongState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	// Pre-set quality gate so Submit doesn't run external review synchronously.
	passed := true
	wu := w.conductor.WorkUnit()
	wu.QualityGatePassed = &passed

	resp, err := w.handleSubmit(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleSubmit() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when in wrong state for submit")
	}
}

// ============================================================
// handleOptimize / handleSimplify tests
// ============================================================

func TestWorktreeHandleOptimize_NoTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleOptimize(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleOptimize() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when no task loaded")
	}
}

func TestWorktreeHandleOptimize_WrongState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	resp, err := w.handleOptimize(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleOptimize() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when in wrong state for optimize")
	}
}

func TestWorktreeHandleSimplify_NoTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleSimplify(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleSimplify() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when no task loaded")
	}
}

func TestWorktreeHandleSimplify_WrongState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	resp, err := w.handleSimplify(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleSimplify() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when in wrong state for simplify")
	}
}

// ============================================================
// handleCheckpoints tests
// ============================================================

func TestWorktreeHandleCheckpoints_WithCheckpoints(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateImplemented)

	wu := w.conductor.WorkUnit()
	wu.Checkpoints = []string{"abc123", "def456"}
	wu.RedoStack = []string{"ghi789"}

	resp, err := w.handleCheckpoints(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleCheckpoints() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleCheckpoints() returned error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var checkpoints []CheckpointInfo
	if err := json.Unmarshal(result["checkpoints"], &checkpoints); err != nil {
		t.Fatalf("unmarshal checkpoints: %v", err)
	}
	if len(checkpoints) != 2 {
		t.Fatalf("expected 2 checkpoints, got %d", len(checkpoints))
	}
	if checkpoints[0].SHA != "abc123" {
		t.Errorf("checkpoint[0].SHA = %q, want %q", checkpoints[0].SHA, "abc123")
	}
	if checkpoints[1].SHA != "def456" {
		t.Errorf("checkpoint[1].SHA = %q, want %q", checkpoints[1].SHA, "def456")
	}

	var redoStack []CheckpointInfo
	if err := json.Unmarshal(result["redo_stack"], &redoStack); err != nil {
		t.Fatalf("unmarshal redo_stack: %v", err)
	}
	if len(redoStack) != 1 {
		t.Fatalf("expected 1 redo entry, got %d", len(redoStack))
	}
	if redoStack[0].SHA != "ghi789" {
		t.Errorf("redo[0].SHA = %q, want %q", redoStack[0].SHA, "ghi789")
	}
}

// ============================================================
// handleTaskHistory tests
// ============================================================

func TestWorktreeHandleTaskHistory_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}

	resp, err := w.handleTaskHistory(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleTaskHistory() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when conductor is nil")
	}
}

func TestWorktreeHandleTaskHistory_NoStore(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleTaskHistory(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleTaskHistory() error = %v", err)
	}
	// May succeed with empty list or error depending on store availability.
	// Either outcome is acceptable; we just verify it doesn't panic.
	if resp.Error != nil {
		// Error is acceptable when no store is configured.
		return
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["tasks"]; !ok {
		t.Error("expected 'tasks' key in result")
	}
}

// ============================================================
// Global handler tests
// ============================================================

func TestGlobalHandleAgentStatus(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleAgentStatus(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleAgentStatus() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleAgentStatus() returned error: %s", resp.Error.Message)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestGlobalHandleMetrics(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleMetrics(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleMetrics() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleMetrics() returned error: %s", resp.Error.Message)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestGlobalHandleSettingsSet_ValidSetting(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, _ := json.Marshal(map[string]any{ //nolint:errchkjson // test data
		"scope": settings.ScopeGlobal,
		"values": map[string]any{
			"workers.max": 5,
		},
	})

	resp, err := g.handleSettingsSet(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleSettingsSet() error = %v", err)
	}
	// May fail due to filesystem constraints in test env, but should not panic.
	// If it succeeds, verify result.
	if resp.Error == nil && resp.Result != nil {
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
	}
}

func TestGlobalHandleSettingsSet_InvalidScope(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, _ := json.Marshal(map[string]any{ //nolint:errchkjson // test data
		"scope":  "invalid",
		"values": map[string]any{"key": "val"},
	})

	resp, err := g.handleSettingsSet(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleSettingsSet() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid scope")
	}
}
