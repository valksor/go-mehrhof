package openai_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/agent/apiagent"
	"github.com/valksor/kvelmo/agent/openai"
)

func TestParseOpenAIStreamText(t *testing.T) {
	sse := `data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":" world"},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`
	chunks := openai.ParseOpenAIStream(context.Background(), io.NopCloser(strings.NewReader(sse)))

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

	if text := sb.String(); text != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", text)
	}

	if !hasDone {
		t.Error("expected ChunkDone")
	}
}

func TestParseOpenAIStreamToolCall(t *testing.T) {
	// Simulate incremental tool call chunks
	sse := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","function":{"name":"bash","arguments":""}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"command\":"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"echo hi\"}"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]

`
	chunks := openai.ParseOpenAIStream(context.Background(), io.NopCloser(strings.NewReader(sse)))

	var toolUse *apiagent.ToolUseChunk

	for c := range chunks {
		if c.Type == apiagent.ChunkToolUse && c.ToolUse != nil {
			toolUse = c.ToolUse
		}
	}

	if toolUse == nil {
		t.Fatal("expected a tool use chunk")
	}

	if toolUse.ID != "call_abc" {
		t.Errorf("expected id 'call_abc', got %q", toolUse.ID)
	}

	if toolUse.Name != "bash" {
		t.Errorf("expected name 'bash', got %q", toolUse.Name)
	}

	cmd, ok := toolUse.Input["command"].(string)
	if !ok || cmd != "echo hi" {
		t.Errorf("expected command 'echo hi', got %v", toolUse.Input)
	}
}

func TestParseOpenAIStreamMultipleToolCalls(t *testing.T) {
	sse := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"read_file","arguments":"{\"path\":\"a.go\"}"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_2","function":{"name":"read_file","arguments":"{\"path\":\"b.go\"}"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]

`
	chunks := openai.ParseOpenAIStream(context.Background(), io.NopCloser(strings.NewReader(sse)))

	var toolUses []*apiagent.ToolUseChunk
	for c := range chunks {
		if c.Type == apiagent.ChunkToolUse && c.ToolUse != nil {
			toolUses = append(toolUses, c.ToolUse)
		}
	}

	if len(toolUses) != 2 {
		t.Fatalf("expected 2 tool uses, got %d", len(toolUses))
	}

	if toolUses[0].ID != "call_1" || toolUses[1].ID != "call_2" {
		t.Errorf("unexpected tool call IDs: %s, %s", toolUses[0].ID, toolUses[1].ID)
	}
}

func TestProviderName(t *testing.T) {
	p := openai.NewProvider(openai.Config{})
	if p.Name() != "openai" {
		t.Errorf("expected 'openai', got %q", p.Name())
	}
}

func TestProviderAvailableNoKey(t *testing.T) {
	p := openai.NewProvider(openai.Config{})
	if err := p.Available(); err == nil {
		t.Error("expected error when API key not set")
	}
}

func TestProviderAvailableWithKey(t *testing.T) {
	p := openai.NewProvider(openai.Config{APIKey: "test-key"})
	if err := p.Available(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
