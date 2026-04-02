package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/coder/websocket"
)

func newWsTransport(conn *websocket.Conn) *wsTransport {
	return &wsTransport{
		conn:    conn,
		pending: make(map[int64]chan *rpcResponse),
		closeCh: make(chan struct{}),
	}
}

func (t *wsTransport) readLoop(ctx context.Context) {
	// Create a context that cancels when the transport is closed
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-t.closeCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	defer cancel()

	for {
		_, data, err := t.conn.Read(ctx)
		if err != nil {
			if !t.closed.Load() {
				slog.Debug("codex ws read error", "error", err)
			}

			return
		}

		var msg rpcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		t.dispatch(msg)
	}
}

func (t *wsTransport) dispatch(msg rpcMessage) {
	// Response
	if msg.ID != nil && msg.Method == "" {
		t.pendingM.Lock()
		ch, ok := t.pending[*msg.ID]
		if ok {
			delete(t.pending, *msg.ID)
		}
		t.pendingM.Unlock()

		if ok {
			ch <- &rpcResponse{
				ID:     *msg.ID,
				Result: msg.Result,
				Error:  msg.Error,
			}
		}

		return
	}

	// Request
	if msg.ID != nil && msg.Method != "" {
		if t.requestHandler != nil {
			t.requestHandler(msg.Method, *msg.ID, msg.Params)
		}

		return
	}

	// Notification
	if msg.Method != "" {
		if t.notificationHandler != nil {
			t.notificationHandler(msg.Method, msg.Params)
		}
	}
}

func (t *wsTransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
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
	t.pendingM.Lock()
	t.pending[id] = respCh
	t.pendingM.Unlock()

	defer func() {
		t.pendingM.Lock()
		delete(t.pending, id)
		t.pendingM.Unlock()
	}()

	if err := t.write(ctx, req); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

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

func (t *wsTransport) Notify(ctx context.Context, method string, params any) error {
	if t.closed.Load() {
		return errors.New("transport closed")
	}

	msg := rpcRequest{
		JsonRpc: "2.0",
		Method:  method,
		Params:  params,
	}

	return t.write(ctx, msg)
}

func (t *wsTransport) Respond(ctx context.Context, id int64, result any) error {
	if t.closed.Load() {
		return errors.New("transport closed")
	}

	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}

	return t.write(ctx, msg)
}

func (t *wsTransport) write(ctx context.Context, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	t.connMu.Lock()
	defer t.connMu.Unlock()

	// coder/websocket is concurrent-write safe, but we keep the mutex
	// to serialize writes for protocol ordering guarantees.
	return t.conn.Write(ctx, websocket.MessageText, data)
}

func (t *wsTransport) Close() error {
	if t.closed.Swap(true) {
		return nil
	}
	close(t.closeCh)

	t.pendingM.Lock()
	for id := range t.pending {
		delete(t.pending, id)
	}
	t.pendingM.Unlock()

	return nil
}
