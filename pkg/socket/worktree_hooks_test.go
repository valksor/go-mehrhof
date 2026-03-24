package socket

import (
	"context"
	"testing"

	"github.com/valksor/kvelmo/pkg/conductor"
)

func TestHandleHooksList(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	req := &Request{ID: "1", Method: "hooks.list"}
	resp, err := w.handleHooksList(ctx, req)
	if err != nil {
		t.Fatalf("handleHooksList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleHooksList() returned error: %s", resp.Error.Message)
	}
	if resp.Result == nil {
		t.Fatal("handleHooksList() returned nil result")
	}
}

func TestHandleHooksList_NoConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{
		server:  NewServer(""),
		streams: make(map[string]chan []byte),
	}

	req := &Request{ID: "2", Method: "hooks.list"}
	resp, err := w.handleHooksList(ctx, req)
	if err != nil {
		t.Fatalf("handleHooksList() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleHooksList() should return error when conductor is nil")
	}
}
