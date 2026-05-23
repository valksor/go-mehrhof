package claudemcp

import (
	"slices"
	"testing"
	"time"
)

// TestDefaultConfig asserts the documented defaults of DefaultConfig.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.PermissionMode != PermissionModeAcceptEdits {
		t.Errorf("PermissionMode = %q, want %q", cfg.PermissionMode, PermissionModeAcceptEdits)
	}
	if !cfg.StrictMCPConfig {
		t.Error("StrictMCPConfig = false, want true (default isolates MCP servers)")
	}
	if !slices.Equal(cfg.Command, []string{"claude"}) {
		t.Errorf("Command = %v, want [claude]", cfg.Command)
	}
	if !slices.Equal(cfg.MCPServerCommand, []string{"kvelmo", "mcp", "--stdio"}) {
		t.Errorf("MCPServerCommand = %v, want [kvelmo mcp --stdio]", cfg.MCPServerCommand)
	}
	// agent.DefaultConfig() seeds a 30m timeout, so the claudemcp 60m fallback
	// (guarded on Timeout == 0) never fires for DefaultConfig.
	if cfg.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want 30m", cfg.Timeout)
	}
	// The embedded agent.Config defaults should be present too.
	if cfg.RetryCount == 0 {
		t.Error("embedded agent.Config defaults not applied (RetryCount = 0)")
	}
}
