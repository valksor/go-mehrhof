package socket

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/ratelimit"
	"github.com/valksor/kvelmo/internal/trace"
	"github.com/valksor/kvelmo/metrics"
)

// echoHandler returns the request params as the response result.
func echoHandler(_ context.Context, req *Request) *Response {
	resp, _ := NewResultResponse(req.ID, req.Params)

	return resp
}

func newTestRequest(method string) *Request {
	return &Request{
		JSONRPC: JSONRPCVersion,
		ID:      "test-1",
		Method:  method,
		Params:  json.RawMessage(`{"key":"value"}`),
	}
}

func TestRecoveryMiddleware_CatchesPanic(t *testing.T) {
	panicking := func(_ context.Context, _ *Request) *Response {
		panic("test panic")
	}

	mw := RecoveryMiddleware()
	wrapped := mw(panicking)

	resp := wrapped(context.Background(), newTestRequest("boom"))
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != ErrCodeInternal {
		t.Errorf("expected code %d, got %d", ErrCodeInternal, resp.Error.Code)
	}
}

func TestRecoveryMiddleware_PassesThrough(t *testing.T) {
	mw := RecoveryMiddleware()
	wrapped := mw(echoHandler)

	req := newTestRequest("echo")
	resp := wrapped(context.Background(), req)
	if resp == nil || resp.Error != nil {
		t.Fatal("expected successful response")
	}
}

func TestTraceMiddleware_AddsID(t *testing.T) {
	captured := make([]context.Context, 0, 1)
	capture := func(ctx context.Context, req *Request) *Response {
		captured = append(captured, ctx)

		return echoHandler(ctx, req)
	}

	mw := TraceMiddleware()
	wrapped := mw(capture)
	wrapped(context.Background(), newTestRequest("test"))

	if len(captured) == 0 {
		t.Fatal("handler was not called")
	}
	corrID := trace.ID(captured[0])
	if corrID == "" {
		t.Error("expected non-empty correlation ID in context")
	}
}

func TestMetricsMiddleware_RecordsSuccess(t *testing.T) {
	m := metrics.New()
	mw := MetricsMiddleware(m)
	wrapped := mw(echoHandler)

	wrapped(context.Background(), newTestRequest("echo"))

	if m.RPCRequests.Load() != 1 {
		t.Errorf("expected 1 RPC request, got %d", m.RPCRequests.Load())
	}
	if m.RPCErrors.Load() != 0 {
		t.Errorf("expected 0 errors, got %d", m.RPCErrors.Load())
	}
}

func TestMetricsMiddleware_RecordsError(t *testing.T) {
	m := metrics.New()
	mw := MetricsMiddleware(m)

	errHandler := func(_ context.Context, req *Request) *Response {
		return NewErrorResponse(req.ID, ErrCodeInternal, "something broke")
	}
	wrapped := mw(errHandler)

	wrapped(context.Background(), newTestRequest("fail"))

	if m.RPCRequests.Load() != 1 {
		t.Errorf("expected 1 RPC request, got %d", m.RPCRequests.Load())
	}
	if m.RPCErrors.Load() != 1 {
		t.Errorf("expected 1 error, got %d", m.RPCErrors.Load())
	}
}

type mockActivityLogger struct {
	mu      sync.Mutex
	entries []ActivityEntry
}

func (m *mockActivityLogger) Record(entry ActivityEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
}

func (m *mockActivityLogger) Entries() []ActivityEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]ActivityEntry{}, m.entries...)
}

func TestActivityLogMiddleware_Records(t *testing.T) {
	logger := &mockActivityLogger{}
	mw := ActivityLogMiddleware(func() ActivityLogger { return logger }, false)
	wrapped := mw(echoHandler)

	ctx := trace.WithID(context.Background(), "corr-123")
	wrapped(ctx, newTestRequest("echo"))

	entries := logger.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Method != "echo" {
		t.Errorf("expected method echo, got %s", entries[0].Method)
	}
	if entries[0].CorrelationID != "corr-123" {
		t.Errorf("expected corr-123, got %s", entries[0].CorrelationID)
	}
}

func TestActivityLogMiddleware_WriteOnly(t *testing.T) {
	logger := &mockActivityLogger{}
	mw := ActivityLogMiddleware(func() ActivityLogger { return logger }, true)
	wrapped := mw(echoHandler)

	// Read method should not be logged
	wrapped(context.Background(), newTestRequest("status"))
	if len(logger.Entries()) != 0 {
		t.Errorf("expected 0 entries for read method, got %d", len(logger.Entries()))
	}

	// Write method should be logged
	wrapped(context.Background(), newTestRequest("start"))
	if len(logger.Entries()) != 1 {
		t.Errorf("expected 1 entry for write method, got %d", len(logger.Entries()))
	}
}

func TestActivityLogMiddleware_NilLogger(t *testing.T) {
	mw := ActivityLogMiddleware(func() ActivityLogger { return nil }, false)
	wrapped := mw(echoHandler)

	// Should not panic with nil logger
	resp := wrapped(context.Background(), newTestRequest("echo"))
	if resp == nil || resp.Error != nil {
		t.Fatal("expected successful response with nil logger")
	}
}

func TestRateLimitMiddleware_Allows(t *testing.T) {
	limiter := ratelimit.NewLimiter(1000, 100)
	mw := RateLimitMiddleware(limiter, func(_ context.Context) string { return "test-key" })
	wrapped := mw(echoHandler)

	resp := wrapped(context.Background(), newTestRequest("echo"))
	if resp == nil || resp.Error != nil {
		t.Fatal("expected successful response within rate limit")
	}
}

func TestRateLimitMiddleware_Rejects(t *testing.T) {
	// Create a limiter with a very small burst to exhaust quickly
	limiter := ratelimit.NewLimiter(0.001, 1)
	mw := RateLimitMiddleware(limiter, func(_ context.Context) string { return "test-key" })
	wrapped := mw(echoHandler)

	// First request should succeed (uses the single burst token)
	resp := wrapped(context.Background(), newTestRequest("echo"))
	if resp.Error != nil {
		t.Fatal("first request should succeed")
	}

	// Second request should be rate limited
	resp = wrapped(context.Background(), newTestRequest("echo"))
	if resp == nil || resp.Error == nil {
		t.Fatal("expected rate limit error")
	}
	if resp.Error.Code != ErrCodeRateLimited {
		t.Errorf("expected code %d, got %d", ErrCodeRateLimited, resp.Error.Code)
	}
}

func TestRateLimitMiddleware_NilLimiter(t *testing.T) {
	mw := RateLimitMiddleware(nil, nil)
	wrapped := mw(echoHandler)

	resp := wrapped(context.Background(), newTestRequest("echo"))
	if resp == nil || resp.Error != nil {
		t.Fatal("expected passthrough with nil limiter")
	}
}

func TestBuildChain_Order(t *testing.T) {
	var order []string
	makeMW := func(name string) Middleware {
		return func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, req *Request) *Response {
				order = append(order, name+"-before")
				resp := next(ctx, req)
				order = append(order, name+"-after")

				return resp
			}
		}
	}

	chain := buildChain(echoHandler, []Middleware{
		makeMW("first"),
		makeMW("second"),
		makeMW("third"),
	})

	chain(context.Background(), newTestRequest("test"))

	expected := []string{
		"first-before", "second-before", "third-before",
		"third-after", "second-after", "first-after",
	}
	if len(order) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("position %d: expected %s, got %s", i, v, order[i])
		}
	}
}

func TestAdaptConnHandler(t *testing.T) {
	adapted := adaptConnHandler(func(_ context.Context, req *Request, _ net.Conn) (*Response, error) {
		return NewResultResponse(req.ID, map[string]string{"ok": "true"})
	})

	resp := adapted(context.Background(), newTestRequest("test"))
	if resp == nil || resp.Error != nil {
		t.Fatal("expected successful response from adapted handler")
	}

	// Test adaptConnHandler with error
	errAdapted := adaptConnHandler(func(_ context.Context, _ *Request, _ net.Conn) (*Response, error) {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "fail"}
	})
	resp = errAdapted(context.Background(), newTestRequest("test"))
	if resp == nil || resp.Error == nil {
		t.Fatal("expected error response from adapted handler returning error")
	}
}

func TestAdaptHandler(t *testing.T) {
	adapted := adaptHandler(func(_ context.Context, req *Request) (*Response, error) {
		return NewResultResponse(req.ID, map[string]string{"ok": "true"})
	})

	resp := adapted(context.Background(), newTestRequest("test"))
	if resp == nil || resp.Error != nil {
		t.Fatal("expected successful response")
	}
}

func TestAdaptHandler_Error(t *testing.T) {
	adapted := adaptHandler(func(_ context.Context, _ *Request) (*Response, error) {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "handler error"}
	})

	resp := adapted(context.Background(), newTestRequest("test"))
	if resp == nil || resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != ErrCodeInternal {
		t.Errorf("expected code %d, got %d", ErrCodeInternal, resp.Error.Code)
	}
}

func TestIsWriteMethod(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"start", true},
		{"stop", true},
		{"submit", true},
		{"status", false},
		{"projects.list", false},
		{"task.start", true},
		{"task.delete", true},
		{"task.update", true},
		{"echo", false},
		{"shutdown", true},
		{"projects.register", false}, // "register" is not a write verb
		{"plan", true},
		{"implement", true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := isWriteMethod(tt.method)
			if got != tt.want {
				t.Errorf("isWriteMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestConnInfoContext(t *testing.T) {
	ctx := context.Background()
	info := ConnInfoFromContext(ctx)
	if info.Key != "" {
		t.Error("expected empty key from bare context")
	}

	ctx = withConnInfo(ctx, ConnInfo{Key: "test-conn"})
	info = ConnInfoFromContext(ctx)
	if info.Key != "test-conn" {
		t.Errorf("expected test-conn, got %s", info.Key)
	}
}

func TestDefaultConnKeyFn(t *testing.T) {
	ctx := withConnInfo(context.Background(), ConnInfo{Key: "my-conn"})
	key := defaultConnKeyFn(ctx)
	if key != "my-conn" {
		t.Errorf("expected my-conn, got %s", key)
	}

	// Without conn info, should return a fallback
	key = defaultConnKeyFn(context.Background())
	if key == "" {
		t.Error("expected non-empty fallback key")
	}
}

func TestFullChain_EndToEnd(t *testing.T) {
	logger := &mockActivityLogger{}
	m := metrics.New()

	chain := []Middleware{
		RecoveryMiddleware(),
		TraceMiddleware(),
		RateLimitMiddleware(ratelimit.NewLimiter(1000, 100), defaultConnKeyFn),
		MetricsMiddleware(m),
		ActivityLogMiddleware(func() ActivityLogger { return logger }, false),
	}

	handler := buildChain(echoHandler, chain)

	ctx := withConnInfo(context.Background(), ConnInfo{Key: "full-test"})
	resp := handler(ctx, newTestRequest("echo"))

	if resp == nil || resp.Error != nil {
		t.Fatal("expected successful response from full chain")
	}

	// Verify metrics recorded
	if m.RPCRequests.Load() != 1 {
		t.Errorf("expected 1 RPC request, got %d", m.RPCRequests.Load())
	}

	// Verify activity logged
	entries := logger.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(entries))
	}
	if entries[0].CorrelationID == "" {
		t.Error("expected correlation ID to be set by trace middleware")
	}
}

func TestFullChain_PanicRecovery(t *testing.T) {
	m := metrics.New()

	chain := []Middleware{
		RecoveryMiddleware(),
		TraceMiddleware(),
		MetricsMiddleware(m),
	}

	panicker := func(_ context.Context, _ *Request) *Response {
		panic("boom")
	}

	handler := buildChain(panicker, chain)
	resp := handler(context.Background(), newTestRequest("panic"))

	if resp == nil || resp.Error == nil {
		t.Fatal("expected error response from panic")
	}
	if resp.Error.Code != ErrCodeInternal {
		t.Errorf("expected internal error code, got %d", resp.Error.Code)
	}
}

func TestServerUse_AddsMiddleware(t *testing.T) {
	srv := NewServer(filepath.Join(t.TempDir(), "test.sock"))

	initialCount := len(srv.middleware)

	srv.Use(func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *Request) *Response {
			return next(ctx, req)
		}
	})

	if len(srv.middleware) != initialCount+1 {
		t.Errorf("expected %d middleware, got %d", initialCount+1, len(srv.middleware))
	}

	// Middleware invocation is covered by TestServerMW_Roundtrip.
}

func TestServerMW_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "s.sock")

	logger := &mockActivityLogger{}
	m := metrics.New()

	srv := NewServer(sockPath,
		WithActivityLogger(logger),
		WithMetrics(m),
	)
	srv.Handle("echo", func(_ context.Context, req *Request) (*Response, error) {
		return NewResultResponse(req.ID, req.Params)
	})

	ctx := t.Context()

	go func() { _ = srv.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	client, err := NewClient(sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	params := json.RawMessage(`{"test":"middleware"}`)
	resp, err := client.Call(ctx, "echo", params)
	if err != nil {
		t.Fatal(err)
	}

	if string(resp.Result) != `{"test":"middleware"}` {
		t.Errorf("got %s", resp.Result)
	}

	// Give activity logger a moment to record
	time.Sleep(10 * time.Millisecond)

	entries := logger.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(entries))
	}
	if entries[0].Method != "echo" {
		t.Errorf("expected echo method, got %s", entries[0].Method)
	}
	if entries[0].CorrelationID == "" {
		t.Error("expected correlation ID from trace middleware")
	}

	if m.RPCRequests.Load() != 1 {
		t.Errorf("expected 1 RPC request in metrics, got %d", m.RPCRequests.Load())
	}
}
