package claudemcp

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/internal/mcp"
	"github.com/valksor/kvelmo/internal/testutil"
)

// writeFakeClaude writes a POSIX-shell stand-in for the `claude` binary that
// emits a couple of transcript lines then blocks on a read so the session
// stays alive until the rendezvous "complete"/"failure" event (driven by the
// test) tells pump to interrupt it. This exercises startPTY/readLoop/waitLoop
// and pump without the real claude binary.
func writeFakeClaude(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("PTY fake-binary harness requires a POSIX shell")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-claude.sh")
	const script = `#!/bin/sh
case "$1" in
  --version) echo "claude 1.0.0-fake"; exit 0;;
esac
printf '%s\n' 'fake claude TUI line one'
printf '%s\n' 'fake claude TUI line two'
# Block until killed (pump interrupts us after the rendezvous event).
while :; do sleep 1; done
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}

	return path
}

// newLifecycleAgent builds an Agent wired to the fake claude binary with a
// short socket-friendly WorkRoot and a worktree dir, so SendPrompt can run all
// the way through the real PTY spawn against the fake binary.
func newLifecycleAgent(t *testing.T) *Agent {
	t.Helper()
	worktree := testutil.TempDir(t)
	workRoot := testutil.TempDir(t) // short /tmp path keeps adapter.sock < 104 chars
	cfg := Config{
		WorkRoot:       workRoot,
		PermissionMode: PermissionModeAcceptEdits,
	}
	cfg.Command = []string{writeFakeClaude(t)}
	cfg.WorkDir = worktree
	cfg.Timeout = 30 * time.Second

	return NewWithConfig(cfg)
}

// sendRendezvous connects to the adapter socket the running session created and
// writes one newline-delimited RendezvousEvent.
func sendRendezvous(t *testing.T, sockPath string, evt mcp.RendezvousEvent) {
	t.Helper()
	var conn net.Conn
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var d net.Dialer
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		conn, err = d.DialContext(ctx, "unix", sockPath)
		cancel()
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dial rendezvous %s: %v", sockPath, err)
	}
	defer func() { _ = conn.Close() }()
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if _, err := conn.Write(append(data, '\n')); err != nil {
		t.Fatalf("write event: %v", err)
	}
}

// collectUntilClosed drains the agent event channel until it closes (pump
// closes it after a terminal event), returning everything seen.
func collectUntilClosed(t *testing.T, ch <-chan agent.Event) []agent.Event {
	t.Helper()
	var events []agent.Event
	timeout := time.After(15 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, e)
		case <-timeout:
			t.Fatal("timed out waiting for event channel to close")

			return events
		}
	}
}

// TestAvailableWithFakeBinary covers the success path of Available(): the fake
// binary answers --version with exit 0, so resolution + the version probe both
// succeed.
func TestAvailableWithFakeBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY fake-binary harness requires a POSIX shell")
	}
	var cfg Config
	cfg.Command = []string{writeFakeClaude(t)}
	a := NewWithConfig(cfg)
	if err := a.Available(); err != nil {
		t.Errorf("Available() with fake --version = %v, want nil", err)
	}
}

// TestAvailableVersionFails covers the branch where the binary resolves but
// its --version invocation exits nonzero.
func TestAvailableVersionFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY fake-binary harness requires a POSIX shell")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "broken-claude.sh")
	// Always exits 1, so --version fails even though the binary resolves.
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var cfg Config
	cfg.Command = []string{path}
	a := NewWithConfig(cfg)
	if err := a.Available(); err == nil {
		t.Error("Available() with failing --version = nil, want error")
	}
}

// TestSendPromptLifecycleComplete drives SendPrompt end to end: real PTY spawn
// of the fake binary, stream lines pumped as EventStream, then a rendezvous
// "complete" event that produces EventComplete and tears the session down.
func TestSendPromptLifecycleComplete(t *testing.T) {
	a := newLifecycleAgent(t)

	ch, err := a.SendPrompt(context.Background(), "implement the thing")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	// Drain at least one transcript line before triggering completion. pump's
	// select picks randomly among ready cases, so sending the rendezvous event
	// before consuming the stream lines could race the session to "finished"
	// and drop the stream output. Reading the stream event first makes the
	// ordering deterministic.
	sawStream := false
	streamTimeout := time.After(10 * time.Second)
streamLoop:
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				t.Fatal("channel closed before any stream event")
			}
			if e.Type == agent.EventStream {
				// readLoop emits raw PTY chunks (not trimmed lines), so the
				// fake transcript may arrive with trailing newlines or several
				// lines coalesced into one chunk — match by substring.
				if strings.Contains(e.Content, "fake claude TUI line one") {
					sawStream = true
				}
				if raw, _ := e.Data["raw_ansi"].(bool); !raw {
					t.Errorf("stream event missing raw_ansi marker: %+v", e)
				}

				break streamLoop
			}
		case <-streamTimeout:
			t.Fatal("timed out waiting for first stream event")
		}
	}

	// The adapter socket lives in the per-session work dir.
	sockPath := filepath.Join(a.workDir, "adapter.sock")
	sendRendezvous(t, sockPath, mcp.RendezvousEvent{
		Type:    "complete",
		Phase:   "implement",
		Summary: "all done",
	})

	events := collectUntilClosed(t, ch)

	var sawComplete bool
	var completeSummary, completePhase string
	for _, e := range events {
		if e.Type == agent.EventComplete {
			sawComplete = true
			completeSummary = e.Content
			completePhase, _ = e.Data["phase"].(string)
		}
	}
	if !sawStream {
		t.Error("expected an EventStream with fake TUI content")
	}
	if !sawComplete {
		t.Fatalf("expected an EventComplete, got %+v", events)
	}
	if completeSummary != "all done" {
		t.Errorf("complete summary = %q, want %q", completeSummary, "all done")
	}
	if completePhase != "implement" {
		t.Errorf("complete phase = %q, want %q", completePhase, "implement")
	}

	// After a terminal event the session resources are released, so a fresh
	// SendPrompt is allowed again (no "session already active" error).
	if a.pty != nil {
		t.Error("pty should be nil after session completes")
	}

	_ = a.Close()
}

// TestSendPromptLifecycleFailure drives the rendezvous "failure" branch of pump
// and asserts an EventError carrying the phase, reason, and retryable flag.
func TestSendPromptLifecycleFailure(t *testing.T) {
	a := newLifecycleAgent(t)

	ch, err := a.SendPrompt(context.Background(), "do it")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	sockPath := filepath.Join(a.workDir, "adapter.sock")
	sendRendezvous(t, sockPath, mcp.RendezvousEvent{
		Type:      "failure",
		Phase:     "optimize",
		Reason:    "compiler exploded",
		Retryable: true,
	})

	events := collectUntilClosed(t, ch)

	var sawError bool
	for _, e := range events {
		if e.Type == agent.EventError {
			sawError = true
			if e.Error == "" {
				t.Error("error event has empty Error string")
			}
			if got, _ := e.Data["phase"].(string); got != "optimize" {
				t.Errorf("error phase = %q, want optimize", got)
			}
			if retryable, _ := e.Data["retryable"].(bool); !retryable {
				t.Error("error event missing retryable=true")
			}
		}
	}
	if !sawError {
		t.Fatalf("expected an EventError, got %+v", events)
	}

	_ = a.Close()
}

// TestSendPromptAlreadyActive verifies the single-session guard: a second
// SendPrompt while one is active returns an error and does not disturb the
// running session.
func TestSendPromptAlreadyActive(t *testing.T) {
	a := newLifecycleAgent(t)

	ch, err := a.SendPrompt(context.Background(), "first")
	if err != nil {
		t.Fatalf("first SendPrompt: %v", err)
	}

	if _, err := a.SendPrompt(context.Background(), "second"); err == nil {
		t.Error("second SendPrompt while active = nil, want error")
	}

	// Complete the first session so resources are released cleanly.
	sockPath := filepath.Join(a.workDir, "adapter.sock")
	sendRendezvous(t, sockPath, mcp.RendezvousEvent{Type: "complete", Phase: "plan"})
	_ = collectUntilClosed(t, ch)
	_ = a.Close()
}

// TestSendPromptContextCancel drives the ctx.Done() branch of pump: cancelling
// the context produces an EventError and interrupts the PTY.
func TestSendPromptContextCancel(t *testing.T) {
	a := newLifecycleAgent(t)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := a.SendPrompt(ctx, "long running")
	if err != nil {
		cancel()
		t.Fatalf("SendPrompt: %v", err)
	}

	// Let the fake binary emit at least its first line, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	events := collectUntilClosed(t, ch)
	var sawCancelErr bool
	for _, e := range events {
		if e.Type == agent.EventError && e.Error == "claudemcp: context cancelled" {
			sawCancelErr = true
		}
	}
	if !sawCancelErr {
		t.Errorf("expected context-cancelled EventError, got %+v", events)
	}
	_ = a.Close()
}

// TestSendPromptInterrupt verifies Interrupt terminates an active session,
// which causes the PTY to exit and pump to emit the no-completion error.
func TestSendPromptInterrupt(t *testing.T) {
	a := newLifecycleAgent(t)

	ch, err := a.SendPrompt(context.Background(), "interrupt me")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if err := a.Interrupt(); err != nil {
		t.Errorf("Interrupt: %v", err)
	}

	events := collectUntilClosed(t, ch)
	// When the PTY is killed without a rendezvous completion, pump records the
	// "exited without signalling completion" error.
	var sawExitErr bool
	for _, e := range events {
		if e.Type == agent.EventError {
			sawExitErr = true
		}
	}
	if !sawExitErr {
		t.Errorf("expected an EventError after interrupt, got %+v", events)
	}
	_ = a.Close()
}

// TestSendPromptStandaloneIdleCompletes verifies the serverless completion path:
// a standalone session (no conductor/worktree socket — what `kvelmo pipe` uses)
// has no rendezvous signal_complete coming, because the interactive claude TUI
// answers a one-shot prompt then idles. The fake claude emits a couple of lines
// then goes quiet; pump must surface an EventComplete (phase "pipe", reason
// "idle") once the idle grace elapses instead of hanging until ctx deadline.
func TestSendPromptStandaloneIdleCompletes(t *testing.T) {
	a := newLifecycleAgent(t)
	a.idleGrace = 400 * time.Millisecond // fire promptly once the fake goes quiet

	ch, err := a.SendPrompt(context.Background(), "list the files")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	events := collectUntilClosed(t, ch)

	var sawStream, sawComplete bool
	var phase, reason string
	for _, e := range events {
		if e.Type == agent.EventStream && strings.Contains(e.Content, "fake claude TUI line one") {
			sawStream = true
		}
		if e.Type == agent.EventComplete {
			sawComplete = true
			phase, _ = e.Data["phase"].(string)
			reason, _ = e.Data["reason"].(string)
		}
		if e.Type == agent.EventError {
			t.Fatalf("unexpected error event in idle path: %s", e.Error)
		}
	}
	if !sawStream {
		t.Error("expected an EventStream with the fake transcript before idle completion")
	}
	if !sawComplete {
		t.Fatalf("expected an idle EventComplete, got %+v", events)
	}
	if phase != "pipe" || reason != "idle" {
		t.Errorf("idle complete: phase=%q reason=%q, want phase=pipe reason=idle", phase, reason)
	}
	if a.pty != nil {
		t.Error("pty should be nil after idle completion")
	}
	_ = a.Close()
}
