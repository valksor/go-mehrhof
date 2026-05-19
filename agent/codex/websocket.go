package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/valksor/kvelmo/agent"
)

// WebSocketConnection manages Codex via app-server with WebSocket transport.
// Unlike Claude, Codex HOSTS the WebSocket server and we connect as a client.
type WebSocketConnection struct {
	config   Config
	port     int
	threadID string

	cmd    *exec.Cmd
	cmdMu  sync.Mutex
	cmdErr error

	conn      *websocket.Conn
	connMu    sync.Mutex
	transport *wsTransport

	// State
	connected  atomic.Bool
	closed     atomic.Bool
	closedOnce sync.Once

	// Event channel
	events   chan agent.Event
	eventsMu sync.Mutex

	// Subagent tracker
	subagents *agent.SubagentTracker

	// Pending approval requests
	pendingApprovals   map[string]int64
	pendingApprovalsMu sync.Mutex

	// Track current turn
	turnActive atomic.Bool
}

// wsTransport adapts WebSocket for JSON-RPC transport.
type wsTransport struct {
	conn     *websocket.Conn
	connMu   sync.Mutex
	pending  map[int64]chan *rpcResponse
	pendingM sync.Mutex
	nextID   atomic.Int64

	notificationHandler func(method string, params json.RawMessage)
	requestHandler      func(method string, id int64, params json.RawMessage)

	closed  atomic.Bool
	closeCh chan struct{}
}

// NewWebSocketConnection creates a new WebSocket connection for Codex.
func NewWebSocketConnection(cfg Config) *WebSocketConnection {
	events := make(chan agent.Event, 100)

	return &WebSocketConnection{
		config:           cfg,
		port:             cfg.WebSocketPort,
		events:           events,
		subagents:        agent.NewSubagentTracker(events),
		pendingApprovals: make(map[string]int64),
	}
}

// Connect starts Codex app-server and connects via WebSocket.
func (w *WebSocketConnection) Connect(ctx context.Context) error {
	// Find a free port
	port, err := findFreePort(ctx)
	if err != nil {
		return fmt.Errorf("find free port: %w", err)
	}
	w.port = port

	// Launch Codex app-server
	if err := w.launchCodex(ctx); err != nil {
		return err
	}

	// Connect to Codex with exponential backoff retry
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d", w.port)
	var conn *websocket.Conn
	var lastErr error

	// Retry with exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms (total ~3.1s)
	backoff := 100 * time.Millisecond
	maxAttempts := 5

	for attempt := range maxAttempts {
		if attempt > 0 {
			slog.Debug("retrying codex connection", "attempt", attempt+1, "backoff", backoff)
			time.Sleep(backoff)
			backoff *= 2
		}

		var dialErr error
		conn, _, dialErr = websocket.Dial(ctx, wsURL, nil)
		if dialErr == nil {
			break // Success
		}
		lastErr = dialErr

		// Check if context was cancelled
		if ctx.Err() != nil {
			w.killProcess()

			return fmt.Errorf("connect to codex: %w", ctx.Err())
		}
	}

	if conn == nil {
		w.killProcess()

		return fmt.Errorf("connect to codex after %d attempts: %w", maxAttempts, lastErr)
	}

	w.connMu.Lock()
	w.conn = conn
	w.connMu.Unlock()

	// Create transport
	w.transport = newWsTransport(conn)
	w.transport.notificationHandler = w.handleNotification
	w.transport.requestHandler = w.handleRequest
	go w.transport.readLoop(ctx)

	// Initialize JSON-RPC
	if err := w.initialize(ctx); err != nil {
		_ = w.Close()

		return fmt.Errorf("initialize: %w", err)
	}

	w.connected.Store(true)

	w.events <- agent.Event{
		Type:      agent.EventInit,
		Content:   "Codex WebSocket connected",
		Timestamp: time.Now(),
	}

	return nil
}

// findFreePort finds an available TCP port.
func findFreePort(ctx context.Context) (int, error) {
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close() //nolint:errcheck // port already read from addr; close failure is harmless

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected address type: %T", listener.Addr())
	}

	return addr.Port, nil
}

// launchCodex starts the Codex app-server process.
func (w *WebSocketConnection) launchCodex(ctx context.Context) error {
	w.cmdMu.Lock()
	defer w.cmdMu.Unlock()

	args := make([]string, 0, 3+len(w.config.Args))
	args = append(
		args,
		"app-server",
		"--listen", fmt.Sprintf("ws://127.0.0.1:%d", w.port),
		// Multi-agent mode configured via ~/.codex/config.toml, not CLI flags
	)

	// Add configured arguments
	args = append(args, w.config.Args...)

	w.cmd = exec.CommandContext(ctx, w.config.Command[0], args...)

	if w.config.WorkDir != "" {
		w.cmd.Dir = w.config.WorkDir
	}

	// Set environment
	for k, v := range w.config.Environment {
		w.cmd.Env = append(w.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Capture stderr
	stderr, err := w.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := w.cmd.Start(); err != nil {
		return fmt.Errorf("start codex: %w", err)
	}

	// Log stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				slog.Debug("codex stderr", "output", line)
			}
		}
	}()

	// Monitor process
	go func() {
		err := w.cmd.Wait()
		w.cmdMu.Lock()
		w.cmdErr = err
		w.cmdMu.Unlock()
		w.connected.Store(false)
	}()

	return nil
}

// initialize performs the JSON-RPC initialization handshake.
func (w *WebSocketConnection) initialize(ctx context.Context) error {
	// Step 1: initialize
	_, err := w.transport.Call(ctx, "initialize", map[string]any{
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
		return fmt.Errorf("initialize: %w", err)
	}

	// Step 2: initialized notification
	if err := w.transport.Notify(ctx, "initialized", map[string]any{}); err != nil {
		return fmt.Errorf("initialized: %w", err)
	}

	// Step 3: thread/start
	result, err := w.transport.Call(ctx, "thread/start", map[string]any{
		"model":          w.config.Model,
		"cwd":            w.config.WorkDir,
		"approvalPolicy": "always",
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

	w.threadID = threadResult.Thread.ID
	slog.Debug("codex thread started", "threadId", w.threadID)

	return nil
}

// Connected returns true if connected.
func (w *WebSocketConnection) Connected() bool {
	return w.connected.Load()
}

func (w *WebSocketConnection) killProcess() {
	w.cmdMu.Lock()
	defer w.cmdMu.Unlock()

	if w.cmd != nil && w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
	}
}

// Close stops the connection.
func (w *WebSocketConnection) Close() error {
	w.closedOnce.Do(func() {
		w.closed.Store(true)
		w.connected.Store(false)

		if w.transport != nil {
			_ = w.transport.Close()
		}

		w.connMu.Lock()
		if w.conn != nil {
			_ = w.conn.CloseNow()
		}
		w.connMu.Unlock()

		w.killProcess()

		close(w.events)
	})

	return nil
}
