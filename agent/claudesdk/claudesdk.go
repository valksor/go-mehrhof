// Package claudesdk implements the Claude adapter that speaks the Agent SDK
// protocol over WebSocket (`claude --print --sdk-url ws://127.0.0.1:<port>`).
//
// **WARNING — broken against the official Anthropic `claude` CLI since
// version 2.1.121.** Anthropic restricted `--sdk-url` to their own backend
// in that release, and spawning the adapter against the official binary
// now fails immediately with:
//
//	Error: --sdk-url rejected: host "127.0.0.1" is not an approved
//	Anthropic endpoint. This flag is reserved for Remote Control worker
//	processes connecting to Anthropic's backend.
//
// Whether `--sdk-url` will be unrestricted again at any point is unknown.
// The 2026-06-15 Agent SDK credit-pool split is a separate (billing) event
// that does NOT affect whether the SDK transport itself works — Anthropic
// has not announced any change to the `--sdk-url` restriction. Treat the
// SDK path as broken for the foreseeable future on the official CLI.
//
// This adapter is retained for two reasons:
//
//  1. Some proxy setups (Claude Code Router and similar) still accept
//     `--sdk-url`. A user with such a proxy can select `agent.default:
//     claude-sdk` explicitly and use this code path.
//  2. The WebSocket protocol implementation is a useful historical
//     artifact for anyone studying how kvelmo originally orchestrated
//     claude.
//
// For vanilla Anthropic claude on the official CLI, use one of:
//
//   - `agent/claude/` (registered as `claude`) — plain `claude --print`
//     over stdin/stdout. Works today. Billing classification: `claude -p`,
//     which means the 2026-06-15 Agent SDK credit-pool split applies to
//     this path even though the SDK transport (`--sdk-url`) does not.
//   - `agent/claudemcp/` (registered as `claude-mcp`, the default) — spawns
//     claude in interactive TUI mode under a PTY with `--mcp-config`.
//     Works today **and** preserves Max-subscription billing after the
//     2026-06-15 Agent SDK credit-pool split.
//
// Custom agents that `extends: claude` (e.g. `glm` via Claude Code Router)
// should extend `claude` (CLI), not `claude-sdk` — the Router accepts
// stream-json over stdio without needing `--sdk-url`.
package claudesdk

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

const AgentName = "claude-sdk"

// Agent implements the Agent SDK WebSocket variant of the Claude adapter.
type Agent struct {
	config Config

	ws *WebSocketConnection
	mu sync.RWMutex
}

// Config holds claude-sdk agent configuration.
type Config struct {
	agent.Config

	// Model to use (e.g., "claude-3-opus", "claude-3-sonnet").
	Model string
}

// DefaultConfig returns default claude-sdk configuration.
func DefaultConfig() Config {
	cfg := Config{
		Config: agent.DefaultConfig(),
	}
	cfg.Command = []string{"claude"}

	return cfg
}

// New creates a claude-sdk agent with default configuration.
func New() *Agent {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a claude-sdk agent with custom configuration.
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

// Available checks if the Claude CLI binary is installed and runnable.
// This does not detect whether `--sdk-url` will be accepted at spawn time —
// that only surfaces during Connect when the spawned process exits with the
// Anthropic rejection error.
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

// Connect spawns `claude --sdk-url ws://...` and waits for the WebSocket
// handshake. There is intentionally no fallback to CLI mode: if the user
// asked for the SDK adapter, they get the SDK adapter, and the WS-rejected
// error is surfaced directly.
func (a *Agent) Connect(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.ws != nil && a.ws.Connected() {
		return nil
	}

	ws := NewWebSocketConnection(a.config)
	if err := ws.Connect(ctx); err != nil {
		return fmt.Errorf("claude-sdk connect: %w", err)
	}
	a.ws = ws

	return nil
}

// Connected reports whether the WebSocket session is live.
func (a *Agent) Connected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.ws == nil {
		return false
	}

	return a.ws.Connected()
}

// Mode always returns agent.ModeWebSocket for this single-mode adapter.
func (a *Agent) Mode() agent.ConnectionMode { return agent.ModeWebSocket }

// SendPrompt forwards to the WebSocket connection.
func (a *Agent) SendPrompt(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.ws == nil {
		return nil, errors.New("claude-sdk: not connected")
	}

	return a.ws.SendPrompt(ctx, prompt)
}

// HandlePermission forwards to the WebSocket connection.
func (a *Agent) HandlePermission(requestID string, approved bool) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.ws == nil {
		return nil
	}

	return a.ws.HandlePermission(requestID, approved)
}

// Interrupt forwards to the WebSocket connection.
func (a *Agent) Interrupt() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.ws == nil {
		return nil
	}

	return a.ws.Interrupt()
}

// Close tears down the WebSocket session.
func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ws == nil {
		return nil
	}
	err := a.ws.Close()
	a.ws = nil

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

// Register adds the claude-sdk agent to a registry with default config.
func Register(r *agent.Registry) error { return r.Register(New()) }

// RegisterWithPermissionHandler adds the claude-sdk agent with a custom
// permission handler.
func RegisterWithPermissionHandler(r *agent.Registry, handler agent.PermissionHandler) error {
	cfg := agent.DefaultConfig()
	cfg.Command = []string{"claude"}
	cfg.PermissionHandler = handler

	return r.Register(NewWithConfig(Config{Config: cfg}))
}

// Ensure Agent implements agent.Agent.
var _ agent.Agent = (*Agent)(nil)
