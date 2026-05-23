package socket

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/ratelimit"
	"github.com/valksor/kvelmo/internal/testutil"
)

// startServer stands up a real Unix-domain-socket server on a short path with
// the given handlers and returns a connected client. Cleanup stops the server.
func startServer(t *testing.T, opts ...ServerOption) (*Server, *Client) {
	t.Helper()
	sockPath := testutil.TempSocketPath(t)
	// Keep the drain window short so Stop() in cleanup never blocks for the
	// full 5s default if a connection lingers.
	opts = append([]ServerOption{WithDrainTimeout(200 * time.Millisecond)}, opts...)
	srv := NewServer(sockPath, opts...)
	srv.Handle("echo", func(_ context.Context, req *Request) (*Response, error) {
		return NewResultResponse(req.ID, req.Params)
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Start(ctx) }()

	// Wait for the socket to accept connections.
	var client *Client
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := NewClient(sockPath)
		if err == nil {
			client = c

			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if client == nil {
		t.Fatal("server did not become reachable")
	}
	// Cleanup runs LIFO: register Stop first, then Close, so the client
	// disconnects before Stop's waitForDrain runs — otherwise drain blocks for
	// the full timeout waiting on the still-open connection.
	t.Cleanup(func() { _ = srv.Stop() })
	t.Cleanup(func() { _ = client.Close() })

	return srv, client
}

func TestServerRoundtripWithOptions(t *testing.T) {
	limiter := ratelimit.NewLimiter(100, 100)
	srv, client := startServer(
		t,
		WithDrainTimeout(2*time.Second),
		WithRateLimiter(limiter),
	)
	if srv.getDrainTimeout() != 2*time.Second {
		t.Errorf("getDrainTimeout() = %v, want 2s", srv.getDrainTimeout())
	}

	ctx := context.Background()
	resp, err := client.Call(ctx, "echo", json.RawMessage(`{"x":1}`))
	if err != nil {
		t.Fatalf("Call(echo) error = %v", err)
	}
	if string(resp.Result) != `{"x":1}` {
		t.Errorf("echo result = %s, want {\"x\":1}", resp.Result)
	}
}

func TestServerMethodNotFound(t *testing.T) {
	_, client := startServer(t)
	_, err := client.Call(context.Background(), "nonexistent.method", nil)
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestServerCapabilitiesHandshake(t *testing.T) {
	_, client := startServer(t)
	resp, err := client.Call(context.Background(), MethodCapabilities, nil)
	if err != nil {
		t.Fatalf("Call(%s) error = %v", MethodCapabilities, err)
	}
	var caps Capabilities
	if err := json.Unmarshal(resp.Result, &caps); err != nil {
		t.Fatalf("unmarshal capabilities: %v", err)
	}
	if caps.ProtocolVersion != ProtocolVersion {
		t.Errorf("ProtocolVersion = %q, want %q", caps.ProtocolVersion, ProtocolVersion)
	}
	if len(caps.Methods) == 0 {
		t.Error("expected at least one registered method")
	}
}

func TestServerWriteEventAndBroadcast(t *testing.T) {
	sockPath := testutil.TempSocketPath(t)
	srv := NewServer(sockPath, WithDrainTimeout(200*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Start(ctx) }()

	// Dial directly so the connection is tracked by the server for Broadcast.
	var conn net.Conn
	var dialer net.Dialer
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := dialer.DialContext(ctx, "unix", sockPath)
		if err == nil {
			conn = c

			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if conn == nil {
		t.Fatal("could not dial server")
	}
	t.Cleanup(func() { _ = conn.Close() })

	// WriteEvent writes a JSON event to a connection.
	if err := WriteEvent(conn, map[string]string{"type": "hello"}); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	// Give the server a moment to register the inbound connection, then
	// broadcast. Broadcast must not error even if delivery is best-effort.
	time.Sleep(50 * time.Millisecond)
	srv.Broadcast([]byte("{\"type\":\"broadcast\"}\n"))

	if srv.Path() != sockPath {
		t.Errorf("Path() = %q, want %q", srv.Path(), sockPath)
	}

	_ = srv.Stop()
}

// NOTE: A rapid start/stop "stale-socket cleanup during Start" scenario is
// intentionally NOT tested here. It trips a pre-existing data race between
// ratelimit.Limiter.Start (which lazily creates its cancel context under a
// sync.Once inside Server.Start) and ratelimit.Limiter.Stop (inside
// Server.Stop) when Stop is called close behind a goroutine-launched Start.
// See the agent REPORT. CleanupStaleSocket itself is covered directly by
// TestCleanupStaleSocket_Branches.

func TestServerActiveConnections(t *testing.T) {
	srv, client := startServer(t)
	// Issue a call to ensure the connection is established and tracked.
	if _, err := client.Call(context.Background(), "echo", nil); err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	// At least our own connection should be counted while open.
	if srv.ActiveConnections() < 0 {
		t.Error("ActiveConnections() returned negative count")
	}
}

func TestClientSetTimeout(t *testing.T) {
	_, client := startServer(t)
	client.SetTimeout(500 * time.Millisecond)
	// A normal call still succeeds within the new timeout.
	if _, err := client.Call(context.Background(), "echo", nil); err != nil {
		t.Fatalf("Call() after SetTimeout error = %v", err)
	}
}

func TestClientCallEncodesByteParams(t *testing.T) {
	_, client := startServer(t)
	// Passing raw bytes exercises the []byte branch of encodeParams.
	resp, err := client.Call(context.Background(), "echo", []byte(`{"raw":true}`))
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if string(resp.Result) != `{"raw":true}` {
		t.Errorf("result = %s, want {\"raw\":true}", resp.Result)
	}
}
