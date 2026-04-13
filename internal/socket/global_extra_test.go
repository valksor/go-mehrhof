package socket

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestGlobalHandleConfigValidate_AllowsOllamaDefault(t *testing.T) {
	ctx := context.Background()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	configDir := filepath.Join(homeDir, ".valksor", "kvelmo")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	configPath := filepath.Join(configDir, "kvelmo.yaml")
	config := "agent:\n  default: ollama\n"
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	g := newTestGlobalSocket(t)

	resp, err := g.handleConfigValidate(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleConfigValidate() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleConfigValidate() returned error: %s", resp.Error.Message)
	}

	var result struct {
		Valid  bool `json:"valid"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.Valid {
		t.Fatalf("valid = false, want true; checks = %+v", result.Checks)
	}
	for _, check := range result.Checks {
		if check.Name == "agent.default" && check.Status == "error" {
			t.Fatalf("agent.default unexpectedly failed: %+v", check)
		}
	}
}
