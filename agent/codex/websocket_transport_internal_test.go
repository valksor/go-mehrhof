package codex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// wsPair spins up an in-process WebSocket server and returns the client-side
// connection plus the server-side connection. Both are closed via t.Cleanup.
// This gives the wsTransport a real *websocket.Conn to exercise its read/write
// paths without the real codex binary.
func wsPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()

	srvCh := make(chan *websocket.Conn, 1)
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		srvCh <- c
		// Keep the handler alive until the test finishes by blocking on the
		// request context; CloseNow in cleanup unblocks the read loop.
		<-r.Context().Done()
	}))
	t.Cleanup(hs.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := strings.Replace(hs.URL, "http://", "ws://", 1)
	client, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	select {
	case server := <-srvCh:
		t.Cleanup(func() { _ = server.CloseNow() })
		t.Cleanup(func() { _ = client.CloseNow() })

		return client, server
	case <-time.After(5 * time.Second):
		t.Fatal("server side of websocket never accepted")

		return nil, nil
	}
}

// readFrame reads one JSON frame from a server-side connection, deriving a
// short read deadline from the supplied parent context.
func readFrame(t *testing.T, ctx context.Context, conn *websocket.Conn) map[string]any {
	t.Helper()
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, data, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}

	return m
}

// frameID returns the numeric "id" of a decoded frame, failing if absent.
func frameID(t *testing.T, frame map[string]any) float64 {
	t.Helper()
	id, ok := frame["id"].(float64)
	if !ok {
		t.Fatalf("frame missing numeric id: %v", frame)
	}

	return id
}

// frameResult returns the "result" object of a decoded frame, failing if absent.
func frameResult(t *testing.T, frame map[string]any) map[string]any {
	t.Helper()
	result, ok := frame["result"].(map[string]any)
	if !ok {
		t.Fatalf("frame missing result object: %v", frame)
	}

	return result
}

// TestWsTransport_NotifyAndRespond writes a notification and a response over a
// real WS connection and asserts the server receives the exact JSON-RPC frames.
func TestWsTransport_NotifyAndRespond(t *testing.T) {
	client, server := wsPair(t)
	tr := newWsTransport(client)
	defer func() { _ = tr.Close() }()

	ctx := context.Background()

	if err := tr.Notify(ctx, "ping", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("Notify() = %v", err)
	}
	frame := readFrame(t, context.Background(), server)
	if frame["method"] != "ping" {
		t.Errorf("method = %v, want ping", frame["method"])
	}
	if _, hasID := frame["id"]; hasID {
		t.Error("notification should not carry an id")
	}
	if frame["jsonrpc"] != jsonrpcVersion {
		t.Errorf("jsonrpc = %v, want %s", frame["jsonrpc"], jsonrpcVersion)
	}

	if err := tr.Respond(ctx, 55, map[string]any{keyDecision: decisionAccept}); err != nil {
		t.Fatalf("Respond() = %v", err)
	}
	resp := readFrame(t, context.Background(), server)
	if frameID(t, resp) != 55 {
		t.Errorf("response id = %v, want 55", resp["id"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok || result[keyDecision] != decisionAccept {
		t.Errorf("response result = %v, want accept decision", resp["result"])
	}
}

// TestWsTransport_Call_RoundTrip drives a request from the transport, has the
// server reply with a matching id, and asserts the decoded result.
func TestWsTransport_Call_RoundTrip(t *testing.T) {
	client, server := wsPair(t)
	tr := newWsTransport(client)
	defer func() { _ = tr.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go tr.readLoop(ctx)

	// Server echoes a result for whatever id it receives.
	go func() {
		req := readFrame(t, context.Background(), server)
		id := frameID(t, req)
		reply := `{"jsonrpc":"2.0","id":` + jsonFloat(id) + `,"result":{"ok":true}}`
		_ = server.Write(context.Background(), websocket.MessageText, []byte(reply))
	}()

	result, err := tr.Call(ctx, "do/thing", map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("Call() = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if got["ok"] != true {
		t.Errorf("result[ok] = %v, want true", got["ok"])
	}
}

// TestWsTransport_Call_ServerError surfaces a JSON-RPC error frame as a Go error.
func TestWsTransport_Call_ServerError(t *testing.T) {
	client, server := wsPair(t)
	tr := newWsTransport(client)
	defer func() { _ = tr.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go tr.readLoop(ctx)

	go func() {
		req := readFrame(t, context.Background(), server)
		id := frameID(t, req)
		reply := `{"jsonrpc":"2.0","id":` + jsonFloat(id) + `,"error":{"code":-32000,"message":"boom"}}`
		_ = server.Write(context.Background(), websocket.MessageText, []byte(reply))
	}()

	_, err := tr.Call(ctx, "do/thing", nil)
	if err == nil {
		t.Fatal("Call() should error on server error frame")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want it to mention boom", err)
	}
}

// TestWsTransport_Call_ContextCanceled returns the context error when the
// caller's context is canceled before a response arrives.
func TestWsTransport_Call_ContextCanceled(t *testing.T) {
	client, _ := wsPair(t)
	tr := newWsTransport(client)
	defer func() { _ = tr.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	go tr.readLoop(context.Background())
	cancel()

	if _, err := tr.Call(ctx, "do/thing", nil); err == nil {
		t.Error("Call() should error when context is canceled")
	}
}

// TestWsTransport_Call_Closed errors immediately on a closed transport.
func TestWsTransport_Call_Closed(t *testing.T) {
	client, _ := wsPair(t)
	tr := newWsTransport(client)
	_ = tr.Close()

	if _, err := tr.Call(context.Background(), "x", nil); err == nil {
		t.Error("Call() on closed transport should error")
	}
	if err := tr.Notify(context.Background(), "x", nil); err == nil {
		t.Error("Notify() on closed transport should error")
	}
	if err := tr.Respond(context.Background(), 1, nil); err == nil {
		t.Error("Respond() on closed transport should error")
	}
}

// TestWsTransport_Close_Idempotent verifies Close is safe twice.
func TestWsTransport_Close_Idempotent(t *testing.T) {
	client, _ := wsPair(t)
	tr := newWsTransport(client)
	if err := tr.Close(); err != nil {
		t.Errorf("first Close() = %v", err)
	}
	if err := tr.Close(); err != nil {
		t.Errorf("second Close() = %v", err)
	}
}

// TestWsTransport_Dispatch routes the three message shapes (response, request,
// notification) to the correct sinks. This is pure logic — no connection.
func TestWsTransport_Dispatch(t *testing.T) {
	tr := newWsTransport(nil)

	t.Run("response routes to pending channel", func(t *testing.T) {
		id := tr.nextID.Add(1)
		respCh := make(chan *rpcResponse, 1)
		tr.pendingM.Lock()
		tr.pending[id] = respCh
		tr.pendingM.Unlock()

		tr.dispatch(rpcMessage{ID: &id, Result: json.RawMessage(`{"v":1}`)})

		select {
		case resp := <-respCh:
			if resp.ID != id {
				t.Errorf("resp.ID = %d, want %d", resp.ID, id)
			}
		case <-time.After(time.Second):
			t.Error("dispatch did not deliver response")
		}
	})

	t.Run("request routes to request handler", func(t *testing.T) {
		var gotMethod string
		var gotID int64
		tr.requestHandler = func(method string, id int64, _ json.RawMessage) {
			gotMethod = method
			gotID = id
		}
		reqID := int64(77)
		tr.dispatch(rpcMessage{ID: &reqID, Method: "needs/response"})
		if gotMethod != "needs/response" || gotID != 77 {
			t.Errorf("request handler got (%q,%d), want (needs/response,77)", gotMethod, gotID)
		}
	})

	t.Run("notification routes to notification handler", func(t *testing.T) {
		var gotMethod string
		tr.notificationHandler = func(method string, _ json.RawMessage) {
			gotMethod = method
		}
		tr.dispatch(rpcMessage{Method: "fire/forget"})
		if gotMethod != "fire/forget" {
			t.Errorf("notification handler method = %q, want fire/forget", gotMethod)
		}
	})

	t.Run("unknown response id is ignored", func(t *testing.T) {
		unknown := int64(99999)
		// No pending channel registered; should not panic.
		tr.dispatch(rpcMessage{ID: &unknown, Result: json.RawMessage(`{}`)})
	})
}

// jsonFloat renders a float id as an integer string for embedding in a reply.
func jsonFloat(f float64) string {
	return strconv.FormatInt(int64(f), 10)
}
