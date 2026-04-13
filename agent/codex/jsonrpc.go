package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// JsonRpcTransport handles JSON-RPC 2.0 communication for Codex app-server.
// Used by both WebSocket and stdio modes.
type JsonRpcTransport struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex

	// Pending requests awaiting responses
	pending   map[int64]chan *rpcResponse
	pendingMu sync.Mutex
	nextID    atomic.Int64

	// Handlers for incoming messages
	notificationHandler func(method string, params json.RawMessage)
	requestHandler      func(method string, id int64, params json.RawMessage)

	closed  atomic.Bool
	closeCh chan struct{}
}

type rpcRequest struct {
	JsonRpc string `json:"jsonrpc,omitempty"`
	Method  string `json:"method"`
	ID      *int64 `json:"id,omitempty"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	ID     int64           `json:"id,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type rpcMessage struct {
	JsonRpc string          `json:"jsonrpc,omitempty"`
	Method  string          `json:"method,omitempty"`
	ID      *int64          `json:"id,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// NewJsonRpcTransport creates a transport for JSON-RPC communication.
func NewJsonRpcTransport(reader io.Reader, writer io.Writer) *JsonRpcTransport {
	return &JsonRpcTransport{
		reader:  bufio.NewReader(reader),
		writer:  writer,
		pending: make(map[int64]chan *rpcResponse),
		closeCh: make(chan struct{}),
	}
}

// Start begins reading messages from the transport.
// Call this in a goroutine after setting up handlers.
func (t *JsonRpcTransport) Start(ctx context.Context) {
	go t.readLoop(ctx)
}

func (t *JsonRpcTransport) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.closeCh:
			return
		default:
		}

		line, err := t.reader.ReadString('\n')
		if err != nil {
			// Only close on unexpected errors (not EOF or already closed)
			_ = t.Close()

			return
		}

		if line == "" || line == "\n" {
			continue
		}

		var msg rpcMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		t.dispatch(msg)
	}
}

func (t *JsonRpcTransport) dispatch(msg rpcMessage) {
	// Response to our request (has ID but no method)
	if msg.ID != nil && msg.Method == "" {
		t.pendingMu.Lock()
		ch, ok := t.pending[*msg.ID]
		if ok {
			delete(t.pending, *msg.ID)
		}
		t.pendingMu.Unlock()

		if ok {
			ch <- &rpcResponse{
				ID:     *msg.ID,
				Result: msg.Result,
				Error:  msg.Error,
			}
		}

		return
	}

	// Request from server (has both ID and method) - needs response
	if msg.ID != nil && msg.Method != "" {
		if t.requestHandler != nil {
			t.requestHandler(msg.Method, *msg.ID, msg.Params)
		}

		return
	}

	// Notification (has method but no ID)
	if msg.Method != "" {
		if t.notificationHandler != nil {
			t.notificationHandler(msg.Method, msg.Params)
		}
	}
}

// Call sends a request and waits for the response.
func (t *JsonRpcTransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if t.closed.Load() {
		return nil, errors.New("transport closed")
	}

	id := t.nextID.Add(1)
	req := rpcRequest{
		JsonRpc: "2.0",
		Method:  method,
		ID:      &id,
		Params:  params,
	}

	respCh := make(chan *rpcResponse, 1)
	t.pendingMu.Lock()
	t.pending[id] = respCh
	t.pendingMu.Unlock()

	defer func() {
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()
	}()

	if err := t.write(req); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Default timeout for RPC calls
	timeout := 60 * time.Second
	if method == "thread/start" || method == "thread/resume" {
		timeout = 120 * time.Second
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}

		return resp.Result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("rpc timeout: %s", method)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.closeCh:
		return nil, errors.New("transport closed")
	}
}

// Notify sends a notification (no response expected).
func (t *JsonRpcTransport) Notify(method string, params any) error {
	if t.closed.Load() {
		return errors.New("transport closed")
	}

	msg := rpcRequest{
		JsonRpc: "2.0",
		Method:  method,
		Params:  params,
	}

	return t.write(msg)
}

// Respond sends a response to a server request (e.g., approval).
func (t *JsonRpcTransport) Respond(id int64, result any) error {
	if t.closed.Load() {
		return errors.New("transport closed")
	}

	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}

	return t.write(msg)
}

// OnNotification sets the handler for incoming notifications.
func (t *JsonRpcTransport) OnNotification(handler func(method string, params json.RawMessage)) {
	t.notificationHandler = handler
}

// OnRequest sets the handler for incoming requests (that need responses).
func (t *JsonRpcTransport) OnRequest(handler func(method string, id int64, params json.RawMessage)) {
	t.requestHandler = handler
}

func (t *JsonRpcTransport) write(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	data = append(data, '\n')
	_, err = t.writer.Write(data)

	return err
}

// Close shuts down the transport.
func (t *JsonRpcTransport) Close() error {
	if t.closed.Swap(true) {
		return nil
	}
	close(t.closeCh)

	// Cancel all pending requests
	t.pendingMu.Lock()
	for id, ch := range t.pending {
		close(ch)
		delete(t.pending, id)
	}
	t.pendingMu.Unlock()

	return nil
}

// Connected returns true if the transport is open.
func (t *JsonRpcTransport) Connected() bool {
	return !t.closed.Load()
}
