package claudesdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/valksor/kvelmo/agent"
)

// startServer brings up the WebSocket server half of a connection without
// spawning claude. It mirrors the listener/server/sendLoop setup that
// Connect() performs before launchClaude, so the real handleConnection,
// sendLoop, Connected and Close paths are exercised by a test WebSocket
// client standing in for the Claude CLI. Returns the dial URL.
func startServer(t *testing.T, w *WebSocketConnection) string {
	t.Helper()

	w.lifecycleCtx, w.lifecycleCancel = context.WithCancel(context.Background())
	w.done = make(chan struct{})
	go func() {
		<-w.lifecycleCtx.Done()
		close(w.done)
	}()
	w.subagents.SetDoneChannel(w.done)

	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	w.listener = listener
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		w.port = tcpAddr.Port
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", w.handleConnection)
	w.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() { _ = w.server.Serve(listener) }()
	go w.sendLoop(w.lifecycleCtx)

	t.Cleanup(func() { _ = w.Close() })

	return fmt.Sprintf("ws://127.0.0.1:%d", w.port)
}

// dialAsCLI connects to the agent's server the way the Claude CLI would.
func dialAsCLI(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	return conn
}

// sendJSON marshals and writes a frame from the fake CLI to the agent server.
func sendJSON(t *testing.T, conn *websocket.Conn, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// readJSON reads one frame the agent server sent back to the fake CLI.
func readJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal server frame %q: %v", string(data), err)
	}

	return m
}

// waitReady blocks until the connection reports a session, the way Connect's
// caller would, with a fail-fast timeout.
func waitReady(t *testing.T, w *WebSocketConnection) {
	t.Helper()
	select {
	case <-w.sessionReady:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for sessionReady")
	}
}

// TestServerLoop_HandshakeAndStream stands up the real server loop, dials back
// as the fake CLI, and drives a session init + stream + result. It asserts the
// connection becomes connected and the filtered SendPrompt stream surfaces the
// stream and completion events end to end over a real WebSocket.
func TestServerLoop_HandshakeAndStream(t *testing.T) {
	w := newTestConn(t)
	url := startServer(t, w)
	conn := dialAsCLI(t, url)

	// Wait for the server to register the connection (ready closes in
	// handleConnection). Then send the session init.
	select {
	case <-w.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("server never accepted the connection")
	}

	sendJSON(t, conn, map[string]any{"type": "system", "session_id": "S1"})
	waitReady(t, w)
	if !w.Connected() {
		t.Error("Connected() = false after session init")
	}

	ch, err := w.SendPrompt(context.Background(), "hello")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	// The fake CLI receives the user prompt the agent sent out.
	userMsg := readJSON(t, conn)
	if userMsg["type"] != "user" {
		t.Errorf("forwarded prompt type = %v, want user", userMsg["type"])
	}

	// CLI streams a delta then completes.
	sendJSON(t, conn, map[string]any{
		"type":  "stream_event",
		"event": map[string]any{"delta": map[string]any{"type": "text_delta", "text": "hi"}},
	})
	sendJSON(t, conn, map[string]any{"type": "result", "subtype": "success"})

	var sawStream, sawComplete bool
	deadline := time.After(5 * time.Second)
	for !sawComplete {
		select {
		case e, ok := <-ch:
			if !ok {
				goto done
			}
			switch e.Type {
			case agent.EventStream:
				if e.Content == "hi" {
					sawStream = true
				}
			case agent.EventComplete:
				sawComplete = true
			case agent.EventAssistant, agent.EventToolUse, agent.EventToolResult,
				agent.EventPermission, agent.EventError, agent.EventInit,
				agent.EventKeepAlive, agent.EventSubagent, agent.EventProgress,
				agent.EventToolProgress, agent.EventInterrupted, agent.EventPromptSuggestion:
				// Not relevant to this assertion.
			}
		case <-deadline:
			t.Fatal("timed out reading filtered events")
		}
	}
done:
	if !sawStream {
		t.Error("expected a stream event with content 'hi'")
	}
	if !sawComplete {
		t.Error("expected a completion event")
	}
}

// TestServerLoop_PermissionRoundTrip drives a control_request from the fake CLI
// and asserts the agent's configured permission handler answers it: the fake
// CLI receives a well-formed control_response with the allow behavior and
// original input echoed back via updatedInput.
func TestServerLoop_PermissionRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PermissionHandler = func(_ agent.PermissionRequest) bool { return true }
	w := NewWebSocketConnection(cfg)

	url := startServer(t, w)
	conn := dialAsCLI(t, url)

	select {
	case <-w.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("server never accepted the connection")
	}
	sendJSON(t, conn, map[string]any{"type": "system", "session_id": "S2"})
	waitReady(t, w)

	sendJSON(t, conn, map[string]any{
		"type":       "control_request",
		"request_id": "rr-1",
		"request": map[string]any{
			"subtype":     "can_use_tool",
			"tool_name":   "Read",
			"tool_use_id": "tu-7",
			"input":       map[string]any{"path": "/etc/hosts"},
		},
	})

	resp := readJSON(t, conn)
	if resp["type"] != "control_response" {
		t.Fatalf("response type = %v, want control_response", resp["type"])
	}
	inner, ok := resp["response"].(map[string]any)
	if !ok {
		t.Fatalf("response.response missing: %+v", resp)
	}
	if inner["request_id"] != "rr-1" {
		t.Errorf("response request_id = %v, want rr-1", inner["request_id"])
	}
	deepInner, ok := inner["response"].(map[string]any)
	if !ok {
		t.Fatalf("inner response missing: %+v", inner)
	}
	if deepInner["behavior"] != "allow" {
		t.Errorf("behavior = %v, want allow", deepInner["behavior"])
	}
	upd, ok := deepInner["updatedInput"].(map[string]any)
	if !ok || upd["path"] != "/etc/hosts" {
		t.Errorf("updatedInput = %v, want original input echoed", deepInner["updatedInput"])
	}
}

// TestServerLoop_InterruptOverWire asserts Interrupt enqueues a control_request
// (subtype interrupt) that reaches the fake CLI and emits an interrupted event.
func TestServerLoop_InterruptOverWire(t *testing.T) {
	w := newTestConn(t)
	url := startServer(t, w)
	conn := dialAsCLI(t, url)

	select {
	case <-w.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("server never accepted the connection")
	}
	sendJSON(t, conn, map[string]any{"type": "system", "session_id": "S3"})
	waitReady(t, w)

	// Drain the EventInit so the next read is the interrupted event.
	if e, _ := recvEvent(t, w.events); e.Type != agent.EventInit {
		t.Fatalf("first event = %v, want EventInit", e.Type)
	}

	if err := w.Interrupt(); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}

	got := readJSON(t, conn)
	if got["type"] != "control_request" {
		t.Errorf("interrupt frame type = %v, want control_request", got["type"])
	}
	req, ok := got["request"].(map[string]any)
	if !ok || req["subtype"] != "interrupt" {
		t.Errorf("interrupt request = %v, want subtype interrupt", got["request"])
	}

	e, ok := recvEvent(t, w.events)
	if !ok || e.Type != agent.EventInterrupted {
		t.Errorf("expected EventInterrupted, got %+v", e)
	}
}

// TestServerLoop_CloseStopsConnection verifies Close tears down the server,
// flips Connected to false, and is idempotent.
func TestServerLoop_CloseStopsConnection(t *testing.T) {
	w := newTestConn(t)
	url := startServer(t, w)
	conn := dialAsCLI(t, url)

	select {
	case <-w.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("server never accepted the connection")
	}
	sendJSON(t, conn, map[string]any{"type": "system", "session_id": "S4"})
	waitReady(t, w)
	if !w.Connected() {
		t.Fatal("expected Connected before Close")
	}

	if err := w.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
	if w.Connected() {
		t.Error("Connected() should be false after Close")
	}
	if !w.closed.Load() {
		t.Error("closed flag should be set after Close")
	}
	// Idempotent.
	if err := w.Close(); err != nil {
		t.Errorf("second Close() = %v", err)
	}
}

// TestSendPrompt_Concurrent verifies the in-flight guard rejects a second
// concurrent prompt.
func TestSendPrompt_Concurrent(t *testing.T) {
	w := newTestConn(t)
	url := startServer(t, w)
	conn := dialAsCLI(t, url)

	select {
	case <-w.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("server never accepted the connection")
	}
	sendJSON(t, conn, map[string]any{"type": "system", "session_id": "S5"})
	waitReady(t, w)

	if _, err := w.SendPrompt(context.Background(), "first"); err != nil {
		t.Fatalf("first SendPrompt: %v", err)
	}
	if _, err := w.SendPrompt(context.Background(), "second"); err == nil {
		t.Error("second concurrent SendPrompt should be rejected")
	}
}

// TestSendPrompt_NoSession verifies SendPrompt fails before a session exists
// and releases the in-flight guard so a later prompt can proceed.
func TestSendPrompt_NoSession(t *testing.T) {
	w := newTestConn(t)
	if _, err := w.SendPrompt(context.Background(), "x"); err == nil {
		t.Error("SendPrompt without session should error")
	}
	if w.promptInFlight.Load() {
		t.Error("in-flight guard should be released after the no-session error")
	}
}

// TestSendPrompt_ContextCancel verifies a cancelled context terminates the
// filtered stream goroutine (channel closes) without delivering a terminal
// event.
func TestSendPrompt_ContextCancel(t *testing.T) {
	w := newTestConn(t)
	url := startServer(t, w)
	conn := dialAsCLI(t, url)

	select {
	case <-w.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("server never accepted the connection")
	}
	sendJSON(t, conn, map[string]any{"type": "system", "session_id": "S6"})
	waitReady(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := w.SendPrompt(ctx, "hello")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			// A buffered event may arrive first; drain until close.
			for range ch { //nolint:revive // draining until close
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("filtered stream did not close after context cancel")
	}
}

// TestHandlePermission_DenyPath asserts HandlePermission(false) enqueues a deny
// control_response with the denial message and no updated input.
func TestHandlePermission_DenyPath(t *testing.T) {
	w := newTestConn(t)
	w.lifecycleCtx, w.lifecycleCancel = context.WithCancel(context.Background())
	t.Cleanup(w.lifecycleCancel)

	// Seed a pending request to mimic a prior control_request.
	w.pendingRequests["d-1"] = pendingRequest{
		Input:     map[string]any{"path": "/secret"},
		ToolUseID: "tu-d",
	}

	if err := w.HandlePermission("d-1", false); err != nil {
		t.Fatalf("HandlePermission: %v", err)
	}
	// Pending entry consumed.
	if _, ok := w.pendingRequests["d-1"]; ok {
		t.Error("pending request should be deleted after HandlePermission")
	}

	select {
	case out := <-w.outgoing:
		if out.Response == nil || out.Response.Response == nil {
			t.Fatalf("missing control_response payload: %+v", out)
		}
		inner := out.Response.Response
		if inner.Behavior != "deny" {
			t.Errorf("behavior = %q, want deny", inner.Behavior)
		}
		if inner.Message == "" {
			t.Error("deny response should carry a message")
		}
		if inner.UpdatedInput != nil {
			t.Errorf("deny should not echo updatedInput, got %v", inner.UpdatedInput)
		}
		if inner.ToolUseID != "tu-d" {
			t.Errorf("toolUseID = %q, want tu-d", inner.ToolUseID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no control_response enqueued")
	}
}

// TestHandlePermission_UnknownRequest verifies answering an unknown request id
// still enqueues a response (with empty pending fields) rather than erroring.
func TestHandlePermission_UnknownRequest(t *testing.T) {
	w := newTestConn(t)
	w.lifecycleCtx, w.lifecycleCancel = context.WithCancel(context.Background())
	t.Cleanup(w.lifecycleCancel)

	if err := w.HandlePermission("missing", true); err != nil {
		t.Fatalf("HandlePermission: %v", err)
	}
	select {
	case out := <-w.outgoing:
		if out.Response == nil || out.Response.RequestID != "missing" {
			t.Fatalf("unexpected response: %+v", out.Response)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no response enqueued for unknown request")
	}
}

// TestInterrupt_NotConnected verifies Interrupt is a no-op when not connected.
func TestInterrupt_NotConnected(t *testing.T) {
	w := newTestConn(t)
	if err := w.Interrupt(); err != nil {
		t.Errorf("Interrupt() while disconnected = %v, want nil", err)
	}
	// Nothing should have been enqueued.
	select {
	case out := <-w.outgoing:
		t.Errorf("Interrupt enqueued %+v while disconnected", out)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestTrySendOutgoing_ClosedConnection verifies trySendOutgoing reports an
// error once the lifecycle context is cancelled.
func TestTrySendOutgoing_ClosedConnection(t *testing.T) {
	w := newTestConn(t)
	w.lifecycleCtx, w.lifecycleCancel = context.WithCancel(context.Background())
	w.lifecycleCancel() // immediately cancel

	// Fill the buffer so the non-blocking send cannot succeed, forcing the
	// closed-context branch.
	for range cap(w.outgoing) {
		w.outgoing <- outgoingMessage{Type: "user"}
	}
	if err := w.trySendOutgoing(outgoingMessage{Type: "user"}); err == nil {
		t.Error("trySendOutgoing on cancelled context should error")
	}
	// control_response on a cancelled context also errors.
	if err := w.trySendOutgoing(outgoingMessage{Type: "control_response"}); err == nil {
		t.Error("control_response on cancelled context should error")
	}
}

// TestTrySendOutgoing_BufferFull verifies a non-control message is dropped with
// an error when the outgoing buffer is full and the context is still live.
func TestTrySendOutgoing_BufferFull(t *testing.T) {
	w := newTestConn(t)
	w.lifecycleCtx, w.lifecycleCancel = context.WithCancel(context.Background())
	t.Cleanup(w.lifecycleCancel)

	for range cap(w.outgoing) {
		w.outgoing <- outgoingMessage{Type: "user"}
	}
	if err := w.trySendOutgoing(outgoingMessage{Type: "user"}); err == nil {
		t.Error("expected error when outgoing buffer is full")
	}
}

// TestTrySendEvent_AfterClose verifies events are dropped once closed is set.
func TestTrySendEvent_AfterClose(t *testing.T) {
	w := newTestConn(t)
	w.closed.Store(true)
	w.trySendEvent(agent.Event{Type: agent.EventStream, Content: "x"})
	w.trySendEvent(agent.Event{Type: agent.EventComplete})
	select {
	case e := <-w.events:
		t.Errorf("event delivered after close: %+v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestTrySendEvent_NonTerminalDropWhenFull verifies a non-terminal event is
// dropped (not blocked) when the events channel is full.
func TestTrySendEvent_NonTerminalDropWhenFull(t *testing.T) {
	w := newTestConn(t)
	for range cap(w.events) {
		w.events <- agent.Event{Type: agent.EventStream}
	}
	done := make(chan struct{})
	go func() {
		w.trySendEvent(agent.Event{Type: agent.EventStream, Content: "dropped"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("trySendEvent blocked on a full channel for a non-terminal event")
	}
}

// TestIsClosed exercises both branches of the non-blocking closed check.
func TestIsClosed(t *testing.T) {
	open := make(chan struct{})
	if isClosed(open) {
		t.Error("isClosed(open) = true")
	}
	closed := make(chan struct{})
	close(closed)
	if !isClosed(closed) {
		t.Error("isClosed(closed) = false")
	}
}

// TestNewWebSocketConnection verifies the constructor wires channels and the
// subagent tracker, and seeds the port from config.
func TestNewWebSocketConnection(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WebSocketPort = 4242
	w := NewWebSocketConnection(cfg)
	if w.port != 4242 {
		t.Errorf("port = %d, want 4242", w.port)
	}
	if w.outgoing == nil || w.events == nil {
		t.Error("channels should be initialized")
	}
	if w.pendingRequests == nil {
		t.Error("pendingRequests should be initialized")
	}
	if w.subagents == nil {
		t.Error("subagent tracker should be initialized")
	}
}

// TestBuildArgs verifies the fixed --sdk-url stream-json flags plus model and
// extra-arg wiring.
func TestBuildArgs(t *testing.T) {
	w := NewWebSocketConnection(DefaultConfig())
	w.port = 9999
	base := w.buildArgs()
	for _, want := range []string{
		"--sdk-url", "ws://127.0.0.1:9999", "--print", "--verbose",
		"--output-format", "stream-json", "--input-format",
		"--include-partial-messages", "--permission-mode", "bypassPermissions",
	} {
		if !slices.Contains(base, want) {
			t.Errorf("buildArgs() = %v, missing %q", base, want)
		}
	}

	cfg := DefaultConfig()
	cfg.Model = "claude-sonnet-test"
	cfg.Args = []string{"--extra-flag"}
	w2 := NewWebSocketConnection(cfg)
	got := w2.buildArgs()
	if !slices.Contains(got, "--model") || !slices.Contains(got, "claude-sonnet-test") {
		t.Errorf("buildArgs() = %v, want --model claude-sonnet-test", got)
	}
	if !slices.Contains(got, "--extra-flag") {
		t.Errorf("buildArgs() = %v, want --extra-flag", got)
	}
}
