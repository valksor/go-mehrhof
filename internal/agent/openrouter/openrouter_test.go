package openrouter

import (
	"strings"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

func TestNewAgent(t *testing.T) {
	a := New()

	if a.Name() != AgentName {
		t.Errorf("Name() = %q, want %q", a.Name(), AgentName)
	}

	if a.model != DefaultModel {
		t.Errorf("default model = %q, want %q", a.model, DefaultModel)
	}
}

func TestNewWithModel(t *testing.T) {
	a := NewWithModel("openai/gpt-4-turbo")

	if a.model != "openai/gpt-4-turbo" {
		t.Errorf("model = %q, want %q", a.model, "openai/gpt-4-turbo")
	}
}

func TestWithModel(t *testing.T) {
	a := New()
	b := a.WithModel("openai/gpt-4")

	// Original should be unchanged
	if a.model != DefaultModel {
		t.Error("WithModel modified original agent")
	}

	// New agent should have new model
	if b.model != "openai/gpt-4" {
		t.Errorf("WithModel() model = %q, want %q", b.model, "openai/gpt-4")
	}
}

func TestWithEnv(t *testing.T) {
	a := New()
	b := a.WithEnv("TEST_KEY", "test_value").(*Agent)

	// Original should not have the env var
	if _, ok := a.config.Environment["TEST_KEY"]; ok {
		t.Error("WithEnv modified original agent")
	}

	// New agent should have the env var
	if v, ok := b.config.Environment["TEST_KEY"]; !ok || v != "test_value" {
		t.Errorf("WithEnv() environment = %v, want TEST_KEY=test_value", b.config.Environment)
	}
}

func TestWithEnv_APIKey(t *testing.T) {
	a := New()
	b := a.WithEnv("OPENROUTER_API_KEY", "test-key").(*Agent)

	if b.apiKey != "test-key" {
		t.Errorf("WithEnv(OPENROUTER_API_KEY) apiKey = %q, want %q", b.apiKey, "test-key")
	}
}

func TestWithArgs(t *testing.T) {
	a := New()
	b := a.WithArgs("--model", "openai/gpt-4").(*Agent)

	if len(a.config.Args) != 0 {
		t.Error("WithArgs modified original agent")
	}

	if len(b.config.Args) != 2 {
		t.Errorf("WithArgs() args len = %d, want 2", len(b.config.Args))
	}

	if b.config.Args[0] != "--model" || b.config.Args[1] != "openai/gpt-4" {
		t.Errorf("WithArgs() args = %v, want [--model openai/gpt-4]", b.config.Args)
	}
}

func TestWithWorkDir(t *testing.T) {
	a := New()
	b := a.WithWorkDir("/tmp/test")

	if a.config.WorkDir != "" {
		t.Error("WithWorkDir modified original agent")
	}

	if b.config.WorkDir != "/tmp/test" {
		t.Errorf("WithWorkDir() = %q, want /tmp/test", b.config.WorkDir)
	}
}

func TestWithTimeout(t *testing.T) {
	a := New()
	b := a.WithTimeout(10 * time.Minute)

	if a.config.Timeout != 5*time.Minute {
		t.Error("WithTimeout modified original agent")
	}

	if b.config.Timeout != 10*time.Minute {
		t.Errorf("WithTimeout() = %v, want 10m", b.config.Timeout)
	}
}

func TestSetModel(t *testing.T) {
	a := New()
	a.SetModel("meta-llama/llama-3.1-405b-instruct")

	if a.model != "meta-llama/llama-3.1-405b-instruct" {
		t.Errorf("SetModel() model = %q, want meta-llama/llama-3.1-405b-instruct", a.model)
	}
}

func TestGetModel(t *testing.T) {
	a := NewWithModel("google/gemini-pro-1.5")

	if a.GetModel() != "google/gemini-pro-1.5" {
		t.Errorf("GetModel() = %q, want google/gemini-pro-1.5", a.GetModel())
	}
}

func TestMetadata(t *testing.T) {
	a := New()
	meta := a.Metadata()

	if meta.Name != "OpenRouter" {
		t.Errorf("Metadata().Name = %q, want OpenRouter", meta.Name)
	}

	if !meta.Capabilities.Streaming {
		t.Error("Metadata().Capabilities.Streaming should be true")
	}

	if !meta.Capabilities.MultiTurn {
		t.Error("Metadata().Capabilities.MultiTurn should be true")
	}

	if len(meta.Models) == 0 {
		t.Error("Metadata().Models should not be empty")
	}

	// Check that default model is in the list
	foundDefault := false
	for _, m := range meta.Models {
		if m.Default {
			foundDefault = true
			break
		}
	}
	if !foundDefault {
		t.Error("Metadata().Models should have a default model")
	}
}

func TestAvailable_NoAPIKey(t *testing.T) {
	a := &Agent{} // No API key

	err := a.Available()
	if err == nil {
		t.Error("Available() should return error when no API key")
	}

	if !strings.Contains(err.Error(), "API key not configured") {
		t.Errorf("Available() error = %v, want API key not configured error", err)
	}
}

func TestAvailable_WithAPIKey(t *testing.T) {
	a := &Agent{apiKey: "test-key"}

	err := a.Available()
	if err != nil {
		t.Errorf("Available() error = %v, want nil", err)
	}
}

func TestExtractSummary(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single line",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "multiple lines",
			input: "First\nSecond\nThird",
			want:  "First",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "long line truncation",
			input: strings.Repeat("a", 300),
			want:  strings.Repeat("a", 200) + "...",
		},
		{
			name:  "skip empty lines",
			input: "\n\nActual content\n",
			want:  "Actual content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSummary(tt.input)
			if got != tt.want {
				t.Errorf("extractSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseEvents(t *testing.T) {
	events := []agent.Event{
		{Type: agent.EventText, Text: "Hello "},
		{Type: agent.EventText, Text: "world"},
		{Type: agent.EventComplete, Data: map[string]any{}},
	}

	resp, err := parseEvents(events)
	if err != nil {
		t.Fatalf("parseEvents() error = %v", err)
	}

	if len(resp.Messages) != 1 {
		t.Errorf("parseEvents() messages len = %d, want 1", len(resp.Messages))
	}

	if resp.Messages[0] != "Hello world" {
		t.Errorf("parseEvents() message = %q, want %q", resp.Messages[0], "Hello world")
	}
}

func TestParseEvents_WithUsage(t *testing.T) {
	usage := &agent.UsageStats{
		InputTokens:  100,
		OutputTokens: 50,
	}

	events := []agent.Event{
		{Type: agent.EventText, Text: "Response"},
		{Type: agent.EventComplete, Data: map[string]any{"usage": usage}},
	}

	resp, err := parseEvents(events)
	if err != nil {
		t.Fatalf("parseEvents() error = %v", err)
	}

	if resp.Usage == nil {
		t.Fatal("parseEvents() usage should not be nil")
	}

	if resp.Usage.InputTokens != 100 {
		t.Errorf("parseEvents() InputTokens = %d, want 100", resp.Usage.InputTokens)
	}

	if resp.Usage.OutputTokens != 50 {
		t.Errorf("parseEvents() OutputTokens = %d, want 50", resp.Usage.OutputTokens)
	}
}

func TestAgentInterface(t *testing.T) {
	// Verify Agent implements agent.Agent
	var _ agent.Agent = (*Agent)(nil)

	// Verify Agent implements MetadataProvider
	var _ agent.MetadataProvider = (*Agent)(nil)
}
