// Package claude implements the Claude AI agent.
// Supports WebSocket (primary) and CLI (fallback) connection modes.
package claude

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"sync"
	"time"

	"github.com/valksor/kvelmo/pkg/agent"
)

const AgentName = "claude"

// Agent wraps the Claude CLI with WebSocket and CLI modes.
type Agent struct {
	config Config
	mode   agent.ConnectionMode

	// WebSocket mode
	ws *WebSocketConnection

	// CLI mode
	cli *CLIConnection

	mu sync.RWMutex
}

// Config holds Claude agent configuration.
type Config struct {
	agent.Config

	// Model to use (e.g., "claude-3-opus", "claude-3-sonnet")
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

	return &Agent{
		config: cfg,
	}
}

// Name returns the agent identifier.
func (a *Agent) Name() string {
	return AgentName
}

// Available checks if the Claude CLI is installed.
func (a *Agent) Available() error {
	binary := a.config.Command[0]
	path, err := agent.ResolveCommandPath(binary)
	if err != nil {
		return fmt.Errorf("claude CLI not found: %w", err)
	}

	// Verify it runs
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude CLI not working: %w", err)
	}

	return nil
}

// Connect establishes connection. Tries WebSocket first, falls back to CLI.
func (a *Agent) Connect(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Already connected?
	if a.ws != nil && a.ws.Connected() {
		return nil
	}
	if a.cli != nil && a.cli.Connected() {
		return nil
	}

	// Try WebSocket first if preferred
	if a.config.PreferWebSocket {
		ws := NewWebSocketConnection(a.config)
		if err := ws.Connect(ctx); err == nil {
			a.ws = ws
			a.mode = agent.ModeWebSocket

			return nil
		}
		// WebSocket failed, try CLI
	}

	// Fallback to CLI
	cli := NewCLIConnection(a.config)
	if err := cli.Connect(ctx); err != nil {
		return fmt.Errorf("connect failed (both WebSocket and CLI): %w", err)
	}

	a.cli = cli
	a.mode = agent.ModeCLI

	return nil
}

// Connected returns true if the agent is connected.
func (a *Agent) Connected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.ws != nil {
		return a.ws.Connected()
	}
	if a.cli != nil {
		return a.cli.Connected()
	}

	return false
}

// Mode returns the current connection mode.
func (a *Agent) Mode() agent.ConnectionMode {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.mode
}

// SendPrompt sends a prompt and returns streaming events.
func (a *Agent) SendPrompt(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	switch a.mode {
	case agent.ModeWebSocket:
		if a.ws == nil {
			return nil, errors.New("websocket not connected")
		}

		return a.ws.SendPrompt(ctx, prompt)
	case agent.ModeCLI:
		if a.cli == nil {
			return nil, errors.New("cli not connected")
		}

		return a.cli.SendPrompt(ctx, prompt)
	case agent.ModeAPI:
		return nil, errors.New("ModeAPI not yet implemented")
	default:
		return nil, errors.New("not connected")
	}
}

// HandlePermission responds to a permission request.
func (a *Agent) HandlePermission(requestID string, approved bool) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.mode == agent.ModeWebSocket && a.ws != nil {
		return a.ws.HandlePermission(requestID, approved)
	}
	// CLI mode doesn't support interactive permission handling
	return nil
}

// Interrupt aborts the current agent turn.
func (a *Agent) Interrupt() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.mode == agent.ModeWebSocket && a.ws != nil {
		return a.ws.Interrupt()
	}
	// CLI mode doesn't support interrupt (would need to kill process)
	return nil
}

// Close closes the connection.
func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var err error
	if a.ws != nil {
		err = a.ws.Close()
		a.ws = nil
	}
	if a.cli != nil {
		err = a.cli.Close()
		a.cli = nil
	}
	a.mode = ""

	return err
}

// WithEnv returns a new Agent with an added environment variable.
func (a *Agent) WithEnv(key, value string) agent.Agent {
	newCfg := a.config
	if newCfg.Environment == nil {
		newCfg.Environment = make(map[string]string)
	}
	// Copy existing env
	env := make(map[string]string, len(a.config.Environment)+1)
	maps.Copy(env, a.config.Environment)
	env[key] = value
	newCfg.Environment = env

	return NewWithConfig(newCfg)
}

// WithArgs returns a new Agent with additional CLI arguments.
func (a *Agent) WithArgs(args ...string) agent.Agent {
	newCfg := a.config
	newArgs := make([]string, len(a.config.Args)+len(args))
	copy(newArgs, a.config.Args)
	copy(newArgs[len(a.config.Args):], args)
	newCfg.Args = newArgs

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
func Register(r *agent.Registry) error {
	return r.Register(New())
}

// RegisterWithPermissionHandler adds the Claude agent with a custom permission handler.
func RegisterWithPermissionHandler(r *agent.Registry, handler agent.PermissionHandler) error {
	cfg := agent.DefaultConfig()
	cfg.Command = []string{"claude"}
	cfg.PermissionHandler = handler

	return r.Register(NewWithConfig(Config{Config: cfg}))
}

// Ensure Agent implements agent.Agent.
var _ agent.Agent = (*Agent)(nil)
