package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
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

	cmd    *exec.Cmd
	cmdMu  sync.Mutex
	cmdErr error

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

	// Subagent tracker for detecting Task tool calls
	subagents *agent.SubagentTracker
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
	w.done = make(chan struct{})
	go func() {
		<-ctx.Done()
		close(w.done)
	}()

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

	// Start outgoing message sender
	go w.sendLoop(ctx)

	// Launch Claude CLI
	if err := w.launchClaude(ctx); err != nil {
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

// launchClaude starts the Claude CLI process with --sdk-url.
func (w *WebSocketConnection) launchClaude(ctx context.Context) error {
	w.cmdMu.Lock()
	defer w.cmdMu.Unlock()

	args := w.buildArgs()
	w.cmd = exec.CommandContext(ctx, w.config.Command[0], args...)

	if w.config.WorkDir != "" {
		w.cmd.Dir = w.config.WorkDir
	}

	// Build environment: start with parent env, exclude CLAUDECODE to allow nested sessions
	env := make([]string, 0, len(os.Environ())+len(w.config.Environment))
	for _, e := range os.Environ() {
		// Skip CLAUDECODE to allow running Claude CLI from within Claude Code
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			env = append(env, e)
		}
	}
	// Add custom config environment variables
	for k, v := range w.config.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	w.cmd.Env = env

	// Capture stdout for debugging
	stdout, err := w.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	// Capture stderr for debugging
	stderr, err := w.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := w.cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	slog.Info("claude CLI started", "pid", w.cmd.Process.Pid)

	// Log stdout in background
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			slog.Info("claude CLI stdout", "line", line)
		}
		slog.Info("claude CLI stdout closed")
	}()

	// Log stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			slog.Error("claude CLI stderr", "line", line)
			if strings.TrimSpace(line) != "" {
				w.events <- agent.Event{
					Type:      agent.EventError,
					Content:   line,
					Timestamp: time.Now(),
				}
			}
		}
		slog.Info("claude CLI stderr closed")
	}()

	// Wait for process completion in background
	go func() {
		err := w.cmd.Wait()
		slog.Info("claude CLI exited", "error", err)
		w.cmdMu.Lock()
		w.cmdErr = err
		w.cmdMu.Unlock()
		w.connected.Store(false)
	}()

	return nil
}

// buildArgs constructs CLI arguments for Claude with WebSocket.
func (w *WebSocketConnection) buildArgs() []string {
	args := []string{
		"--sdk-url", fmt.Sprintf("ws://127.0.0.1:%d", w.port),
		"--print",
		"--verbose",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--include-partial-messages",
		"--permission-mode", "bypassPermissions", // kvelmo manages permissions via KvelmoPermissionHandler
	}
	slog.Info("claude websocket buildArgs", "args", args)

	// Add configured arguments
	args = append(args, w.config.Args...)

	// Add model if specified
	if w.config.Model != "" {
		args = append(args, "--model", w.config.Model)
	}

	return args
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

			slog.Info("claude websocket: parsed message", "type", msg.Type, "session_id", msg.SessionID)
			w.handleIncomingMessage(msg)
		}
	}
}

// handleIncomingMessage processes messages from Claude CLI.
func (w *WebSocketConnection) handleIncomingMessage(msg incomingMessage) {
	switch msg.Type {
	case "system/init", "system":
		// Handle both old "system/init" and new "system" message formats
		if msg.SessionID != "" && w.sessionID == "" {
			slog.Info("claude websocket: session initialized", "session_id", msg.SessionID, "type", msg.Type)
			w.sessionID = msg.SessionID
			w.connected.Store(true)
			// Signal that session is ready
			w.sessionOnce.Do(func() {
				close(w.sessionReady)
			})
			w.events <- agent.Event{
				Type:      agent.EventInit,
				Content:   "Session initialized: " + w.sessionID,
				Timestamp: time.Now(),
			}
		}

	case "stream_event":
		content := msg.Content
		if content == "" {
			content = msg.Delta
		}
		w.events <- agent.Event{
			Type:      agent.EventStream,
			Content:   content,
			Timestamp: time.Now(),
		}

	case "assistant":
		if msg.Message != nil {
			content := extractTextContent(msg.Message.Content)
			if content != "" {
				w.events <- agent.Event{
					Type:      agent.EventAssistant,
					Content:   content,
					Timestamp: time.Now(),
				}
			}
		}

	case "control_request":
		if msg.Request != nil && msg.RequestID != "" {
			var input map[string]any
			_ = json.Unmarshal(msg.Request.Input, &input)

			// Store pending request for later response
			w.pendingRequestsMu.Lock()
			w.pendingRequests[msg.RequestID] = pendingRequest{
				Input:     input,
				ToolUseID: msg.Request.ToolUseID,
			}
			w.pendingRequestsMu.Unlock()

			req := agent.PermissionRequest{
				ID:     msg.RequestID,
				Tool:   msg.Request.ToolName,
				Input:  input,
				Action: msg.Request.Subtype,
			}
			slog.Info("claude websocket: control_request received", "id", req.ID, "tool", req.Tool)

			// Auto-handle with permission handler
			if w.config.PermissionHandler != nil {
				approved := w.config.PermissionHandler(req)
				_ = w.HandlePermission(req.ID, approved)
			} else {
				// Send event for external handling
				w.events <- agent.Event{
					Type:              agent.EventPermission,
					PermissionRequest: &req,
					Timestamp:         time.Now(),
				}
			}
		}

	case "result":
		// Result uses subtype:"success" for success, or is_error:true for errors
		if msg.Subtype == "success" && !msg.IsError {
			w.events <- agent.Event{
				Type:      agent.EventComplete,
				Timestamp: time.Now(),
			}
		} else {
			w.events <- agent.Event{
				Type:      agent.EventError,
				Error:     msg.Error,
				Timestamp: time.Now(),
			}
		}

	case "keep_alive":
		// Heartbeat - no action needed

	case "tool_use":
		var input map[string]any
		if msg.Input != nil {
			_ = json.Unmarshal(msg.Input, &input)
		}

		toolCallID := msg.ID
		if toolCallID == "" {
			toolCallID = uuid.NewString()
		}
		w.subagents.OnToolUse(toolCallID, msg.Name, input)

		w.events <- agent.Event{
			Type:      agent.EventToolUse,
			Content:   msg.Name,
			Data:      input,
			Timestamp: time.Now(),
		}

	case "tool_result":
		toolCallID := msg.ToolUseID
		if toolCallID != "" {
			w.subagents.OnToolResult(toolCallID, !msg.IsError, msg.Error)
		}

		w.events <- agent.Event{
			Type:      agent.EventToolResult,
			Content:   msg.Result,
			Timestamp: time.Now(),
		}

	case "tool_progress":
		// Tool execution heartbeat - shows elapsed time for long-running tools
		w.events <- agent.Event{
			Type:      agent.EventToolProgress,
			Timestamp: time.Now(),
			Data: map[string]any{
				"tool_use_id":     msg.ToolUseID,
				"tool_name":       msg.ToolName,
				"elapsed_seconds": msg.ElapsedTimeSeconds,
			},
		}

	case "streamlined_text":
		// Simplified text output mode (Claude v2.1.81+)
		content := msg.Content
		if content == "" {
			content = msg.Delta
		}
		if content != "" {
			w.events <- agent.Event{
				Type:      agent.EventStream,
				Content:   content,
				Timestamp: time.Now(),
			}
		}

	case "streamlined_tool_use_summary":
		// Compact tool usage summary (e.g., "Read 2 files, wrote 1 file")
		w.events <- agent.Event{
			Type:      agent.EventToolUse,
			Content:   msg.Content,
			Timestamp: time.Now(),
		}

	case "prompt_suggestion":
		w.events <- agent.Event{
			Type:      agent.EventPromptSuggestion,
			Timestamp: time.Now(),
			Data: map[string]any{
				"suggestions": msg.Suggestions,
			},
		}

	default:
		slog.Debug("claude websocket: unhandled message type", "type", msg.Type)
	}
}

// extractTextContent extracts text content from Claude's message content field.
// Content can be either a simple string or an array of content blocks.
func extractTextContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try string first
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}

	// Try array of content blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var texts []string
		for _, block := range blocks {
			if block.Type == "text" && block.Text != "" {
				texts = append(texts, block.Text)
			}
		}

		return strings.Join(texts, "\n")
	}

	return ""
}

// sendLoop sends outgoing messages to Claude CLI.
func (w *WebSocketConnection) sendLoop(ctx context.Context) {
	for {
		select {
		case msg := <-w.outgoing:
			w.connMu.Lock()
			if w.conn != nil {
				data, err := json.Marshal(msg)
				if err == nil {
					slog.Info("claude websocket: sending message", "type", msg.Type, "len", len(data), "data", string(data))
					data = append(data, '\n')
					_ = w.conn.Write(ctx, websocket.MessageText, data)
				}
			}
			w.connMu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

// Connected returns true if connected.
func (w *WebSocketConnection) Connected() bool {
	return w.connected.Load()
}

// SendPrompt sends a user prompt and returns the event stream.
func (w *WebSocketConnection) SendPrompt(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	if w.sessionID == "" {
		return nil, errors.New("not connected (no session)")
	}

	w.outgoing <- outgoingMessage{
		Type:      "user",
		SessionID: w.sessionID,
		Message: &struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			Role:    "user",
			Content: prompt,
		},
	}

	// Return filtered event stream
	filtered := make(chan agent.Event, 100)
	go func() {
		defer close(filtered)
		for {
			select {
			case event, ok := <-w.events:
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

// HandlePermission sends a permission response.
func (w *WebSocketConnection) HandlePermission(requestID string, approved bool) error {
	// Get the stored request info
	w.pendingRequestsMu.Lock()
	pending, ok := w.pendingRequests[requestID]
	if ok {
		delete(w.pendingRequests, requestID)
	}
	w.pendingRequestsMu.Unlock()

	var inner *controlResponseInner
	if approved {
		inner = &controlResponseInner{
			Behavior:     "allow",
			UpdatedInput: pending.Input, // Pass through original input
			ToolUseID:    pending.ToolUseID,
		}
	} else {
		inner = &controlResponseInner{
			Behavior:  "deny",
			Message:   "Permission denied by kvelmo",
			Interrupt: false,
			ToolUseID: pending.ToolUseID,
		}
	}

	slog.Info("claude websocket: sending control_response", "request_id", requestID, "behavior", inner.Behavior)
	w.outgoing <- outgoingMessage{
		Type: "control_response",
		Response: &controlResponsePayload{
			Subtype:   "success",
			RequestID: requestID,
			Response:  inner,
		},
	}

	return nil
}

// Interrupt sends an interrupt control request to abort the current agent turn.
func (w *WebSocketConnection) Interrupt() error {
	if !w.connected.Load() {
		return nil // Not connected, nothing to interrupt
	}

	requestID := uuid.NewString()
	slog.Info("claude websocket: sending interrupt", "request_id", requestID)

	w.outgoing <- outgoingMessage{
		Type:      "control_request",
		RequestID: requestID,
		Request: &controlRequestPayload{
			Subtype: "interrupt",
		},
	}

	// Emit interrupted event
	w.events <- agent.Event{
		Type:      agent.EventInterrupted,
		Content:   "Agent turn interrupted",
		Timestamp: time.Now(),
	}

	return nil
}

// Close stops the connection.
func (w *WebSocketConnection) Close() error {
	w.closedOnce.Do(func() {
		w.closed.Store(true)
		w.connected.Store(false)

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
