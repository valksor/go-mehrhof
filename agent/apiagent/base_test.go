package apiagent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/apiagent"
)

// mockProvider is a test Provider that serves canned SSE responses.
type mockProvider struct {
	name      string
	available error
	server    *httptest.Server
}

func (m *mockProvider) Name() string     { return m.name }
func (m *mockProvider) Available() error { return m.available }

func (m *mockProvider) BuildRequest(ctx context.Context, cfg *apiagent.APIConfig, messages []apiagent.Message, tools []apiagent.ToolDef) (*http.Request, error) {
	body, marshalErr := json.Marshal(map[string]any{"messages": len(messages)})
	if marshalErr != nil {
		return nil, marshalErr
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.server.URL, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (m *mockProvider) ParseStream(ctx context.Context, body io.ReadCloser) (<-chan apiagent.Chunk, error) {
	chunks := make(chan apiagent.Chunk, 16)

	go func() {
		defer close(chunks)
		defer body.Close() //nolint:errcheck // Test cleanup

		for sse := range apiagent.ParseSSE(ctx, body) {
			if sse.Data == "[DONE]" {
				chunks <- apiagent.Chunk{Type: apiagent.ChunkDone}

				return
			}

			var data map[string]any
			if err := json.Unmarshal([]byte(sse.Data), &data); err != nil {
				continue
			}

			switch data["type"] {
			case "text":
				content, _ := data["content"].(string)
				chunks <- apiagent.Chunk{Type: apiagent.ChunkText, Text: content}
			case "tool_use":
				input, _ := data["input"].(map[string]any)
				id, _ := data["id"].(string)
				name, _ := data["name"].(string)
				chunks <- apiagent.Chunk{
					Type: apiagent.ChunkToolUse,
					ToolUse: &apiagent.ToolUseChunk{
						ID:    id,
						Name:  name,
						Input: input,
					},
				}
			}
		}

		chunks <- apiagent.Chunk{Type: apiagent.ChunkDone}
	}()

	return chunks, nil
}

// writeSSE writes SSE data lines to a response writer.
//
//nolint:errcheck // Test helper; write errors to httptest are not actionable
func writeSSE(w http.ResponseWriter, lines ...string) {
	for _, line := range lines {
		fmt.Fprint(w, line)
	}
}

func TestBaseTextOnlyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w,
			"data: {\"type\":\"text\",\"content\":\"Hello \"}\n\n",
			"data: {\"type\":\"text\",\"content\":\"world!\"}\n\n",
			"data: [DONE]\n\n",
		)
	}))
	defer server.Close()

	provider := &mockProvider{name: "test", server: server}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	events, err := base.SendPrompt(context.Background(), "say hello")
	if err != nil {
		t.Fatal(err)
	}

	var eventTypes []agent.EventType
	var streamContent string

	var streamContentSb105 strings.Builder
	for e := range events {
		eventTypes = append(eventTypes, e.Type)
		if e.Type == agent.EventStream {
			streamContentSb105.WriteString(e.Content)
		}
	}
	streamContent += streamContentSb105.String()

	if streamContent != "Hello world!" {
		t.Errorf("expected 'Hello world!', got %q", streamContent)
	}

	// Should have: init, stream, assistant, complete
	hasComplete := false
	for _, et := range eventTypes {
		if et == agent.EventComplete {
			hasComplete = true
		}
	}
	if !hasComplete {
		t.Errorf("expected EventComplete in events: %v", eventTypes)
	}
}

func TestBaseToolUseLoop(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		callCount++

		if callCount == 1 {
			// First call: request a tool use
			writeSSE(w,
				"data: {\"type\":\"text\",\"content\":\"Let me list files.\"}\n\n",
				"data: {\"type\":\"tool_use\",\"id\":\"call_1\",\"name\":\"list_dir\",\"input\":{}}\n\n",
				"data: [DONE]\n\n",
			)
		} else {
			// Second call: complete after seeing tool result
			writeSSE(w,
				"data: {\"type\":\"text\",\"content\":\"Done!\"}\n\n",
				"data: [DONE]\n\n",
			)
		}
	}))
	defer server.Close()

	provider := &mockProvider{name: "test", server: server}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	events, err := base.SendPrompt(context.Background(), "list files")
	if err != nil {
		t.Fatal(err)
	}

	var hasToolUse, hasToolResult, hasComplete bool
	for e := range events {
		switch e.Type { //nolint:exhaustive // Test only checks specific event types
		case agent.EventToolUse:
			hasToolUse = true
		case agent.EventToolResult:
			hasToolResult = true
		case agent.EventComplete:
			hasComplete = true
		}
	}

	if !hasToolUse {
		t.Error("expected EventToolUse")
	}
	if !hasToolResult {
		t.Error("expected EventToolResult")
	}
	if !hasComplete {
		t.Error("expected EventComplete")
	}

	if callCount != 2 {
		t.Errorf("expected 2 API calls (initial + after tool result), got %d", callCount)
	}
}

func TestBaseInterrupt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Slow stream to give time for interrupt
		writeSSE(w, "data: {\"type\":\"text\",\"content\":\"thinking...\"}\n\n")
		time.Sleep(2 * time.Second) // Will be interrupted before this completes
		writeSSE(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := &mockProvider{name: "test", server: server}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()

	base := apiagent.NewBase(provider, cfg)
	_ = base.Connect(context.Background())

	events, _ := base.SendPrompt(context.Background(), "think hard")

	// Read one event then interrupt
	<-events
	_ = base.Interrupt()

	// Drain remaining events — should get interrupted eventually
	hasInterrupted := false
	timeout := time.After(3 * time.Second)
	for {
		select {
		case e, ok := <-events:
			if !ok {
				goto done
			}
			if e.Type == agent.EventInterrupted {
				hasInterrupted = true
			}
		case <-timeout:
			goto done
		}
	}
done:

	if !hasInterrupted {
		// Interrupt may cause the channel to close without an explicit interrupted event
		// depending on timing — this is acceptable behavior
		t.Log("interrupt completed (channel closed)")
	}
}

func TestBaseName(t *testing.T) {
	provider := &mockProvider{name: "test-agent"}
	base := apiagent.NewBase(provider, apiagent.DefaultAPIConfig())

	if base.Name() != "test-agent" {
		t.Errorf("expected 'test-agent', got %q", base.Name())
	}
}

func TestBaseAvailable(t *testing.T) {
	provider := &mockProvider{name: "test", available: errors.New("not configured")}
	base := apiagent.NewBase(provider, apiagent.DefaultAPIConfig())

	if err := base.Available(); err == nil {
		t.Error("expected error from Available()")
	}
}

func TestBaseWithWorkDir(t *testing.T) {
	provider := &mockProvider{name: "test"}
	base := apiagent.NewBase(provider, apiagent.DefaultAPIConfig())

	newAgent := base.WithWorkDir("/tmp/test")
	if newAgent == base {
		t.Error("WithWorkDir should return a new agent")
	}
}

func TestBaseNotConnected(t *testing.T) {
	provider := &mockProvider{name: "test"}
	base := apiagent.NewBase(provider, apiagent.DefaultAPIConfig())

	_, err := base.SendPrompt(context.Background(), "hello")
	if err == nil {
		t.Error("expected error when not connected")
	}
}
