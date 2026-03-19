package socket

import (
	"context"
	"log/slog"
	"net"
	"testing"
)

func TestRPCContextRoundTrip(t *testing.T) {
	t.Parallel()

	rc := &RPCContext{
		TraceID:    "trace-123",
		TaskID:     "task-456",
		WorktreeID: "wt-789",
		Phase:      "implementing",
		AgentID:    "claude-1",
		ConnID:     "conn-abc",
		Method:     "status.get",
		RemoteAddr: "/tmp/test.sock",
	}

	ctx := WithRPCContext(context.Background(), rc)
	got := RPCContextFromCtx(ctx)

	if got == nil {
		t.Fatal("expected non-nil RPCContext")
	}
	if got != rc {
		t.Fatal("expected same RPCContext pointer")
	}
	if got.TraceID != "trace-123" {
		t.Errorf("TraceID = %q, want %q", got.TraceID, "trace-123")
	}
	if got.TaskID != "task-456" {
		t.Errorf("TaskID = %q, want %q", got.TaskID, "task-456")
	}
	if got.WorktreeID != "wt-789" {
		t.Errorf("WorktreeID = %q, want %q", got.WorktreeID, "wt-789")
	}
	if got.Phase != "implementing" {
		t.Errorf("Phase = %q, want %q", got.Phase, "implementing")
	}
	if got.AgentID != "claude-1" {
		t.Errorf("AgentID = %q, want %q", got.AgentID, "claude-1")
	}
	if got.ConnID != "conn-abc" {
		t.Errorf("ConnID = %q, want %q", got.ConnID, "conn-abc")
	}
	if got.Method != "status.get" {
		t.Errorf("Method = %q, want %q", got.Method, "status.get")
	}
	if got.RemoteAddr != "/tmp/test.sock" {
		t.Errorf("RemoteAddr = %q, want %q", got.RemoteAddr, "/tmp/test.sock")
	}
}

func TestRPCContextFromCtxReturnsNilForEmptyContext(t *testing.T) {
	t.Parallel()

	got := RPCContextFromCtx(context.Background())
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestConnRoundTrip(t *testing.T) {
	t.Parallel()

	// Create a pipe to get a real net.Conn
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	ctx := WithConn(context.Background(), client)
	got := ConnFromCtx(ctx)

	if got == nil {
		t.Fatal("expected non-nil Conn")
	}
	if got != client {
		t.Fatal("expected same Conn pointer")
	}
}

func TestConnFromCtxReturnsNilForEmptyContext(t *testing.T) {
	t.Parallel()

	got := ConnFromCtx(context.Background())
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestSlogAttrsAllFields(t *testing.T) {
	t.Parallel()

	rc := &RPCContext{
		TraceID:    "trace-123",
		TaskID:     "task-456",
		WorktreeID: "wt-789",
		Phase:      "planning",
		AgentID:    "claude-1",
		ConnID:     "conn-abc",
		Method:     "task.plan",
		RemoteAddr: "/tmp/test.sock",
	}

	attrs := rc.SlogAttrs()

	expected := map[string]string{
		"trace_id":    "trace-123",
		"method":      "task.plan",
		"task_id":     "task-456",
		"worktree_id": "wt-789",
		"phase":       "planning",
		"agent_id":    "claude-1",
		"conn_id":     "conn-abc",
		"remote_addr": "/tmp/test.sock",
	}

	if len(attrs) != len(expected) {
		t.Fatalf("got %d attrs, want %d", len(attrs), len(expected))
	}

	for _, attr := range attrs {
		want, ok := expected[attr.Key]
		if !ok {
			t.Errorf("unexpected attr key %q", attr.Key)
			continue
		}
		if attr.Value.String() != want {
			t.Errorf("attr %q = %q, want %q", attr.Key, attr.Value.String(), want)
		}
	}
}

func TestSlogAttrsOmitsEmptyFields(t *testing.T) {
	t.Parallel()

	rc := &RPCContext{
		TraceID: "trace-only",
		Method:  "status.get",
	}

	attrs := rc.SlogAttrs()

	if len(attrs) != 2 {
		t.Fatalf("got %d attrs, want 2; attrs: %v", len(attrs), attrsToStrings(attrs))
	}

	keys := make(map[string]bool)
	for _, attr := range attrs {
		keys[attr.Key] = true
	}

	if !keys["trace_id"] {
		t.Error("expected trace_id attr")
	}
	if !keys["method"] {
		t.Error("expected method attr")
	}
}

func TestSlogAttrsEmptyContext(t *testing.T) {
	t.Parallel()

	rc := &RPCContext{}
	attrs := rc.SlogAttrs()

	if len(attrs) != 0 {
		t.Errorf("got %d attrs for empty RPCContext, want 0; attrs: %v", len(attrs), attrsToStrings(attrs))
	}
}

// attrsToStrings converts slog attrs to strings for test diagnostics.
func attrsToStrings(attrs []slog.Attr) []string {
	out := make([]string, len(attrs))
	for i, a := range attrs {
		out[i] = a.Key + "=" + a.Value.String()
	}
	return out
}
