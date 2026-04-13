package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
)

// --- Checkpoints handler tests ---

func TestHandleCheckpoints_NoConductor(t *testing.T) {
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}
	ctx := context.Background()

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
	if _, ok := result["checkpoints"]; !ok {
		t.Error("result should have checkpoints key")
	}
	if _, ok := result["redo_stack"]; !ok {
		t.Error("result should have redo_stack key")
	}
}

func TestHandleCheckpoints_NoWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleCheckpoints(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleCheckpoints() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleCheckpoints() returned error: %s", resp.Error.Message)
	}
}

func TestHandleCheckpoints_WithWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	resp, err := w.handleCheckpoints(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleCheckpoints() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleCheckpoints() returned error: %s", resp.Error.Message)
	}
}

// --- Discovery handler tests ---

func TestHandleDiscoveryScan(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleDiscoveryScan(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleDiscoveryScan() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleDiscoveryScan() returned error: %s", resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["commands"]; !ok {
		t.Error("result should have commands key")
	}
	if _, ok := result["count"]; !ok {
		t.Error("result should have count key")
	}
}

// --- Eventlog handler tests ---

func TestHandleEventlogQuery_NoParams(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	w.path = t.TempDir() // ensure .kvelmo dir doesn't exist yet

	resp, err := w.handleEventlogQuery(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleEventlogQuery() error = %v", err)
	}
	// May return error or empty list depending on dir existence
	_ = resp
}

func TestHandleEventlogQuery_InvalidSince(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	params := json.RawMessage(`{"since": "not-a-duration"}`)
	resp, err := w.handleEventlogQuery(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleEventlogQuery() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleEventlogQuery() should return error for invalid since")
	}
}

func TestHandleEventlogQuery_InvalidUntil(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	params := json.RawMessage(`{"until": "not-a-time"}`)
	resp, err := w.handleEventlogQuery(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleEventlogQuery() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleEventlogQuery() should return error for invalid until")
	}
}

// --- ReviewList / ReviewView handler tests ---

func TestHandleReviewList_NoConductor(t *testing.T) {
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}
	ctx := context.Background()

	resp, err := w.handleReviewList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleReviewList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleReviewList() returned error: %s", resp.Error.Message)
	}
	// Returns empty list when no conductor
}

func TestHandleReviewView_NoConductor(t *testing.T) {
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}
	ctx := context.Background()

	resp, err := w.handleReviewView(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleReviewView() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleReviewView() should return error when no conductor")
	}
}

// --- CheckpointGoto handler tests ---

func TestHandleCheckpointGoto_NoConductor(t *testing.T) {
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}
	ctx := context.Background()

	resp, err := w.handleCheckpointGoto(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleCheckpointGoto() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleCheckpointGoto() should return error when no conductor")
	}
}

func TestHandleCheckpointPreview_NoConductor(t *testing.T) {
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}
	ctx := context.Background()

	resp, err := w.handleCheckpointPreview(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleCheckpointPreview() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleCheckpointPreview() should return error when no conductor")
	}
}
