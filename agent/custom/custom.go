// Package custom implements a configurable custom agent.
// This allows users to integrate any CLI tool that follows the streaming JSON output pattern.
package custom

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valksor/kvelmo/agent"
)

// Agent wraps a custom CLI command as an agent.
type Agent struct {
	name   string
	config Config

	cmd       *exec.Cmd
	cmdMu     sync.Mutex
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	connected atomic.Bool
	closed    atomic.Bool

	eventsMu      sync.Mutex
	events        chan agent.Event
	timeoutCancel context.CancelFunc
}

// Config holds custom agent configuration.
type Config struct {
	// Name is the agent's identifier
	Name string

	// Command is the CLI command to execute (e.g., ["my-agent", "run"])
	Command []string

	// Args are additional arguments to pass
	Args []string

	// Environment variables to set
	Environment map[string]string

	// WorkDir is the working directory
	WorkDir string

	// Timeout for execution
	Timeout time.Duration

	// PermissionHandler evaluates permission requests
	PermissionHandler agent.PermissionHandler

	// InputFormat defines how prompts are sent to the command
	// Options: "json" (default), "text", "stdin"
	InputFormat string

	// OutputFormat defines how responses are parsed
	// Options: "ndjson" (default), "json", "text"
	OutputFormat string
}

// DefaultConfig returns sensible defaults for custom agents.
func DefaultConfig(name string, command []string) Config {
	return Config{
		Name:              name,
		Command:           command,
		Environment:       make(map[string]string),
		Timeout:           30 * time.Minute,
		PermissionHandler: agent.DefaultPermissionHandler,
		InputFormat:       "json",
		OutputFormat:      "ndjson",
	}
}

// New creates a custom agent with the given name and command.
func New(name string, command []string) *Agent {
	return NewWithConfig(DefaultConfig(name, command))
}

// NewWithConfig creates a custom agent with full configuration.
func NewWithConfig(cfg Config) *Agent {
	if cfg.Name == "" {
		cfg.Name = "custom"
	}
	if len(cfg.Command) == 0 {
		cfg.Command = []string{"echo"}
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Minute
	}
	if cfg.InputFormat == "" {
		cfg.InputFormat = "json"
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = "ndjson"
	}
	if cfg.PermissionHandler == nil {
		cfg.PermissionHandler = agent.DefaultPermissionHandler
	}

	return &Agent{
		name:   cfg.Name,
		config: cfg,
		events: make(chan agent.Event, 100),
	}
}

// Name returns the agent identifier.
func (a *Agent) Name() string {
	return a.name
}

// Available checks if the command is available.
func (a *Agent) Available() error {
	if len(a.config.Command) == 0 {
		return errors.New("no command configured")
	}

	binary := a.config.Command[0]
	_, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("command not found: %s: %w", binary, err)
	}

	return nil
}

// Connect starts the custom command process.
func (a *Agent) Connect(ctx context.Context) error {
	a.cmdMu.Lock()
	defer a.cmdMu.Unlock()

	if a.connected.Load() {
		return nil
	}

	// Enforce configured timeout
	if a.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.config.Timeout)
		a.timeoutCancel = cancel
	}

	// Copy command args to avoid mutating the original Config.Command slice
	cmdArgs := make([]string, 0, len(a.config.Command)-1+len(a.config.Args))
	cmdArgs = append(cmdArgs, a.config.Command[1:]...)
	args := append(cmdArgs, a.config.Args...)
	a.cmd = exec.CommandContext(ctx, a.config.Command[0], args...)

	if a.config.WorkDir != "" {
		a.cmd.Dir = a.config.WorkDir
	}

	// Set environment
	for k, v := range a.config.Environment {
		a.cmd.Env = append(a.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Get stdin for sending prompts
	stdin, err := a.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	a.stdin = stdin

	// Get stdout for reading responses
	stdout, err := a.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	a.stdout = stdout

	// Get stderr for errors
	stderr, err := a.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	a.stderr = stderr

	if err := a.cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", a.name, err)
	}

	a.connected.Store(true)

	// Start output reader
	go a.readOutput()

	// Start error reader
	go a.readErrors()

	// Wait for process in background
	go func() {
		_ = a.cmd.Wait()
		a.connected.Store(false)
	}()

	return nil
}

// Connected returns true if the process is running.
func (a *Agent) Connected() bool {
	return a.connected.Load()
}

// getEvents returns the current events channel under lock.
func (a *Agent) getEvents() chan agent.Event {
	a.eventsMu.Lock()
	ch := a.events
	a.eventsMu.Unlock()

	return ch
}

// sendEvent sends an event to the current events channel.
func (a *Agent) sendEvent(event agent.Event) {
	if ch := a.getEvents(); ch != nil {
		ch <- event
	}
}

// readOutput reads from stdout based on OutputFormat.
func (a *Agent) readOutput() {
	scanner := bufio.NewScanner(a.stdout)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		switch a.config.OutputFormat {
		case "ndjson", "json":
			a.parseJSONLine(line)
		case "text":
			a.sendEvent(agent.Event{
				Type:      agent.EventStream,
				Content:   line,
				Timestamp: time.Now(),
			})
		}
	}

	if !a.closed.Load() {
		ch := a.getEvents()
		if ch != nil {
			ch <- agent.Event{
				Type:      agent.EventComplete,
				Timestamp: time.Now(),
			}
			close(ch)
		}
	}
}

// parseJSONLine parses a single JSON line.
func (a *Agent) parseJSONLine(line string) {
	var msg struct {
		Type    string          `json:"type"`
		Content string          `json:"content,omitempty"`
		Text    string          `json:"text,omitempty"`
		Delta   string          `json:"delta,omitempty"`
		Error   string          `json:"error,omitempty"`
		Tool    string          `json:"tool,omitempty"`
		Input   json.RawMessage `json:"input,omitempty"`
		Success bool            `json:"success,omitempty"`
	}

	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		// Not JSON, treat as text
		a.sendEvent(agent.Event{
			Type:      agent.EventStream,
			Content:   line,
			Timestamp: time.Now(),
		})

		return
	}

	switch msg.Type {
	case "stream", "text", "content", "assistant", "delta":
		content := msg.Content
		if content == "" {
			content = msg.Text
		}
		if content == "" {
			content = msg.Delta
		}
		a.sendEvent(agent.Event{
			Type:      agent.EventStream,
			Content:   content,
			Timestamp: time.Now(),
		})

	case "tool_use":
		var input map[string]any
		if msg.Input != nil {
			_ = json.Unmarshal(msg.Input, &input)
		}
		a.sendEvent(agent.Event{
			Type:      agent.EventToolUse,
			Content:   msg.Tool,
			Data:      input,
			Timestamp: time.Now(),
		})

	case "error":
		a.sendEvent(agent.Event{
			Type:      agent.EventError,
			Error:     msg.Error,
			Timestamp: time.Now(),
		})

	case "complete", "done", "result":
		if msg.Success || msg.Type == "complete" || msg.Type == "done" {
			a.sendEvent(agent.Event{
				Type:      agent.EventComplete,
				Timestamp: time.Now(),
			})
		} else {
			a.sendEvent(agent.Event{
				Type:      agent.EventError,
				Error:     msg.Error,
				Timestamp: time.Now(),
			})
		}

	default:
		// Unknown type, output as stream
		content := msg.Content
		if content == "" {
			content = msg.Text
		}
		if content != "" {
			a.sendEvent(agent.Event{
				Type:      agent.EventStream,
				Content:   content,
				Timestamp: time.Now(),
			})
		}
	}
}

// readErrors reads stderr for error messages.
func (a *Agent) readErrors() {
	scanner := bufio.NewScanner(a.stderr)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			a.sendEvent(agent.Event{
				Type:      agent.EventError,
				Content:   line,
				Timestamp: time.Now(),
			})
		}
	}
}

// SendPrompt sends a prompt to the custom command.
func (a *Agent) SendPrompt(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	if !a.connected.Load() {
		return nil, errors.New("not connected")
	}

	// Create new event channel for this prompt (under lock to synchronize with readOutput)
	a.eventsMu.Lock()
	a.events = make(chan agent.Event, 100)
	eventsCh := a.events
	a.eventsMu.Unlock()

	// Write prompt based on input format
	a.cmdMu.Lock()
	if a.stdin != nil {
		var data []byte
		switch a.config.InputFormat {
		case "json":
			msg := map[string]string{"prompt": prompt}
			if jsonData, err := json.Marshal(msg); err == nil {
				data = append(jsonData, '\n')
			}
		case "text", "stdin":
			data = []byte(prompt + "\n")
		}
		_, _ = a.stdin.Write(data)
	}
	a.cmdMu.Unlock()

	// Return filtered event channel
	filtered := make(chan agent.Event, 100)
	go func() {
		defer close(filtered)
		for {
			select {
			case event, ok := <-eventsCh:
				if !ok {
					return
				}
				filtered <- event
				if event.Type == agent.EventComplete || event.Type == agent.EventError {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return filtered, nil
}

// HandlePermission is a no-op for custom agents.
func (a *Agent) HandlePermission(requestID string, approved bool) error {
	return nil
}

// Interrupt is a no-op for custom agents (process-based, no interrupt protocol).
func (a *Agent) Interrupt() error {
	return nil
}

// Close stops the process.
func (a *Agent) Close() error {
	if a.closed.Swap(true) {
		return nil
	}

	a.connected.Store(false)

	// Cancel timeout context if set
	if a.timeoutCancel != nil {
		a.timeoutCancel()
	}

	a.cmdMu.Lock()
	defer a.cmdMu.Unlock()

	if a.stdin != nil {
		_ = a.stdin.Close()
	}

	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
	}

	return nil
}

// Ensure Agent implements agent.Agent.
var _ agent.Agent = (*Agent)(nil)
