package socket

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHandleSystemHealth_Empty(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	req := &Request{ID: "1", Method: "system.health"}
	resp, err := g.handleSystemHealth(ctx, req)
	if err != nil {
		t.Fatalf("handleSystemHealth() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleSystemHealth() returned error: %s", resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	worktrees, ok := result["worktrees"].([]any)
	if !ok {
		t.Fatal("worktrees should be an array")
	}
	if len(worktrees) != 0 {
		t.Errorf("worktrees length = %d, want 0", len(worktrees))
	}
}

func TestHandleSystemHealth_WithWorktrees(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	// Register a worktree directly in the map.
	g.mu.Lock()
	g.worktrees["test-wt"] = &WorktreeInfo{
		ID:    "test-wt",
		Path:  "/tmp/test-project",
		State: "loaded",
	}
	g.mu.Unlock()

	req := &Request{ID: "2", Method: "system.health"}
	resp, err := g.handleSystemHealth(ctx, req)
	if err != nil {
		t.Fatalf("handleSystemHealth() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleSystemHealth() returned error: %s", resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	worktrees, ok := result["worktrees"].([]any)
	if !ok {
		t.Fatal("worktrees should be an array")
	}
	if len(worktrees) != 1 {
		t.Fatalf("worktrees length = %d, want 1", len(worktrees))
	}
}
