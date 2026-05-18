package claudesdk

import (
	"errors"
	"strings"
	"testing"
)

// earlyExitError is the helper that formats the Connect() error when the
// child claude process exits before completing the WebSocket handshake.
// Its message contains the actionable hint pointing operators at the
// `claude` (CLI) or `claude-mcp` (TUI) sibling adapters. If the message
// text drifts, anyone who hits the broken `claude-sdk` path against the
// official Anthropic CLI loses that hint. Lock the contract here.

func TestEarlyExitError_WithCmdErr(t *testing.T) {
	w := &WebSocketConnection{cmdErr: errors.New("exit status 1")}
	got := earlyExitError(w, "before WebSocket connection").Error()
	wantFragments := []string{
		"before WebSocket connection",
		"exit status 1",
		"rejects --sdk-url",
		"claude", "claude-mcp",
	}
	for _, frag := range wantFragments {
		if !strings.Contains(got, frag) {
			t.Errorf("earlyExitError missing %q\nfull message: %s", frag, got)
		}
	}
}

func TestEarlyExitError_NilCmdErr(t *testing.T) {
	// Fallback path: cmd.Wait() hasn't populated cmdErr yet (race with
	// external Close()). Message must still carry the actionable hint.
	w := &WebSocketConnection{cmdErr: nil}
	got := earlyExitError(w, "before session initialization").Error()
	wantFragments := []string{
		"before session initialization",
		"without error code",
		"rejects --sdk-url",
		"claude", "claude-mcp",
	}
	for _, frag := range wantFragments {
		if !strings.Contains(got, frag) {
			t.Errorf("earlyExitError missing %q\nfull message: %s", frag, got)
		}
	}
}
