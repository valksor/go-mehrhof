package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/valksor/kvelmo/pkg/agent"
)

// CLIConnection manages Claude via CLI subprocess.
// This is the fallback mode when WebSocket is unavailable.
// Per flow_v2.md: "fallback to CLI subprocess".
type CLIConnection struct {
	config Config

	cmd       *exec.Cmd
	cmdMu     sync.Mutex
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	connected atomic.Bool
	closed    atomic.Bool

	// Event channel for current prompt
	events chan agent.Event

	// Subagent tracker for detecting Task tool calls
	subagents *agent.SubagentTracker
}

// cliMessage represents NDJSON output from Claude CLI.
type cliMessage struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"` // For result: "success" or error type

	// For assistant/content_block_delta
	Content string `json:"content,omitempty"`
	Delta   *struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"delta,omitempty"`

	// For tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// For tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`

	// For result
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`

	// For message - can be string or object, use RawMessage
	Role    string          `json:"role,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`
}

// NewCLIConnection creates a new CLI connection for Claude.
func NewCLIConnection(cfg Config) *CLIConnection {
	events := make(chan agent.Event, 100)

	return &CLIConnection{
		config:    cfg,
		events:    events,
		subagents: agent.NewSubagentTracker(events),
	}
}

// Connect starts the Claude CLI process.
func (c *CLIConnection) Connect(ctx context.Context) error {
	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()

	if c.connected.Load() {
		return nil
	}

	args := c.buildArgs()
	// Use Background context so the process lifetime is not tied to the
	// caller's context (which may only gate the connection handshake).
	// The process is killed explicitly by Close().
	c.cmd = exec.CommandContext(context.Background(), c.config.Command[0], args...) //nolint:contextcheck // Process lifetime decoupled from handshake; killed by Close()

	if c.config.WorkDir != "" {
		c.cmd.Dir = c.config.WorkDir
	}

	// Build environment: start with parent env, exclude CLAUDECODE to allow nested sessions
	env := make([]string, 0, len(os.Environ())+len(c.config.Environment))
	for _, e := range os.Environ() {
		// Skip CLAUDECODE to allow running Claude CLI from within Claude Code
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			env = append(env, e)
		}
	}
	// Add custom config environment variables
	for k, v := range c.config.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	c.cmd.Env = env

	// Get stdin for sending prompts
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	c.stdin = stdin

	// Get stdout for reading responses
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	c.stdout = stdout

	// Get stderr for errors
	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	c.stderr = stderr

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	c.connected.Store(true)

	// Start output reader
	go c.readOutput()

	// Start error reader
	go c.readErrors()

	// Wait for process in background
	go func() {
		_ = c.cmd.Wait()
		c.connected.Store(false)
	}()

	return nil
}

// buildArgs constructs CLI arguments for Claude.
func (c *CLIConnection) buildArgs() []string {
	args := []string{
		"--print",
		"--verbose", // Required for stream-json output
		"--output-format", "stream-json",
		"--input-format", "stream-json", // Enable stdin-based prompts
		"--permission-mode", "bypassPermissions", // Skip plan mode, allow direct file writes
	}

	// Add configured arguments
	args = append(args, c.config.Args...)

	// Add model if specified
	if c.config.Model != "" {
		args = append(args, "--model", c.config.Model)
	}

	return args
}

// readOutput reads NDJSON from Claude's stdout.
func (c *CLIConnection) readOutput() {
	scanner := bufio.NewScanner(c.stdout)
	// Increase buffer for large outputs
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		slog.Debug("claude cli stdout", "line", line)

		var msg cliMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			slog.Warn("claude cli json parse error", "error", err, "line", line)

			continue
		}

		c.handleMessage(msg)
	}

	// Scanner done - close events
	if !c.closed.Load() {
		close(c.events)
	}
}

// readErrors reads stderr for error messages.
// Stderr is logged but not treated as errors since Claude CLI outputs
// debug/info to stderr which should not fail the job.
func (c *CLIConnection) readErrors() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// Log stderr but don't send as error event - Claude CLI outputs
			// debug info and warnings to stderr that shouldn't fail jobs
			slog.Debug("claude cli stderr", "output", line)
		}
	}
}

// handleMessage processes a CLI message.
func (c *CLIConnection) handleMessage(msg cliMessage) {
	switch msg.Type {
	case "assistant", "text":
		content := msg.Content
		if content == "" && msg.Delta != nil {
			content = msg.Delta.Text
		}
		if content != "" {
			c.events <- agent.Event{
				Type:      agent.EventStream,
				Content:   content,
				Timestamp: time.Now(),
			}
		}

	case "content_block_delta":
		if msg.Delta != nil && msg.Delta.Text != "" {
			c.events <- agent.Event{
				Type:      agent.EventStream,
				Content:   msg.Delta.Text,
				Timestamp: time.Now(),
			}
		}

	case "tool_use":
		var input map[string]any
		if msg.Input != nil {
			_ = json.Unmarshal(msg.Input, &input)
		}

		// Check for subagent spawn (Task tool)
		toolCallID := msg.ID
		if toolCallID == "" {
			toolCallID = uuid.NewString()
		}
		c.subagents.OnToolUse(toolCallID, msg.Name, input)

		c.events <- agent.Event{
			Type:      agent.EventToolUse,
			Content:   msg.Name,
			Data:      input,
			Timestamp: time.Now(),
		}

	case "tool_result":
		// Check for subagent completion
		toolCallID := msg.ToolUseID
		if toolCallID != "" {
			c.subagents.OnToolResult(toolCallID, !msg.IsError, msg.Error)
		}

		c.events <- agent.Event{
			Type:      agent.EventToolResult,
			Content:   msg.Result,
			Timestamp: time.Now(),
		}

	case "result":
		// Success when subtype is "success" or result has no error flag
		isSuccess := msg.Subtype == "success" || !msg.IsError
		if isSuccess {
			c.events <- agent.Event{
				Type:      agent.EventComplete,
				Timestamp: time.Now(),
			}
		} else {
			slog.Error("claude cli result error", "error", msg.Error, "subtype", msg.Subtype)
			c.events <- agent.Event{
				Type:      agent.EventError,
				Error:     msg.Error,
				Timestamp: time.Now(),
			}
		}

	case "error":
		slog.Error("claude cli error message", "error", msg.Error, "message", string(msg.Message))
		c.events <- agent.Event{
			Type:      agent.EventError,
			Error:     msg.Error,
			Content:   string(msg.Message),
			Timestamp: time.Now(),
		}

	case "message_stop", "message_end":
		c.events <- agent.Event{
			Type:      agent.EventComplete,
			Timestamp: time.Now(),
		}
	}
}

// Connected returns true if the CLI process is running.
func (c *CLIConnection) Connected() bool {
	return c.connected.Load()
}

// SendPrompt sends a prompt to Claude CLI.
// In CLI mode, we spawn a new process for each prompt.
func (c *CLIConnection) SendPrompt(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	slog.Info("claude cli SendPrompt called", "prompt_len", len(prompt), "connected", c.connected.Load())

	if !c.connected.Load() {
		slog.Error("claude cli SendPrompt: not connected")

		return nil, errors.New("not connected")
	}

	// Write prompt to stdin using stream-json format
	c.cmdMu.Lock()

	// Create new event channel for this prompt (inside mutex to prevent race with readOutput)
	c.events = make(chan agent.Event, 100)
	c.subagents.SetEventChannel(c.events)

	slog.Info("claude cli SendPrompt: stdin check", "stdin_nil", c.stdin == nil)
	if c.stdin == nil {
		c.cmdMu.Unlock()

		return nil, errors.New("stdin not available")
	}

	// Format as NDJSON message per Claude Code stream-json spec:
	// {"type":"user","message":{"role":"user","content":"..."},"session_id":"default","parent_tool_use_id":null}
	msg := map[string]any{
		"type": "user",
		"message": map[string]string{
			"role":    "user",
			"content": prompt,
		},
		"session_id":         "default",
		"parent_tool_use_id": nil,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		c.cmdMu.Unlock()

		return nil, fmt.Errorf("marshal prompt: %w", err)
	}
	data = append(data, '\n')
	slog.Debug("claude cli stdin", "data", string(data))
	if _, writeErr := c.stdin.Write(data); writeErr != nil {
		c.cmdMu.Unlock()

		return nil, fmt.Errorf("write prompt to stdin: %w", writeErr)
	}
	c.cmdMu.Unlock()

	// Return filtered event channel
	filtered := make(chan agent.Event, 100)
	go func() {
		defer close(filtered)
		for {
			select {
			case event, ok := <-c.events:
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

// HandlePermission is a no-op in CLI mode.
// CLI mode doesn't support interactive permission handling.
// Permissions must be pre-approved via --dangerously-skip-permissions
// or handled by the permission handler in config.
func (c *CLIConnection) HandlePermission(requestID string, approved bool) error {
	// CLI mode doesn't support interactive permissions
	return nil
}

// Close stops the CLI process.
func (c *CLIConnection) Close() error {
	if c.closed.Swap(true) {
		return nil // Already closed
	}

	c.connected.Store(false)

	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()

	// Close stdin to signal EOF
	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	// Kill process if still running
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}

	return nil
}
