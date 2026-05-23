package codex

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/valksor/kvelmo/agent"
)

// drainWsEvent reads one event from w.events with a timeout.
func drainWsEvent(t *testing.T, w *WebSocketConnection) agent.Event {
	t.Helper()
	select {
	case e := <-w.events:
		return e
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ws event")

		return agent.Event{}
	}
}

func expectNoWsEvent(t *testing.T, w *WebSocketConnection) {
	t.Helper()
	select {
	case e := <-w.events:
		t.Fatalf("expected no event, got %+v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

// newWsConn builds a WebSocketConnection wired to a real wsTransport over an
// in-process WS pair, returning the connection and the server side so tests can
// observe frames the connection writes (approvals, interrupts).
func newWsConn(t *testing.T, cfg Config) (*WebSocketConnection, *websocket.Conn) {
	t.Helper()
	client, server := wsPair(t)
	w := NewWebSocketConnection(cfg)
	w.transport = newWsTransport(client)
	t.Cleanup(func() { _ = w.transport.Close() })

	return w, server
}

// TestWsHandleNotification covers every notification branch on the WebSocket
// connection, asserting the translated event and turn-state side effects. Note
// the WS variant emits the raw item type as the tool name for unknown types,
// unlike the CLI variant which drops them.
func TestWsHandleNotification(t *testing.T) {
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
			name:        "delta emits stream",
			method:      "item/agentMessage/delta",
			params:      `{"text":"chunk"}`,
			wantType:    agent.EventStream,
			wantContent: "chunk",
		},
		{
			name:     "empty delta emits nothing",
			method:   "item/agentMessage/delta",
			params:   `{"text":""}`,
			wantNone: true,
		},
		{
			name:        "started commandExecution -> Bash",
			method:      "item/started",
			params:      `{"itemId":"a","type":"commandExecution"}`,
			wantType:    agent.EventToolUse,
			wantContent: "Bash",
		},
		{
			name:        "started fileChange -> Edit",
			method:      "item/started",
			params:      `{"itemId":"b","type":"fileChange"}`,
			wantType:    agent.EventToolUse,
			wantContent: "Edit",
		},
		{
			name:        "started unknown type passes raw type through",
			method:      "item/started",
			params:      `{"itemId":"c","type":"reasoning"}`,
			wantType:    agent.EventToolUse,
			wantContent: "reasoning",
		},
		{
			name:        "completed emits tool result",
			method:      "item/completed",
			params:      `{"itemId":"d","type":"fileChange"}`,
			wantType:    agent.EventToolResult,
			wantContent: "fileChange completed",
		},
		{
			name:        "turn completed",
			method:      "turn/completed",
			params:      `{}`,
			wantType:    agent.EventComplete,
			wantTurnOff: true,
		},
		{
			name:        "turn failed",
			method:      "turn/failed",
			params:      `{"error":"kaboom"}`,
			wantType:    agent.EventError,
			wantError:   "kaboom",
			wantTurnOff: true,
		},
		{
			name:     "unknown method emits nothing",
			method:   "no/such/method",
			params:   `{}`,
			wantNone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWebSocketConnection(DefaultConfig())
			w.turnActive.Store(true)

			w.handleNotification(tt.method, json.RawMessage(tt.params))

			if tt.wantNone {
				expectNoWsEvent(t, w)

				return
			}

			e := drainWsEvent(t, w)
			if e.Type != tt.wantType {
				t.Errorf("type = %q, want %q", e.Type, tt.wantType)
			}
			if tt.wantContent != "" && e.Content != tt.wantContent {
				t.Errorf("content = %q, want %q", e.Content, tt.wantContent)
			}
			if tt.wantError != "" && e.Error != tt.wantError {
				t.Errorf("error = %q, want %q", e.Error, tt.wantError)
			}
			if tt.wantTurnOff && w.turnActive.Load() {
				t.Error("turnActive should be cleared")
			}
		})
	}
}

// TestWsHandleRequest_UnknownAutoApproves verifies an unknown server request is
// auto-approved over the wire.
func TestWsHandleRequest_UnknownAutoApproves(t *testing.T) {
	w, server := newWsConn(t, DefaultConfig())

	go w.handleRequest("item/whatever/requestApproval", 8, json.RawMessage(`{}`))

	frame := readFrame(t, context.Background(), server)
	if frameID(t, frame) != 8 {
		t.Errorf("id = %v, want 8", frame["id"])
	}
	result := frameResult(t, frame)
	if result[keyDecision] != decisionAccept {
		t.Errorf("decision = %v, want accept", result[keyDecision])
	}
}

// TestWsHandleMcpApproval auto-approves MCP tool calls over the wire.
func TestWsHandleMcpApproval(t *testing.T) {
	w, server := newWsConn(t, DefaultConfig())

	go w.handleRequest("item/mcpToolCall/requestApproval", 9, json.RawMessage(`{}`))

	frame := readFrame(t, context.Background(), server)
	result := frameResult(t, frame)
	if result[keyDecision] != decisionAccept {
		t.Errorf("decision = %v, want accept", result[keyDecision])
	}
}

// TestWsHandleCommandApproval_HandlerApproves drives command approval with an
// approving handler and asserts both the surfaced request and the accept frame.
func TestWsHandleCommandApproval_HandlerApproves(t *testing.T) {
	var gotReq agent.PermissionRequest
	cfg := DefaultConfig()
	cfg.PermissionHandler = func(req agent.PermissionRequest) bool {
		gotReq = req

		return true
	}
	w, server := newWsConn(t, cfg)

	go w.handleRequest("item/commandExecution/requestApproval", 41,
		json.RawMessage(`{"itemId":"x","command":["make","build"]}`))

	frame := readFrame(t, context.Background(), server)
	result := frameResult(t, frame)
	if result[keyDecision] != decisionAccept {
		t.Errorf("decision = %v, want accept", result[keyDecision])
	}
	if gotReq.Tool != "Bash" || gotReq.Input["command"] != "make" {
		t.Errorf("permission request = %+v, want Bash/make", gotReq)
	}
}

// TestWsHandleCommandApproval_Malformed rejects unparseable command params.
func TestWsHandleCommandApproval_Malformed(t *testing.T) {
	w, server := newWsConn(t, DefaultConfig())

	go w.handleRequest("item/commandExecution/requestApproval", 42,
		json.RawMessage(`{"command":"oops"}`))

	frame := readFrame(t, context.Background(), server)
	result := frameResult(t, frame)
	if result[keyDecision] != decisionReject {
		t.Errorf("decision = %v, want reject", result[keyDecision])
	}
}

// TestWsHandleCommandApproval_NoHandler surfaces a permission event.
func TestWsHandleCommandApproval_NoHandler(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PermissionHandler = nil
	w := NewWebSocketConnection(cfg)

	w.handleRequest("item/commandExecution/requestApproval", 43,
		json.RawMessage(`{"itemId":"x","command":["pwd"]}`))

	e := drainWsEvent(t, w)
	if e.Type != agent.EventPermission || e.PermissionRequest == nil {
		t.Fatalf("expected permission event, got %+v", e)
	}
	if e.PermissionRequest.Input["command"] != "pwd" {
		t.Errorf("command = %v, want pwd", e.PermissionRequest.Input["command"])
	}
}

// TestWsHandleFileChangeApproval_HandlerApproves drives file-change approval.
func TestWsHandleFileChangeApproval_HandlerApproves(t *testing.T) {
	var gotReq agent.PermissionRequest
	cfg := DefaultConfig()
	cfg.PermissionHandler = func(req agent.PermissionRequest) bool {
		gotReq = req

		return true
	}
	w, server := newWsConn(t, cfg)

	go w.handleRequest("item/fileChange/requestApproval", 51,
		json.RawMessage(`{"itemId":"y","changes":[{"path":"/main.go","kind":"modify"}]}`))

	frame := readFrame(t, context.Background(), server)
	result := frameResult(t, frame)
	if result[keyDecision] != decisionAccept {
		t.Errorf("decision = %v, want accept", result[keyDecision])
	}
	paths, ok := gotReq.Input["paths"].([]string)
	if !ok {
		t.Fatalf("permission input paths is not []string: %T", gotReq.Input["paths"])
	}
	if len(paths) != 1 || paths[0] != "/main.go" {
		t.Errorf("paths = %v, want [/main.go]", paths)
	}
}

// TestWsHandleFileChangeApproval_Malformed rejects bad params.
func TestWsHandleFileChangeApproval_Malformed(t *testing.T) {
	w, server := newWsConn(t, DefaultConfig())

	go w.handleRequest("item/fileChange/requestApproval", 52,
		json.RawMessage(`{"changes":"nope"}`))

	frame := readFrame(t, context.Background(), server)
	result := frameResult(t, frame)
	if result[keyDecision] != decisionReject {
		t.Errorf("decision = %v, want reject", result[keyDecision])
	}
}

// TestWsHandleFileChangeApproval_NoHandler surfaces a permission event.
func TestWsHandleFileChangeApproval_NoHandler(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PermissionHandler = nil
	w := NewWebSocketConnection(cfg)

	w.handleRequest("item/fileChange/requestApproval", 53,
		json.RawMessage(`{"changes":[{"path":"/z","kind":"add"}]}`))

	e := drainWsEvent(t, w)
	if e.Type != agent.EventPermission {
		t.Fatalf("expected permission event, got %+v", e)
	}
}

// TestWsHandlePermission_RoundTrip resolves a tracked approval over the wire and
// asserts the matching decision frame, plus that unknown IDs error.
func TestWsHandlePermission_RoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PermissionHandler = nil
	w, server := newWsConn(t, cfg)

	w.handleRequest("item/commandExecution/requestApproval", 61,
		json.RawMessage(`{"itemId":"z","command":["go","vet"]}`))
	e := drainWsEvent(t, w)
	reqID := e.PermissionRequest.ID

	go func() { _ = w.HandlePermission(reqID, false) }()
	frame := readFrame(t, context.Background(), server)
	result := frameResult(t, frame)
	if result[keyDecision] != decisionReject {
		t.Errorf("decision = %v, want reject", result[keyDecision])
	}

	if err := w.HandlePermission("unknown-id", true); err == nil {
		t.Error("HandlePermission for unknown id should error")
	}
}

// TestWsSendPrompt_NotConnected errors when not connected.
func TestWsSendPrompt_NotConnected(t *testing.T) {
	w := NewWebSocketConnection(DefaultConfig())
	if _, err := w.SendPrompt(context.Background(), "hi"); err == nil {
		t.Error("SendPrompt() without connection should error")
	}
}

// TestWsSendPrompt_NoThread errors when connected but no thread was started.
func TestWsSendPrompt_NoThread(t *testing.T) {
	w := NewWebSocketConnection(DefaultConfig())
	w.connected.Store(true)
	if _, err := w.SendPrompt(context.Background(), "hi"); err == nil {
		t.Error("SendPrompt() with empty threadID should error")
	}
}

// TestWsSendPrompt_TurnStartAndStream wires a fake server that answers
// turn/start and streams a delta + completion, then asserts the filtered
// channel surfaces them and closes on the terminal event.
func TestWsSendPrompt_TurnStartAndStream(t *testing.T) {
	w, server := newWsConn(t, DefaultConfig())
	w.connected.Store(true)
	w.threadID = "thread-1"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go w.transport.readLoop(ctx)
	w.transport.notificationHandler = w.handleNotification
	w.transport.requestHandler = w.handleRequest

	// Server: answer turn/start, then stream a delta and turn/completed.
	go func() {
		req := readFrame(t, ctx, server)
		id := frameID(t, req)
		_ = server.Write(ctx, websocket.MessageText,
			[]byte(`{"jsonrpc":"2.0","id":`+jsonFloat(id)+`,"result":{}}`))
		_ = server.Write(ctx, websocket.MessageText,
			[]byte(`{"jsonrpc":"2.0","method":"item/agentMessage/delta","params":{"text":"ws reply"}}`))
		_ = server.Write(ctx, websocket.MessageText,
			[]byte(`{"jsonrpc":"2.0","method":"turn/completed","params":{}}`))
	}()

	ch, err := w.SendPrompt(ctx, "go")
	if err != nil {
		t.Fatalf("SendPrompt() = %v", err)
	}

	var sawStream, sawComplete bool
	timeout := time.After(5 * time.Second)
	for !sawComplete {
		select {
		case e, ok := <-ch:
			if !ok {
				goto done
			}
			if e.Type == agent.EventStream && e.Content == "ws reply" {
				sawStream = true
			}
			if e.Type == agent.EventComplete {
				sawComplete = true
			}
		case <-timeout:
			t.Fatal("timed out waiting for ws stream events")
		}
	}
done:
	if !sawStream {
		t.Error("expected a stream event with 'ws reply'")
	}
	if !sawComplete {
		t.Error("expected a completion event")
	}
}

// TestWsInterrupt_States covers the early-return branches plus the active path.
func TestWsInterrupt_States(t *testing.T) {
	t.Run("not connected is a no-op", func(t *testing.T) {
		w := NewWebSocketConnection(DefaultConfig())
		if err := w.Interrupt(); err != nil {
			t.Errorf("Interrupt() = %v, want nil", err)
		}
	})

	t.Run("connected but no active turn is a no-op", func(t *testing.T) {
		w := NewWebSocketConnection(DefaultConfig())
		w.connected.Store(true)
		if err := w.Interrupt(); err != nil {
			t.Errorf("Interrupt() = %v, want nil", err)
		}
	})

	t.Run("active turn sends interrupt and emits interrupted event", func(t *testing.T) {
		w, server := newWsConn(t, DefaultConfig())
		w.connected.Store(true)
		w.threadID = "thread-1"
		w.turnActive.Store(true)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		go w.transport.readLoop(ctx)

		// Server answers the turn/interrupt call.
		go func() {
			req := readFrame(t, ctx, server)
			method, _ := req["method"].(string)
			if !strings.Contains(method, "interrupt") {
				t.Errorf("expected turn/interrupt, got %v", req["method"])
			}
			id := frameID(t, req)
			_ = server.Write(ctx, websocket.MessageText,
				[]byte(`{"jsonrpc":"2.0","id":`+jsonFloat(id)+`,"result":{}}`))
		}()

		if err := w.Interrupt(); err != nil {
			t.Errorf("Interrupt() = %v", err)
		}
		if w.turnActive.Load() {
			t.Error("turnActive should be cleared after interrupt")
		}
		e := drainWsEvent(t, w)
		if e.Type != agent.EventInterrupted {
			t.Errorf("event type = %q, want interrupted", e.Type)
		}
	})
}

// TestWsEmitEvent_DropsWhenFull asserts emitEvent never blocks on a full buffer.
func TestWsEmitEvent_DropsWhenFull(t *testing.T) {
	w := NewWebSocketConnection(DefaultConfig())
	for range 150 {
		w.emitEvent(agent.Event{Type: agent.EventStream})
	}
	got := 0
	for {
		select {
		case <-w.events:
			got++

			continue
		default:
		}

		break
	}
	if got != cap(w.events) {
		t.Errorf("buffered events = %d, want %d", got, cap(w.events))
	}
}

// TestWsConnected_FalseInitially verifies the connected flag starts false and
// the helpers behave on a fresh connection.
func TestWsConnected_FalseInitially(t *testing.T) {
	w := NewWebSocketConnection(DefaultConfig())
	if w.Connected() {
		t.Error("Connected() should be false initially")
	}
	// killProcess is safe to call with no process.
	w.killProcess()
}
