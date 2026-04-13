package socket

import (
	"context"
	"encoding/json"
	"testing"
)

// ============================================================
// handleListProjects tests
// ============================================================

func TestGlobalHandleListProjects_Empty(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleListProjects(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleListProjects() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleListProjects() returned error: %s", resp.Error.Message)
	}

	var result ProjectListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Projects == nil {
		t.Error("projects should not be nil (may be empty slice)")
	}
}

func TestGlobalHandleListProjects_WithOfflineProject(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	// Register a project with no real socket path
	g.mu.Lock()
	g.worktrees["wt-1"] = &WorktreeInfo{
		ID:         "wt-1",
		Path:       "/some/nonexistent/path",
		SocketPath: "", // empty = offline
		State:      "none",
	}
	g.mu.Unlock()

	resp, err := g.handleListProjects(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleListProjects() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleListProjects() returned error: %s", resp.Error.Message)
	}

	var result ProjectListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(result.Projects))
	}
	// Project with empty SocketPath should be marked offline
	if result.Projects[0].State != "offline" {
		t.Errorf("project state = %q, want offline", result.Projects[0].State)
	}
}

// ============================================================
// handleRegisterProject tests
// ============================================================

func TestGlobalHandleRegisterProject_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleRegisterProject(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleRegisterProject() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON params")
	}
}

func TestGlobalHandleRegisterProject_ValidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, _ := json.Marshal(RegisterParams{Path: "/tmp/test-project", SocketPath: "/tmp/test.sock"}) //nolint:errchkjson // test data
	resp, err := g.handleRegisterProject(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleRegisterProject() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleRegisterProject() returned error: %s", resp.Error.Message)
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["id"] == "" {
		t.Error("expected non-empty project id")
	}
}

func TestGlobalHandleRegisterProject_DuplicatePath(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, _ := json.Marshal(RegisterParams{Path: "/tmp/same-project", SocketPath: "/tmp/same.sock"}) //nolint:errchkjson // test data

	// First registration
	resp1, err := g.handleRegisterProject(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("first handleRegisterProject() error = %v", err)
	}
	if resp1.Error != nil {
		t.Fatalf("first handleRegisterProject() returned error: %s", resp1.Error.Message)
	}

	// Second registration with the same path should update, not error
	resp2, err := g.handleRegisterProject(ctx, &Request{ID: "2", Params: params})
	if err != nil {
		t.Fatalf("second handleRegisterProject() error = %v", err)
	}
	if resp2.Error != nil {
		t.Fatalf("second handleRegisterProject() returned error: %s", resp2.Error.Message)
	}

	var result1 map[string]string
	var result2 map[string]string
	_ = json.Unmarshal(resp1.Result, &result1)
	_ = json.Unmarshal(resp2.Result, &result2)

	// Same path should produce the same id (idempotent)
	if result1["id"] != result2["id"] {
		t.Errorf("duplicate registration produced different IDs: %q vs %q", result1["id"], result2["id"])
	}
}

// ============================================================
// handleUnregisterProject tests
// ============================================================

func TestGlobalHandleUnregisterProject_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleUnregisterProject(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleUnregisterProject() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON params")
	}
}

func TestGlobalHandleUnregisterProject_NonexistentID(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, _ := json.Marshal(UnregisterParams{ID: "nonexistent-id"}) //nolint:errchkjson // test data
	resp, err := g.handleUnregisterProject(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleUnregisterProject() error = %v", err)
	}
	// Unregistering a nonexistent ID is a no-op that succeeds (idempotent delete)
	if resp.Error != nil {
		t.Errorf("handleUnregisterProject() unexpected error for nonexistent ID: %s", resp.Error.Message)
	}
}

// ============================================================
// handleGetJob tests
// ============================================================

func TestGlobalHandleGetJob_WithPool_NotFound(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocketWithPool2(t)

	params := mustMarshal(t, map[string]string{"id": "nonexistent-job-id"})
	resp, err := g.handleGetJob(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for nonexistent job")
	}
}

func TestGlobalHandleGetJob_MalformedParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocketWithPool2(t)

	resp, err := g.handleGetJob(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for malformed params")
	}
}

// ============================================================
// handleProvidersTest tests
// ============================================================

func TestGlobalHandleProvidersTest_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	// handleProvidersTest returns a Go error (not an error response) for parse failures
	_, err := g.handleProvidersTest(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err == nil {
		t.Fatal("expected Go error for invalid JSON params")
	}
}

func TestGlobalHandleProvidersTest_UnknownProvider(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, _ := json.Marshal(map[string]string{"provider": "nonexistent"}) //nolint:errchkjson // test data
	resp, err := g.handleProvidersTest(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleProvidersTest() error = %v", err)
	}
	// Unknown provider should return error or "not connected" status
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// ============================================================
// handleSettingsGet with project_path tests
// ============================================================

func TestGlobalHandleSettingsGet_WithProjectPath(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, _ := json.Marshal(SettingsGetParams{ProjectPath: t.TempDir()}) //nolint:errchkjson // test data
	resp, err := g.handleSettingsGet(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleSettingsGet() error = %v", err)
	}
	// May succeed or fail gracefully; either is acceptable
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := result["effective"]; !ok {
			t.Error("result should have 'effective' key")
		}
	}
}

func TestGlobalHandleSettingsGet_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleSettingsGet(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleSettingsGet() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON params")
	}
}

// ============================================================
// handleSettingsSet – project scope requires path
// ============================================================

func TestGlobalHandleSettingsSet_ProjectScopeNeedsPath(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t) // no registered worktrees

	params, _ := json.Marshal(map[string]any{ //nolint:errchkjson // test data
		"scope":  "project",
		"values": map[string]any{"workers.max": 3},
		// no project_path and no registered worktrees → error
	})
	resp, err := g.handleSettingsSet(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleSettingsSet() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for project scope without path or registered project")
	}
}
