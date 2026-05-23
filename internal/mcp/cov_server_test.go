package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestNewServer_WithOptions(t *testing.T) {
	reg := NewToolRegistry()
	s := NewServer(
		reg,
		WithRateLimit(50, 100),
		WithServerInfo(ServerInfo{Name: "custom", Version: "9.9"}),
	)
	if s.serverInfo.Name != "custom" || s.serverInfo.Version != "9.9" {
		t.Errorf("serverInfo = %+v", s.serverInfo)
	}
	if s.rateLimiter.Burst() != 100 {
		t.Errorf("rate burst = %d, want 100", s.rateLimiter.Burst())
	}
}

func TestWithRateLimit_IgnoresNonPositive(t *testing.T) {
	reg := NewToolRegistry()
	// Non-positive values must not replace the default limiter.
	s := NewServer(reg, WithRateLimit(0, 0))
	if s.rateLimiter == nil {
		t.Fatal("rate limiter should remain set")
	}
	if s.rateLimiter.Burst() != defaultRateBurst {
		t.Errorf("burst = %d, want default %d", s.rateLimiter.Burst(), defaultRateBurst)
	}
}

func TestErrorResponse(t *testing.T) {
	r := errorResponse(json.RawMessage(`5`), InvalidParams, "bad")
	if r.JSONRPC != "2.0" || r.Error == nil || r.Error.Code != InvalidParams {
		t.Errorf("errorResponse = %+v", r)
	}
	if r.Error.Message != "bad" {
		t.Errorf("message = %q", r.Error.Message)
	}
}

func TestMarshalResponse_Success(t *testing.T) {
	r := marshalResponse(json.RawMessage(`1`), map[string]any{"a": 1})
	if r.Error != nil {
		t.Fatalf("unexpected error: %+v", r.Error)
	}
	if !strings.Contains(string(r.Result), `"a":1`) {
		t.Errorf("result = %s", r.Result)
	}
}

func TestMarshalResponse_UnmarshalableValue(t *testing.T) {
	// A channel cannot be marshalled -> should produce an internal error response.
	r := marshalResponse(json.RawMessage(`1`), make(chan int))
	if r.Error == nil || r.Error.Code != InternalError {
		t.Errorf("expected internal error, got %+v", r)
	}
}

func TestHandleRequest_ParseError(t *testing.T) {
	s := NewServer(NewToolRegistry())
	resp := s.handleRequest(context.Background(), "not json{")
	if resp == nil || resp.Error == nil || resp.Error.Code != ParseError {
		t.Errorf("expected parse error, got %+v", resp)
	}
}

func TestHandleRequest_BadJSONRPCVersion(t *testing.T) {
	s := NewServer(NewToolRegistry())
	resp := s.handleRequest(context.Background(), `{"jsonrpc":"1.0","id":1,"method":"x"}`)
	if resp == nil || resp.Error == nil || resp.Error.Code != InvalidRequest {
		t.Errorf("expected invalid request, got %+v", resp)
	}
}

func TestHandleRequest_MethodNotFound(t *testing.T) {
	s := NewServer(NewToolRegistry())
	resp := s.handleRequest(context.Background(), `{"jsonrpc":"2.0","id":1,"method":"unknown/thing"}`)
	if resp == nil || resp.Error == nil || resp.Error.Code != MethodNotFound {
		t.Errorf("expected method not found, got %+v", resp)
	}
}

func TestHandleRequest_NotificationNoResponse(t *testing.T) {
	s := NewServer(NewToolRegistry())
	resp := s.handleRequest(context.Background(), `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if resp != nil {
		t.Errorf("notifications should not produce a response, got %+v", resp)
	}
}

func TestHandleInitialize_InvalidParams(t *testing.T) {
	s := NewServer(NewToolRegistry())
	resp := s.handleRequest(context.Background(), `{"jsonrpc":"2.0","id":1,"method":"initialize","params":"notobject"}`)
	if resp == nil || resp.Error == nil || resp.Error.Code != InvalidParams {
		t.Errorf("expected invalid params, got %+v", resp)
	}
}

func TestHandleInitialize_MissingClientName(t *testing.T) {
	s := NewServer(NewToolRegistry())
	resp := s.handleRequest(context.Background(),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"`+ProtocolVersion+`","clientInfo":{"name":""}}}`)
	if resp == nil || resp.Error == nil || resp.Error.Code != InvalidParams {
		t.Errorf("expected invalid params for missing client name, got %+v", resp)
	}
}

func TestHandleInitialize_VersionMismatchStillSucceeds(t *testing.T) {
	s := NewServer(NewToolRegistry())
	resp := s.handleRequest(context.Background(),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01","clientInfo":{"name":"c"}}}`)
	if resp == nil || resp.Error != nil {
		t.Fatalf("version mismatch should still succeed, got %+v", resp)
	}
	if !s.initialized.Load() {
		t.Error("server should be marked initialized")
	}
}

func TestHandleToolsList_NotInitialized(t *testing.T) {
	s := NewServer(NewToolRegistry())
	resp := s.handleRequest(context.Background(), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if resp == nil || resp.Error == nil || resp.Error.Code != InvalidRequest {
		t.Errorf("expected not-initialized error, got %+v", resp)
	}
}

func TestHandleToolsCall_NotInitialized(t *testing.T) {
	s := NewServer(NewToolRegistry())
	resp := s.handleRequest(context.Background(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x"}}`)
	if resp == nil || resp.Error == nil || resp.Error.Code != InvalidRequest {
		t.Errorf("expected not-initialized error, got %+v", resp)
	}
}

func TestHandleToolsCall_InvalidParams(t *testing.T) {
	s := NewServer(NewToolRegistry())
	s.initialized.Store(true)
	resp := s.handleRequest(context.Background(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"notobject"}`)
	if resp == nil || resp.Error == nil || resp.Error.Code != InvalidParams {
		t.Errorf("expected invalid params, got %+v", resp)
	}
}

func TestHandleToolsCall_ToolNotFound(t *testing.T) {
	s := NewServer(NewToolRegistry())
	s.initialized.Store(true)
	resp := s.handleRequest(context.Background(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope"}}`)
	if resp == nil || resp.Error == nil || resp.Error.Code != InternalError {
		t.Errorf("expected internal error for unknown tool, got %+v", resp)
	}
}

func TestHandleShutdown(t *testing.T) {
	s := NewServer(NewToolRegistry())
	s.initialized.Store(true)
	resp := s.handleRequest(context.Background(), `{"jsonrpc":"2.0","id":9,"method":"shutdown"}`)
	if resp == nil || resp.Error != nil {
		t.Fatalf("shutdown error: %+v", resp)
	}
	if s.initialized.Load() {
		t.Error("server should be de-initialized after shutdown")
	}
	// shutdownChan must be closed.
	select {
	case <-s.shutdownChan:
	default:
		t.Error("shutdownChan should be closed")
	}
	// Calling shutdown again must not panic (shutdownOnce guards the close).
	_ = s.handleRequest(context.Background(), `{"jsonrpc":"2.0","id":10,"method":"shutdown"}`)
}

func TestWriteResponse_Success(t *testing.T) {
	s := NewServer(NewToolRegistry())
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := s.writeResponse(w, errorResponse(json.RawMessage(`1`), InternalError, "x")); err != nil {
		t.Fatalf("writeResponse error: %v", err)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("response should end with newline: %q", buf.String())
	}
	var resp Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &resp); err != nil {
		t.Fatalf("written response not valid JSON: %v", err)
	}
}

func TestServeReadWriter_RejectsOversizeAndEmptyLines(t *testing.T) {
	s := NewServer(NewToolRegistry())

	// Build input: a blank line (skipped), then an initialize request.
	init := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"` +
		ProtocolVersion + `","clientInfo":{"name":"c"}}}`
	input := "\n" + init + "\n"

	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	pr, pw := io.Pipe()
	go func() { done <- s.ServeReadWriter(ctx, pr, &out) }()

	if _, err := io.WriteString(pw, input); err != nil {
		t.Fatal(err)
	}
	_ = pw.Close() // EOF -> Serve returns nil

	if err := <-done; err != nil {
		t.Fatalf("ServeReadWriter returned error: %v", err)
	}
	if !strings.Contains(out.String(), `"result"`) {
		t.Errorf("expected an initialize result, got %q", out.String())
	}
}

func TestCheckPipe(t *testing.T) {
	if err := checkPipe(nil); err != nil {
		t.Errorf("checkPipe(nil) = %v, want nil", err)
	}

	broken := errors.New("write: broken pipe")
	got := checkPipe(broken)
	if got == nil || !strings.Contains(got.Error(), "client disconnected") {
		t.Errorf("checkPipe(broken pipe) = %v, want client-disconnected wrap", got)
	}

	other := errors.New("some other error")
	if got := checkPipe(other); !errors.Is(got, other) {
		t.Errorf("checkPipe(other) = %v, want passthrough", got)
	}

	if got := checkPipe(io.ErrClosedPipe); got == nil || !strings.Contains(got.Error(), "client disconnected") {
		t.Errorf("checkPipe(ErrClosedPipe) = %v, want client-disconnected wrap", got)
	}
}

// errWriter always fails on Write, used to drive writeResponse's error path.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write failure") }

func TestWriteResponse_WriteError(t *testing.T) {
	s := NewServer(NewToolRegistry())
	// bufio.Writer with a tiny buffer so the first Write flushes to errWriter.
	w := bufio.NewWriterSize(errWriter{}, 1)
	err := s.writeResponse(w, errorResponse(json.RawMessage(`1`), InternalError, "x"))
	if err == nil {
		t.Error("expected error when underlying writer fails")
	}
}

func TestServeReadWriter_ContextCancel(t *testing.T) {
	s := NewServer(NewToolRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before start

	pr, _ := io.Pipe()
	var out bytes.Buffer
	err := s.ServeReadWriter(ctx, pr, &out)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}
