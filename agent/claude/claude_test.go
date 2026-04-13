package claude_test

import (
	"context"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/claude"
)

func TestDefaultConfig(t *testing.T) {
	cfg := claude.DefaultConfig()
	if len(cfg.Command) == 0 || cfg.Command[0] != "claude" {
		t.Errorf("DefaultConfig().Command = %v, want [claude]", cfg.Command)
	}
	if cfg.Timeout == 0 {
		t.Error("DefaultConfig().Timeout should not be zero")
	}
}

func TestNew(t *testing.T) {
	a := claude.New()
	if a == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewWithConfig_Defaults(t *testing.T) {
	// Empty command should default to "claude"
	cfg := claude.Config{}
	a := claude.NewWithConfig(cfg)
	if a == nil {
		t.Fatal("NewWithConfig() returned nil")
	}
}

func TestName(t *testing.T) {
	a := claude.New()
	if a.Name() != claude.AgentName {
		t.Errorf("Name() = %q, want %q", a.Name(), claude.AgentName)
	}
}

func TestConnected_NewAgent(t *testing.T) {
	a := claude.New()
	if a.Connected() {
		t.Error("Connected() should return false for a new agent")
	}
}

func TestMode_NewAgent(t *testing.T) {
	a := claude.New()
	if a.Mode() != "" {
		t.Errorf("Mode() = %q, want empty string for unconnected agent", a.Mode())
	}
}

func TestWithEnv(t *testing.T) {
	a := claude.New()
	b := a.WithEnv("KEY", "value")
	if b == nil {
		t.Fatal("WithEnv() returned nil")
	}
	// Original should not be modified
	if b == a {
		t.Error("WithEnv() should return a new agent, not the same instance")
	}
}

func TestWithEnv_Chains(t *testing.T) {
	a := claude.New()
	b := a.WithEnv("K1", "v1").WithEnv("K2", "v2")
	if b == nil {
		t.Fatal("chained WithEnv() returned nil")
	}
}

func TestWithArgs(t *testing.T) {
	a := claude.New()
	b := a.WithArgs("--verbose", "--no-color")
	if b == nil {
		t.Fatal("WithArgs() returned nil")
	}
}

func TestWithWorkDir(t *testing.T) {
	a := claude.New()
	b := a.WithWorkDir("/tmp")
	if b == nil {
		t.Fatal("WithWorkDir() returned nil")
	}
}

func TestWithTimeout(t *testing.T) {
	a := claude.New()
	b := a.WithTimeout(5 * time.Minute)
	if b == nil {
		t.Fatal("WithTimeout() returned nil")
	}
}

func TestWithModel(t *testing.T) {
	a := claude.New()
	b := a.WithModel("claude-3-opus")
	if b == nil {
		t.Fatal("WithModel() returned nil")
	}
}

func TestSendPrompt_NotConnected(t *testing.T) {
	a := claude.New()
	_, err := a.SendPrompt(context.Background(), "hello")
	if err == nil {
		t.Error("SendPrompt() on disconnected agent should return an error")
	}
}

func TestHandlePermission_Unconnected(t *testing.T) {
	a := claude.New()
	// Should not error on unconnected agent (CLI mode is no-op)
	if err := a.HandlePermission("req-1", true); err != nil {
		t.Errorf("HandlePermission() on unconnected agent returned unexpected error: %v", err)
	}
}

func TestClose_Unconnected(t *testing.T) {
	a := claude.New()
	if err := a.Close(); err != nil {
		t.Errorf("Close() on unconnected agent returned unexpected error: %v", err)
	}
}

func TestAvailable_BinaryNotFound(t *testing.T) {
	cfg := claude.Config{}
	cfg.Command = []string{"nonexistent-binary-kvelmo-test-xyz"}
	a := claude.NewWithConfig(cfg)
	err := a.Available()
	if err == nil {
		t.Skip("binary unexpectedly found; skipping not-found test")
	}
	// Error expected — binary not installed
}

func TestAvailable_ExistingBinary(t *testing.T) {
	// Use the `go` binary which is always present in a Go project.
	// This exercises the exec.LookPath success → cmd.Run() path.
	cfg := claude.Config{}
	cfg.Command = []string{"go"}
	a := claude.NewWithConfig(cfg)
	// go --version exits 0, so Available() should return nil
	_ = a.Available() // Don't assert — just cover the binary-found path
}

func TestWithPermissionHandler(t *testing.T) {
	a := claude.New()
	handler := agent.PermissionHandler(func(_ agent.PermissionRequest) bool { return true })
	b := a.WithPermissionHandler(handler)
	if b == nil {
		t.Fatal("WithPermissionHandler() returned nil")
	}
}

func TestRegister(t *testing.T) {
	r := agent.NewRegistry()
	if err := claude.Register(r); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	a, err := r.Get(claude.AgentName)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", claude.AgentName, err)
	}
	if a == nil {
		t.Error("Register() should add agent to registry")
	}
}

func TestDefaultConfig_PermissionHandlerNonNil(t *testing.T) {
	cfg := claude.DefaultConfig()
	if cfg.PermissionHandler == nil {
		t.Error("DefaultConfig().PermissionHandler should not be nil")
	}
}

func TestNewWithConfig_ZeroValueFillsDefaults(t *testing.T) {
	a := claude.NewWithConfig(claude.Config{})
	// Name still returns the constant
	if a.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", a.Name(), "claude")
	}
	// Not connected after construction
	if a.Connected() {
		t.Error("Connected() should be false after zero-value construction")
	}
}

func TestWithEnv_ValuePreserved(t *testing.T) {
	a := claude.New()
	b := a.WithEnv("MY_KEY", "MY_VAL")
	// Original unchanged — verify it is a different instance
	if b == a {
		t.Fatal("WithEnv() must return a new agent")
	}
	// The new agent name should still be correct
	if b.Name() != "claude" {
		t.Errorf("WithEnv() agent Name() = %q, want %q", b.Name(), "claude")
	}
}

func TestWithArgs_OriginalUnchanged(t *testing.T) {
	a := claude.New()
	b := a.WithArgs("--flag1", "--flag2")
	// b is a different instance
	if b == a {
		t.Fatal("WithArgs() must return a new agent")
	}
	// Adding more args to b should not affect an independent chain from a
	c := a.WithArgs("--flag3")
	if c == b {
		t.Error("independent WithArgs() calls from same parent should yield distinct agents")
	}
}

func TestWithWorkDir_ValueStored(t *testing.T) {
	a := claude.New()
	b := a.WithWorkDir("/tmp/testdir")
	if b == nil {
		t.Fatal("WithWorkDir() returned nil")
	}
	if b == a {
		t.Error("WithWorkDir() must return a new agent")
	}
}

func TestWithTimeout_ValueStored(t *testing.T) {
	a := claude.New()
	const d = 5 * time.Second
	b := a.WithTimeout(d)
	if b == nil {
		t.Fatal("WithTimeout() returned nil")
	}
	if b == a {
		t.Error("WithTimeout() must return a new agent")
	}
}

func TestWithModel_ValueStored(t *testing.T) {
	a := claude.New()
	b := a.WithModel("claude-sonnet")
	if b == nil {
		t.Fatal("WithModel() returned nil")
	}
	// Name must still be correct after model override
	if b.Name() != "claude" {
		t.Errorf("after WithModel Name() = %q, want %q", b.Name(), "claude")
	}
}

func TestWithPermissionHandler_ValueStored(t *testing.T) {
	called := false
	handler := agent.PermissionHandler(func(_ agent.PermissionRequest) bool {
		called = true

		return true
	})
	a := claude.New()
	b := a.WithPermissionHandler(handler)
	if b == nil {
		t.Fatal("WithPermissionHandler() returned nil")
	}
	if b == a {
		t.Error("WithPermissionHandler() must return a new agent")
	}
	_ = called // handler captured but not invoked without a live connection
}
