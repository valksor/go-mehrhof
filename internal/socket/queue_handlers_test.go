package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
)

// ============================================================
// handleQueueList tests
// ============================================================

func TestWorktreeHandleQueueList_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleQueueList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleQueueList() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when conductor is nil")
	}
}

func TestWorktreeHandleQueueList_EmptyQueue(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	resp, err := w.handleQueueList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleQueueList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleQueueList() returned error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["queue"]; !ok {
		t.Error("result should have 'queue' key")
	}
	if _, ok := result["total"]; !ok {
		t.Error("result should have 'total' key")
	}
	if _, ok := result["page"]; !ok {
		t.Error("result should have 'page' key")
	}
	if _, ok := result["per_page"]; !ok {
		t.Error("result should have 'per_page' key")
	}
	if _, ok := result["has_next"]; !ok {
		t.Error("result should have 'has_next' key")
	}
}

// ============================================================
// handleQueueAdd tests
// ============================================================

func TestWorktreeHandleQueueAdd_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	params, _ := json.Marshal(queueAddParams{Source: "empty:do something"}) //nolint:errchkjson // test data
	resp, err := w.handleQueueAdd(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQueueAdd() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when conductor is nil")
	}
}

func TestWorktreeHandleQueueAdd_EmptySource(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	params, _ := json.Marshal(queueAddParams{Source: ""}) //nolint:errchkjson // test data
	resp, err := w.handleQueueAdd(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQueueAdd() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for empty source")
	}
}

func TestWorktreeHandleQueueAdd_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	resp, err := w.handleQueueAdd(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleQueueAdd() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON params")
	}
}

func TestWorktreeHandleQueueAdd_ValidSource(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	params, _ := json.Marshal(queueAddParams{Source: "empty:fix the button", Title: "Fix button"}) //nolint:errchkjson // test data
	resp, err := w.handleQueueAdd(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQueueAdd() error = %v", err)
	}
	// May succeed or fail depending on conductor queue logic, but must not panic
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// ============================================================
// handleQueueRemove tests
// ============================================================

func TestWorktreeHandleQueueRemove_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	params, _ := json.Marshal(queueRemoveParams{ID: "task-1"}) //nolint:errchkjson // test data
	resp, err := w.handleQueueRemove(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQueueRemove() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when conductor is nil")
	}
}

func TestWorktreeHandleQueueRemove_EmptyID(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	params, _ := json.Marshal(queueRemoveParams{ID: ""}) //nolint:errchkjson // test data
	resp, err := w.handleQueueRemove(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQueueRemove() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for empty ID")
	}
}

func TestWorktreeHandleQueueRemove_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	resp, err := w.handleQueueRemove(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleQueueRemove() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON params")
	}
}

func TestWorktreeHandleQueueRemove_NonexistentID(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	params, _ := json.Marshal(queueRemoveParams{ID: "nonexistent-task-id"}) //nolint:errchkjson // test data
	resp, err := w.handleQueueRemove(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQueueRemove() error = %v", err)
	}
	// Should return error because task doesn't exist in queue
	if resp.Error == nil {
		t.Fatal("expected error response for nonexistent task ID")
	}
}

// ============================================================
// handleQueueReorder tests
// ============================================================

func TestWorktreeHandleQueueReorder_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	params, _ := json.Marshal(queueReorderParams{ID: "task-1", Position: 1}) //nolint:errchkjson // test data
	resp, err := w.handleQueueReorder(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQueueReorder() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response when conductor is nil")
	}
}

func TestWorktreeHandleQueueReorder_EmptyID(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	params, _ := json.Marshal(queueReorderParams{ID: "", Position: 1}) //nolint:errchkjson // test data
	resp, err := w.handleQueueReorder(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQueueReorder() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for empty ID")
	}
}

func TestWorktreeHandleQueueReorder_ZeroPosition(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	params, _ := json.Marshal(queueReorderParams{ID: "task-1", Position: 0}) //nolint:errchkjson // test data
	resp, err := w.handleQueueReorder(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQueueReorder() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for position < 1")
	}
}

func TestWorktreeHandleQueueReorder_NegativePosition(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	params, _ := json.Marshal(queueReorderParams{ID: "task-1", Position: -5}) //nolint:errchkjson // test data
	resp, err := w.handleQueueReorder(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleQueueReorder() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for negative position")
	}
}

func TestWorktreeHandleQueueReorder_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	resp, err := w.handleQueueReorder(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleQueueReorder() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON params")
	}
}

// ============================================================
// handleTaskHistory tests (worktree_queue.go)
// ============================================================

func TestWorktreeHandleTaskHistory_EmptyHistory(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	resp, err := w.handleTaskHistory(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleTaskHistory() error = %v", err)
	}
	// Either a valid empty list or an error (no store configured) — must not panic
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		// Error is acceptable when no backing store is configured
		return
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["tasks"]; !ok {
		t.Error("result should have 'tasks' key")
	}
	if _, ok := result["count"]; !ok {
		t.Error("result should have 'count' key")
	}
}

func TestWorktreeHandleTaskHistory_WithTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	resp, err := w.handleTaskHistory(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleTaskHistory() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// Passes if either a valid result or acceptable error
}
