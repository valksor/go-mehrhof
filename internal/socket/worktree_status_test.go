package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
)

func TestHandleStatus_NoWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleStatus(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStatus() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleStatus() returned error: %s", resp.Error.Message)
	}

	var result StatusResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.State != StateNone {
		t.Errorf("State = %q, want %q", result.State, StateNone)
	}
	if result.Task != nil {
		t.Error("Task should be nil when no work unit loaded")
	}
}

func TestHandleStatus_WithWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	resp, err := w.handleStatus(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStatus() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleStatus() returned error: %s", resp.Error.Message)
	}

	var result StatusResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.State != TaskState(conductor.StateLoaded) {
		t.Errorf("State = %q, want %q", result.State, conductor.StateLoaded)
	}
	if result.Task == nil {
		t.Fatal("Task should not be nil when work unit loaded")
	}
	if result.Task.ID != "test-task-id" {
		t.Errorf("Task.ID = %q, want test-task-id", result.Task.ID)
	}
}

func TestHandleStatus_ImplementingState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateImplementing)

	resp, err := w.handleStatus(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStatus() error = %v", err)
	}

	var result StatusResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.State != TaskState(conductor.StateImplementing) {
		t.Errorf("State = %q, want %q", result.State, conductor.StateImplementing)
	}
}

func TestHandleRecap_NoWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleRecap(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRecap() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleRecap() returned error: %s", resp.Error.Message)
	}

	var result RecapResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.State != StateNone {
		t.Errorf("State = %q, want %q", result.State, StateNone)
	}
}

func TestHandleRecap_WithWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StatePlanned)

	resp, err := w.handleRecap(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRecap() error = %v", err)
	}

	var result RecapResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.State != TaskState(conductor.StatePlanned) {
		t.Errorf("State = %q, want %q", result.State, conductor.StatePlanned)
	}
}

func TestHandleShowSpec_NoWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleShowSpec(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleShowSpec() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleShowSpec() returned error: %s", resp.Error.Message)
	}

	var result ShowSpecResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Specifications == nil {
		t.Error("Specifications should be non-nil empty slice")
	}
}

func TestHandleShowSpec_WithWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	resp, err := w.handleShowSpec(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleShowSpec() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleShowSpec() returned error: %s", resp.Error.Message)
	}
}

func TestHandleStop_NoConductor(t *testing.T) {
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}
	ctx := context.Background()

	resp, err := w.handleStop(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStop() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleStop() should return error when no conductor")
	}
}

func TestHandleStop_FromNoneState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	// State is "none" by default

	resp, err := w.handleStop(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStop() error = %v", err)
	}
	// Should return error because nothing to stop
	if resp.Error == nil {
		t.Fatal("handleStop() from none state should return error")
	}
}
