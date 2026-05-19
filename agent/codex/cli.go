package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valksor/kvelmo/agent"
)

// CLIConnection manages Codex via app-server subprocess with JSON-RPC.
// This is the fallback mode when WebSocket is unavailable.
type CLIConnection struct {
	config Config

	cmd       *exec.Cmd
	cmdMu     sync.Mutex
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	transport *JsonRpcTransport
	threadID  string

	connected atomic.Bool
	closed    atomic.Bool

	// Event channel for current prompt
	events   chan agent.Event
	eventsMu sync.Mutex

	// Subagent tracker for detecting Task tool calls
	subagents *agent.SubagentTracker

	// Pending approval requests
	pendingApprovals   map[string]int64 // requestID -> jsonrpc ID
	pendingApprovalsMu sync.Mutex

	// Track current turn state
	turnActive atomic.Bool
}

// NewCLIConnection creates a new CLI connection for Codex.
func NewCLIConnection(cfg Config) *CLIConnection {
	events := make(chan agent.Event, 100)

	return &CLIConnection{
		config:           cfg,
		events:           events,
		subagents:        agent.NewSubagentTracker(events),
		pendingApprovals: make(map[string]int64),
	}
}

// Connect starts the Codex app-server process and initializes JSON-RPC.
func (c *CLIConnection) Connect(ctx context.Context) error {
	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()

	if c.connected.Load() {
		return nil
	}

	args := c.buildArgs()
	c.cmd = exec.CommandContext(ctx, c.config.Command[0], args...)

	if c.config.WorkDir != "" {
		c.cmd.Dir = c.config.WorkDir
	}

	// Set environment
	for k, v := range c.config.Environment {
		c.cmd.Env = append(c.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Get stdin for JSON-RPC
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	c.stdin = stdin

	// Get stdout for JSON-RPC
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
		return fmt.Errorf("start codex: %w", err)
	}

	// Create JSON-RPC transport
	c.transport = NewJsonRpcTransport(c.stdout, c.stdin)
	c.transport.OnNotification(c.handleNotification)
	c.transport.OnRequest(c.handleRequest)
	c.transport.Start(ctx)

	// Start stderr reader
	go c.readErrors()

	// Wait for process in background
	go func() {
		_ = c.cmd.Wait()
		c.connected.Store(false)
	}()

	// Initialize JSON-RPC handshake
	if err := c.initialize(ctx); err != nil {
		_ = c.Close()

		return fmt.Errorf("initialize: %w", err)
	}

	c.connected.Store(true)

	c.events <- agent.Event{
		Type:      agent.EventInit,
		Content:   "Codex app-server initialized",
		Timestamp: time.Now(),
	}

	return nil
}

// buildArgs constructs CLI arguments for Codex app-server.
func (c *CLIConnection) buildArgs() []string {
	args := make([]string, 0, 1+len(c.config.Args))
	args = append(
		args,
		"app-server",
		// Multi-agent mode configured via ~/.codex/config.toml, not CLI flags
	)

	// Add configured arguments
	args = append(args, c.config.Args...)

	return args
}

// initialize performs the JSON-RPC initialization handshake.
func (c *CLIConnection) initialize(ctx context.Context) error {
	// Step 1: Send initialize request
	_, err := c.transport.Call(ctx, "initialize", map[string]any{
		"clientInfo": map[string]any{
			"name":    "kvelmo",
			"title":   "kvelmo",
			"version": "1.0.0",
		},
		"capabilities": map[string]any{
			"experimentalApi": false,
		},
	})
	if err != nil {
		return fmt.Errorf("initialize call: %w", err)
	}

	// Step 2: Send initialized notification
	if err := c.transport.Notify("initialized", map[string]any{}); err != nil {
		return fmt.Errorf("initialized notify: %w", err)
	}

	// Step 3: Start a thread
	result, err := c.transport.Call(ctx, "thread/start", map[string]any{
		"model":          c.config.Model,
		"cwd":            c.config.WorkDir,
		"approvalPolicy": "always", // Ask for approval
		"sandbox":        "workspace-write",
	})
	if err != nil {
		return fmt.Errorf("thread/start: %w", err)
	}

	// Extract thread ID
	var threadResult struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(result, &threadResult); err != nil {
		return fmt.Errorf("parse thread result: %w", err)
	}

	c.threadID = threadResult.Thread.ID
	slog.Debug("codex thread started", "threadId", c.threadID)

	return nil
}

// Connected returns true if the CLI process is running.
func (c *CLIConnection) Connected() bool {
	return c.connected.Load()
}

// SendPrompt sends a prompt to Codex via JSON-RPC.
func (c *CLIConnection) SendPrompt(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	if !c.connected.Load() {
		return nil, errors.New("not connected")
	}

	if c.threadID == "" {
		return nil, errors.New("no thread started")
	}

	// Create new event channel for this prompt
	c.eventsMu.Lock()
	c.events = make(chan agent.Event, 100)
	ch := c.events
	c.eventsMu.Unlock()
	c.subagents.SetEventChannel(ch)

	// Start a turn
	c.turnActive.Store(true)
	_, err := c.transport.Call(ctx, "turn/start", map[string]any{
		"threadId": c.threadID,
		"message": map[string]any{
			"role":    "user",
			"content": prompt,
		},
	})
	if err != nil {
		c.turnActive.Store(false)

		return nil, fmt.Errorf("turn/start: %w", err)
	}

	// Return filtered event channel
	filtered := make(chan agent.Event, 100)
	go func() {
		defer close(filtered)
		for {
			select {
			case event, ok := <-ch:
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

// HandlePermission responds to a permission request.
func (c *CLIConnection) HandlePermission(requestID string, approved bool) error {
	c.pendingApprovalsMu.Lock()
	rpcID, ok := c.pendingApprovals[requestID]
	if ok {
		delete(c.pendingApprovals, requestID)
	}
	c.pendingApprovalsMu.Unlock()

	if !ok {
		return fmt.Errorf("no pending approval for %s", requestID)
	}

	decision := decisionAccept
	if !approved {
		decision = decisionReject
	}

	return c.transport.Respond(rpcID, map[string]any{keyDecision: decision})
}

// Close stops the CLI process.
func (c *CLIConnection) Close() error {
	if c.closed.Swap(true) {
		return nil // Already closed
	}

	c.connected.Store(false)

	if c.transport != nil {
		_ = c.transport.Close()
	}

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
