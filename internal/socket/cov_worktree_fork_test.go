package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
)

// --- worktree_fork.go ---

func TestWorktreeHandleForkCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleForkCreate(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleForkCreate() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleForkCreate(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleForkCreate() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing label", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(map[string]string{"label": ""})
		resp, err := w.handleForkCreate(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleForkCreate() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing label")
		}
	})

	t.Run("forking disabled without repo", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplemented)
		params, _ := json.Marshal(map[string]string{"label": "alt-approach"})
		resp, err := w.handleForkCreate(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleForkCreate() error = %v", err)
		}
		// No git/forking configured: must surface an error response, not a panic.
		if resp.Error == nil {
			t.Fatal("expected error response when forking unavailable")
		}
	})
}

func TestWorktreeHandleForkList(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleForkList(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleForkList() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("empty fork list", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleForkList(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleForkList() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result struct {
			Forks []conductor.ForkInfo `json:"forks"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if result.Forks == nil {
			t.Error("forks should be a non-nil empty slice")
		}
		if len(result.Forks) != 0 {
			t.Errorf("forks = %v, want empty", result.Forks)
		}
	})
}

func TestWorktreeHandleForkCompare(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleForkCompare(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleForkCompare() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("no forks", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplemented)
		resp, err := w.handleForkCompare(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleForkCompare() error = %v", err)
		}
		// With no forks it may return an empty comparison or an error; either is
		// fine as long as a response comes back without panicking.
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
	})
}

func TestWorktreeHandleForkSelect(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleForkSelect(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleForkSelect() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleForkSelect(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleForkSelect() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing fork_id", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(map[string]string{"fork_id": ""})
		resp, err := w.handleForkSelect(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleForkSelect() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing fork_id")
		}
	})

	t.Run("nonexistent fork", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplemented)
		params, _ := json.Marshal(map[string]string{"fork_id": "nonexistent"})
		resp, err := w.handleForkSelect(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleForkSelect() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for nonexistent fork")
		}
	})
}
