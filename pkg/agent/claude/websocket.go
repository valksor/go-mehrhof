package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/valksor/kvelmo/pkg/agent"
)

// localAcceptOptions restricts WebSocket connections to localhost only.
var localAcceptOptions = &websocket.AcceptOptions{
	OriginPatterns: []string{"localhost:*", "127.0.0.1:*", "[::1]:*"},
}

// WebSocketConnection manages a Claude CLI connection via WebSocket.
// Per flow_v2.md: "We act as WebSocket SERVER, Claude CLI connects to us.".
type WebSocketConnection struct {
	config    Config
	port      int
	sessionID string
	done      chan struct{} // closed when Connect's context is canceled; signals shutdown to read loop

	server   *http.Server
	listener net.Listener
	conn     *websocket.Conn
	connMu   sync.Mutex

	cmd             *exec.Cmd
	cmdMu           sync.Mutex
	cmdErr          error
	lifecycleCtx    context.Context //nolint:containedctx // Process lifetime is decoupled from handshake context
	lifecycleCancel context.CancelFunc

	// Message channels
	outgoing chan outgoingMessage
	events   chan agent.Event

	// Pending permission requests awaiting response
	pendingRequests   map[string]pendingRequest
	pendingRequestsMu sync.Mutex

	// State
	ready        chan struct{} // Signaled when WebSocket connects
	sessionReady chan struct{} // Signaled when session ID is received
	readyOnce    sync.Once
	sessionOnce  sync.Once
	connected    atomic.Bool
	closed       atomic.Bool
	closedOnce   sync.Once
	connectMu    sync.Mutex // Guards Connect() against concurrent callers

	// Subagent tracker for detecting Task tool calls
	subagents *agent.SubagentTracker

	// Guards SendPrompt against concurrent callers
	promptInFlight atomic.Bool
}

// Incoming message types from Claude CLI.
type incomingMessage struct {
	Type string `json:"type"`

	// system/init fields
	SessionID    string   `json:"session_id,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Tools        []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"tools,omitempty"`

	// stream_event fields
	Content string `json:"content,omitempty"`
	Delta   string `json:"delta,omitempty"`

	// assistant fields
	Message *struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"` // Can be string or array
	} `json:"message,omitempty"`

	// control_request fields - top-level request_id and nested request object
	RequestID string `json:"request_id,omitempty"`
	Request   *struct {
		Subtype   string          `json:"subtype,omitempty"`
		ToolName  string          `json:"tool_name,omitempty"`
		Input     json.RawMessage `json:"input,omitempty"`
		ToolUseID string          `json:"tool_use_id,omitempty"`
	} `json:"request,omitempty"`

	// result fields
	Subtype string `json:"subtype,omitempty"` // "success" or error type
	IsError bool   `json:"is_error,omitempty"`
	Error   string `json:"error,omitempty"`

	// tool_use fields
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result fields
	Result string `json:"result,omitempty"`

	// tool_progress fields
	ToolUseID          string  `json:"tool_use_id,omitempty"`
	ToolName           string  `json:"tool_name,omitempty"`
	ElapsedTimeSeconds float64 `json:"elapsed_time_seconds,omitempty"`

	// prompt_suggestion fields
	Suggestions []string `json:"suggestions,omitempty"`
}

// Outgoing message types to Claude CLI.
type outgoingMessage struct {
	Type string `json:"type"`

	// user message fields
	Message *struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message,omitempty"`
	SessionID string `json:"session_id,omitempty"`

	// control_response fields - nested response object
	Response *controlResponsePayload `json:"response,omitempty"`

	// control_request fields (for interrupt, set_model, etc.)
	RequestID string                 `json:"request_id,omitempty"`
	Request   *controlRequestPayload `json:"request,omitempty"`
}

// controlRequestPayload is the request payload for control_request messages.
type controlRequestPayload struct {
	Subtype string `json:"subtype"`
}

// controlResponsePayload is the outer response object for control_response messages.
// Structure: {"type":"control_response","response":{...}}.
type controlResponsePayload struct {
	Subtype   string                `json:"subtype"`
	RequestID string                `json:"request_id"`
	Response  *controlResponseInner `json:"response,omitempty"`
	Error     string                `json:"error,omitempty"`
}

// controlResponseInner is the inner response payload for success responses.
// Structure: {"response":{"response":{...}}}.
type controlResponseInner struct {
	Behavior     string         `json:"behavior"`
	UpdatedInput map[string]any `json:"updatedInput,omitempty"`
	Message      string         `json:"message,omitempty"`
	Interrupt    bool           `json:"interrupt,omitempty"`
	ToolUseID    string         `json:"toolUseID,omitempty"`
}

// pendingRequest stores a control_request awaiting response.
type pendingRequest struct {
	Input     map[string]any
	ToolUseID string
}

// NewWebSocketConnection creates a new WebSocket connection for Claude.
func NewWebSocketConnection(cfg Config) *WebSocketConnection {
	events := make(chan agent.Event, 100)

	return &WebSocketConnection{
		config:          cfg,
		port:            cfg.WebSocketPort,
		outgoing:        make(chan outgoingMessage, 100),
		events:          events,
		pendingRequests: make(map[string]pendingRequest),
		ready:           make(chan struct{}),
		sessionReady:    make(chan struct{}),
		subagents:       agent.NewSubagentTracker(events),
	}
}

// Connect starts the WebSocket server and launches Claude CLI.
func (w *WebSocketConnection) Connect(ctx context.Context) error {
	w.connectMu.Lock()
	defer w.connectMu.Unlock()

	if w.closed.Load() {
		return errors.New("connection has been closed; create a new WebSocketConnection to reconnect")
	}

	if w.connected.Load() {
		return nil
	}

	// Lifecycle context controls the process and connection lifetime.
	// It is canceled only by Close(), not by the caller's handshake context.
	w.lifecycleCtx, w.lifecycleCancel = context.WithCancel(context.Background())
	w.done = make(chan struct{})
	go func() {
		<-w.lifecycleCtx.Done()
		close(w.done)
	}()
	w.subagents.SetDoneChannel(w.done)

	// Create listener
	addr := fmt.Sprintf("127.0.0.1:%d", w.port)
	listener, err := net.Listen("tcp", addr) //nolint:noctx // Context cancellation handled via server shutdown
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	w.listener = listener

	// Get actual port (if 0 was specified)
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		w.port = tcpAddr.Port
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", w.handleConnection)

	w.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start HTTP server
	go func() {
		if err := w.server.Serve(listener); err != http.ErrServerClosed {
			w.connected.Store(false)
		}
	}()

	// Start outgoing message sender (uses lifecycle context, not handshake context)
	go w.sendLoop(w.lifecycleCtx) //nolint:contextcheck // Must use lifecycle context, not handshake context

	// Launch Claude CLI (uses lifecycle context for process lifetime)
	if err := w.launchClaude(w.lifecycleCtx); err != nil { //nolint:contextcheck // Must use lifecycle context, not handshake context
		_ = w.Close()

		return err
	}

	// Wait for WebSocket connection
	select {
	case <-w.ready:
		// WebSocket connected, now wait for session initialization
	case <-time.After(30 * time.Second):
		_ = w.Close()

		return errors.New("timeout waiting for Claude CLI connection")
	case <-ctx.Done():
		_ = w.Close()

		return ctx.Err()
	}

	// Wait for session initialization (system message with session_id)
	select {
	case <-w.sessionReady:
		w.connected.Store(true)

		return nil
	case <-time.After(30 * time.Second):
		_ = w.Close()

		return errors.New("timeout waiting for Claude CLI session initialization")
	case <-ctx.Done():
		_ = w.Close()

		return ctx.Err()
	}
}

// handleConnection handles incoming WebSocket connections from Claude CLI.
func (w *WebSocketConnection) handleConnection(rw http.ResponseWriter, r *http.Request) {
	slog.Info("claude websocket: incoming connection", "remote", r.RemoteAddr)
	conn, err := websocket.Accept(rw, r, localAcceptOptions)
	if err != nil {
		slog.Error("claude websocket: accept failed", "error", err)

		return
	}

	w.connMu.Lock()
	w.conn = conn
	w.connMu.Unlock()

	slog.Info("claude websocket: connection established")

	// Signal ready
	w.readyOnce.Do(func() {
		close(w.ready)
	})

	// Use a context that cancels when done is closed
	ctx, cancel := context.WithCancel(r.Context())
	go func() {
		select {
		case <-w.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	defer cancel()

	// Read messages
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			// Shutdown in progress — context canceled or explicit Close
			status := websocket.CloseStatus(err)
			if w.closed.Load() || isClosed(w.done) ||
				status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway ||
				strings.Contains(err.Error(), "use of closed") {
				slog.Debug("claude websocket: connection closed", "error", err)
			} else {
				slog.Warn("claude websocket: read error", "error", err)
			}
			w.connected.Store(false)

			return
		}

		slog.Debug("claude websocket: raw message", "data", string(data))

		// Handle NDJSON - may have multiple JSON objects per message
		for line := range strings.SplitSeq(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var msg incomingMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				slog.Warn("claude websocket: invalid json", "line", line, "error", err)

				continue
			}

			slog.Debug("claude websocket: parsed message", "type", msg.Type, "session_id", msg.SessionID)
			w.handleIncomingMessage(msg)
		}
	}
}

// Connected returns true if connected.
func (w *WebSocketConnection) Connected() bool {
	return w.connected.Load()
}

// Close stops the connection.
func (w *WebSocketConnection) Close() error {
	w.closedOnce.Do(func() {
		w.closed.Store(true)
		w.connected.Store(false)

		// Cancel lifecycle context (stops sendLoop, closes done channel)
		if w.lifecycleCancel != nil {
			w.lifecycleCancel()
		}

		// Kill Claude process
		w.cmdMu.Lock()
		if w.cmd != nil && w.cmd.Process != nil {
			_ = w.cmd.Process.Kill()
		}
		w.cmdMu.Unlock()

		// Close WebSocket
		w.connMu.Lock()
		if w.conn != nil {
			_ = w.conn.CloseNow()
		}
		w.connMu.Unlock()

		// Stop HTTP server
		if w.server != nil {
			_ = w.server.Close()
		}

		close(w.events)
	})

	return nil
}

// isClosed reports whether ch has been closed (non-blocking).
func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
