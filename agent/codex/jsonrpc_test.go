package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewJsonRpcTransport(t *testing.T) {
	r := strings.NewReader("")
	w := &bytes.Buffer{}

	transport := NewJsonRpcTransport(r, w)
	if transport == nil {
		t.Fatal("NewJsonRpcTransport() returned nil")
	}
}

func TestJsonRpcTransport_Connected(t *testing.T) {
	r := strings.NewReader("")
	w := &bytes.Buffer{}

	transport := NewJsonRpcTransport(r, w)
	if !transport.Connected() {
		t.Error("Connected() should return true for new transport")
	}

	_ = transport.Close()
	if transport.Connected() {
		t.Error("Connected() should return false after Close()")
	}
}

func TestJsonRpcTransport_Close_Idempotent(t *testing.T) {
	r := strings.NewReader("")
	w := &bytes.Buffer{}

	transport := NewJsonRpcTransport(r, w)

	// First close
	if err := transport.Close(); err != nil {
		t.Errorf("first Close() error = %v", err)
	}

	// Second close should not error
	if err := transport.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}
}

func TestJsonRpcTransport_Notify(t *testing.T) {
	r := strings.NewReader("")
	w := &bytes.Buffer{}

	transport := NewJsonRpcTransport(r, w)
	defer func() { _ = transport.Close() }()

	err := transport.Notify("test/method", map[string]any{"key": "value"})
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	// Parse written output
	var msg rpcRequest
	if err := json.Unmarshal(w.Bytes(), &msg); err != nil {
		t.Fatalf("failed to parse notification: %v", err)
	}

	if msg.Method != "test/method" {
		t.Errorf("Method = %q, want test/method", msg.Method)
	}
	if msg.ID != nil {
		t.Errorf("ID = %v, want nil (notification)", msg.ID)
	}
	if msg.JsonRpc != "2.0" {
		t.Errorf("JsonRpc = %q, want 2.0", msg.JsonRpc)
	}
}

func TestJsonRpcTransport_Notify_Closed(t *testing.T) {
	r := strings.NewReader("")
	w := &bytes.Buffer{}

	transport := NewJsonRpcTransport(r, w)
	_ = transport.Close()

	err := transport.Notify("test/method", nil)
	if err == nil {
		t.Error("Notify() on closed transport should return error")
	}
}

func TestJsonRpcTransport_Respond(t *testing.T) {
	r := strings.NewReader("")
	w := &bytes.Buffer{}

	transport := NewJsonRpcTransport(r, w)
	defer func() { _ = transport.Close() }()

	err := transport.Respond(42, map[string]any{"decision": "accept"})
	if err != nil {
		t.Fatalf("Respond() error = %v", err)
	}

	// Parse written output
	var msg map[string]any
	if err := json.Unmarshal(w.Bytes(), &msg); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	idVal, ok := msg["id"].(float64)
	if !ok {
		t.Fatal("id is not a float64")
	}
	if idVal != 42 {
		t.Errorf("id = %v, want 42", msg["id"])
	}
	if msg["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", msg["jsonrpc"])
	}
}

func TestJsonRpcTransport_Respond_Closed(t *testing.T) {
	r := strings.NewReader("")
	w := &bytes.Buffer{}

	transport := NewJsonRpcTransport(r, w)
	_ = transport.Close()

	err := transport.Respond(1, nil)
	if err == nil {
		t.Error("Respond() on closed transport should return error")
	}
}

func TestJsonRpcTransport_Call_Closed(t *testing.T) {
	r := strings.NewReader("")
	w := &bytes.Buffer{}

	transport := NewJsonRpcTransport(r, w)
	_ = transport.Close()

	_, err := transport.Call(context.Background(), "test/method", nil)
	if err == nil {
		t.Error("Call() on closed transport should return error")
	}
}

func TestJsonRpcTransport_OnNotification(t *testing.T) {
	// Create pipes for testing
	inputReader, inputWriter := io.Pipe()
	output := &bytes.Buffer{}

	transport := NewJsonRpcTransport(inputReader, output)
	defer func() { _ = transport.Close() }()

	var receivedMethod string
	var receivedParams json.RawMessage
	var wg sync.WaitGroup
	wg.Add(1)

	transport.OnNotification(func(method string, params json.RawMessage) {
		receivedMethod = method
		receivedParams = params
		wg.Done()
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	transport.Start(ctx)

	// Write a notification
	notification := `{"method":"test/notification","params":{"key":"value"}}` + "\n"
	go func() {
		_, _ = inputWriter.Write([]byte(notification))
		_ = inputWriter.Close()
	}()

	wg.Wait()

	if receivedMethod != "test/notification" {
		t.Errorf("received method = %q, want test/notification", receivedMethod)
	}
	if receivedParams == nil {
		t.Error("received params should not be nil")
	}
}

func TestJsonRpcTransport_OnRequest(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	output := &bytes.Buffer{}

	transport := NewJsonRpcTransport(inputReader, output)
	defer func() { _ = transport.Close() }()

	var receivedMethod string
	var receivedID int64
	var wg sync.WaitGroup
	wg.Add(1)

	transport.OnRequest(func(method string, id int64, params json.RawMessage) {
		receivedMethod = method
		receivedID = id
		wg.Done()
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	transport.Start(ctx)

	// Write a request (has both method and id)
	request := `{"method":"item/requestApproval","id":42,"params":{}}` + "\n"
	go func() {
		_, _ = inputWriter.Write([]byte(request))
		_ = inputWriter.Close()
	}()

	wg.Wait()

	if receivedMethod != "item/requestApproval" {
		t.Errorf("received method = %q, want item/requestApproval", receivedMethod)
	}
	if receivedID != 42 {
		t.Errorf("received id = %d, want 42", receivedID)
	}
}

func TestJsonRpcTransport_Call_Response(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()

	transport := NewJsonRpcTransport(inputReader, outputWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	transport.Start(ctx)

	// Start a goroutine to respond to the call
	go func() {
		// Read the request
		buf := make([]byte, 1024)
		n, _ := outputReader.Read(buf)
		var req rpcRequest
		_ = json.Unmarshal(buf[:n], &req)

		// Send response using typed struct to satisfy errchkjson
		type testResp struct {
			ID     int64          `json:"id"`
			Result map[string]any `json:"result"`
		}
		resp := testResp{
			ID:     *req.ID,
			Result: map[string]any{"success": true},
		}
		data, _ := json.Marshal(resp) //nolint:errchkjson // Test helper, struct is safe
		_, _ = inputWriter.Write(append(data, '\n'))
	}()

	// Make the call
	result, err := transport.Call(ctx, "test/method", map[string]any{"arg": 1})
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	var resultMap map[string]any
	if err := json.Unmarshal(result, &resultMap); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if resultMap["success"] != true {
		t.Errorf("result[success] = %v, want true", resultMap["success"])
	}
}

func TestJsonRpcTransport_Call_Error(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()

	transport := NewJsonRpcTransport(inputReader, outputWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	transport.Start(ctx)

	// Start a goroutine to respond with an error
	go func() {
		buf := make([]byte, 1024)
		n, _ := outputReader.Read(buf)
		var req rpcRequest
		_ = json.Unmarshal(buf[:n], &req)

		// Use typed struct to satisfy errchkjson
		type testErrorResp struct {
			ID    int64 `json:"id"`
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		resp := testErrorResp{ID: *req.ID}
		resp.Error.Code = -32600
		resp.Error.Message = "Invalid request"
		data, _ := json.Marshal(resp) //nolint:errchkjson // Test helper, struct is safe
		_, _ = inputWriter.Write(append(data, '\n'))
	}()

	_, err := transport.Call(ctx, "test/method", nil)
	if err == nil {
		t.Error("Call() should return error when server returns error")
	}
	if !strings.Contains(err.Error(), "Invalid request") {
		t.Errorf("error should contain 'Invalid request', got: %v", err)
	}
}

func TestJsonRpcTransport_Call_Timeout(t *testing.T) {
	r := strings.NewReader("") // Never responds
	w := &bytes.Buffer{}

	transport := NewJsonRpcTransport(r, w)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := transport.Call(ctx, "test/method", nil)
	if err == nil {
		t.Error("Call() should return error on timeout")
	}
}

func TestJsonRpcTransport_Call_ContextCanceled(t *testing.T) {
	inputReader, _ := io.Pipe()
	output := &bytes.Buffer{}

	transport := NewJsonRpcTransport(inputReader, output)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	transport.Start(ctx)

	// Cancel immediately
	cancel()

	_, err := transport.Call(ctx, "test/method", nil)
	if err == nil {
		t.Error("Call() should return error when context is canceled")
	}
}

func TestJsonRpcTransport_Dispatch_Response(t *testing.T) {
	r := strings.NewReader("")
	w := &bytes.Buffer{}

	transport := NewJsonRpcTransport(r, w)
	defer func() { _ = transport.Close() }()

	// Manually add a pending request
	id := transport.nextID.Add(1)
	respCh := make(chan *rpcResponse, 1)
	transport.pendingMu.Lock()
	transport.pending[id] = respCh
	transport.pendingMu.Unlock()

	// Dispatch a response
	msg := rpcMessage{
		ID:     &id,
		Result: json.RawMessage(`{"ok":true}`),
	}
	transport.dispatch(msg)

	select {
	case resp := <-respCh:
		if resp == nil {
			t.Error("received nil response")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("response not received")
	}
}
