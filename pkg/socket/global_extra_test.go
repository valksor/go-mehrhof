package socket

import (
	"context"
	"encoding/json"
	"testing"
)

// --- TasksList handler tests ---

func TestGlobalHandleTasksList_WithWorktrees(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	g.mu.Lock()
	g.worktrees["wt-1"] = &WorktreeInfo{
		ID:    "wt-1",
		Path:  "/test/project",
		State: "loaded",
	}
	g.mu.Unlock()

	resp, err := g.handleTasksList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleTasksList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleTasksList() returned error: %s", resp.Error.Message)
	}
}

// --- ListJobs handler tests ---

func TestGlobalHandleListJobs_NoPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleListJobs(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleListJobs() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleListJobs() returned error: %s", resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	jobs, ok := result["jobs"].([]any)
	if !ok {
		t.Fatal("jobs should be an array")
	}
	if len(jobs) != 0 {
		t.Errorf("jobs length = %d, want 0", len(jobs))
	}
}

// --- ConfigValidate handler tests ---

func TestGlobalHandleConfigValidate(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleConfigValidate(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleConfigValidate() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleConfigValidate() returned error: %s", resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["valid"]; !ok {
		t.Error("result should contain valid key")
	}
	if _, ok := result["checks"]; !ok {
		t.Error("result should contain checks key")
	}
}
