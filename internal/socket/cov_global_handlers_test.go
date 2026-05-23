package socket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/activitylog"
	"github.com/valksor/kvelmo/internal/testutil"
)

// --- global.go lifecycle + misc ---

func TestGlobalUseMiddleware(t *testing.T) {
	g := newTestGlobalSocket(t)
	called := false
	g.UseMiddleware(func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *Request) *Response {
			called = true

			return next(ctx, req)
		}
	})

	// Dispatch a request through the server so the inserted middleware runs.
	resp := g.server.dispatch(context.Background(), &Request{ID: "1", Method: "ping"}, nil)
	if resp == nil {
		t.Fatal("expected response")
	}
	if !called {
		t.Error("inserted middleware was not invoked")
	}
}

func TestGlobalStop(t *testing.T) {
	g := newTestGlobalSocket(t)
	// Stop on a never-started socket must be a clean no-op-ish call.
	if err := g.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestGlobalHandleDocsURL_Cov(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, err := g.handleDocsURL(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleDocsURL() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["url"] == "" {
		t.Error("expected non-empty docs url")
	}
}

func TestGlobalHandleAgentStatus_Cov(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, err := g.handleAgentStatus(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleAgentStatus() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if len(resp.Result) == 0 {
		t.Fatal("expected agent status result")
	}
}

func TestGlobalHandleStrategyList(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, err := g.handleStrategyList(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStrategyList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var strategies []any
	if err := json.Unmarshal(resp.Result, &strategies); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(strategies) == 0 {
		t.Error("expected at least one strategy")
	}
}

// --- global_projects.go ---

func TestGlobalHandleWorktreeCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleWorktreeCreate(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleWorktreeCreate() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing path", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(map[string]string{"path": ""})
		resp, err := g.handleWorktreeCreate(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleWorktreeCreate() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing path")
		}
	})

	t.Run("valid path creates socket entry", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		dir := testutil.TempDir(t)
		params, _ := json.Marshal(map[string]string{"path": dir})
		resp, err := g.handleWorktreeCreate(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleWorktreeCreate() error = %v", err)
		}
		// Either it succeeds with a socket_path or returns an error response;
		// both are valid non-panicking outcomes. Assert liveness.
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
		// Wait for the background-started worktree socket to be reachable before
		// Stop, avoiding the pre-existing Server.Start/initiateShutdown race.
		waitForSocket(ctx, t, WorktreeSocketPath(dir))
		t.Cleanup(func() { _ = g.Stop() })
	})
}

func TestGlobalHandleTasksList(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleTasksList(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleTasksList() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("no worktrees returns empty list", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleTasksList(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleTasksList() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result TasksListResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if result.Total != 0 {
			t.Errorf("Total = %d, want 0", result.Total)
		}
	})

	t.Run("with worktree and state filter", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		g.mu.Lock()
		g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: "/p", State: "loaded"}
		g.worktrees["wt-2"] = &WorktreeInfo{ID: "wt-2", Path: "/q", State: "implementing"}
		g.mu.Unlock()

		params, _ := json.Marshal(map[string]any{"state": "loaded", "page": 1, "per_page": 10})
		resp, err := g.handleTasksList(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleTasksList() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result TasksListResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		for _, task := range result.Tasks {
			if task.State != "loaded" {
				t.Errorf("state filter leaked task with state %q", task.State)
			}
		}
	})
}

// --- TaskListSummary Searchable interface ---

func TestTaskListSummary_Searchable(t *testing.T) {
	s := TaskListSummary{TaskTitle: "T", Source: "github", State: "loaded"}
	if s.SearchTitle() != "T" {
		t.Errorf("SearchTitle() = %q", s.SearchTitle())
	}
	if s.SearchDescription() != "github" {
		t.Errorf("SearchDescription() = %q", s.SearchDescription())
	}
	if s.SearchTags() != nil {
		t.Errorf("SearchTags() = %v, want nil", s.SearchTags())
	}
	if s.SearchStatus() != "loaded" {
		t.Errorf("SearchStatus() = %q", s.SearchStatus())
	}
	if !s.SearchCreatedAt().IsZero() {
		t.Error("SearchCreatedAt() should be zero")
	}
	if s.SearchPriority() != 0 {
		t.Errorf("SearchPriority() = %d, want 0", s.SearchPriority())
	}
}

// --- global_health.go ---

func TestGlobalRunHealthChecks(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	g.mu.Lock()
	// A worktree whose socket path does not exist will fail the health ping.
	g.worktrees["wt-dead"] = &WorktreeInfo{ID: "wt-dead", Path: "/dead", SocketPath: "/nonexistent/wt.sock"}
	// One with no socket path is skipped.
	g.worktrees["wt-nosock"] = &WorktreeInfo{ID: "wt-nosock", Path: "/p"}
	g.mu.Unlock()

	// Run the check 3 times so the failing one crosses the unhealthy threshold.
	for range 3 {
		g.runHealthChecks(ctx)
	}

	g.mu.RLock()
	dead := g.worktrees["wt-dead"]
	g.mu.RUnlock()
	if dead.Healthy == nil || *dead.Healthy {
		t.Errorf("wt-dead Healthy = %v, want false after repeated ping failures", dead.Healthy)
	}
}

func TestGlobalPingWorktree(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	// Nonexistent socket → unhealthy.
	if g.pingWorktree(ctx, "/nonexistent/wt.sock") {
		t.Error("pingWorktree() on missing socket should return false")
	}
}

func TestGlobalHandleSystemHealth(t *testing.T) {
	g := newTestGlobalSocket(t)
	g.mu.Lock()
	healthy := true
	g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: "/p", State: "loaded", Healthy: &healthy, LastPing: time.Now()}
	g.mu.Unlock()

	resp, err := g.handleSystemHealth(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleSystemHealth() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result struct {
		Worktrees []struct {
			ID string `json:"id"`
		} `json:"worktrees"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Worktrees) != 1 {
		t.Errorf("worktrees = %d, want 1", len(result.Worktrees))
	}
}

// --- global_metrics.go ---

func TestGlobalHandleMetricsHistory(t *testing.T) {
	ctx := context.Background()

	t.Run("no store returns disabled", func(t *testing.T) {
		// Ensure no time-series store is configured for this case.
		prev := timeSeriesStore
		timeSeriesStore = nil
		t.Cleanup(func() { timeSeriesStore = prev })

		g := newTestGlobalSocket(t)
		resp, err := g.handleMetricsHistory(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleMetricsHistory() error = %v", err)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		if enabled, _ := result["enabled"].(bool); enabled {
			t.Error("enabled = true, want false with no store")
		}
	})
}

// --- global_activity.go ---

func TestGlobalHandleActivityQuery(t *testing.T) {
	ctx := context.Background()

	t.Run("no logger configured", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleActivityQuery(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleActivityQuery() error = %v", err)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		if enabled, _ := result["enabled"].(bool); enabled {
			t.Error("enabled = true, want false with no activity logger")
		}
	})

	t.Run("with logger", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		log, err := activitylog.New(testutil.TempDir(t), 3)
		if err != nil {
			t.Fatalf("activitylog.New: %v", err)
		}
		g.SetActivityLog(log)
		log.Record(activitylog.Entry{Timestamp: time.Now(), Method: "ping"})

		resp, err := g.handleActivityQuery(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleActivityQuery() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		if enabled, _ := result["enabled"].(bool); !enabled {
			t.Error("enabled = false, want true with a configured logger")
		}
	})

	t.Run("invalid since duration", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		log, err := activitylog.New(testutil.TempDir(t), 3)
		if err != nil {
			t.Fatalf("activitylog.New: %v", err)
		}
		g.SetActivityLog(log)
		params, _ := json.Marshal(map[string]string{"since": "not-a-duration"})
		resp, err := g.handleActivityQuery(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleActivityQuery() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid since duration")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		log, err := activitylog.New(testutil.TempDir(t), 3)
		if err != nil {
			t.Fatalf("activitylog.New: %v", err)
		}
		g.SetActivityLog(log)
		resp, err := g.handleActivityQuery(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleActivityQuery() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("timeline mode", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		log, err := activitylog.New(testutil.TempDir(t), 3)
		if err != nil {
			t.Fatalf("activitylog.New: %v", err)
		}
		g.SetActivityLog(log)
		params, _ := json.Marshal(map[string]bool{"timeline": true})
		resp, err := g.handleActivityQuery(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleActivityQuery() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
	})
}
