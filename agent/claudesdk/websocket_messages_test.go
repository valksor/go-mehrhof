package claudesdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
)

// recvEvent reads one event with a timeout so a missing emission fails fast
// instead of hanging the test.
func recvEvent(t *testing.T, ch <-chan agent.Event) (agent.Event, bool) {
	t.Helper()
	select {
	case e, ok := <-ch:
		return e, ok
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")

		return agent.Event{}, false
	}
}

// expectNoEvent asserts the connection emits nothing for a short window.
func expectNoEvent(t *testing.T, ch <-chan agent.Event) {
	t.Helper()
	select {
	case e := <-ch:
		t.Fatalf("expected no event, got %+v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

// newTestConn builds a WebSocketConnection without spawning claude. The
// outgoing channel and a non-cancelled lifecycle context are wired so the
// send-side helpers (trySendOutgoing) behave as they do post-Connect.
func newTestConn(t *testing.T) *WebSocketConnection {
	t.Helper()
	cfg := DefaultConfig()
	// No permission handler by default so control_request emits an
	// EventPermission instead of auto-answering. Individual tests override.
	cfg.PermissionHandler = nil

	return NewWebSocketConnection(cfg)
}

// TestHandleIncomingMessage drives every branch of the incoming-message
// decoder by feeding it the message shapes the Claude CLI emits over the
// SDK WebSocket and asserting the translated agent.Event. This is the bulk
// of the adapter's protocol logic and needs no subprocess or socket.
func TestHandleIncomingMessage(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantNone    bool
		wantType    agent.EventType
		wantContent string
		check       func(t *testing.T, e agent.Event)
	}{
		{
			name:        "stream_event nested delta text",
			raw:         `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"nested chunk"}}}`,
			wantType:    agent.EventStream,
			wantContent: "nested chunk",
		},
		{
			name:        "stream_event legacy top-level content",
			raw:         `{"type":"stream_event","content":"legacy content"}`,
			wantType:    agent.EventStream,
			wantContent: "legacy content",
		},
		{
			name:        "stream_event legacy delta fallback",
			raw:         `{"type":"stream_event","delta":"delta fallback"}`,
			wantType:    agent.EventStream,
			wantContent: "delta fallback",
		},
		{
			name:     "stream_event empty emits nothing",
			raw:      `{"type":"stream_event"}`,
			wantNone: true,
		},
		{
			name:        "assistant string content",
			raw:         `{"type":"assistant","message":{"role":"assistant","content":"full reply"}}`,
			wantType:    agent.EventAssistant,
			wantContent: "full reply",
		},
		{
			name:        "assistant array content blocks",
			raw:         `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}}`,
			wantType:    agent.EventAssistant,
			wantContent: "a\nb",
		},
		{
			name:     "assistant empty content emits nothing",
			raw:      `{"type":"assistant","message":{"role":"assistant","content":""}}`,
			wantNone: true,
		},
		{
			name:     "assistant nil message emits nothing",
			raw:      `{"type":"assistant"}`,
			wantNone: true,
		},
		{
			name:     "result success",
			raw:      `{"type":"result","subtype":"success"}`,
			wantType: agent.EventComplete,
		},
		{
			name:     "result error via is_error",
			raw:      `{"type":"result","subtype":"success","is_error":true,"error":"boom"}`,
			wantType: agent.EventError,
			check: func(t *testing.T, e agent.Event) {
				t.Helper()
				if e.Error != "boom" {
					t.Errorf("error = %q, want %q", e.Error, "boom")
				}
			},
		},
		{
			name:     "result non-success subtype is error",
			raw:      `{"type":"result","subtype":"max_turns","error":"too many"}`,
			wantType: agent.EventError,
			check: func(t *testing.T, e agent.Event) {
				t.Helper()
				if e.Error != "too many" {
					t.Errorf("error = %q, want %q", e.Error, "too many")
				}
			},
		},
		{
			name:        "tool_use",
			raw:         `{"type":"tool_use","id":"tu-1","name":"Read","input":{"path":"/x"}}`,
			wantType:    agent.EventToolUse,
			wantContent: "Read",
			check: func(t *testing.T, e agent.Event) {
				t.Helper()
				if e.Data["path"] != "/x" {
					t.Errorf("Data[path] = %v, want /x", e.Data["path"])
				}
			},
		},
		{
			name:        "tool_use no input",
			raw:         `{"type":"tool_use","id":"tu-2","name":"Bash"}`,
			wantType:    agent.EventToolUse,
			wantContent: "Bash",
		},
		{
			name:        "tool_result",
			raw:         `{"type":"tool_result","tool_use_id":"tu-1","result":"done"}`,
			wantType:    agent.EventToolResult,
			wantContent: "done",
		},
		{
			name:     "tool_progress",
			raw:      `{"type":"tool_progress","tool_use_id":"tu-1","tool_name":"Bash","elapsed_time_seconds":3.5}`,
			wantType: agent.EventToolProgress,
			check: func(t *testing.T, e agent.Event) {
				t.Helper()
				if e.Data["tool_name"] != "Bash" {
					t.Errorf("Data[tool_name] = %v, want Bash", e.Data["tool_name"])
				}
				if e.Data["elapsed_seconds"] != 3.5 {
					t.Errorf("Data[elapsed_seconds] = %v, want 3.5", e.Data["elapsed_seconds"])
				}
			},
		},
		{
			name:        "streamlined_text content",
			raw:         `{"type":"streamlined_text","content":"streamlined"}`,
			wantType:    agent.EventStream,
			wantContent: "streamlined",
		},
		{
			name:        "streamlined_text delta fallback",
			raw:         `{"type":"streamlined_text","delta":"sd"}`,
			wantType:    agent.EventStream,
			wantContent: "sd",
		},
		{
			name:     "streamlined_text empty emits nothing",
			raw:      `{"type":"streamlined_text"}`,
			wantNone: true,
		},
		{
			name:        "streamlined_tool_use_summary",
			raw:         `{"type":"streamlined_tool_use_summary","content":"Read 2 files"}`,
			wantType:    agent.EventToolUse,
			wantContent: "Read 2 files",
		},
		{
			name:     "prompt_suggestion",
			raw:      `{"type":"prompt_suggestion","suggestions":["next","then"]}`,
			wantType: agent.EventPromptSuggestion,
			check: func(t *testing.T, e agent.Event) {
				t.Helper()
				sug, ok := e.Data["suggestions"].([]string)
				if !ok || len(sug) != 2 || sug[0] != "next" {
					t.Errorf("Data[suggestions] = %v, want [next then]", e.Data["suggestions"])
				}
			},
		},
		{
			name:     "keep_alive emits nothing",
			raw:      `{"type":"keep_alive"}`,
			wantNone: true,
		},
		{
			name:     "unknown type emits nothing",
			raw:      `{"type":"something_new"}`,
			wantNone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg incomingMessage
			if err := json.Unmarshal([]byte(tt.raw), &msg); err != nil {
				t.Fatalf("unmarshal test input: %v", err)
			}

			w := newTestConn(t)
			w.handleIncomingMessage(msg)

			if tt.wantNone {
				expectNoEvent(t, w.events)

				return
			}

			e, ok := recvEvent(t, w.events)
			if !ok {
				t.Fatal("channel closed without event")
			}
			if e.Type != tt.wantType {
				t.Errorf("event type = %v, want %v", e.Type, tt.wantType)
			}
			if tt.wantContent != "" && e.Content != tt.wantContent {
				t.Errorf("event content = %q, want %q", e.Content, tt.wantContent)
			}
			if tt.check != nil {
				tt.check(t, e)
			}
		})
	}
}

// TestHandleIncomingMessage_SystemInit asserts the system/init message sets the
// session ID, marks the connection connected, closes sessionReady exactly once,
// and emits an EventInit carrying the session ID.
func TestHandleIncomingMessage_SystemInit(t *testing.T) {
	for _, typ := range []string{"system", "system/init"} {
		t.Run(typ, func(t *testing.T) {
			w := newTestConn(t)
			w.handleIncomingMessage(incomingMessage{Type: typ, SessionID: "sess-123"})

			if w.sessionID != "sess-123" {
				t.Errorf("sessionID = %q, want sess-123", w.sessionID)
			}
			if !w.connected.Load() {
				t.Error("connected should be true after system init")
			}
			select {
			case <-w.sessionReady:
			default:
				t.Error("sessionReady should be closed after system init")
			}

			e, ok := recvEvent(t, w.events)
			if !ok {
				t.Fatal("expected EventInit")
			}
			if e.Type != agent.EventInit {
				t.Errorf("event type = %v, want EventInit", e.Type)
			}
			if e.Content != "Session initialized: sess-123" {
				t.Errorf("content = %q", e.Content)
			}

			// A second init with a different ID is ignored (session already set).
			w.handleIncomingMessage(incomingMessage{Type: "system", SessionID: "other"})
			if w.sessionID != "sess-123" {
				t.Errorf("sessionID changed to %q on second init", w.sessionID)
			}
			expectNoEvent(t, w.events)
		})
	}
}

// TestHandleIncomingMessage_SystemInitNoSessionID verifies a system message
// without a session ID is a no-op (does not mark connected or emit an event).
func TestHandleIncomingMessage_SystemInitNoSessionID(t *testing.T) {
	w := newTestConn(t)
	w.handleIncomingMessage(incomingMessage{Type: "system"})
	if w.connected.Load() {
		t.Error("connected should remain false without session_id")
	}
	expectNoEvent(t, w.events)
}

// TestHandleIncomingMessage_ToolUseSubagent verifies a Task tool_use is routed
// through the subagent tracker, which emits a subagent-started event in
// addition to the tool_use event.
func TestHandleIncomingMessage_ToolUseSubagent(t *testing.T) {
	w := newTestConn(t)
	raw := `{"type":"tool_use","id":"task-1","name":"Task","input":{"subagent_type":"reviewer","description":"review code"}}`
	var msg incomingMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	w.handleIncomingMessage(msg)

	var sawSubagent, sawToolUse bool
	deadline := time.After(2 * time.Second)
	for !sawSubagent || !sawToolUse {
		select {
		case e := <-w.events:
			switch e.Type {
			case agent.EventSubagent:
				sawSubagent = true
				if e.Subagent == nil || e.Subagent.Type != "reviewer" {
					t.Errorf("subagent event missing reviewer type: %+v", e.Subagent)
				}
			case agent.EventToolUse:
				sawToolUse = true
				if e.Content != "Task" {
					t.Errorf("tool_use content = %q, want Task", e.Content)
				}
			case agent.EventStream, agent.EventAssistant, agent.EventToolResult,
				agent.EventPermission, agent.EventComplete, agent.EventError,
				agent.EventInit, agent.EventKeepAlive, agent.EventProgress,
				agent.EventToolProgress, agent.EventInterrupted, agent.EventPromptSuggestion:
				// Not relevant to this assertion.
			}
		case <-deadline:
			t.Fatalf("timed out; sawSubagent=%v sawToolUse=%v", sawSubagent, sawToolUse)
		}
	}
}

// TestHandleIncomingMessage_ControlRequestAutoApprove verifies that when a
// permission handler is configured, a control_request is auto-answered: the
// pending request is recorded then cleared by HandlePermission, and the
// matching control_response is enqueued on the outgoing channel.
func TestHandleIncomingMessage_ControlRequestAutoApprove(t *testing.T) {
	cfg := DefaultConfig()
	var gotReq agent.PermissionRequest
	cfg.PermissionHandler = func(req agent.PermissionRequest) bool {
		gotReq = req

		return true
	}
	w := NewWebSocketConnection(cfg)
	// trySendOutgoing's control_response branch selects on lifecycleCtx.Done();
	// give it a live context so the auto-answer can enqueue without panicking.
	w.lifecycleCtx, w.lifecycleCancel = context.WithCancel(context.Background())
	t.Cleanup(w.lifecycleCancel)

	raw := `{"type":"control_request","request_id":"req-9","request":{"subtype":"can_use_tool","tool_name":"Bash","tool_use_id":"tu-9","input":{"command":"ls"}}}`
	var msg incomingMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	w.handleIncomingMessage(msg)

	if gotReq.ID != "req-9" || gotReq.Tool != "Bash" || gotReq.Action != "can_use_tool" {
		t.Errorf("permission request = %+v, want id=req-9 tool=Bash action=can_use_tool", gotReq)
	}
	if gotReq.Input["command"] != "ls" {
		t.Errorf("request input command = %v, want ls", gotReq.Input["command"])
	}

	// HandlePermission deleted the pending entry after answering.
	w.pendingRequestsMu.Lock()
	_, stillPending := w.pendingRequests["req-9"]
	w.pendingRequestsMu.Unlock()
	if stillPending {
		t.Error("pending request should be cleared after auto-approve")
	}

	// The control_response (allow) was enqueued.
	select {
	case out := <-w.outgoing:
		if out.Type != "control_response" {
			t.Fatalf("outgoing type = %q, want control_response", out.Type)
		}
		if out.Response == nil || out.Response.RequestID != "req-9" {
			t.Fatalf("control_response payload = %+v", out.Response)
		}
		if out.Response.Response == nil || out.Response.Response.Behavior != "allow" {
			t.Errorf("inner behavior = %+v, want allow", out.Response.Response)
		}
		if out.Response.Response.ToolUseID != "tu-9" {
			t.Errorf("inner toolUseID = %q, want tu-9", out.Response.Response.ToolUseID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no control_response enqueued")
	}
}

// TestHandleIncomingMessage_ControlRequestNoHandler verifies that without a
// permission handler the request surfaces to the caller as an EventPermission
// and the pending request remains recorded for an external HandlePermission.
func TestHandleIncomingMessage_ControlRequestNoHandler(t *testing.T) {
	w := newTestConn(t) // no handler
	raw := `{"type":"control_request","request_id":"req-x","request":{"subtype":"can_use_tool","tool_name":"Write","tool_use_id":"tu-x","input":{"path":"/f"}}}`
	var msg incomingMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	w.handleIncomingMessage(msg)

	e, ok := recvEvent(t, w.events)
	if !ok {
		t.Fatal("expected EventPermission")
	}
	if e.Type != agent.EventPermission {
		t.Fatalf("event type = %v, want EventPermission", e.Type)
	}
	if e.PermissionRequest == nil || e.PermissionRequest.ID != "req-x" {
		t.Fatalf("permission request = %+v", e.PermissionRequest)
	}

	w.pendingRequestsMu.Lock()
	pending, ok := w.pendingRequests["req-x"]
	w.pendingRequestsMu.Unlock()
	if !ok {
		t.Fatal("pending request should remain recorded for external handling")
	}
	if pending.ToolUseID != "tu-x" {
		t.Errorf("pending ToolUseID = %q, want tu-x", pending.ToolUseID)
	}
}

// TestHandleIncomingMessage_ControlRequestMissingFields verifies a malformed
// control_request (no request_id or no nested request) is ignored.
func TestHandleIncomingMessage_ControlRequestMissingFields(t *testing.T) {
	w := newTestConn(t)
	w.handleIncomingMessage(incomingMessage{Type: "control_request", RequestID: "req-1"}) // no Request
	expectNoEvent(t, w.events)
	w.pendingRequestsMu.Lock()
	n := len(w.pendingRequests)
	w.pendingRequestsMu.Unlock()
	if n != 0 {
		t.Errorf("pendingRequests = %d, want 0", n)
	}
}
