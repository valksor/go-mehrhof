package apiagent_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/apiagent"
)

// drainEvents collects every event until the channel closes, failing on timeout.
func drainEvents(t *testing.T, ch <-chan agent.Event) []agent.Event {
	t.Helper()
	var events []agent.Event
	timeout := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, e)
		case <-timeout:
			t.Fatal("timed out draining events")

			return events
		}
	}
}

func hasErrorEvent(events []agent.Event) (agent.Event, bool) {
	for _, e := range events {
		if e.Type == agent.EventError {
			return e, true
		}
	}

	return agent.Event{}, false
}

// errBuildProvider fails in BuildRequest.
type errBuildProvider struct {
	mockProvider

	buildErr error
}

func (p *errBuildProvider) BuildRequest(_ context.Context, _ *apiagent.APIConfig, _ []apiagent.Message, _ []apiagent.ToolDef) (*http.Request, error) {
	return nil, p.buildErr
}

func TestBaseBuildRequestError(t *testing.T) {
	provider := &errBuildProvider{
		mockProvider: mockProvider{name: "test"},
		buildErr:     errors.New("cannot build"),
	}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	events, err := base.SendPrompt(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}

	collected := drainEvents(t, events)
	ev, ok := hasErrorEvent(collected)
	if !ok {
		t.Fatalf("expected error event, got %+v", collected)
	}
	if !contains(ev.Error, "build request") {
		t.Errorf("expected build request error, got %q", ev.Error)
	}
}

func TestBaseNon200Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := &mockProvider{name: "test", server: server}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	events, err := base.SendPrompt(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}

	collected := drainEvents(t, events)
	ev, ok := hasErrorEvent(collected)
	if !ok {
		t.Fatalf("expected error event, got %+v", collected)
	}
	if !contains(ev.Error, "status 500") {
		t.Errorf("expected status 500 error, got %q", ev.Error)
	}
}

// httpErrProvider builds a request to an unreachable URL so the HTTP Do fails.
type httpErrProvider struct {
	mockProvider
}

func (p *httpErrProvider) BuildRequest(ctx context.Context, _ *apiagent.APIConfig, _ []apiagent.Message, _ []apiagent.ToolDef) (*http.Request, error) {
	// 127.0.0.1:0 is not connectable.
	return http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:0", nil)
}

func TestBaseHTTPRequestError(t *testing.T) {
	provider := &httpErrProvider{mockProvider: mockProvider{name: "test"}}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()
	cfg.Timeout = 2 * time.Second

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	events, err := base.SendPrompt(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}

	collected := drainEvents(t, events)
	ev, ok := hasErrorEvent(collected)
	if !ok {
		t.Fatalf("expected error event, got %+v", collected)
	}
	if !contains(ev.Error, "API request") {
		t.Errorf("expected API request error, got %q", ev.Error)
	}
}

// parseErrProvider returns an error from ParseStream.
type parseErrProvider struct {
	mockProvider
}

func (p *parseErrProvider) ParseStream(_ context.Context, body io.ReadCloser) (<-chan apiagent.Chunk, error) {
	_ = body.Close()

	return nil, errors.New("parse boom")
}

func TestBaseParseStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := &parseErrProvider{mockProvider: mockProvider{name: "test", server: server}}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	events, err := base.SendPrompt(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}

	collected := drainEvents(t, events)
	ev, ok := hasErrorEvent(collected)
	if !ok {
		t.Fatalf("expected error event, got %+v", collected)
	}
	if !contains(ev.Error, "parse stream") {
		t.Errorf("expected parse stream error, got %q", ev.Error)
	}
}

// chunkErrProvider emits a ChunkError mid-stream.
type chunkErrProvider struct {
	mockProvider
}

func (p *chunkErrProvider) ParseStream(_ context.Context, body io.ReadCloser) (<-chan apiagent.Chunk, error) {
	ch := make(chan apiagent.Chunk, 2)
	go func() {
		defer close(ch)
		defer body.Close() //nolint:errcheck // test cleanup
		ch <- apiagent.Chunk{Type: apiagent.ChunkText, Text: "partial "}
		ch <- apiagent.Chunk{Type: apiagent.ChunkError, Error: "stream exploded"}
	}()

	return ch, nil
}

func TestBaseChunkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "data: anything\n\n")
	}))
	defer server.Close()

	provider := &chunkErrProvider{mockProvider: mockProvider{name: "test", server: server}}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	events, err := base.SendPrompt(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}

	collected := drainEvents(t, events)
	ev, ok := hasErrorEvent(collected)
	if !ok {
		t.Fatalf("expected error event, got %+v", collected)
	}
	if ev.Error != "stream exploded" {
		t.Errorf("expected 'stream exploded', got %q", ev.Error)
	}
}

func TestBaseMaxTurnsExceeded(t *testing.T) {
	// Always request a tool call so the loop never finishes; cap MaxTurns low.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(
			w,
			"data: {\"type\":\"tool_use\",\"id\":\"c1\",\"name\":\"list_dir\",\"input\":{}}\n\n",
			"data: [DONE]\n\n",
		)
	}))
	defer server.Close()

	provider := &mockProvider{name: "test", server: server}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()
	cfg.MaxTurns = 2

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	events, err := base.SendPrompt(context.Background(), "loop forever")
	if err != nil {
		t.Fatal(err)
	}

	collected := drainEvents(t, events)
	ev, ok := hasErrorEvent(collected)
	if !ok {
		t.Fatalf("expected error event for max turns, got %+v", collected)
	}
	if !contains(ev.Error, "maximum conversation turns") {
		t.Errorf("expected max turns error, got %q", ev.Error)
	}
}

// usageProvider reports token usage on the done chunk and keeps requesting tools
// so the budget can be exceeded.
type usageProvider struct {
	mockProvider
}

func (p *usageProvider) ParseStream(_ context.Context, body io.ReadCloser) (<-chan apiagent.Chunk, error) {
	ch := make(chan apiagent.Chunk, 3)
	go func() {
		defer close(ch)
		defer body.Close() //nolint:errcheck // test cleanup
		ch <- apiagent.Chunk{
			Type:    apiagent.ChunkToolUse,
			ToolUse: &apiagent.ToolUseChunk{ID: "c1", Name: "list_dir", Input: map[string]any{}},
		}
		ch <- apiagent.Chunk{Type: apiagent.ChunkDone, Usage: &apiagent.UsageData{InputTokens: 100, OutputTokens: 100}}
	}()

	return ch, nil
}

func TestBaseTokenBudgetExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "data: x\n\n")
	}))
	defer server.Close()

	provider := &usageProvider{mockProvider: mockProvider{name: "test", server: server}}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()
	cfg.TokenBudget = 150 // first turn uses 200; second turn check trips the budget
	cfg.MaxTurns = 10

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	events, err := base.SendPrompt(context.Background(), "spend tokens")
	if err != nil {
		t.Fatal(err)
	}

	collected := drainEvents(t, events)
	ev, ok := hasErrorEvent(collected)
	if !ok {
		t.Fatalf("expected error event for budget, got %+v", collected)
	}
	if !contains(ev.Error, "token budget exceeded") {
		t.Errorf("expected token budget error, got %q", ev.Error)
	}
}

func TestBaseContextCancellation(t *testing.T) {
	// Server holds the connection open; canceling the context should unblock the loop.
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "data: {\"type\":\"text\",\"content\":\"hi\"}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)

	provider := &mockProvider{name: "test", server: server}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	events, err := base.SendPrompt(ctx, "go")
	if err != nil {
		t.Fatal(err)
	}

	// Consume the first stream event, then cancel.
	<-events
	cancel()

	// Channel must close without hanging.
	collected := drainEvents(t, events)
	_ = collected
}

func TestBaseConnectWithConnector(t *testing.T) {
	provider := &connectorProvider{mockProvider: mockProvider{name: "ollama"}}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatalf("Connect with connector: %v", err)
	}
	if !provider.connectCalled {
		t.Error("expected Connector.Connect to be called")
	}
	if !base.Connected() {
		t.Error("expected Connected() true after Connect")
	}
}

func TestBaseConnectorError(t *testing.T) {
	provider := &connectorProvider{
		mockProvider: mockProvider{name: "ollama"},
		connectErr:   errors.New("model pull failed"),
	}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()

	base := apiagent.NewBase(provider, cfg)
	err := base.Connect(context.Background())
	if err == nil {
		t.Fatal("expected connector error")
	}
	if base.Connected() {
		t.Error("should not be connected after connector error")
	}
}

// connectorProvider implements the optional Connector interface.
type connectorProvider struct {
	mockProvider

	connectCalled bool
	connectErr    error
}

func (p *connectorProvider) Connect(_ context.Context, _ *apiagent.APIConfig) error {
	p.connectCalled = true

	return p.connectErr
}
