package openai_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/valksor/kvelmo/agent/apiagent"
	"github.com/valksor/kvelmo/agent/openai"
)

func TestNew_Provider(t *testing.T) {
	cfg := openai.Config{Model: "gpt-4o", APIKey: "sk-test", BaseURL: "https://api.example.com"}
	apiCfg := apiagent.APIConfig{}

	base := openai.New(cfg, apiCfg)
	if base == nil {
		t.Fatal("New returned nil")
	}
}

func TestProvider_Name(t *testing.T) {
	p := openai.NewProvider(openai.Config{Model: "gpt-4o"})
	if got := p.Name(); got != "openai" {
		t.Errorf("Name = %q, want openai", got)
	}
}

func TestProvider_Available_NoKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	p := openai.NewProvider(openai.Config{Model: "gpt-4o"})
	if err := p.Available(); err == nil {
		t.Error("Available should error when no API key")
	}
}

func TestProvider_Available_WithKey(t *testing.T) {
	p := openai.NewProvider(openai.Config{Model: "gpt-4o", APIKey: "sk-test"})
	if err := p.Available(); err != nil {
		t.Errorf("Available with key: %v", err)
	}
}

func TestProvider_BuildRequest(t *testing.T) {
	p := openai.NewProvider(openai.Config{Model: "gpt-4o", APIKey: "sk-test", BaseURL: "https://api.openai.com"})
	cfg := &apiagent.APIConfig{Model: "gpt-4o", APIKey: "sk-test", BaseURL: "https://api.openai.com", MaxTokens: 100}
	msgs := []apiagent.Message{{Role: apiagent.RoleUser, Content: "hi"}}
	tools := []apiagent.ToolDef{{Name: "bash", Description: "run shell", Parameters: map[string]any{"type": "object"}}}

	req, err := p.BuildRequest(context.Background(), cfg, msgs, tools)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	if req.URL.Path != "/v1/chat/completions" {
		t.Errorf("path = %q", req.URL.Path)
	}
	if req.Header.Get("Authorization") != "Bearer sk-test" {
		t.Errorf("authorization header = %q", req.Header.Get("Authorization"))
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("content-type = %q", req.Header.Get("Content-Type"))
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload["model"] != "gpt-4o" {
		t.Errorf("model = %v", payload["model"])
	}
	if payload["stream"] != true {
		t.Errorf("stream not set")
	}
	if payload["max_tokens"] == nil {
		t.Errorf("max_tokens missing")
	}
	if payload["tools"] == nil {
		t.Errorf("tools missing")
	}
}

func TestProvider_BuildRequest_NoTools(t *testing.T) {
	p := openai.NewProvider(openai.Config{Model: "gpt-4o", APIKey: "sk-test", BaseURL: "https://api.openai.com"})
	cfg := &apiagent.APIConfig{Model: "gpt-4o", APIKey: "sk-test", BaseURL: "https://api.openai.com"}
	msgs := []apiagent.Message{{Role: apiagent.RoleUser, Content: "hi"}}

	req, err := p.BuildRequest(context.Background(), cfg, msgs, nil)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}

	body, _ := io.ReadAll(req.Body)
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)

	if _, ok := payload["tools"]; ok {
		t.Error("tools should not be set when no tools provided")
	}
	if _, ok := payload["max_tokens"]; ok {
		t.Error("max_tokens should not be set when zero")
	}
}

func TestConvertMessages(t *testing.T) {
	msgs := []apiagent.Message{
		{Role: apiagent.RoleSystem, Content: "you are helpful"},
		{Role: apiagent.RoleUser, Content: "hello"},
		{Role: apiagent.RoleAssistant, Content: "hi back", ToolCalls: []apiagent.ToolCall{
			{ID: "call_1", Name: "bash", Input: map[string]any{"cmd": "ls"}},
		}},
		{Role: apiagent.RoleTool, ToolResult: &apiagent.ToolResult{ToolCallID: "call_1", Content: "file1\nfile2"}},
	}

	got := openai.ConvertMessages(msgs)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}

	if got[0]["role"] != "system" {
		t.Errorf("got[0].role = %v", got[0]["role"])
	}
	if got[2]["role"] != "assistant" {
		t.Errorf("got[2].role = %v", got[2]["role"])
	}
	if got[2]["tool_calls"] == nil {
		t.Error("got[2].tool_calls missing")
	}
	if got[3]["role"] != "tool" {
		t.Errorf("got[3].role = %v", got[3]["role"])
	}
	if got[3]["tool_call_id"] != "call_1" {
		t.Errorf("tool_call_id = %v", got[3]["tool_call_id"])
	}
}

func TestConvertMessages_AssistantWithoutContent(t *testing.T) {
	// Assistant with tool calls but no text content — content key should be omitted.
	msgs := []apiagent.Message{
		{Role: apiagent.RoleAssistant, ToolCalls: []apiagent.ToolCall{
			{ID: "c1", Name: "bash", Input: map[string]any{}},
		}},
	}
	got := openai.ConvertMessages(msgs)
	if _, hasContent := got[0]["content"]; hasContent {
		t.Error("content key should be omitted when empty alongside tool_calls")
	}
}

func TestConvertTools(t *testing.T) {
	tools := []apiagent.ToolDef{
		{Name: "bash", Description: "run shell", Parameters: map[string]any{"type": "object", "properties": map[string]any{"cmd": map[string]any{"type": "string"}}}},
		{Name: "read", Description: "read file"},
	}
	got := openai.ConvertTools(tools)
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	for i, entry := range got {
		if entry["type"] != "function" {
			t.Errorf("entry %d type = %v", i, entry["type"])
		}
		fn, ok := entry["function"].(map[string]any)
		if !ok {
			t.Fatalf("entry %d function not a map", i)
		}
		if fn["name"] == nil {
			t.Errorf("entry %d function.name missing", i)
		}
	}
}

func TestConvertTools_Empty(t *testing.T) {
	if got := openai.ConvertTools(nil); len(got) != 0 {
		t.Errorf("nil tools should give empty slice, got %d", len(got))
	}
}
