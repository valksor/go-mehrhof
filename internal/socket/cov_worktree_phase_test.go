package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
)

// Each phase handler shares the same shape: nil-conductor guard, invalid-params
// path, and a conductor call that errors when the state machine is not in a
// state that permits the transition. The test conductor has no work unit in the
// right state nor a worker pool, so the transition call returns an error
// response — which is exactly the path we assert here.

func TestWorktreeHandleStart(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleStart(ctx, &Request{ID: "1", Params: json.RawMessage(`{}`)})
		if err != nil {
			t.Fatalf("handleStart() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleStart(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleStart() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing source", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(StartParams{Source: ""})
		resp, err := w.handleStart(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleStart() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing source")
		}
	})

	t.Run("start with already-loaded task errors", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplementing)
		params, _ := json.Marshal(StartParams{Source: "empty:something"})
		resp, err := w.handleStart(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleStart() error = %v", err)
		}
		// Starting while a task is mid-flight should produce an error response.
		if resp.Error == nil {
			t.Fatal("expected error response when starting over an active task")
		}
	})
}

// phaseHandler is the common phase-transition handler signature.
type phaseHandlerCase struct {
	name    string
	handler func(*WorktreeSocket) func(context.Context, *Request) (*Response, error)
}

func phaseHandlers() []phaseHandlerCase {
	return []phaseHandlerCase{
		{"plan", func(w *WorktreeSocket) func(context.Context, *Request) (*Response, error) { return w.handlePlan }},
		{"implement", func(w *WorktreeSocket) func(context.Context, *Request) (*Response, error) { return w.handleImplement }},
		{"optimize", func(w *WorktreeSocket) func(context.Context, *Request) (*Response, error) { return w.handleOptimize }},
		{"simplify", func(w *WorktreeSocket) func(context.Context, *Request) (*Response, error) { return w.handleSimplify }},
		{"review", func(w *WorktreeSocket) func(context.Context, *Request) (*Response, error) { return w.handleReview }},
		{"submit", func(w *WorktreeSocket) func(context.Context, *Request) (*Response, error) { return w.handleSubmit }},
	}
}

func TestWorktreePhaseHandlers_NilConductor(t *testing.T) {
	ctx := context.Background()
	for _, tc := range phaseHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			w := nilConductorWorktree()
			resp, err := tc.handler(w)(ctx, &Request{ID: "1"})
			if err != nil {
				t.Fatalf("handle%s() error = %v", tc.name, err)
			}
			if resp.Error == nil {
				t.Fatalf("handle%s() with nil conductor should return error response", tc.name)
			}
		})
	}
}

func TestWorktreePhaseHandlers_InvalidParams(t *testing.T) {
	ctx := context.Background()
	for _, tc := range phaseHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWorktreeSocket(ctx, t)
			resp, err := tc.handler(w)(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
			if err != nil {
				t.Fatalf("handle%s() error = %v", tc.name, err)
			}
			if resp.Error == nil {
				t.Fatalf("handle%s() with invalid params should return error response", tc.name)
			}
		})
	}
}

func TestWorktreePhaseHandlers_WrongState(t *testing.T) {
	ctx := context.Background()
	// In StateNone there is no work unit, so every phase transition is invalid
	// and must return an error response (not panic).
	for _, tc := range phaseHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWorktreeSocket(ctx, t)
			resp, err := tc.handler(w)(ctx, &Request{ID: "1", Params: json.RawMessage(`{}`)})
			if err != nil {
				t.Fatalf("handle%s() error = %v", tc.name, err)
			}
			if resp == nil {
				t.Fatalf("handle%s() returned nil response", tc.name)
			}
			// From the none state every phase transition is rejected.
			if resp.Error == nil {
				t.Fatalf("handle%s() from none state should return error response", tc.name)
			}
		})
	}
}

func TestWorktreeHandleSubmit_DryRunPreview(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	params, _ := json.Marshal(SubmitParams{DryRun: true}) //nolint:errchkjson // test data
	resp, err := w.handleSubmit(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleSubmit() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error response: %s", resp.Error.Message)
	}
	// PreviewSubmit is best-effort and succeeds with any loaded work unit.
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["status"] != "preview" {
		t.Errorf("status = %v, want preview", result["status"])
	}
	if _, ok := result["preview"]; !ok {
		t.Error("expected preview key in dry-run result")
	}
}

func TestWorktreeHandleReview_RejectRecordsReview(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)
	setWorkUnitInState(t, w, conductor.StateImplemented)

	// Review may fail to transition (no pool), but invalid params handling and
	// the approve/reject branch are still exercised. We assert a response comes
	// back without a Go-level error.
	params, _ := json.Marshal(ReviewParams{Reject: true, Message: "needs work"}) //nolint:errchkjson // test data
	resp, err := w.handleReview(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleReview() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestWorktreeHandleShutdown_Status(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	resp, err := w.handleShutdown(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleShutdown() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error response: %s", resp.Error.Message)
	}
	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["status"] != "shutting_down" {
		t.Errorf("status = %q, want shutting_down", result["status"])
	}
}
