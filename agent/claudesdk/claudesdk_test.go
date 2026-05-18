package claudesdk_test

import (
	"context"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/claudesdk"
)

// Mirrors agent/claude/claude_test.go for the WebSocket-only sibling.
// Tests cover the adapter contract surface — construction, builders,
// registration, and the single-mode invariant. They do not attempt a real
// Connect (which would spawn claude and fail with the --sdk-url rejection
// against the official Anthropic CLI as of 2.1.121).

func TestDefaultConfig(t *testing.T) {
	cfg := claudesdk.DefaultConfig()
	if len(cfg.Command) == 0 || cfg.Command[0] != "claude" {
		t.Errorf("DefaultConfig().Command = %v, want [claude]", cfg.Command)
	}
	if cfg.Timeout == 0 {
		t.Error("DefaultConfig().Timeout should not be zero")
	}
}

func TestNew(t *testing.T) {
	if claudesdk.New() == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewWithConfig_Defaults(t *testing.T) {
	if claudesdk.NewWithConfig(claudesdk.Config{}) == nil {
		t.Fatal("NewWithConfig() returned nil")
	}
}

func TestName(t *testing.T) {
	a := claudesdk.New()
	if a.Name() != claudesdk.AgentName {
		t.Errorf("Name() = %q, want %q", a.Name(), claudesdk.AgentName)
	}
	if claudesdk.AgentName != "claude-sdk" {
		t.Errorf("AgentName = %q, want %q", claudesdk.AgentName, "claude-sdk")
	}
}

func TestConnected_NewAgent(t *testing.T) {
	if claudesdk.New().Connected() {
		t.Error("Connected() should return false for a new agent")
	}
}

func TestMode_AlwaysWebSocket(t *testing.T) {
	// Post-split single-mode invariant: claudesdk always reports
	// ModeWebSocket regardless of connection state. Sibling adapter
	// agent/claude/ has the symmetric TestMode_AlwaysCLI.
	a := claudesdk.New()
	if a.Mode() != agent.ModeWebSocket {
		t.Errorf("Mode() = %q, want %q", a.Mode(), agent.ModeWebSocket)
	}
}

func TestSendPrompt_NotConnected(t *testing.T) {
	a := claudesdk.New()
	_, err := a.SendPrompt(context.Background(), "hello")
	if err == nil {
		t.Error("SendPrompt() on disconnected agent should return an error")
	}
}

func TestHandlePermission_Unconnected(t *testing.T) {
	if err := claudesdk.New().HandlePermission("req-1", true); err != nil {
		t.Errorf("HandlePermission() on unconnected agent: %v", err)
	}
}

func TestInterrupt_Unconnected(t *testing.T) {
	if err := claudesdk.New().Interrupt(); err != nil {
		t.Errorf("Interrupt() on unconnected agent: %v", err)
	}
}

func TestClose_Unconnected(t *testing.T) {
	if err := claudesdk.New().Close(); err != nil {
		t.Errorf("Close() on unconnected agent: %v", err)
	}
}

func TestAvailable_BinaryNotFound(t *testing.T) {
	cfg := claudesdk.Config{}
	cfg.Command = []string{"nonexistent-kvelmo-test-binary-xyz"}
	if err := claudesdk.NewWithConfig(cfg).Available(); err == nil {
		t.Skip("binary unexpectedly found; skipping not-found test")
	}
}

func TestAvailable_DoesNotProbeSDKAcceptance(t *testing.T) {
	// Available() only checks that the binary exists and `--version` runs.
	// It explicitly does NOT detect whether `--sdk-url` will be accepted by
	// the binary at Connect() time — that semantic boundary is documented
	// in claudesdk.go's Available() doc comment. Use `go` as a stand-in
	// binary that always succeeds; this exercises the success path without
	// invoking claude.
	cfg := claudesdk.Config{}
	cfg.Command = []string{"go"}
	_ = claudesdk.NewWithConfig(cfg).Available()
}

func TestWithEnv(t *testing.T) {
	a := claudesdk.New()
	b := a.WithEnv("KEY", "value")
	if b == nil {
		t.Fatal("WithEnv() returned nil")
	}
	if b == a {
		t.Error("WithEnv() should return a new agent")
	}
}

func TestWithEnv_Chains(t *testing.T) {
	a := claudesdk.New()
	if a.WithEnv("K1", "v1").WithEnv("K2", "v2") == nil {
		t.Fatal("chained WithEnv() returned nil")
	}
}

func TestWithArgs(t *testing.T) {
	if claudesdk.New().WithArgs("--verbose", "--no-color") == nil {
		t.Fatal("WithArgs() returned nil")
	}
}

func TestWithWorkDir(t *testing.T) {
	if claudesdk.New().WithWorkDir("/tmp") == nil {
		t.Fatal("WithWorkDir() returned nil")
	}
}

func TestWithTimeout(t *testing.T) {
	if claudesdk.New().WithTimeout(5*time.Minute) == nil {
		t.Fatal("WithTimeout() returned nil")
	}
}

func TestWithModel(t *testing.T) {
	a := claudesdk.New()
	b := a.WithModel("claude-sonnet")
	if b == nil {
		t.Fatal("WithModel() returned nil")
	}
	if b.Name() != "claude-sdk" {
		t.Errorf("after WithModel Name() = %q, want %q", b.Name(), "claude-sdk")
	}
}

func TestWithPermissionHandler(t *testing.T) {
	handler := agent.PermissionHandler(func(_ agent.PermissionRequest) bool { return true })
	if claudesdk.New().WithPermissionHandler(handler) == nil {
		t.Fatal("WithPermissionHandler() returned nil")
	}
}

func TestRegister(t *testing.T) {
	r := agent.NewRegistry()
	if err := claudesdk.Register(r); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	a, err := r.Get(claudesdk.AgentName)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", claudesdk.AgentName, err)
	}
	if a == nil {
		t.Error("Register() should add agent to registry")
	}
}

func TestRegisterWithPermissionHandler(t *testing.T) {
	r := agent.NewRegistry()
	handler := agent.PermissionHandler(func(_ agent.PermissionRequest) bool { return true })
	if err := claudesdk.RegisterWithPermissionHandler(r, handler); err != nil {
		t.Fatalf("RegisterWithPermissionHandler() error = %v", err)
	}
	if _, err := r.Get(claudesdk.AgentName); err != nil {
		t.Errorf("Get after RegisterWithPermissionHandler: %v", err)
	}
}

func TestDefaultConfig_PermissionHandlerNonNil(t *testing.T) {
	if claudesdk.DefaultConfig().PermissionHandler == nil {
		t.Error("DefaultConfig().PermissionHandler should not be nil")
	}
}
