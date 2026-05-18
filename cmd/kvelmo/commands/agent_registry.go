package commands

import (
	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/claudemcp"
	"github.com/valksor/kvelmo/settings"
)

// registerClaudeMCP registers the claude-mcp adapter with user yaml settings
// applied. The bare default is used when no settings are loaded. Lives in a
// shared file rather than start.go because every entry point that builds an
// agent registry (start, serve, changelog, pipe) must call it — the default
// agent is `claude-mcp` and omitting it breaks the default config.
func registerClaudeMCP(registry *agent.Registry, effective *settings.Settings) error {
	cfg := claudemcp.DefaultConfig()
	if effective != nil {
		s := effective.Agent.ClaudeMCP
		if s.Model != "" {
			cfg.Model = s.Model
		}
		if s.PermissionMode != "" {
			cfg.PermissionMode = s.PermissionMode
		}
		if len(s.ExtraArgs) > 0 {
			cfg.Args = append(append([]string(nil), cfg.Args...), s.ExtraArgs...)
		}
		if s.SystemPromptOverride != "" {
			cfg.SystemPromptOverride = s.SystemPromptOverride
		}
		if len(s.MCPServerCommand) > 0 {
			cfg.MCPServerCommand = append([]string(nil), s.MCPServerCommand...)
		}
		if s.StrictMCPConfig != nil {
			cfg.StrictMCPConfig = *s.StrictMCPConfig
		}
	}

	return registry.Register(claudemcp.NewWithConfig(cfg))
}
