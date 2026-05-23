package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
)

// nilConductorWorktree builds a WorktreeSocket with no conductor wired, so the
// nil-conductor guard at the top of every handler can be exercised.
func nilConductorWorktree() *WorktreeSocket {
	return &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}
}

// --- worktree_cache.go ---

func TestWorktreeHandleCacheStats(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleCacheStats(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleCacheStats() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("no cache configured returns disabled", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleCacheStats(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleCacheStats() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		// The test conductor has no response cache, so enabled must be false.
		if enabled, _ := result["enabled"].(bool); enabled {
			t.Errorf("enabled = true, want false (no cache configured)")
		}
	})
}

func TestWorktreeHandleCacheClear(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleCacheClear(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleCacheClear() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("clears successfully", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleCacheClear(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleCacheClear() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result map[string]string
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if result["status"] != "ok" {
			t.Errorf("status = %q, want ok", result["status"])
		}
	})
}

// --- worktree_tags.go ---

func TestWorktreeHandleTaskTag(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleTaskTag(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleTaskTag() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleTaskTag(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleTaskTag() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("no active task", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(map[string]any{"action": "list"})
		resp, err := w.handleTaskTag(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleTaskTag() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no work unit")
		}
	})

	t.Run("add then list then remove", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateLoaded)

		addParams, _ := json.Marshal(map[string]any{"action": "add", "tags": []string{"bug", "urgent"}})
		resp, err := w.handleTaskTag(ctx, &Request{ID: "1", Params: addParams})
		if err != nil {
			t.Fatalf("add: %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("add returned error: %s", resp.Error.Message)
		}
		var addResult struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(resp.Result, &addResult); err != nil {
			t.Fatalf("unmarshal add: %v", err)
		}
		if len(addResult.Tags) != 2 {
			t.Fatalf("after add, tags = %v, want 2", addResult.Tags)
		}

		// Adding a duplicate must not grow the slice.
		dupParams, _ := json.Marshal(map[string]any{"action": "add", "tags": []string{"bug"}})
		resp, _ = w.handleTaskTag(ctx, &Request{ID: "2", Params: dupParams})
		_ = json.Unmarshal(resp.Result, &addResult)
		if len(addResult.Tags) != 2 {
			t.Errorf("after duplicate add, tags = %v, want 2", addResult.Tags)
		}

		// List.
		listParams, _ := json.Marshal(map[string]any{"action": "list"})
		resp, _ = w.handleTaskTag(ctx, &Request{ID: "3", Params: listParams})
		var listResult struct {
			Tags []string `json:"tags"`
		}
		_ = json.Unmarshal(resp.Result, &listResult)
		if len(listResult.Tags) != 2 {
			t.Errorf("list tags = %v, want 2", listResult.Tags)
		}

		// Remove.
		rmParams, _ := json.Marshal(map[string]any{"action": "remove", "tags": []string{"urgent"}})
		resp, _ = w.handleTaskTag(ctx, &Request{ID: "4", Params: rmParams})
		var rmResult struct {
			Tags []string `json:"tags"`
		}
		_ = json.Unmarshal(resp.Result, &rmResult)
		if len(rmResult.Tags) != 1 || rmResult.Tags[0] != "bug" {
			t.Errorf("after remove, tags = %v, want [bug]", rmResult.Tags)
		}
	})

	t.Run("unknown action", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateLoaded)
		params, _ := json.Marshal(map[string]any{"action": "frobnicate"})
		resp, err := w.handleTaskTag(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleTaskTag() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for unknown action")
		}
	})
}

// --- worktree_risk.go ---

func TestWorktreeHandleRiskEvaluate(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleRiskEvaluate(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleRiskEvaluate() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("no task loaded", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleRiskEvaluate(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleRiskEvaluate() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no work unit")
		}
	})

	t.Run("with task returns a score", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplemented)
		resp, err := w.handleRiskEvaluate(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleRiskEvaluate() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		if len(resp.Result) == 0 {
			t.Fatal("expected a risk score result")
		}
	})
}

func TestWorktreeHandleRiskHistory(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleRiskHistory(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleRiskHistory() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("no event log returns empty entries", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleRiskHistory(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleRiskHistory() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := result["entries"]; !ok {
			t.Error("expected entries key in result")
		}
	})
}

// --- worktree_ci.go ---

func TestWorktreeHandleCIStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleCIStatus(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleCIStatus() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("no active task", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleCIStatus(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleCIStatus() error = %v", err)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		if result["state"] != "unknown" {
			t.Errorf("state = %v, want unknown", result["state"])
		}
		if result["message"] != "no active task" {
			t.Errorf("message = %v, want 'no active task'", result["message"])
		}
	})

	t.Run("task without PR", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplemented)
		resp, err := w.handleCIStatus(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleCIStatus() error = %v", err)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		if result["message"] != "no PR submitted yet" {
			t.Errorf("message = %v, want 'no PR submitted yet'", result["message"])
		}
	})

	t.Run("task with PR", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateSubmitted)
		w.conductor.WorkUnit().PRID = "https://github.com/o/r/pull/1"
		resp, err := w.handleCIStatus(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleCIStatus() error = %v", err)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		if result["pr_id"] != "https://github.com/o/r/pull/1" {
			t.Errorf("pr_id = %v, want PR URL", result["pr_id"])
		}
	})
}

// --- worktree_autofix.go ---

func TestWorktreeHandleAutoFixStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleAutoFixStatus(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleAutoFixStatus() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("returns status", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleAutoFixStatus(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleAutoFixStatus() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		if len(resp.Result) == 0 {
			t.Fatal("expected an auto-fix status result")
		}
	})
}

// --- worktree_failclass.go ---

func TestWorktreeHandleFailclassStats(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleFailclassStats(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleFailclassStats() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("no task returns empty stats", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleFailclassStats(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleFailclassStats() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
	})

	t.Run("with quality gate error", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplemented)
		w.conductor.WorkUnit().QualityGateError = "lint: 3 errors found"
		resp, err := w.handleFailclassStats(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleFailclassStats() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		if len(resp.Result) == 0 {
			t.Fatal("expected stats result")
		}
	})
}

// --- worktree_stream.go: handleProgressGet ---

func TestWorktreeHandleProgressGet(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleProgressGet(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleProgressGet() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("no estimate returns inactive", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleProgressGet(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleProgressGet() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		if active, _ := result["active"].(bool); active {
			t.Errorf("active = true, want false (no estimate)")
		}
	})
}

// --- worktree_queue.go: handleSuggestionsList + handleTaskSearch ---

func TestWorktreeHandleSuggestionsList(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleSuggestionsList(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleSuggestionsList() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("returns skip and agent keys", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleSuggestionsList(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleSuggestionsList() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := result["skip"]; !ok {
			t.Error("missing skip key")
		}
		if _, ok := result["agent"]; !ok {
			t.Error("missing agent key")
		}
	})
}

func TestWorktreeHandleTaskSearch(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleTaskSearch(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleTaskSearch() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleTaskSearch(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleTaskSearch() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("invalid since timestamp", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(map[string]string{"since": "not-a-time"})
		resp, err := w.handleTaskSearch(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleTaskSearch() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid since timestamp")
		}
	})

	t.Run("invalid until timestamp", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(map[string]string{"until": "not-a-time"})
		resp, err := w.handleTaskSearch(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleTaskSearch() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid until timestamp")
		}
	})

	t.Run("valid query against empty history", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(map[string]any{"query": "anything", "limit": 5})
		resp, err := w.handleTaskSearch(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleTaskSearch() error = %v", err)
		}
		// No store wired: either an empty result or an error response; both are
		// acceptable, but the call must not panic and must return a response.
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
	})
}
