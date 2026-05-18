// Package claude implements the plain CLI variant of the Claude adapter:
// it spawns `claude --print` with stream-json over stdin/stdout. No
// `--sdk-url`, no `--mcp-config`, no PTY — just the binary in print mode.
//
// Status: works on the current Anthropic claude CLI. Billing
// classification: `claude -p` (Agent SDK by Anthropic's terminology),
// which means the 2026-06-15 policy change — Anthropic moving Agent SDK
// usage off Max subscription limits onto a new $200/mo credit pool —
// applies to this path. Today (before the change) it still bills the
// subscription. Mechanically the spawn is unchanged either way; only the
// upstream billing classification shifts.
//
// Sibling adapters:
//
//   - `agent/claudesdk/` (registered as `claude-sdk`) — the WebSocket
//     Agent SDK variant (`claude --sdk-url ws://...`). **Broken** on the
//     official Anthropic CLI since version 2.1.121 because Anthropic
//     restricted `--sdk-url` to their own backend. Whether the restriction
//     will be lifted (e.g. coinciding with the 2026-06-15 billing
//     reorganisation) is unknown. Retained for proxy setups (Claude Code
//     Router and similar) that still accept the flag.
//   - `agent/claudemcp/` (registered as `claude-mcp`, the default) —
//     spawns claude in interactive TUI mode under a PTY with
//     `--mcp-config`. Works today **and** preserves Max-subscription
//     billing after the 2026-06-15 credit-pool split.
//
// Custom agents that `extends: claude` (e.g. the user's `glm` routing
// through Claude Code Router) inherit this CLI behaviour. After the split
// of the legacy dual-mode adapter, those extensions now use the CLI path
// exclusively — no more silent WebSocket attempt + fallback on every
// spawn.
package claude

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"sync"
	"time"

	"github.com/valksor/kvelmo/agent"
)

const AgentName = "claude"

// Agent is the CLI-only Claude adapter. It always operates via a single
// CLIConnection — there is no mode field, no fallback path, no `--sdk-url`
// attempt. The legacy dual-mode behaviour was split when claude CLI
// 2.1.121 made the WebSocket SDK path nonfunctional on the official
// binary; the WebSocket half moved to `agent/claudesdk/`.
type Agent struct {
	config Config

	cli *CLIConnection
	mu  sync.RWMutex
}

// Config holds Claude agent configuration.
type Config struct {
	agent.Config

	// Model to use (e.g., "claude-3-opus", "claude-3-sonnet").
	Model string
}

// DefaultConfig returns default Claude configuration.
func DefaultConfig() Config {
	cfg := Config{
		Config: agent.DefaultConfig(),
	}
	cfg.Command = []string{"claude"}

	return cfg
}

// New creates a Claude agent with default configuration.
func New() *Agent {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a Claude agent with custom configuration.
func NewWithConfig(cfg Config) *Agent {
	if len(cfg.Command) == 0 {
		cfg.Command = []string{"claude"}
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Minute
	}
	if cfg.PermissionHandler == nil {
		cfg.PermissionHandler = agent.DefaultPermissionHandler
	}

	return &Agent{config: cfg}
}

// Name returns the agent identifier.
func (a *Agent) Name() string { return AgentName }

// Available checks if the Claude CLI is installed and runnable.
func (a *Agent) Available() error {
	binary := a.config.Command[0]
	path, err := agent.ResolveCommandPath(binary)
	if err != nil {
		return fmt.Errorf("claude CLI not found: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, path, "--version").Run(); err != nil {
		return fmt.Errorf("claude CLI not working: %w", err)
	}

	return nil
}

// Connect spawns `claude --print` with stream-json. Always CLI mode —
// the agent.Config.PreferWebSocket flag is ignored here (it is consumed
// only by the legacy dual-mode codex adapter; the WebSocket variant of
// claude now lives in agent/claudesdk/).
func (a *Agent) Connect(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cli != nil && a.cli.Connected() {
		return nil
	}

	cli := NewCLIConnection(a.config)
	if err := cli.Connect(ctx); err != nil {
		return fmt.Errorf("claude connect: %w", err)
	}
	a.cli = cli

	return nil
}

// Connected reports whether the CLI subprocess is running.
func (a *Agent) Connected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.cli == nil {
		return false
	}

	return a.cli.Connected()
}

// Mode always returns agent.ModeCLI for this single-mode adapter.
// Retained for callers that introspect; not part of the agent.Agent
// interface.
func (a *Agent) Mode() agent.ConnectionMode { return agent.ModeCLI }

// SendPrompt forwards to the CLI connection.
func (a *Agent) SendPrompt(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.cli == nil {
		return nil, errors.New("claude: not connected")
	}

	return a.cli.SendPrompt(ctx, prompt)
}

// HandlePermission is a no-op in CLI mode (the stream-json protocol does
// not surface tool-approval round-trips — claude self-handles them
// according to the --permission-mode flag).
func (a *Agent) HandlePermission(_ string, _ bool) error { return nil }

// Interrupt is a no-op in CLI mode (would require killing the
// subprocess; not currently wired).
func (a *Agent) Interrupt() error { return nil }

// Close tears down the CLI subprocess.
func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cli == nil {
		return nil
	}
	err := a.cli.Close()
	a.cli = nil

	return err
}

// WithEnv returns a new Agent with an added environment variable.
func (a *Agent) WithEnv(key, value string) agent.Agent {
	newCfg := a.config
	env := make(map[string]string, len(a.config.Environment)+1)
	maps.Copy(env, a.config.Environment)
	env[key] = value
	newCfg.Environment = env

	return NewWithConfig(newCfg)
}

// WithArgs returns a new Agent with additional CLI arguments.
func (a *Agent) WithArgs(args ...string) agent.Agent {
	newCfg := a.config
	newCfg.Args = append(append([]string(nil), a.config.Args...), args...)

	return NewWithConfig(newCfg)
}

// WithWorkDir returns a new Agent with a different working directory.
func (a *Agent) WithWorkDir(dir string) agent.Agent {
	newCfg := a.config
	newCfg.WorkDir = dir

	return NewWithConfig(newCfg)
}

// WithTimeout returns a new Agent with a different timeout.
func (a *Agent) WithTimeout(d time.Duration) agent.Agent {
	newCfg := a.config
	newCfg.Timeout = d

	return NewWithConfig(newCfg)
}

// WithModel returns a new Agent with a specific model.
func (a *Agent) WithModel(model string) *Agent {
	newCfg := a.config
	newCfg.Model = model

	return NewWithConfig(newCfg)
}

// WithPermissionHandler returns a new Agent with a custom permission handler.
func (a *Agent) WithPermissionHandler(handler agent.PermissionHandler) *Agent {
	newCfg := a.config
	newCfg.PermissionHandler = handler

	return NewWithConfig(newCfg)
}

// Register adds the Claude agent to a registry with default config.
func Register(r *agent.Registry) error { return r.Register(New()) }

// RegisterWithPermissionHandler adds the Claude agent with a custom
// permission handler.
func RegisterWithPermissionHandler(r *agent.Registry, handler agent.PermissionHandler) error {
	cfg := agent.DefaultConfig()
	cfg.Command = []string{"claude"}
	cfg.PermissionHandler = handler

	return r.Register(NewWithConfig(Config{Config: cfg}))
}

// Ensure Agent implements agent.Agent.
var _ agent.Agent = (*Agent)(nil)
