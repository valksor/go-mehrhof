package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
)

func TestHandlePolicyCheck_NoWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	// No work unit set → should return empty violations with "no active task" message.

	req := &Request{ID: "1", Method: "policy.check"}
	resp, err := w.handlePolicyCheck(ctx, req)
	if err != nil {
		t.Fatalf("handlePolicyCheck() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handlePolicyCheck() returned error: %s", resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["message"] != "no active task" {
		t.Errorf("message = %q, want %q", result["message"], "no active task")
	}
	violations, ok := result["violations"].([]any)
	if !ok {
		t.Fatal("violations should be an array")
	}
	if len(violations) != 0 {
		t.Errorf("violations length = %d, want 0", len(violations))
	}
}

func TestHandlePolicyCheck_WithWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	req := &Request{ID: "2", Method: "policy.check"}
	resp, err := w.handlePolicyCheck(ctx, req)
	if err != nil {
		t.Fatalf("handlePolicyCheck() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handlePolicyCheck() returned error: %s", resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := result["violations"]; !ok {
		t.Error("result should contain 'violations' key")
	}
	if _, ok := result["findings"]; !ok {
		t.Error("result should contain 'findings' key")
	}
}
