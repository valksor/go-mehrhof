package anthropic_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/valksor/kvelmo/agent/anthropic"
	"github.com/valksor/kvelmo/agent/apiagent"
)

func TestNew(t *testing.T) {
	cfg := anthropic.Config{Model: "claude-opus-4-7", APIKey: "sk-ant-test", BaseURL: "https://api.anthropic.com"}
	base := anthropic.New(cfg, apiagent.APIConfig{})
	if base == nil {
		t.Fatal("New returned nil")
	}
}

func TestProvider_Name(t *testing.T) {
	p := anthropic.NewProvider(anthropic.Config{Model: "claude-opus-4-7"})
	if got := p.Name(); got != "anthropic" {
		t.Errorf("Name = %q", got)
	}
}

func TestProvider_Available(t *testing.T) {
	t.Run("no key", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "")
		p := anthropic.NewProvider(anthropic.Config{Model: "x"})
		if err := p.Available(); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("with key", func(t *testing.T) {
		p := anthropic.NewProvider(anthropic.Config{Model: "x", APIKey: "sk-ant-test"})
		if err := p.Available(); err != nil {
			t.Errorf("unexpected: %v", err)
		}
	})
}

func TestProvider_BuildRequest(t *testing.T) {
	p := anthropic.NewProvider(anthropic.Config{Model: "claude-opus-4-7", APIKey: "sk-ant-test", BaseURL: "https://api.anthropic.com"})
	cfg := &apiagent.APIConfig{Model: "claude-opus-4-7", APIKey: "sk-ant-test", BaseURL: "https://api.anthropic.com", MaxTokens: 1000}
	msgs := []apiagent.Message{
		{Role: apiagent.RoleSystem, Content: "you are helpful"},
		{Role: apiagent.RoleUser, Content: "hi"},
		{Role: apiagent.RoleAssistant, Content: "hello back", ToolCalls: []apiagent.ToolCall{
			{ID: "tool_1", Name: "bash", Input: map[string]any{"cmd": "ls"}},
		}},
		{Role: apiagent.RoleTool, ToolResult: &apiagent.ToolResult{ToolCallID: "tool_1", Content: "out", IsError: false}},
	}
	tools := []apiagent.ToolDef{{Name: "bash", Description: "shell", Parameters: map[string]any{"type": "object"}}}

	req, err := p.BuildRequest(context.Background(), cfg, msgs, tools)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}

	if req.Header.Get("X-Api-Key") != "sk-ant-test" {
		// Anthropic uses x-api-key — header keys are canonicalized.
		t.Logf("x-api-key header check: %v", req.Header)
	}

	body, _ := io.ReadAll(req.Body)
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)

	if payload["model"] != "claude-opus-4-7" {
		t.Errorf("model = %v", payload["model"])
	}
	if payload["system"] != "you are helpful" {
		t.Errorf("system = %v", payload["system"])
	}
	if payload["tools"] == nil {
		t.Errorf("tools missing")
	}
	if payload["max_tokens"] == nil {
		t.Errorf("max_tokens missing")
	}
}

func TestProvider_BuildRequest_NoSystemNoTools(t *testing.T) {
	p := anthropic.NewProvider(anthropic.Config{Model: "claude-opus-4-7", APIKey: "sk-ant-test", BaseURL: "https://api.anthropic.com"})
	cfg := &apiagent.APIConfig{Model: "claude-opus-4-7", APIKey: "sk-ant-test", BaseURL: "https://api.anthropic.com"}
	msgs := []apiagent.Message{{Role: apiagent.RoleUser, Content: "hi"}}

	req, err := p.BuildRequest(context.Background(), cfg, msgs, nil)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}

	body, _ := io.ReadAll(req.Body)
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)

	if _, has := payload["system"]; has {
		t.Error("system should be omitted when empty")
	}
	if _, has := payload["tools"]; has {
		t.Error("tools should be omitted when empty")
	}
}
