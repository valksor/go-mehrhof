package plugin

import (
	"encoding/json"
	"testing"

	"github.com/valksor/go-mehrhof/internal/agent"
)

func TestNewAgentAdapter(t *testing.T) {
	manifest := &Manifest{
		Name:        "test-agent",
		Description: "Test agent",
		Type:        "agent",
		Agent: &AgentConfig{
			Name: "test-agent",
		},
	}

	adapter := NewAgentAdapter(manifest, nil)

	if adapter == nil {
		t.Fatal("NewAgentAdapter returned nil")
	}
	if adapter.manifest != manifest {
		t.Error("manifest not set correctly")
	}
	if adapter.env == nil {
		t.Error("env map should be initialized")
	}
	if adapter.parser == nil {
		t.Error("parser should be initialized")
	}
}

func TestAgentAdapter_Name(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Manifest
		want     string
	}{
		{
			name: "uses agent name from manifest",
			manifest: &Manifest{
				Name: "plugin-name",
				Agent: &AgentConfig{
					Name: "agent-name",
				},
			},
			want: "agent-name",
		},
		{
			name: "falls back to manifest name",
			manifest: &Manifest{
				Name:  "plugin-name",
				Agent: nil,
			},
			want: "plugin-name",
		},
		{
			name: "falls back when agent has no name",
			manifest: &Manifest{
				Name: "plugin-name",
				Agent: &AgentConfig{
					Name: "",
				},
			},
			want: "", // Empty agent name is used as-is
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewAgentAdapter(tt.manifest, nil)
			got := adapter.Name()
			if got != tt.want {
				t.Errorf("Name() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAgentAdapter_WithEnv(t *testing.T) {
	manifest := &Manifest{Name: "test"}
	adapter := NewAgentAdapter(manifest, nil)

	// Add first env var
	newAdapter := adapter.WithEnv("KEY1", "value1")
	if newAdapter == adapter {
		t.Error("WithEnv should return a new adapter")
	}

	aa, ok := newAdapter.(*AgentAdapter)
	if !ok {
		t.Fatal("WithEnv should return *AgentAdapter")
	}
	if aa.env["KEY1"] != "value1" {
		t.Errorf("KEY1 = %q, want %q", aa.env["KEY1"], "value1")
	}

	// Add second env var - should preserve first
	newAdapter2 := aa.WithEnv("KEY2", "value2")
	aa2, ok := newAdapter2.(*AgentAdapter)
	if !ok {
		t.Fatal("WithEnv should return *AgentAdapter")
	}
	if aa2.env["KEY1"] != "value1" {
		t.Error("KEY1 should be preserved")
	}
	if aa2.env["KEY2"] != "value2" {
		t.Errorf("KEY2 = %q, want %q", aa2.env["KEY2"], "value2")
	}

	// Original should not be modified
	if len(adapter.env) != 0 {
		t.Error("original adapter should not be modified")
	}
}

func TestAgentAdapter_Metadata(t *testing.T) {
	tests := []struct {
		name          string
		manifest      *Manifest
		wantName      string
		wantDesc      string
		wantStreaming bool
		wantToolUse   bool
	}{
		{
			name: "basic metadata",
			manifest: &Manifest{
				Name:        "test-agent",
				Description: "Test description",
				Agent:       nil,
			},
			wantName:      "test-agent",
			wantDesc:      "Test description",
			wantStreaming: false,
		},
		{
			name: "with streaming capability",
			manifest: &Manifest{
				Name:        "stream-agent",
				Description: "Streaming agent",
				Agent: &AgentConfig{
					Name:      "stream-agent",
					Streaming: true,
				},
			},
			wantName:      "stream-agent",
			wantDesc:      "Streaming agent",
			wantStreaming: true,
		},
		{
			name: "with capabilities list",
			manifest: &Manifest{
				Name:        "full-agent",
				Description: "Full capabilities",
				Agent: &AgentConfig{
					Name:         "full-agent",
					Capabilities: []string{"streaming", "tool_use", "file_operations"},
				},
			},
			wantName:      "full-agent",
			wantDesc:      "Full capabilities",
			wantStreaming: true,
			wantToolUse:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewAgentAdapter(tt.manifest, nil)
			meta := adapter.Metadata()

			if meta.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", meta.Name, tt.wantName)
			}
			if meta.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", meta.Description, tt.wantDesc)
			}
			if meta.Capabilities.Streaming != tt.wantStreaming {
				t.Errorf("Streaming = %v, want %v", meta.Capabilities.Streaming, tt.wantStreaming)
			}
			if meta.Capabilities.ToolUse != tt.wantToolUse {
				t.Errorf("ToolUse = %v, want %v", meta.Capabilities.ToolUse, tt.wantToolUse)
			}
		})
	}
}

func TestAgentAdapter_Manifest(t *testing.T) {
	manifest := &Manifest{Name: "test"}
	adapter := NewAgentAdapter(manifest, nil)

	if adapter.Manifest() != manifest {
		t.Error("Manifest() should return the original manifest")
	}
}

func TestAgentAdapter_SetParser(t *testing.T) {
	adapter := NewAgentAdapter(&Manifest{Name: "test"}, nil)
	originalParser := adapter.parser

	newParser := agent.NewYAMLBlockParser()
	adapter.SetParser(newParser)

	if adapter.parser == originalParser {
		t.Error("parser should have been changed")
	}
}

func TestConvertStreamEvent(t *testing.T) {
	tests := []struct {
		name      string
		input     *StreamEvent
		wantType  agent.EventType
		wantText  string
		checkData bool
	}{
		{
			name: "text event with string",
			input: &StreamEvent{
				Type: StreamEventText,
				Data: json.RawMessage(`"Hello world"`),
			},
			wantType: agent.EventText,
			wantText: "Hello world",
		},
		{
			name: "text event with object",
			input: &StreamEvent{
				Type: StreamEventText,
				Data: json.RawMessage(`{"text": "Object text"}`),
			},
			wantType: agent.EventText,
			wantText: "Object text",
		},
		{
			name: "tool use event",
			input: &StreamEvent{
				Type: StreamEventToolUse,
				Data: json.RawMessage(`{"name": "read_file", "description": "Read a file", "input": {"path": "/tmp/test"}}`),
			},
			wantType: agent.EventToolUse,
		},
		{
			name: "tool result event",
			input: &StreamEvent{
				Type: StreamEventToolResult,
				Data: json.RawMessage(`{"result": "success"}`),
			},
			wantType:  agent.EventToolResult,
			checkData: true,
		},
		{
			name: "file event",
			input: &StreamEvent{
				Type: StreamEventFile,
				Data: json.RawMessage(`{"path": "test.go", "content": "package main"}`),
			},
			wantType:  agent.EventFile,
			checkData: true,
		},
		{
			name: "usage event",
			input: &StreamEvent{
				Type: StreamEventUsage,
				Data: json.RawMessage(`{"tokens": 100}`),
			},
			wantType:  agent.EventUsage,
			checkData: true,
		},
		{
			name: "complete event",
			input: &StreamEvent{
				Type: StreamEventComplete,
				Data: json.RawMessage(`{}`),
			},
			wantType: agent.EventComplete,
		},
		{
			name: "error event",
			input: &StreamEvent{
				Type: StreamEventError,
				Data: json.RawMessage(`"something went wrong"`),
			},
			wantType: agent.EventError,
		},
		{
			name: "unknown event type",
			input: &StreamEvent{
				Type: "custom_event",
				Data: json.RawMessage(`{"custom": "data"}`),
			},
			wantType:  agent.EventType("custom_event"),
			checkData: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertStreamEvent(tt.input)

			if result.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", result.Type, tt.wantType)
			}

			if tt.wantText != "" && result.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", result.Text, tt.wantText)
			}

			if tt.checkData && result.Data == nil {
				t.Error("Data should not be nil")
			}

			// Check timestamp is set
			if result.Timestamp.IsZero() {
				t.Error("Timestamp should be set")
			}

			// Check raw is preserved
			if result.Raw == nil {
				t.Error("Raw should be preserved")
			}
		})
	}
}

func TestConvertStreamEvent_ToolCall(t *testing.T) {
	input := &StreamEvent{
		Type: StreamEventToolUse,
		Data: json.RawMessage(`{
			"name": "read_file",
			"description": "Read a file from disk",
			"input": {"path": "/tmp/test.txt"}
		}`),
	}

	result := convertStreamEvent(input)

	if result.Type != agent.EventToolUse {
		t.Errorf("Type = %v, want EventToolUse", result.Type)
	}

	if result.ToolCall == nil {
		t.Fatal("ToolCall should not be nil")
	}

	if result.ToolCall.Name != "read_file" {
		t.Errorf("ToolCall.Name = %q, want %q", result.ToolCall.Name, "read_file")
	}

	if result.ToolCall.Description != "Read a file from disk" {
		t.Errorf("ToolCall.Description = %q, want %q", result.ToolCall.Description, "Read a file from disk")
	}

	if result.ToolCall.Input == nil {
		t.Error("ToolCall.Input should not be nil")
	}
}
