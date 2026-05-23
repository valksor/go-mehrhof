package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/settings"
)

// newTestWorktreeSocket creates a WorktreeSocket with a conductor configured
// for testing. No git repository or pool is needed because the handler tests
// exercise conductor logic that does not perform git operations.
func newTestWorktreeSocket(ctx context.Context, t *testing.T) *WorktreeSocket {
	t.Helper()
	// ctx is threaded in from the calling test so request-scoped operations on the
	// returned socket use it. The provider/conductor constructors below are
	// synchronous and do no request-scoped I/O (test tokens are empty, so no HTTP
	// client is ever created), hence they take no context.
	_ = ctx
	//nolint:contextcheck // NewRegistry is a context-free constructor; no request-scoped I/O occurs
	providers := provider.NewRegistry(settings.DefaultSettings())
	//nolint:contextcheck // NewConductor is a context-free constructor; no request-scoped I/O occurs
	cond := conductor.NewConductor(conductor.ConductorConfig{
		Providers: providers,
	})
	w := &WorktreeSocket{
		server:    NewServer(""), // path unused when calling handlers directly
		conductor: cond,
		streams:   make(map[string]chan []byte),
	}

	return w
}

// setWorkUnitInState configures the socket's conductor with a work unit and
// forces the machine into the given state without going through the full start
// flow (which requires a git repository and provider fetch).
func setWorkUnitInState(t *testing.T, w *WorktreeSocket, state conductor.State) {
	t.Helper()
	wu := &conductor.WorkUnit{
		ID:             "test-task-id",
		Title:          "Test Task",
		Description:    "Test task description",
		Specifications: []string{"specification-1.md"},
		Source: &conductor.Source{
			Provider:  "empty",
			Reference: "Test task description",
			Content:   "Test task description",
		},
	}
	w.conductor.Machine().SetWorkUnit(wu)
	w.conductor.Machine().ForceState(state)
	// Also set on conductor directly so WorkUnit() returns it.
	w.conductor.ForceWorkUnit(wu)
}

// --- abandon handler tests ---

func TestHandleAbandonFromLoadedState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	if w.conductor.State() != conductor.StateLoaded {
		t.Fatalf("pre-condition: state = %s, want loaded", w.conductor.State())
	}

	req := &Request{ID: "1", Method: "abandon"}
	resp, err := w.handleAbandon(ctx, req)
	if err != nil {
		t.Fatalf("handleAbandon() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleAbandon() returned error response: %s", resp.Error.Message)
	}

	if w.conductor.State() != conductor.StateNone {
		t.Errorf("state after abandon = %s, want none", w.conductor.State())
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["status"] != "abandoned" {
		t.Errorf("result status = %q, want %q", result["status"], "abandoned")
	}
}

func TestHandleAbandonFromNoneState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	// State is already none, abandon should still succeed (it resets to none).
	req := &Request{ID: "1", Method: "abandon"}
	resp, err := w.handleAbandon(ctx, req)
	if err != nil {
		t.Fatalf("handleAbandon() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleAbandon() returned error response: %s", resp.Error.Message)
	}

	if w.conductor.State() != conductor.StateNone {
		t.Errorf("state after abandon = %s, want none", w.conductor.State())
	}
}

// --- delete handler tests ---

func TestHandleDeleteFromSubmittedState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)
	setWorkUnitInState(t, w, conductor.StateSubmitted)

	req := &Request{ID: "2", Method: "delete"}
	resp, err := w.handleDelete(ctx, req)
	if err != nil {
		t.Fatalf("handleDelete() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleDelete() returned error response: %s", resp.Error.Message)
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["status"] != "deleted" {
		t.Errorf("result status = %q, want %q", result["status"], "deleted")
	}
}

func TestHandleDeleteFromPlanningStateReturnsError(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)
	setWorkUnitInState(t, w, conductor.StatePlanning)

	req := &Request{ID: "3", Method: "delete"}
	resp, err := w.handleDelete(ctx, req)
	if err != nil {
		t.Fatalf("handleDelete() returned Go error (expected error response): %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleDelete() should return an error response when in planning state, got success")
	}
}

func TestHandleDeleteFromNoneState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	// None is a valid state for delete.
	req := &Request{ID: "4", Method: "delete"}
	resp, err := w.handleDelete(ctx, req)
	if err != nil {
		t.Fatalf("handleDelete() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleDelete() returned error response in none state: %s", resp.Error.Message)
	}
}

// --- update handler tests ---

func TestHandleUpdateNoChange(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)

	// Set up a work unit whose Description matches what the empty provider returns.
	// The empty provider's FetchTask returns Description = id (the reference).
	description := "Fix the login button"
	wu := &conductor.WorkUnit{
		ID:    "test-task-update",
		Title: "Fix the login button",
		// Description must equal the re-fetched description so changed=false.
		Description: description,
		Source: &conductor.Source{
			Provider:  "empty",
			Reference: description, // empty provider returns Description = reference
			Content:   description,
		},
	}
	w.conductor.Machine().SetWorkUnit(wu)
	w.conductor.ForceWorkUnit(wu)

	req := &Request{ID: "5", Method: "update"}
	resp, err := w.handleUpdate(ctx, req)
	if err != nil {
		t.Fatalf("handleUpdate() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleUpdate() returned error response: %s", resp.Error.Message)
	}

	var result UpdateResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Changed {
		t.Errorf("UpdateResult.Changed = true, want false (same content)")
	}
}

func TestHandleAbandonNoConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
		// conductor is nil
	}

	req := &Request{ID: "6", Method: "abandon"}
	resp, err := w.handleAbandon(ctx, req)
	if err != nil {
		t.Fatalf("handleAbandon() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleAbandon() should return an error response when conductor is nil")
	}
}

func TestHandleDeleteNoConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}

	req := &Request{ID: "7", Method: "delete"}
	resp, err := w.handleDelete(ctx, req)
	if err != nil {
		t.Fatalf("handleDelete() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleDelete() should return an error response when conductor is nil")
	}
}

func TestHandleUpdateNoConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}

	req := &Request{ID: "8", Method: "update"}
	resp, err := w.handleUpdate(ctx, req)
	if err != nil {
		t.Fatalf("handleUpdate() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleUpdate() should return an error response when conductor is nil")
	}
}
