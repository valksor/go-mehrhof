// Package claudemcp implements the "claude-mcp" agent mode: an interactive
// Claude Code session launched under a PTY with a kvelmo-served MCP server
// wired in via --mcp-config. The point is to keep usage on the Max
// subscription (interactive Claude Code is sub-billed) instead of routing
// through `claude --print` (which moves to the new $200 Agent SDK credit
// pool on 2026-06-15).
//
// This adapter is a sibling to agent/claude — the existing claude adapter is
// untouched, and custom agents that `extends: claude` (such as the user's
// `glm`) continue to work unchanged.
package claudemcp

import (
	"time"

	"github.com/valksor/kvelmo/agent"
)

const AgentName = "claude-mcp"

// PermissionMode values accepted by the claude CLI's --permission-mode flag.
const (
	PermissionModeAcceptEdits      = "acceptEdits"       // default — Claude accepts file edits without prompting but still gates dangerous tools
	PermissionModeAuto             = "auto"              // auto mode — fully autonomous
	PermissionModeBypassPermission = "bypassPermissions" // bypass all permissions
	PermissionModeDontAsk          = "dontAsk"           // deny unless explicitly allowed
	PermissionModeDefault          = "default"           // CLI default (interactive prompts)
)

// Config holds claude-mcp adapter configuration.
type Config struct {
	agent.Config

	// Model selects the claude model (e.g. "sonnet", "opus", or a full model ID).
	Model string

	// PermissionMode controls --permission-mode. Default acceptEdits.
	PermissionMode string

	// MCPServerCommand is the command (and args) that claude spawns as the MCP
	// server. Default: ["kvelmo", "mcp", "--stdio"].
	MCPServerCommand []string

	// SystemPromptOverride, if non-empty, replaces the default kvelmo
	// orchestration system prompt that the adapter writes to disk and
	// references via --append-system-prompt.
	SystemPromptOverride string

	// WorkRoot is the directory where the adapter writes the per-task
	// mcp-config.json, system-prompt.md, and adapter.sock. Defaults to
	// $KVELMO_HOME/work or ~/.valksor/kvelmo/work.
	WorkRoot string

	// StrictMCPConfig adds --strict-mcp-config so claude only uses the MCP
	// servers in our generated config (ignores user defaults). Default true.
	StrictMCPConfig bool
}

// DefaultConfig returns sensible defaults for the claude-mcp adapter.
func DefaultConfig() Config {
	cfg := Config{
		Config:          agent.DefaultConfig(),
		PermissionMode:  PermissionModeAcceptEdits,
		StrictMCPConfig: true,
	}
	cfg.Command = []string{"claude"}
	cfg.MCPServerCommand = []string{"kvelmo", "mcp", "--stdio"}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Minute
	}

	return cfg
}
