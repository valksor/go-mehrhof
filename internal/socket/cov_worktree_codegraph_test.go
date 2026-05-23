package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/testutil"
)

// newCodegraphWorktree builds a worktree socket rooted at a real temp dir so
// the lazy codegraph SQLite DB can be created on disk.
func newCodegraphWorktree(ctx context.Context, t *testing.T) *WorktreeSocket {
	t.Helper()
	w := newTestWorktreeSocket(ctx, t)
	w.path = testutil.TempDir(t)

	return w
}

func TestWorktreeHandleCodegraphStats(t *testing.T) {
	ctx := context.Background()
	w := newCodegraphWorktree(ctx, t)

	resp, err := w.handleCodegraphStats(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleCodegraphStats() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error response: %s", resp.Error.Message)
	}
	if len(resp.Result) == 0 {
		t.Fatal("expected stats result")
	}
}

func TestWorktreeHandleCodegraphSearch(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		w := newCodegraphWorktree(ctx, t)
		resp, err := w.handleCodegraphSearch(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleCodegraphSearch() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing name", func(t *testing.T) {
		w := newCodegraphWorktree(ctx, t)
		params, _ := json.Marshal(map[string]any{"name": ""})
		resp, err := w.handleCodegraphSearch(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCodegraphSearch() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing name")
		}
	})

	t.Run("search empty graph", func(t *testing.T) {
		w := newCodegraphWorktree(ctx, t)
		params, _ := json.Marshal(map[string]any{"name": "Foo"})
		resp, err := w.handleCodegraphSearch(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCodegraphSearch() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result struct {
			Symbols []any `json:"symbols"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if result.Symbols == nil {
			t.Error("symbols should be a non-nil empty slice")
		}
	})

	t.Run("pattern search empty graph", func(t *testing.T) {
		w := newCodegraphWorktree(ctx, t)
		params, _ := json.Marshal(map[string]any{"name": "Foo*", "pattern": true})
		resp, err := w.handleCodegraphSearch(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCodegraphSearch() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
	})
}

func TestWorktreeHandleCodegraphCallers(t *testing.T) {
	ctx := context.Background()

	t.Run("missing name", func(t *testing.T) {
		w := newCodegraphWorktree(ctx, t)
		params, _ := json.Marshal(map[string]any{"name": ""})
		resp, err := w.handleCodegraphCallers(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCodegraphCallers() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing name")
		}
	})

	t.Run("query empty graph", func(t *testing.T) {
		w := newCodegraphWorktree(ctx, t)
		params, _ := json.Marshal(map[string]any{"name": "Foo"})
		resp, err := w.handleCodegraphCallers(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCodegraphCallers() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result struct {
			Callers []any `json:"callers"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if result.Callers == nil {
			t.Error("callers should be a non-nil empty slice")
		}
	})
}

func TestWorktreeHandleCodegraphDeps(t *testing.T) {
	ctx := context.Background()

	t.Run("missing package", func(t *testing.T) {
		w := newCodegraphWorktree(ctx, t)
		params, _ := json.Marshal(map[string]any{"package": ""})
		resp, err := w.handleCodegraphDeps(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCodegraphDeps() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing package")
		}
	})

	t.Run("query empty graph", func(t *testing.T) {
		w := newCodegraphWorktree(ctx, t)
		params, _ := json.Marshal(map[string]any{"package": "github.com/x/y"})
		resp, err := w.handleCodegraphDeps(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCodegraphDeps() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result struct {
			Dependencies []string `json:"dependencies"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if result.Dependencies == nil {
			t.Error("dependencies should be a non-nil empty slice")
		}
	})
}

func TestWorktreeHandleCodegraphIndex(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		w := newCodegraphWorktree(ctx, t)
		resp, err := w.handleCodegraphIndex(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleCodegraphIndex() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("path outside worktree rejected", func(t *testing.T) {
		w := newCodegraphWorktree(ctx, t)
		params, _ := json.Marshal(map[string]string{"path": "../../../etc"})
		resp, err := w.handleCodegraphIndex(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCodegraphIndex() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for path outside worktree")
		}
	})

	t.Run("index worktree root", func(t *testing.T) {
		w := newCodegraphWorktree(ctx, t)
		resp, err := w.handleCodegraphIndex(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleCodegraphIndex() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := result["files"]; !ok {
			t.Error("expected files key in index result")
		}
	})
}
