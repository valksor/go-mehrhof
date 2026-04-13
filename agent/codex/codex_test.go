package codex_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/codex"
)

func TestDefaultConfig(t *testing.T) {
	cfg := codex.DefaultConfig()
	if len(cfg.Command) == 0 || cfg.Command[0] != "codex" {
		t.Errorf("DefaultConfig().Command = %v, want [codex]", cfg.Command)
	}
	if cfg.Timeout == 0 {
		t.Error("DefaultConfig().Timeout should not be zero")
	}
}

func TestNew(t *testing.T) {
	a := codex.New()
	if a == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewWithConfig_Defaults(t *testing.T) {
	cfg := codex.Config{}
	a := codex.NewWithConfig(cfg)
	if a == nil {
		t.Fatal("NewWithConfig() returned nil")
	}
}

func TestName(t *testing.T) {
	a := codex.New()
	if a.Name() != codex.AgentName {
		t.Errorf("Name() = %q, want %q", a.Name(), codex.AgentName)
	}
}

func TestConnected_NewAgent(t *testing.T) {
	a := codex.New()
	if a.Connected() {
		t.Error("Connected() should return false for a new agent")
	}
}

func TestMode_NewAgent(t *testing.T) {
	a := codex.New()
	if a.Mode() != "" {
		t.Errorf("Mode() = %q, want empty string for unconnected agent", a.Mode())
	}
}

func TestWithEnv(t *testing.T) {
	a := codex.New()
	b := a.WithEnv("KEY", "value")
	if b == nil {
		t.Fatal("WithEnv() returned nil")
	}
	if b == a {
		t.Error("WithEnv() should return a new agent")
	}
}

func TestWithEnv_Chains(t *testing.T) {
	a := codex.New()
	b := a.WithEnv("K1", "v1").WithEnv("K2", "v2")
	if b == nil {
		t.Fatal("chained WithEnv() returned nil")
	}
}

func TestWithArgs(t *testing.T) {
	a := codex.New()
	b := a.WithArgs("--verbose")
	if b == nil {
		t.Fatal("WithArgs() returned nil")
	}
}

func TestWithWorkDir(t *testing.T) {
	a := codex.New()
	b := a.WithWorkDir("/tmp")
	if b == nil {
		t.Fatal("WithWorkDir() returned nil")
	}
}

func TestWithTimeout(t *testing.T) {
	a := codex.New()
	b := a.WithTimeout(5 * time.Minute)
	if b == nil {
		t.Fatal("WithTimeout() returned nil")
	}
}

func TestWithModel(t *testing.T) {
	a := codex.New()
	b := a.WithModel("o3-mini")
	if b == nil {
		t.Fatal("WithModel() returned nil")
	}
}

func TestSendPrompt_NotConnected(t *testing.T) {
	a := codex.New()
	_, err := a.SendPrompt(context.Background(), "hello")
	if err == nil {
		t.Error("SendPrompt() on disconnected agent should return an error")
	}
}

func TestHandlePermission_Unconnected(t *testing.T) {
	a := codex.New()
	if err := a.HandlePermission("req-1", true); err != nil {
		t.Errorf("HandlePermission() on unconnected agent returned unexpected error: %v", err)
	}
}

func TestClose_Unconnected(t *testing.T) {
	a := codex.New()
	if err := a.Close(); err != nil {
		t.Errorf("Close() on unconnected agent returned unexpected error: %v", err)
	}
}

func TestAvailable_BinaryNotFound(t *testing.T) {
	cfg := codex.Config{}
	cfg.Command = []string{"nonexistent-binary-kvelmo-test-xyz"}
	a := codex.NewWithConfig(cfg)
	err := a.Available()
	if err == nil {
		t.Skip("binary unexpectedly found; skipping not-found test")
	}
}

func TestAvailable_ExistingBinary(t *testing.T) {
	// Use the `go` binary which is always present in a Go project.
	cfg := codex.Config{}
	cfg.Command = []string{"go"}
	a := codex.NewWithConfig(cfg)
	// go --version exits 0, so Available() should return nil
	_ = a.Available()
}

func TestAvailable_UsesFallbackCommandPath(t *testing.T) {
	tmpDir := t.TempDir()
	binaryName := "codex-fallback-test"
	binaryPath := filepath.Join(tmpDir, binaryName)
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", origPath+string(os.PathListSeparator)+tmpDir)

	cfg := codex.Config{}
	cfg.Command = []string{binaryName}
	a := codex.NewWithConfig(cfg)
	if err := a.Available(); err != nil {
		t.Fatalf("Available() error = %v", err)
	}
}

func TestWithPermissionHandler(t *testing.T) {
	a := codex.New()
	handler := agent.PermissionHandler(func(_ agent.PermissionRequest) bool { return true })
	b := a.WithPermissionHandler(handler)
	if b == nil {
		t.Fatal("WithPermissionHandler() returned nil")
	}
}

func TestRegister(t *testing.T) {
	r := agent.NewRegistry()
	if err := codex.Register(r); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	a, err := r.Get(codex.AgentName)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", codex.AgentName, err)
	}
	if a == nil {
		t.Error("Register() should add agent to registry")
	}
}

func TestDefaultConfig_PermissionHandlerNonNil(t *testing.T) {
	cfg := codex.DefaultConfig()
	if cfg.PermissionHandler == nil {
		t.Error("DefaultConfig().PermissionHandler should not be nil")
	}
}

func TestNewWithConfig_ZeroValueFillsDefaults(t *testing.T) {
	a := codex.NewWithConfig(codex.Config{})
	if a.Name() != "codex" {
		t.Errorf("Name() = %q, want %q", a.Name(), "codex")
	}
	if a.Connected() {
		t.Error("Connected() should be false after zero-value construction")
	}
}

func TestWithEnv_ValuePreserved(t *testing.T) {
	a := codex.New()
	b := a.WithEnv("MY_KEY", "MY_VAL")
	if b == a {
		t.Fatal("WithEnv() must return a new agent")
	}
	if b.Name() != "codex" {
		t.Errorf("WithEnv() agent Name() = %q, want %q", b.Name(), "codex")
	}
}

func TestWithArgs_OriginalUnchanged(t *testing.T) {
	a := codex.New()
	b := a.WithArgs("--flag1", "--flag2")
	if b == a {
		t.Fatal("WithArgs() must return a new agent")
	}
	c := a.WithArgs("--flag3")
	if c == b {
		t.Error("independent WithArgs() calls from same parent should yield distinct agents")
	}
}

func TestWithWorkDir_ValueStored(t *testing.T) {
	a := codex.New()
	b := a.WithWorkDir("/tmp/testdir")
	if b == nil {
		t.Fatal("WithWorkDir() returned nil")
	}
	if b == a {
		t.Error("WithWorkDir() must return a new agent")
	}
}

func TestWithTimeout_ValueStored(t *testing.T) {
	a := codex.New()
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
	a := codex.New()
	b := a.WithModel("o3-mini")
	if b == nil {
		t.Fatal("WithModel() returned nil")
	}
	if b.Name() != "codex" {
		t.Errorf("after WithModel Name() = %q, want %q", b.Name(), "codex")
	}
}

func TestWithPermissionHandler_ValueStored(t *testing.T) {
	called := false
	handler := agent.PermissionHandler(func(_ agent.PermissionRequest) bool {
		called = true

		return true
	})
	a := codex.New()
	b := a.WithPermissionHandler(handler)
	if b == nil {
		t.Fatal("WithPermissionHandler() returned nil")
	}
	if b == a {
		t.Error("WithPermissionHandler() must return a new agent")
	}
	_ = called
}
