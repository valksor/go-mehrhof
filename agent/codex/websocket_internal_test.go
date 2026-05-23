package codex

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
)

// TestFindFreePort returns a usable, non-zero TCP port.
func TestFindFreePort(t *testing.T) {
	port, err := findFreePort(context.Background())
	if err != nil {
		t.Fatalf("findFreePort() = %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Errorf("findFreePort() = %d, want a valid TCP port", port)
	}
}

// TestWsClose_NoConnection verifies Close on a never-connected WebSocket
// connection is safe, idempotent, and closes the events channel exactly once.
func TestWsClose_NoConnection(t *testing.T) {
	w := NewWebSocketConnection(DefaultConfig())

	if err := w.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
	// Second Close must not panic (closedOnce guards the channel close).
	if err := w.Close(); err != nil {
		t.Errorf("second Close() = %v", err)
	}

	// The events channel should be closed: a receive returns the zero value
	// with ok=false.
	select {
	case _, ok := <-w.events:
		if ok {
			t.Error("events channel should be closed after Close()")
		}
	case <-time.After(time.Second):
		t.Error("receive on closed events channel blocked")
	}
}

// TestWsConnect_LaunchFailsForMissingBinary drives Connect through findFreePort
// and launchCodex with a non-existent binary, exercising the launch-failure
// path (which also covers killProcess via the dial failure cleanup).
func TestWsConnect_LaunchFailsForMissingBinary(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Command = []string{filepath.Join(t.TempDir(), "no-such-codex")}
	w := NewWebSocketConnection(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := w.Connect(ctx); err == nil {
		t.Error("Connect() with missing binary should fail")
		_ = w.Close()
	}
	if w.Connected() {
		t.Error("Connected() should be false after a failed Connect")
	}
}

// TestWsConnect_DialFailsAfterLaunch starts a process that exits immediately so
// no WebSocket server ever listens; Connect should exhaust its retries and fail,
// driving the dial-retry loop and killProcess cleanup.
func TestWsConnect_DialFailsAfterLaunch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "exit-fast.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Command = []string{script}
	w := NewWebSocketConnection(cfg)

	// Cancel quickly so the exponential-backoff retry loop bails out fast
	// instead of waiting the full ~3s.
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	if err := w.Connect(ctx); err == nil {
		t.Error("Connect() should fail when no WS server listens")
		_ = w.Close()
	}
}

// TestAgent_WSPreferFallsBackToCLI sets PreferWebSocket but points at a fake
// binary that speaks only the stdio JSON-RPC protocol (no WS server). Connect
// must try WS, fail, and fall back to the CLI path successfully.
func TestAgent_WSPreferFallsBackToCLI(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Command = []string{writeFakeCodex(t)}
	cfg.PreferWebSocket = true
	a := NewWithConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := a.Connect(ctx); err != nil {
		t.Fatalf("Connect() = %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })

	// WS dial against the stdio fake never succeeds, so we land in CLI mode.
	if a.Mode() != agent.ModeCLI {
		t.Errorf("Mode() = %q, want CLI fallback", a.Mode())
	}
}
