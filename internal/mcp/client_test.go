package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/valksor/kvelmo/internal/socket"
)

// fakeRPC implements the WorktreeRPC interface for testing.
type fakeRPC struct {
	resp *socket.Response
	err  error
}

func (f *fakeRPC) Call(_ context.Context, _ string, _ any) (*socket.Response, error) {
	return f.resp, f.err
}

func (f *fakeRPC) Close() error { return nil }

func TestNewClient(t *testing.T) {
	rpc := &fakeRPC{}
	c := NewClient(rpc, "t1", "/worktree", "/sock")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.TaskID != "t1" || c.Worktree != "/worktree" || c.HostSocket != "/sock" {
		t.Errorf("client fields = %+v", c)
	}
}

func TestClient_Close_Nil(t *testing.T) {
	c := &Client{}
	if err := c.Close(); err != nil {
		t.Errorf("Close on nil rpc: %v", err)
	}
}

func TestClient_Close(t *testing.T) {
	c := NewClient(&fakeRPC{}, "t1", "/w", "/s")
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestClient_CallInto_NotConnected(t *testing.T) {
	c := &Client{} // rpc is nil
	out := map[string]any{}
	result, ok := c.CallInto(context.Background(), "test", nil, &out)
	if ok {
		t.Error("expected ok=false when rpc is nil")
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

func TestClient_CallInto_RPCError(t *testing.T) {
	c := NewClient(&fakeRPC{err: errors.New("transport down")}, "t", "w", "s")

	out := map[string]any{}
	result, ok := c.CallInto(context.Background(), "test", nil, &out)
	if ok {
		t.Error("expected ok=false on RPC error")
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

func TestClient_CallInto_Success(t *testing.T) {
	c := NewClient(&fakeRPC{resp: &socket.Response{Result: json.RawMessage(`{"key": "value"}`)}}, "t", "w", "s")

	var out map[string]any
	result, ok := c.CallInto(context.Background(), "test", nil, &out)
	if !ok {
		t.Errorf("expected ok=true, got result %+v", result)
	}
	if out["key"] != "value" {
		t.Errorf("decoded out = %v", out)
	}
}

func TestClient_CallInto_DecodeError(t *testing.T) {
	c := NewClient(&fakeRPC{resp: &socket.Response{Result: json.RawMessage(`not valid json`)}}, "t", "w", "s")

	out := map[string]any{}
	result, ok := c.CallInto(context.Background(), "test", nil, &out)
	if ok {
		t.Error("expected ok=false on decode failure")
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

func TestClient_CallInto_NilOut(t *testing.T) {
	// out=nil means "don't decode" — should succeed even with empty result.
	c := NewClient(&fakeRPC{resp: &socket.Response{Result: json.RawMessage(`{}`)}}, "t", "w", "s")

	_, ok := c.CallInto(context.Background(), "test", nil, nil)
	if !ok {
		t.Error("expected ok=true with nil out")
	}
}

func TestErrorResult(t *testing.T) {
	r := errorResult("boom")
	if !r.IsError {
		t.Error("IsError should be true")
	}
	if len(r.Content) != 1 || r.Content[0].Text != "boom" {
		t.Errorf("content = %+v", r.Content)
	}
}

func TestTextResult(t *testing.T) {
	r := textResult("hello")
	if r.IsError {
		t.Error("IsError should be false")
	}
	if r.Content[0].Text != "hello" {
		t.Errorf("text = %q", r.Content[0].Text)
	}
}

func TestJSONResult(t *testing.T) {
	r := jsonResult(map[string]any{"key": "value"})
	if r.IsError {
		t.Error("IsError should be false")
	}
	if r.Content[0].Text == "" {
		t.Error("text should be non-empty")
	}
}

func TestMissing(t *testing.T) {
	r, err := missing("file")
	if err != nil {
		t.Errorf("missing should not return Go error: %v", err)
	}
	if !r.IsError {
		t.Error("IsError should be true for missing arg")
	}
}

func TestArgString(t *testing.T) {
	args := map[string]any{
		"present": "value",
		"wrong":   123,
		"empty":   "",
	}
	if got := argString(args, "present"); got != "value" {
		t.Errorf("present = %q", got)
	}
	if got := argString(args, "wrong"); got != "" {
		t.Errorf("non-string should give empty, got %q", got)
	}
	if got := argString(args, "absent"); got != "" {
		t.Errorf("absent should give empty, got %q", got)
	}
}

func TestArgBool(t *testing.T) {
	args := map[string]any{
		"true_val":  true,
		"false_val": false,
		"wrong":     "yes",
	}
	if got := argBool(args, "true_val", false); !got {
		t.Error("true_val = false")
	}
	if got := argBool(args, "false_val", true); got {
		t.Error("false_val = true")
	}
	if got := argBool(args, "wrong", true); !got {
		t.Error("wrong type should return default")
	}
	if got := argBool(args, "absent", true); !got {
		t.Error("absent should return default")
	}
}
