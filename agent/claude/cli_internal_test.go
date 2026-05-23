package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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

// TestHandleMessage drives every branch of the stream-json decoder by feeding
// it the raw NDJSON shapes the Claude CLI emits and asserting the translated
// agent.Event. handleMessage is the bulk of the CLI adapter's logic and needs
// no subprocess, so it is tested directly.
func TestHandleMessage(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantNone    bool
		wantType    agent.EventType
		wantContent string
	}{
		{
			name:        "assistant with content",
			raw:         `{"type":"assistant","content":"hi there"}`,
			wantType:    agent.EventStream,
			wantContent: "hi there",
		},
		{
			name:        "assistant with delta text",
			raw:         `{"type":"assistant","delta":{"type":"text_delta","text":"from delta"}}`,
			wantType:    agent.EventStream,
			wantContent: "from delta",
		},
		{
			name:     "assistant empty emits nothing",
			raw:      `{"type":"assistant"}`,
			wantNone: true,
		},
		{
			name:        "content_block_delta",
			raw:         `{"type":"content_block_delta","delta":{"type":"text_delta","text":"chunk"}}`,
			wantType:    agent.EventStream,
			wantContent: "chunk",
		},
		{
			name:        "tool_use",
			raw:         `{"type":"tool_use","id":"tu-1","name":"Read","input":{"path":"/x"}}`,
			wantType:    agent.EventToolUse,
			wantContent: "Read",
		},
		{
			name:        "tool_result",
			raw:         `{"type":"tool_result","tool_use_id":"tu-1","result":"done"}`,
			wantType:    agent.EventToolResult,
			wantContent: "done",
		},
		{
			name:     "result success",
			raw:      `{"type":"result","subtype":"success"}`,
			wantType: agent.EventComplete,
		},
		{
			name:     "result error",
			raw:      `{"type":"result","subtype":"error","is_error":true,"error":"boom"}`,
			wantType: agent.EventError,
		},
		{
			name:     "error message",
			raw:      `{"type":"error","error":"oops","message":"detail"}`,
			wantType: agent.EventError,
		},
		{
			name:     "message_stop",
			raw:      `{"type":"message_stop"}`,
			wantType: agent.EventComplete,
		},
		{
			name:     "unknown type emits nothing",
			raw:      `{"type":"something_new"}`,
			wantNone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg cliMessage
			if err := json.Unmarshal([]byte(tt.raw), &msg); err != nil {
				t.Fatalf("unmarshal test input: %v", err)
			}

			c := NewCLIConnection(DefaultConfig())
			c.handleMessage(msg)

			if tt.wantNone {
				select {
				case e := <-c.events:
					t.Fatalf("expected no event, got %+v", e)
				case <-time.After(50 * time.Millisecond):
				}

				return
			}

			e, ok := recvEvent(t, c.events)
			if !ok {
				t.Fatal("channel closed without event")
			}
			if e.Type != tt.wantType {
				t.Errorf("event type = %v, want %v", e.Type, tt.wantType)
			}
			if tt.wantContent != "" && e.Content != tt.wantContent {
				t.Errorf("event content = %q, want %q", e.Content, tt.wantContent)
			}
			if tt.wantType == agent.EventError && e.Error == "" {
				t.Error("error event should carry a non-empty Error")
			}
		})
	}
}

// TestBuildArgs verifies the fixed stream-json flags plus model/extra-arg wiring.
func TestBuildArgs(t *testing.T) {
	base := NewCLIConnection(DefaultConfig()).buildArgs()
	for _, want := range []string{
		"--print", "--verbose", "--output-format", "stream-json",
		"--input-format", "--permission-mode", "bypassPermissions",
	} {
		if !slices.Contains(base, want) {
			t.Errorf("buildArgs() = %v, missing %q", base, want)
		}
	}

	cfg := DefaultConfig()
	cfg.Model = "claude-sonnet-test"
	withModel := NewCLIConnection(cfg).buildArgs()
	if !slices.Contains(withModel, "--model") || !slices.Contains(withModel, "claude-sonnet-test") {
		t.Errorf("buildArgs() with model = %v, want --model claude-sonnet-test", withModel)
	}

	cfg2 := DefaultConfig()
	cfg2.Args = []string{"--extra-flag"}
	withArgs := NewCLIConnection(cfg2).buildArgs()
	if !slices.Contains(withArgs, "--extra-flag") {
		t.Errorf("buildArgs() with extra args = %v, want --extra-flag", withArgs)
	}
}

func TestCLIConnection_SendPromptNotConnected(t *testing.T) {
	c := NewCLIConnection(DefaultConfig())
	if _, err := c.SendPrompt(context.Background(), "hi"); err == nil {
		t.Error("SendPrompt before Connect should error")
	}
}

func TestCLIConnection_CloseIdempotent(t *testing.T) {
	c := NewCLIConnection(DefaultConfig())
	if err := c.Close(); err != nil {
		t.Errorf("first Close() = %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second Close() = %v", err)
	}
}

func TestCLIConnection_HandlePermissionNoop(t *testing.T) {
	c := NewCLIConnection(DefaultConfig())
	if err := c.HandlePermission("req-1", true); err != nil {
		t.Errorf("HandlePermission() = %v, want nil (CLI no-op)", err)
	}
}

// writeFakeClaude writes a POSIX-shell stand-in for the `claude` binary. It
// answers --version for Available(), and otherwise waits for the stream-json
// prompt on stdin before emitting a scripted transcript — so emission always
// happens after SendPrompt swaps in the per-prompt event channel.
func writeFakeClaude(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-binary harness requires a POSIX shell")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-claude.sh")
	const script = `#!/bin/sh
case "$1" in
  --version) echo "claude 1.0.0-fake"; exit 0;;
esac
echo "fake stderr noise" >&2
read _line
printf '%s\n' '{"type":"assistant","content":"hello from fake"}'
printf '%s\n' '{"type":"tool_use","id":"t1","name":"Read","input":{}}'
printf '%s\n' '{"type":"result","subtype":"success"}'
# Stay alive until Close() shuts stdin (real claude keeps the session open too).
# Exiting immediately here lets the process close the event stream before the
# consumer drains it, which races on a fast runner and drops the events.
read _hold
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}

	return path
}

// collectUntilTerminal drains a SendPrompt channel until it closes (the filtered
// goroutine closes it after a terminal event).
func collectUntilTerminal(t *testing.T, ch <-chan agent.Event) []agent.Event {
	t.Helper()
	var events []agent.Event
	timeout := time.After(5 * time.Second)
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

// TestCLIConnection_Lifecycle exercises Connect → SendPrompt → readOutput →
// handleMessage → Close end to end against the fake binary, with no real claude.
func TestCLIConnection_Lifecycle(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Command = []string{writeFakeClaude(t)}
	c := NewCLIConnection(cfg)

	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect() = %v", err)
	}
	if !c.Connected() {
		t.Error("Connected() = false after Connect")
	}

	ch, err := c.SendPrompt(ctx, "do the thing")
	if err != nil {
		t.Fatalf("SendPrompt() = %v", err)
	}

	events := collectUntilTerminal(t, ch)
	var sawStream, sawComplete bool
	for _, e := range events {
		switch e.Type {
		case agent.EventStream:
			if e.Content == "hello from fake" {
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
	}
	if !sawStream {
		t.Errorf("expected a stream event with the fake content, got %+v", events)
	}
	if !sawComplete {
		t.Errorf("expected a completion event, got %+v", events)
	}

	if err := c.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
}

// TestAgent_Lifecycle covers the Agent wrapper's Connect/SendPrompt/Close/
// Interrupt against the fake binary.
func TestAgent_Lifecycle(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Command = []string{writeFakeClaude(t)}
	a := NewWithConfig(cfg)

	ctx := context.Background()
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

	ch, err := a.SendPrompt(ctx, "hi")
	if err != nil {
		t.Fatalf("SendPrompt() = %v", err)
	}
	if events := collectUntilTerminal(t, ch); len(events) == 0 {
		t.Error("SendPrompt produced no events")
	}

	if err := a.Interrupt(); err != nil {
		t.Errorf("Interrupt() = %v", err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
}

func TestAgent_AvailableWithFakeBinary(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Command = []string{writeFakeClaude(t)}
	a := NewWithConfig(cfg)
	if err := a.Available(); err != nil {
		t.Errorf("Available() with fake --version = %v, want nil", err)
	}
}

func TestRegisterWithPermissionHandler(t *testing.T) {
	r := agent.NewRegistry()
	handler := agent.PermissionHandler(func(_ agent.PermissionRequest) bool { return true })
	if err := RegisterWithPermissionHandler(r, handler); err != nil {
		t.Fatalf("RegisterWithPermissionHandler() = %v", err)
	}
	if _, err := r.Get(AgentName); err != nil {
		t.Errorf("Get(%q) after register = %v", AgentName, err)
	}
}
