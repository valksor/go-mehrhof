package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
)

func TestHandleApprove_NoEvent(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	req := &Request{ID: "1", Method: "approve", Params: json.RawMessage(`{}`)}
	resp, err := w.handleApprove(ctx, req)
	if err != nil {
		t.Fatalf("handleApprove() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleApprove() should return error when event is empty")
	}
	if resp.Error.Message != "event is required" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "event is required")
	}
}

func TestHandleApprove_NoWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	// Conductor exists but no work unit → Approve returns "no task loaded".

	req := &Request{ID: "2", Method: "approve", Params: json.RawMessage(`{"event":"submit"}`)}
	resp, err := w.handleApprove(ctx, req)
	if err != nil {
		t.Fatalf("handleApprove() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleApprove() should return error when no work unit is loaded")
	}
}

func TestHandleApproveNode_NoNodeID(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	req := &Request{ID: "3", Method: "approve.node", Params: json.RawMessage(`{}`)}
	resp, err := w.handleApproveNode(ctx, req)
	if err != nil {
		t.Fatalf("handleApproveNode() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleApproveNode() should return error when node_id is empty")
	}
	if resp.Error.Message != "node_id is required" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "node_id is required")
	}
}

func TestHandleReviewChecklistGet(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	req := &Request{ID: "4", Method: "review.checklist.get"}
	resp, err := w.handleReviewChecklistGet(ctx, req)
	if err != nil {
		t.Fatalf("handleReviewChecklistGet() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleReviewChecklistGet() returned error: %s", resp.Error.Message)
	}

	var result map[string][]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["required"] == nil {
		t.Error("result should contain 'required' array")
	}
	if result["checked"] == nil {
		t.Error("result should contain 'checked' array")
	}
}

func TestHandleReviewChecklistCheck_MissingItem(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	req := &Request{ID: "5", Method: "review.checklist.check", Params: json.RawMessage(`{}`)}
	resp, err := w.handleReviewChecklistCheck(ctx, req)
	if err != nil {
		t.Fatalf("handleReviewChecklistCheck() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleReviewChecklistCheck() should return error when item is empty")
	}
	if resp.Error.Message != "item is required" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "item is required")
	}
}

func TestHandleReviewChecklistUncheck_MissingItem(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	req := &Request{ID: "6", Method: "review.checklist.uncheck", Params: json.RawMessage(`{}`)}
	resp, err := w.handleReviewChecklistUncheck(ctx, req)
	if err != nil {
		t.Fatalf("handleReviewChecklistUncheck() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleReviewChecklistUncheck() should return error when item is empty")
	}
	if resp.Error.Message != "item is required" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "item is required")
	}
}
