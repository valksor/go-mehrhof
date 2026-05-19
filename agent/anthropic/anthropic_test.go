package anthropic_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/agent/anthropic"
	"github.com/valksor/kvelmo/agent/apiagent"
)

//nolint:errcheck // Test helper; write errors to httptest are not actionable
func writeSSE(w http.ResponseWriter, lines ...string) {
	for _, line := range lines {
		fmt.Fprint(w, line)
	}
}

func TestParseStreamText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(
			w,
			"event: message_start\ndata: {\"type\":\"message_start\"}\n\n",
			"event: content_block_start\ndata: {\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n",
			"event: content_block_delta\ndata: {\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello \"}}\n\n",
			"event: content_block_delta\ndata: {\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"world!\"}}\n\n",
			"event: content_block_stop\ndata: {\"index\":0}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		)
	}))
	defer server.Close()

	provider := anthropic.NewProvider(anthropic.Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	cfg := &apiagent.APIConfig{
		BaseURL:   server.URL,
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
	}

	req, err := provider.BuildRequest(context.Background(), cfg, []apiagent.Message{
		{Role: apiagent.RoleUser, Content: "hi"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // Body ownership transferred to ParseStream
	if err != nil {
		t.Fatal(err)
	}

	chunks, err := provider.ParseStream(context.Background(), resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	var sb strings.Builder
	var hasDone bool

	for c := range chunks {
		switch c.Type {
		case apiagent.ChunkText:
			sb.WriteString(c.Text)
		case apiagent.ChunkDone:
			hasDone = true
		case apiagent.ChunkToolUse, apiagent.ChunkError:
			// Not expected in this test
		}
	}

	if text := sb.String(); text != "Hello world!" {
		t.Errorf("expected 'Hello world!', got %q", text)
	}

	if !hasDone {
		t.Error("expected ChunkDone")
	}
}

func TestParseStreamToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(
			w,
			"event: message_start\ndata: {\"type\":\"message_start\"}\n\n",
			"event: content_block_start\ndata: {\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"bash\"}}\n\n",
			"event: content_block_delta\ndata: {\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"command\\\"\"}}\n\n",
			"event: content_block_delta\ndata: {\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\": \\\"echo hi\\\"}\"}}\n\n",
			"event: content_block_stop\ndata: {\"index\":0}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		)
	}))
	defer server.Close()

	provider := anthropic.NewProvider(anthropic.Config{APIKey: "test-key", BaseURL: server.URL})
	cfg := &apiagent.APIConfig{BaseURL: server.URL, Model: "test", MaxTokens: 1024}

	req, err := provider.BuildRequest(context.Background(), cfg, []apiagent.Message{
		{Role: apiagent.RoleUser, Content: "run echo"},
	}, apiagent.KvelmoTools())
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // Body ownership transferred to ParseStream
	if err != nil {
		t.Fatal(err)
	}

	chunks, err := provider.ParseStream(context.Background(), resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	var toolUse *apiagent.ToolUseChunk
	for c := range chunks {
		if c.Type == apiagent.ChunkToolUse && c.ToolUse != nil {
			toolUse = c.ToolUse
		}
	}

	if toolUse == nil {
		t.Fatal("expected a tool use chunk")
	}

	if toolUse.ID != "toolu_1" {
		t.Errorf("expected id 'toolu_1', got %q", toolUse.ID)
	}

	if toolUse.Name != "bash" {
		t.Errorf("expected name 'bash', got %q", toolUse.Name)
	}

	cmd, ok := toolUse.Input["command"].(string)
	if !ok || cmd != "echo hi" {
		t.Errorf("expected command 'echo hi', got %v", toolUse.Input)
	}
}

func TestProviderName(t *testing.T) {
	p := anthropic.NewProvider(anthropic.Config{})
	if p.Name() != "anthropic" {
		t.Errorf("expected 'anthropic', got %q", p.Name())
	}
}

func TestProviderAvailableNoKey(t *testing.T) {
	p := anthropic.NewProvider(anthropic.Config{})
	if err := p.Available(); err == nil {
		t.Error("expected error when API key not set")
	}
}

func TestProviderAvailableWithKey(t *testing.T) {
	p := anthropic.NewProvider(anthropic.Config{APIKey: "test-key"})
	if err := p.Available(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
