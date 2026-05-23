package codex

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
)

// writeFakeCodex writes a POSIX-shell stand-in for the `codex app-server`
// binary. It speaks just enough of the JSON-RPC app-server protocol over
// stdin/stdout to drive Connect → initialize → thread/start → turn/start →
// notifications, so the real CLI pipeline runs without the real codex binary.
//
// The script reads each request line, extracts its numeric "id" with sed, and
// emits a matching response. After the turn/start response it streams a couple
// of notifications and a turn/completed so the prompt path reaches a terminal
// event.
func writeFakeCodex(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-binary harness requires a POSIX shell")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-codex.sh")
	const script = `#!/bin/sh
# Answer --version probes immediately.
case "$1" in
  --version) echo "codex 0.0.0-fake"; exit 0;;
esac

emit() { printf '%s\n' "$1"; }
id_of() { printf '%s' "$1" | sed -n 's/.*"id":\([0-9][0-9]*\).*/\1/p'; }

while IFS= read -r line; do
  case "$line" in
    *'"method":"initialize"'*)
      emit "{\"jsonrpc\":\"2.0\",\"id\":$(id_of "$line"),\"result\":{}}"
      ;;
    *'"method":"initialized"'*)
      : # notification, no response
      ;;
    *'"method":"thread/start"'*)
      emit "{\"jsonrpc\":\"2.0\",\"id\":$(id_of "$line"),\"result\":{\"thread\":{\"id\":\"thread-xyz\"}}}"
      ;;
    *'"method":"turn/start"'*)
      emit "{\"jsonrpc\":\"2.0\",\"id\":$(id_of "$line"),\"result\":{}}"
      emit '{"jsonrpc":"2.0","method":"item/agentMessage/delta","params":{"text":"streamed reply"}}'
      emit '{"jsonrpc":"2.0","method":"item/started","params":{"itemId":"i1","type":"commandExecution"}}'
      emit '{"jsonrpc":"2.0","method":"turn/completed","params":{}}'
      ;;
    *'"method":"turn/interrupt"'*)
      emit "{\"jsonrpc\":\"2.0\",\"id\":$(id_of "$line"),\"result\":{}}"
      ;;
    *)
      :
      ;;
  esac
done
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	return path
}

// collectUntilTerminal drains a SendPrompt channel until it closes (the filter
// goroutine closes it after a terminal event).
func collectUntilTerminal(t *testing.T, ch <-chan agent.Event) []agent.Event {
	t.Helper()
	var events []agent.Event
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, e)
		case <-timeout:
			t.Fatal("timed out collecting events")

			return events
		}
	}
}

// TestCLIBuildArgs verifies the app-server invocation and extra-arg passthrough.
func TestCLIBuildArgs(t *testing.T) {
	base := NewCLIConnection(DefaultConfig()).buildArgs()
	if len(base) == 0 || base[0] != "app-server" {
		t.Errorf("buildArgs() = %v, want first arg app-server", base)
	}

	cfg := DefaultConfig()
	cfg.Args = []string{"--profile", "fast"}
	withArgs := NewCLIConnection(cfg).buildArgs()
	if !slices.Contains(withArgs, "--profile") || !slices.Contains(withArgs, "fast") {
		t.Errorf("buildArgs() with extra args = %v, want --profile fast appended", withArgs)
	}
}

// TestCLISendPrompt_NotConnected errors before Connect.
func TestCLISendPrompt_NotConnected(t *testing.T) {
	c := NewCLIConnection(DefaultConfig())
	if _, err := c.SendPrompt(context.Background(), "hi"); err == nil {
		t.Error("SendPrompt before Connect should error")
	}
}

// TestCLIConnected_FalseInitially verifies the connected flag starts false.
func TestCLIConnected_FalseInitially(t *testing.T) {
	c := NewCLIConnection(DefaultConfig())
	if c.Connected() {
		t.Error("Connected() should be false before Connect")
	}
}

// TestCLIClose_Idempotent verifies Close is safe to call twice.
func TestCLIClose_Idempotent(t *testing.T) {
	c := NewCLIConnection(DefaultConfig())
	if err := c.Close(); err != nil {
		t.Errorf("first Close() = %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second Close() = %v", err)
	}
}

// TestCLIConnection_Lifecycle exercises the full Connect → SendPrompt →
// notification dispatch → Close pipeline against the fake binary, with no real
// codex. It asserts the init event, the streamed reply, a tool-use event, and
// the terminal completion.
func TestCLIConnection_Lifecycle(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Command = []string{writeFakeCodex(t)}
	cfg.Model = "fake-model"
	c := NewCLIConnection(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect() = %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if !c.Connected() {
		t.Error("Connected() = false after Connect")
	}
	if c.threadID != "thread-xyz" {
		t.Errorf("threadID = %q, want thread-xyz", c.threadID)
	}

	// Connect emits an init event onto the (pre-prompt) channel.
	e := drainEvent(t, c)
	if e.Type != agent.EventInit {
		t.Errorf("first event type = %q, want init", e.Type)
	}

	ch, err := c.SendPrompt(ctx, "do the thing")
	if err != nil {
		t.Fatalf("SendPrompt() = %v", err)
	}

	events := collectUntilTerminal(t, ch)
	var sawStream, sawToolUse, sawComplete bool
	for _, ev := range events {
		switch {
		case ev.Type == agent.EventStream && ev.Content == "streamed reply":
			sawStream = true
		case ev.Type == agent.EventToolUse && ev.Content == "Bash":
			sawToolUse = true
		case ev.Type == agent.EventComplete:
			sawComplete = true
		}
	}
	if !sawStream {
		t.Errorf("expected streamed reply, got %+v", events)
	}
	if !sawToolUse {
		t.Errorf("expected Bash tool-use event, got %+v", events)
	}
	if !sawComplete {
		t.Errorf("expected completion event, got %+v", events)
	}

	if err := c.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
}

// TestAgent_CLILifecycle drives the Agent wrapper over the CLI fallback path
// (PreferWebSocket left false) against the fake binary.
func TestAgent_CLILifecycle(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Command = []string{writeFakeCodex(t)}
	a := NewWithConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.Connect(ctx); err != nil {
		t.Fatalf("Connect() = %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })

	if a.Mode() != agent.ModeCLI {
		t.Errorf("Mode() = %q, want %q", a.Mode(), agent.ModeCLI)
	}
	if !a.Connected() {
		t.Error("Connected() = false after Connect")
	}

	// Second Connect while connected is a no-op success.
	if err := a.Connect(ctx); err != nil {
		t.Fatalf("second Connect() = %v", err)
	}

	ch, err := a.SendPrompt(ctx, "hello")
	if err != nil {
		t.Fatalf("SendPrompt() = %v", err)
	}
	_ = collectUntilTerminal(t, ch)

	// CLI mode: HandlePermission and Interrupt are documented no-ops.
	if err := a.HandlePermission("anything", true); err != nil {
		t.Errorf("HandlePermission() in CLI mode = %v, want nil", err)
	}
	if err := a.Interrupt(); err != nil {
		t.Errorf("Interrupt() in CLI mode = %v, want nil", err)
	}

	if err := a.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
}

// TestAgent_ConnectFails surfaces a connect error when the binary cannot start.
func TestAgent_ConnectFails(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Command = []string{filepath.Join(t.TempDir(), "does-not-exist")}
	a := NewWithConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.Connect(ctx); err == nil {
		t.Error("Connect() with missing binary should error")
		_ = a.Close()
	}
	if a.Connected() {
		t.Error("Connected() should be false after a failed Connect")
	}
}
