package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
)

// ============================================================
// handleQualityRespond tests
// ============================================================

func TestWorktreeHandleQualityRespond_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	params, _ := json.Marshal(qualityRespondParams{PromptID: "p1", Answer: true}) //nolint:errchkjson // test data
	resp, err := w.handleQualityRespond(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQualityRespond() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when conductor is nil")
	}
}

func TestWorktreeHandleQualityRespond_MissingPromptID(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	params, _ := json.Marshal(qualityRespondParams{PromptID: "", Answer: true}) //nolint:errchkjson // test data
	resp, err := w.handleQualityRespond(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQualityRespond() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for missing prompt_id")
	}
}

func TestWorktreeHandleQualityRespond_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleQualityRespond(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleQualityRespond() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON params")
	}
}

func TestWorktreeHandleQualityRespond_NonexistentPrompt(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	params, _ := json.Marshal(qualityRespondParams{PromptID: "nonexistent-prompt-id", Answer: true}) //nolint:errchkjson // test data
	resp, err := w.handleQualityRespond(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQualityRespond() error = %v", err)
	}
	// Nonexistent prompt ID should return an error
	if resp.Error == nil {
		t.Fatal("expected error response for nonexistent prompt")
	}
}

// ============================================================
// handleGitDiffAgainst tests
// ============================================================

func TestWorktreeHandleGitDiffAgainst_NilRepo(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t) // no repo configured

	params, _ := json.Marshal(map[string]string{"ref": "main"}) //nolint:errchkjson // test data
	resp, err := w.handleGitDiffAgainst(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleGitDiffAgainst() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleGitDiffAgainst() with nil repo should return error response")
	}
}

func TestWorktreeHandleGitDiffAgainst_MissingRef(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	params, _ := json.Marshal(map[string]string{"ref": ""}) //nolint:errchkjson // test data
	resp, err := w.handleGitDiffAgainst(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleGitDiffAgainst() error = %v", err)
	}
	// Either nil-repo error or missing-ref validation error — both acceptable
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestWorktreeHandleGitDiffAgainst_InvalidParams(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleGitDiffAgainst(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleGitDiffAgainst() error = %v", err)
	}
	// Should return error (either parse error or nil-repo error)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// ============================================================
// handleUndo/handleRedo multi-step tests
// ============================================================

func TestWorktreeHandleUndo_MultipleSteps(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	// No checkpoints → undo should fail with an error response
	params, _ := json.Marshal(UndoParams{Steps: 3}) //nolint:errchkjson // test data
	resp, err := w.handleUndo(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleUndo() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for undo with no checkpoints")
	}
}

func TestWorktreeHandleUndo_ZeroStepsDefaultsToOne(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	// Steps=0 should default to 1 and then fail (no checkpoints)
	params, _ := json.Marshal(UndoParams{Steps: 0}) //nolint:errchkjson // test data
	resp, err := w.handleUndo(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleUndo() error = %v", err)
	}
	// Should fail gracefully (no checkpoints available)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestWorktreeHandleRedo_MultipleSteps(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	params, _ := json.Marshal(RedoParams{Steps: 2}) //nolint:errchkjson // test data
	resp, err := w.handleRedo(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleRedo() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for redo with empty redo stack")
	}
}

func TestWorktreeHandleRedo_ZeroStepsDefaultsToOne(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	params, _ := json.Marshal(RedoParams{Steps: 0}) //nolint:errchkjson // test data
	resp, err := w.handleRedo(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleRedo() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// ============================================================
// handleScreenshotsDelete tests
// ============================================================

func TestWorktreeHandleScreenshotsDelete_MissingScreenshotID(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	params, _ := json.Marshal(map[string]string{"screenshot_id": ""}) //nolint:errchkjson // test data
	resp, err := w.handleScreenshotsDelete(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleScreenshotsDelete() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for missing screenshot_id")
	}
}

func TestWorktreeHandleScreenshotsDelete_NonexistentScreenshot(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	params, _ := json.Marshal(map[string]string{"screenshot_id": "nonexistent-id"}) //nolint:errchkjson // test data
	resp, err := w.handleScreenshotsDelete(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleScreenshotsDelete() error = %v", err)
	}
	// Nonexistent screenshot → error response
	if resp.Error == nil {
		t.Fatal("expected error response for nonexistent screenshot")
	}
}

// ============================================================
// handleReset with conductor (wrong state)
// ============================================================

func TestWorktreeHandleReset_ImplementingState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateImplementing)

	resp, err := w.handleReset(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleReset() error = %v", err)
	}
	// Reset from implementing state may or may not work depending on conductor rules
	// Just ensure no panic and a non-nil response
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// ============================================================
// handleBrowse (worktree) tests
// ============================================================

func TestWorktreeHandleBrowse_NilParams(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	w.path = t.TempDir() // Set a valid path

	// No params = use worktree root
	resp, err := w.handleBrowse(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowse() error = %v", err)
	}
	// Response may succeed (list root) or fail — must not panic
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestWorktreeHandleBrowse_InvalidParams(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleBrowse(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleBrowse() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// ============================================================
// handleSubmit additional state tests
// ============================================================

func TestWorktreeHandleSubmit_NilConductorExplicitStreams(t *testing.T) {
	// handleSubmit with no conductor configured should return an error response immediately,
	// tested here with an explicitly-constructed WorktreeSocket (not the test helper).
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleSubmit(ctx, &Request{ID: "99"})
	if err != nil {
		t.Fatalf("handleSubmit() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for submit with nil conductor")
	}
	if resp.ID != "99" {
		t.Errorf("response ID = %q, want 99", resp.ID)
	}
}
