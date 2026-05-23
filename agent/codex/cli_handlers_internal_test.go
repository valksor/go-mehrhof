package codex

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
)

// drainEvent reads one event from c.events with a timeout so a missing emission
// fails fast instead of hanging.
func drainEvent(t *testing.T, c *CLIConnection) agent.Event {
	t.Helper()
	select {
	case e := <-c.events:
		return e
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")

		return agent.Event{}
	}
}

// expectNoEvent asserts no event lands on c.events within a short window.
func expectNoEvent(t *testing.T, c *CLIConnection) {
	t.Helper()
	select {
	case e := <-c.events:
		t.Fatalf("expected no event, got %+v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

// newTestConn builds a CLIConnection wired to an in-memory transport whose
// writes land in the returned buffer. No subprocess is involved — the handlers
// under test are pure translation logic over the JSON-RPC frames.
func newTestConn(cfg Config) (*CLIConnection, *bytes.Buffer) {
	c := NewCLIConnection(cfg)
	out := &bytes.Buffer{}
	c.transport = NewJsonRpcTransport(bytes.NewReader(nil), out)

	return c, out
}

// TestCLIHandleNotification drives every notification branch and asserts the
// translated agent.Event (or absence of one) plus turn-state side effects.
func TestCLIHandleNotification(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		params      string
		wantNone    bool
		wantType    agent.EventType
		wantContent string
		wantError   string
		wantTurnOff bool
	}{
		{
			name:        "agentMessage delta emits stream",
			method:      "item/agentMessage/delta",
			params:      `{"text":"hello world"}`,
			wantType:    agent.EventStream,
			wantContent: "hello world",
		},
		{
			name:     "agentMessage empty delta emits nothing",
			method:   "item/agentMessage/delta",
			params:   `{"text":""}`,
			wantNone: true,
		},
		{
			name:        "item started commandExecution -> Bash tool use",
			method:      "item/started",
			params:      `{"itemId":"i1","type":"commandExecution"}`,
			wantType:    agent.EventToolUse,
			wantContent: "Bash",
		},
		{
			name:        "item started fileChange -> Edit tool use",
			method:      "item/started",
			params:      `{"itemId":"i2","type":"fileChange"}`,
			wantType:    agent.EventToolUse,
			wantContent: "Edit",
		},
		{
			name:     "item started unknown type emits nothing",
			method:   "item/started",
			params:   `{"itemId":"i3","type":"mysteryThing"}`,
			wantNone: true,
		},
		{
			name:        "item completed emits tool result",
			method:      "item/completed",
			params:      `{"itemId":"i4","type":"commandExecution"}`,
			wantType:    agent.EventToolResult,
			wantContent: "commandExecution completed",
		},
		{
			name:        "turn completed emits complete and clears turn",
			method:      "turn/completed",
			params:      `{}`,
			wantType:    agent.EventComplete,
			wantTurnOff: true,
		},
		{
			name:        "turn failed emits error with message",
			method:      "turn/failed",
			params:      `{"error":"model exploded"}`,
			wantType:    agent.EventError,
			wantError:   "model exploded",
			wantTurnOff: true,
		},
		{
			name:     "unknown method emits nothing",
			method:   "totally/unknown",
			params:   `{}`,
			wantNone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestConn(DefaultConfig())
			c.turnActive.Store(true)

			c.handleNotification(tt.method, json.RawMessage(tt.params))

			if tt.wantNone {
				expectNoEvent(t, c)

				return
			}

			e := drainEvent(t, c)
			if e.Type != tt.wantType {
				t.Errorf("event type = %q, want %q", e.Type, tt.wantType)
			}
			if tt.wantContent != "" && e.Content != tt.wantContent {
				t.Errorf("event content = %q, want %q", e.Content, tt.wantContent)
			}
			if tt.wantError != "" && e.Error != tt.wantError {
				t.Errorf("event error = %q, want %q", e.Error, tt.wantError)
			}
			if tt.wantTurnOff && c.turnActive.Load() {
				t.Error("turnActive should be cleared")
			}
		})
	}
}

// TestCLIHandleRequest_UnknownAutoApproves asserts an unrecognized server
// request is auto-approved with an accept decision written to the transport.
func TestCLIHandleRequest_UnknownAutoApproves(t *testing.T) {
	c, out := newTestConn(DefaultConfig())

	c.handleRequest("item/somethingNew/requestApproval", 7, json.RawMessage(`{}`))

	var resp struct {
		ID     int64          `json:"id"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != 7 {
		t.Errorf("response id = %d, want 7", resp.ID)
	}
	if resp.Result[keyDecision] != decisionAccept {
		t.Errorf("decision = %v, want %q", resp.Result[keyDecision], decisionAccept)
	}
}

// TestCLIHandleMcpApproval asserts MCP tool calls are auto-approved.
func TestCLIHandleMcpApproval(t *testing.T) {
	c, out := newTestConn(DefaultConfig())

	c.handleRequest("item/mcpToolCall/requestApproval", 99, json.RawMessage(`{}`))

	if !bytes.Contains(out.Bytes(), []byte(decisionAccept)) {
		t.Errorf("expected accept decision, got %s", out.String())
	}
}

// TestCLIHandleCommandApproval_HandlerApproves wires a permission handler that
// approves and verifies the request reaches it and an accept is written back.
func TestCLIHandleCommandApproval_HandlerApproves(t *testing.T) {
	var gotReq agent.PermissionRequest
	cfg := DefaultConfig()
	cfg.PermissionHandler = func(req agent.PermissionRequest) bool {
		gotReq = req

		return true
	}
	c, out := newTestConn(cfg)

	c.handleRequest("item/commandExecution/requestApproval", 11,
		json.RawMessage(`{"itemId":"x","command":["rm","-rf","/"]}`))

	if gotReq.Tool != "Bash" {
		t.Errorf("permission tool = %q, want Bash", gotReq.Tool)
	}
	if gotReq.Input["command"] != "rm" {
		t.Errorf("command = %v, want rm", gotReq.Input["command"])
	}
	if !bytes.Contains(out.Bytes(), []byte(decisionAccept)) {
		t.Errorf("expected accept written, got %s", out.String())
	}
}

// TestCLIHandleCommandApproval_HandlerRejects verifies a rejecting handler
// produces a reject decision.
func TestCLIHandleCommandApproval_HandlerRejects(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PermissionHandler = func(_ agent.PermissionRequest) bool { return false }
	c, out := newTestConn(cfg)

	c.handleRequest("item/commandExecution/requestApproval", 12,
		json.RawMessage(`{"itemId":"x","command":["ls"]}`))

	if !bytes.Contains(out.Bytes(), []byte(decisionReject)) {
		t.Errorf("expected reject written, got %s", out.String())
	}
}

// TestCLIHandleCommandApproval_NoHandlerEmitsPermissionEvent verifies that with
// no handler the request is surfaced as an EventPermission for external code.
func TestCLIHandleCommandApproval_NoHandlerEmitsPermissionEvent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PermissionHandler = nil
	c, _ := newTestConn(cfg)

	c.handleRequest("item/commandExecution/requestApproval", 13,
		json.RawMessage(`{"itemId":"x","command":["echo","hi"]}`))

	e := drainEvent(t, c)
	if e.Type != agent.EventPermission {
		t.Fatalf("event type = %q, want permission", e.Type)
	}
	if e.PermissionRequest == nil || e.PermissionRequest.Tool != "Bash" {
		t.Errorf("permission request = %+v, want Bash tool", e.PermissionRequest)
	}
	// A pending approval must be tracked so a later HandlePermission can resolve it.
	c.pendingApprovalsMu.Lock()
	n := len(c.pendingApprovals)
	c.pendingApprovalsMu.Unlock()
	if n != 1 {
		t.Errorf("pendingApprovals = %d, want 1", n)
	}
}

// TestCLIHandleCommandApproval_Malformed rejects unparseable params.
func TestCLIHandleCommandApproval_Malformed(t *testing.T) {
	c, out := newTestConn(DefaultConfig())

	c.handleRequest("item/commandExecution/requestApproval", 14,
		json.RawMessage(`{"command": "not-an-array"}`))

	if !bytes.Contains(out.Bytes(), []byte(decisionReject)) {
		t.Errorf("malformed command approval should be rejected, got %s", out.String())
	}
}

// TestCLIHandleFileChangeApproval_HandlerApproves checks file-change approval
// surfaces the changed paths to the handler and writes an accept.
func TestCLIHandleFileChangeApproval_HandlerApproves(t *testing.T) {
	var gotReq agent.PermissionRequest
	cfg := DefaultConfig()
	cfg.PermissionHandler = func(req agent.PermissionRequest) bool {
		gotReq = req

		return true
	}
	c, out := newTestConn(cfg)

	c.handleRequest("item/fileChange/requestApproval", 21,
		json.RawMessage(`{"itemId":"y","changes":[{"path":"/a.go","kind":"modify"},{"path":"/b.go","kind":"add"}]}`))

	if gotReq.Tool != "Edit" {
		t.Errorf("tool = %q, want Edit", gotReq.Tool)
	}
	paths, ok := gotReq.Input["paths"].([]string)
	if !ok || len(paths) != 2 || paths[0] != "/a.go" || paths[1] != "/b.go" {
		t.Errorf("paths = %v, want [/a.go /b.go]", gotReq.Input["paths"])
	}
	if !bytes.Contains(out.Bytes(), []byte(decisionAccept)) {
		t.Errorf("expected accept written, got %s", out.String())
	}
}

// TestCLIHandleFileChangeApproval_Malformed rejects unparseable file changes.
func TestCLIHandleFileChangeApproval_Malformed(t *testing.T) {
	c, out := newTestConn(DefaultConfig())

	c.handleRequest("item/fileChange/requestApproval", 22,
		json.RawMessage(`{"changes": 123}`))

	if !bytes.Contains(out.Bytes(), []byte(decisionReject)) {
		t.Errorf("malformed file change approval should be rejected, got %s", out.String())
	}
}

// TestCLIHandleFileChangeApproval_NoHandler surfaces a permission event when no
// handler is configured.
func TestCLIHandleFileChangeApproval_NoHandler(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PermissionHandler = nil
	c, _ := newTestConn(cfg)

	c.handleRequest("item/fileChange/requestApproval", 23,
		json.RawMessage(`{"changes":[{"path":"/x","kind":"add"}]}`))

	e := drainEvent(t, c)
	if e.Type != agent.EventPermission || e.PermissionRequest == nil {
		t.Fatalf("expected permission event, got %+v", e)
	}
}

// TestCLIHandlePermission_RoundTrip verifies a tracked approval is resolved and
// the matching decision is written, while an unknown ID errors.
func TestCLIHandlePermission_RoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PermissionHandler = nil
	c, out := newTestConn(cfg)

	// Register a pending approval via the no-handler path.
	c.handleRequest("item/commandExecution/requestApproval", 31,
		json.RawMessage(`{"itemId":"z","command":["go","test"]}`))
	e := drainEvent(t, c)
	reqID := e.PermissionRequest.ID

	out.Reset()
	if err := c.HandlePermission(reqID, false); err != nil {
		t.Fatalf("HandlePermission() = %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte(decisionReject)) {
		t.Errorf("expected reject decision, got %s", out.String())
	}

	// Resolving the same ID twice fails: it's no longer pending.
	if err := c.HandlePermission(reqID, true); err == nil {
		t.Error("second HandlePermission for resolved ID should error")
	}
}

// TestCLIHandlePermission_UnknownID errors for an ID that was never tracked.
func TestCLIHandlePermission_UnknownID(t *testing.T) {
	c, _ := newTestConn(DefaultConfig())
	if err := c.HandlePermission("ghost", true); err == nil {
		t.Error("HandlePermission for unknown ID should error")
	}
}

// TestCLIEmitEvent_DropsWhenFull asserts emitEvent never blocks: once the buffer
// is saturated, further events are dropped rather than deadlocking.
func TestCLIEmitEvent_DropsWhenFull(t *testing.T) {
	c, _ := newTestConn(DefaultConfig())
	// events buffer is 100; overfill it and ensure no goroutine blocks.
	for range 150 {
		c.emitEvent(agent.Event{Type: agent.EventStream, Content: "x"})
	}
	// Channel should hold exactly its capacity; drains succeed without panic.
	got := 0
	for {
		select {
		case <-c.events:
			got++

			continue
		default:
		}

		break
	}
	if got != cap(c.events) {
		t.Errorf("buffered events = %d, want %d (cap)", got, cap(c.events))
	}
}
