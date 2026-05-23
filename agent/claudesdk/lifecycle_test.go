package claudesdk

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
)

// fakeSDKSource is a minimal stand-in for the Claude CLI's SDK transport. It
// parses --sdk-url from its args, dials BACK to the agent's local WebSocket
// server (the connect-back direction the real CLI uses), sends a session-init
// frame, streams one delta, then completes. --version answers Available().
// This exercises the real Connect/launchClaude/handleConnection paths without
// the official binary (which rejects --sdk-url since 2.1.121).
const fakeSDKSource = `package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/coder/websocket"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("claude 1.0.0-fake-sdk")
		return
	}
	var url string
	for i, a := range os.Args {
		if a == "--sdk-url" && i+1 < len(os.Args) {
			url = os.Args[i+1]
		}
	}
	if url == "" {
		os.Exit(2)
	}
	ctx := context.Background()
	var conn *websocket.Conn
	// The agent server may not be listening the instant we start; retry briefly.
	for i := 0; i < 50; i++ {
		c, _, err := websocket.Dial(ctx, url, nil)
		if err == nil {
			conn = c
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if conn == nil {
		os.Exit(3)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	write := func(s string) {
		wctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_ = conn.Write(wctx, websocket.MessageText, []byte(s))
	}
	write(` + "`" + `{"type":"system","session_id":"fake-session"}` + "`" + `)
	write(` + "`" + `{"type":"stream_event","event":{"delta":{"type":"text_delta","text":"hi from fake"}}}` + "`" + `)
	write(` + "`" + `{"type":"result","subtype":"success"}` + "`" + `)

	// Stay alive so the parent controls teardown (it kills us on Close()).
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}
`

// buildFakeSDK compiles the fake CLI and returns its path. Skips on Windows
// (the test relies on POSIX process semantics for kill-on-Close).
func buildFakeSDK(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-binary harness requires POSIX process semantics")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(fakeSDKSource), 0o644); err != nil {
		t.Fatalf("write fake source: %v", err)
	}
	bin := filepath.Join(dir, "fake-sdk")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// Build inside the module so the coder/websocket dependency resolves.
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake sdk: %v\n%s", err, out)
	}

	return bin
}

// TestConnect_Lifecycle exercises the real Connect → launchClaude →
// handleConnection → session-init → SendPrompt → result path end to end against
// the compiled fake SDK CLI, with no official claude binary involved.
func TestConnect_Lifecycle(t *testing.T) {
	bin := buildFakeSDK(t)

	cfg := DefaultConfig()
	cfg.Command = []string{bin}
	a := NewWithConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.Connect(ctx); err != nil {
		t.Fatalf("Connect() = %v", err)
	}
	// Second Connect while connected is a no-op success.
	if err := a.Connect(ctx); err != nil {
		t.Fatalf("second Connect() = %v", err)
	}
	if !a.Connected() {
		t.Error("Connected() = false after Connect")
	}

	ch, err := a.SendPrompt(ctx, "do the thing")
	if err != nil {
		t.Fatalf("SendPrompt() = %v", err)
	}

	var sawStream, sawComplete bool
	deadline := time.After(10 * time.Second)
	for !sawComplete {
		select {
		case e, ok := <-ch:
			if !ok {
				goto done
			}
			switch e.Type {
			case agent.EventStream:
				if e.Content == "hi from fake" {
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
			t.Fatal("timed out collecting events")
		}
	}
done:
	if !sawStream {
		t.Error("expected a stream event with the fake content")
	}
	if !sawComplete {
		t.Error("expected a completion event")
	}

	// Agent-level Interrupt/HandlePermission forward to the live connection.
	if err := a.HandlePermission("nope", true); err != nil {
		t.Errorf("HandlePermission() = %v", err)
	}
	if err := a.Interrupt(); err != nil {
		t.Errorf("Interrupt() = %v", err)
	}

	if err := a.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
	// Close clears the connection; a follow-up Close is still a no-op success.
	if err := a.Close(); err != nil {
		t.Errorf("second Close() = %v", err)
	}
}

// TestConnect_EarlyExit verifies that when the spawned process exits before
// completing the handshake, Connect returns the actionable early-exit error
// (the common case against the official CLI which rejects --sdk-url).
func TestConnect_EarlyExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX shell")
	}
	dir := t.TempDir()
	// A binary that ignores --sdk-url and exits non-zero immediately, mimicking
	// the official CLI rejecting the flag.
	script := filepath.Join(dir, "reject.sh")
	const body = "#!/bin/sh\n" +
		"case \"$1\" in --version) echo fake; exit 0;; esac\n" +
		"echo 'Error: --sdk-url rejected' >&2\nexit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write reject script: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Command = []string{script}
	w := NewWebSocketConnection(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := w.Connect(ctx)
	if err == nil {
		t.Fatal("Connect() should fail when the process exits early")
	}
	// The friendly hint must be present.
	if msg := err.Error(); !strings.Contains(msg, "--sdk-url") || !strings.Contains(msg, "claude-mcp") {
		t.Errorf("early-exit error missing actionable hint: %v", err)
	}
}

// TestConnect_AfterClose verifies a closed connection refuses to reconnect.
func TestConnect_AfterClose(t *testing.T) {
	w := NewWebSocketConnection(DefaultConfig())
	w.closed.Store(true)
	if err := w.Connect(context.Background()); err == nil {
		t.Error("Connect() on a closed connection should error")
	}
}

// TestAvailable_FakeBinary verifies Available() succeeds against a binary whose
// --version exits zero.
func TestAvailable_FakeBinary(t *testing.T) {
	bin := buildFakeSDK(t)
	cfg := DefaultConfig()
	cfg.Command = []string{bin}
	if err := NewWithConfig(cfg).Available(); err != nil {
		t.Errorf("Available() with fake --version = %v, want nil", err)
	}
}
