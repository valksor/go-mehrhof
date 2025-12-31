package ollama

import (
	"context"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

func TestNew(t *testing.T) {
	a := New()
	if a == nil {
		t.Fatal("New returned nil")
	}
	if a.config.Command[0] != "ollama" {
		t.Errorf("Command[0] = %q, want %q", a.config.Command[0], "ollama")
	}
	if a.config.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want %v", a.config.Timeout, 30*time.Minute)
	}
	if a.model != DefaultModel {
		t.Errorf("model = %q, want %q", a.model, DefaultModel)
	}
	if a.parser == nil {
		t.Error("parser should not be nil")
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := agent.Config{
		Command:     []string{"custom-ollama"},
		Environment: map[string]string{"KEY": "val"},
		Timeout:     10 * time.Minute,
		WorkDir:     "/tmp",
	}

	a := NewWithConfig(cfg)
	if a == nil {
		t.Fatal("NewWithConfig returned nil")
	}
	if a.config.Command[0] != "custom-ollama" {
		t.Errorf("Command[0] = %q, want %q", a.config.Command[0], "custom-ollama")
	}
	if a.config.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want %v", a.config.Timeout, 10*time.Minute)
	}
}

func TestNewWithConfigEmptyCommand(t *testing.T) {
	cfg := agent.Config{
		Command: []string{}, // Empty command should default to "ollama"
	}

	a := NewWithConfig(cfg)
	if a.config.Command[0] != "ollama" {
		t.Errorf("Command[0] = %q, want %q (default)", a.config.Command[0], "ollama")
	}
}

func TestNewWithModel(t *testing.T) {
	a := NewWithModel("llama3:70b")
	if a.model != "llama3:70b" {
		t.Errorf("model = %q, want %q", a.model, "llama3:70b")
	}
}

func TestName(t *testing.T) {
	a := New()
	if a.Name() != AgentName {
		t.Errorf("Name() = %q, want %q", a.Name(), AgentName)
	}
	if a.Name() != "ollama" {
		t.Errorf("Name() = %q, want %q", a.Name(), "ollama")
	}
}

func TestAgentNameConstant(t *testing.T) {
	if AgentName != "ollama" {
		t.Errorf("AgentName = %q, want %q", AgentName, "ollama")
	}
}

func TestDefaultModelConstant(t *testing.T) {
	if DefaultModel != "codellama" {
		t.Errorf("DefaultModel = %q, want %q", DefaultModel, "codellama")
	}
}

func TestWithWorkDir(t *testing.T) {
	a := New().WithWorkDir("/custom/path")
	if a.config.WorkDir != "/custom/path" {
		t.Errorf("WorkDir = %q, want %q", a.config.WorkDir, "/custom/path")
	}
}

func TestWithTimeout(t *testing.T) {
	a := New().WithTimeout(5 * time.Minute)
	if a.config.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want %v", a.config.Timeout, 5*time.Minute)
	}
}

func TestWithModel(t *testing.T) {
	a := New().WithModel("mistral")
	if a.model != "mistral" {
		t.Errorf("model = %q, want %q", a.model, "mistral")
	}
}

func TestWithEnv(t *testing.T) {
	a := New()
	aAgent := a.WithEnv("OLLAMA_HOST", "http://localhost:11434")
	if aAgent.(*Agent).config.Environment["OLLAMA_HOST"] != "http://localhost:11434" {
		t.Errorf("Environment[OLLAMA_HOST] = %q, want %q",
			aAgent.(*Agent).config.Environment["OLLAMA_HOST"], "http://localhost:11434")
	}
}

func TestWithArgs(t *testing.T) {
	a := New()
	aAgent := a.WithArgs("--verbose")
	typed := aAgent.(*Agent)
	if len(typed.config.Args) != 1 {
		t.Fatalf("Args length = %d, want 1", len(typed.config.Args))
	}
	if typed.config.Args[0] != "--verbose" {
		t.Errorf("Args = %v, want [--verbose]", typed.config.Args)
	}
}

func TestMethodChaining(t *testing.T) {
	a := New().
		WithWorkDir("/work").
		WithTimeout(15 * time.Minute).
		WithModel("llama3")

	// WithEnv returns agent.Agent interface, so capture the result
	aAgent := agent.Agent(a)
	aAgent = aAgent.WithEnv("KEY1", "val1")
	aAgent = aAgent.WithEnv("KEY2", "val2")

	if a.config.WorkDir != "/work" {
		t.Error("WithWorkDir chain failed")
	}
	if a.config.Timeout != 15*time.Minute {
		t.Error("WithTimeout chain failed")
	}
	if a.model != "llama3" {
		t.Error("WithModel chain failed")
	}
	if aAgent.(*Agent).config.Environment["KEY1"] != "val1" {
		t.Error("WithEnv(KEY1) chain failed")
	}
	if aAgent.(*Agent).config.Environment["KEY2"] != "val2" {
		t.Error("WithEnv(KEY2) chain failed")
	}
}

func TestSetParser(t *testing.T) {
	a := New()
	originalParser := a.parser

	newParser := agent.NewJSONLineParser()
	a.SetParser(newParser)

	if a.parser == originalParser {
		t.Error("parser should have been replaced")
	}
}

func TestSetModel(t *testing.T) {
	a := New()
	a.SetModel("gemma")
	if a.model != "gemma" {
		t.Errorf("model = %q, want %q", a.model, "gemma")
	}
}

func TestGetModel(t *testing.T) {
	a := New()
	if a.GetModel() != DefaultModel {
		t.Errorf("GetModel() = %q, want %q", a.GetModel(), DefaultModel)
	}

	a.SetModel("phi")
	if a.GetModel() != "phi" {
		t.Errorf("GetModel() = %q, want %q", a.GetModel(), "phi")
	}
}

func TestBuildArgs_BasicPrompt(t *testing.T) {
	a := New()
	args := a.buildArgs("Hello world")

	// Should be: run, model, prompt
	if len(args) != 3 {
		t.Fatalf("args length = %d, want 3", len(args))
	}
	if args[0] != "run" {
		t.Errorf("args[0] = %q, want %q", args[0], "run")
	}
	if args[1] != DefaultModel {
		t.Errorf("args[1] = %q, want %q", args[1], DefaultModel)
	}
	if args[2] != "Hello world" {
		t.Errorf("args[2] = %q, want %q", args[2], "Hello world")
	}
}

func TestBuildArgs_WithModelFlag(t *testing.T) {
	a := New()
	aAgent := a.WithArgs("--model", "llama3")

	args := aAgent.(*Agent).buildArgs("test")

	// Model from --model flag should be used
	if len(args) < 2 {
		t.Fatal("args should have at least 2 elements")
	}
	if args[1] != "llama3" {
		t.Errorf("args[1] = %q, want %q", args[1], "llama3")
	}
}

func TestBuildArgs_CustomModel(t *testing.T) {
	a := NewWithModel("mistral:7b")
	args := a.buildArgs("test prompt")

	if args[1] != "mistral:7b" {
		t.Errorf("model = %q, want %q", args[1], "mistral:7b")
	}
}

func TestAvailable_NoCLI(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"nonexistent-ollama-cli-binary-12345"},
	}
	a := NewWithConfig(cfg)

	err := a.Available()
	if err == nil {
		t.Error("Available should fail when CLI not found")
	}
}

func TestRegister(t *testing.T) {
	r := agent.NewRegistry()
	err := Register(r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Should be able to get the agent
	a, err := r.Get(AgentName)
	if err != nil {
		t.Fatalf("Get(%s): %v", AgentName, err)
	}
	if a.Name() != AgentName {
		t.Errorf("agent name = %q, want %q", a.Name(), AgentName)
	}
}

func TestAgentInterface(t *testing.T) {
	// Verify Agent implements the interface
	var _ agent.Agent = (*Agent)(nil)
}

func TestRunStream_ContextCancel(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"nonexistent-binary-xyz"},
		Timeout: time.Second,
	}
	a := NewWithConfig(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	events, errCh := a.RunStream(ctx, "test")

	// Drain events
	for range events {
	}

	// Should get an error (either context canceled or binary not found)
	err := <-errCh
	if err == nil {
		t.Log("No error returned (binary not found before context check)")
	}
}

func TestRun_NoCLI(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"nonexistent-binary-abc"},
		Timeout: time.Second,
	}
	a := NewWithConfig(cfg)

	_, err := a.Run(context.Background(), "test")
	if err == nil {
		t.Error("Run should fail when CLI not found")
	}
}

func TestRunWithCallback_NoCLI(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"nonexistent-binary-def"},
		Timeout: time.Second,
	}
	a := NewWithConfig(cfg)

	callbackCalled := false
	cb := func(event agent.Event) error {
		callbackCalled = true
		return nil
	}

	_, err := a.RunWithCallback(context.Background(), "test", cb)
	if err == nil {
		t.Error("RunWithCallback should fail when CLI not found")
	}
	if callbackCalled {
		t.Log("Callback was called (this may vary)")
	}
}

// Test PlainTextParser
func TestPlainTextParser_ParseEvent(t *testing.T) {
	p := NewPlainTextParser()

	line := []byte("Here is a code example...")
	event, err := p.ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event.Type != agent.EventText {
		t.Errorf("event.Type = %v, want %v", event.Type, agent.EventText)
	}
	if event.Text != "Here is a code example..." {
		t.Errorf("event.Text = %q, want %q", event.Text, "Here is a code example...")
	}
}

func TestPlainTextParser_Parse(t *testing.T) {
	p := NewPlainTextParser()

	events := []agent.Event{
		{Type: agent.EventText, Text: "Here is the solution:"},
		{Type: agent.EventText, Text: "func main() { }"},
		{Type: agent.EventText, Text: "This creates a simple Go program."},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(resp.Messages) == 0 {
		t.Fatal("expected at least one message")
	}

	// Check summary is first meaningful line
	if resp.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestSummarizeOllamaOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple response",
			input:    "Here is the answer.",
			expected: "Here is the answer.",
		},
		{
			name:     "multiline response",
			input:    "First line\nSecond line\nThird line",
			expected: "First line",
		},
		{
			name:     "empty lines first",
			input:    "\n\nActual content",
			expected: "Actual content",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "truncate long line",
			input:    string(make([]byte, 300)), // 300 chars
			expected: string(make([]byte, 200)) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := summarizeOllamaOutput(tt.input)
			if result != tt.expected {
				t.Errorf("summarizeOllamaOutput(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Test immutability of WithEnv/WithArgs/WithModel
func TestWithEnvImmutability(t *testing.T) {
	a1 := New()
	a2 := a1.WithEnv("KEY", "value")

	if _, ok := a1.config.Environment["KEY"]; ok {
		t.Error("WithEnv modified original agent")
	}

	if a2.(*Agent).config.Environment["KEY"] != "value" {
		t.Error("WithEnv didn't set value on new agent")
	}
}

func TestWithArgsImmutability(t *testing.T) {
	a1 := New()
	a2 := a1.WithArgs("--flag")

	if len(a1.config.Args) != 0 {
		t.Error("WithArgs modified original agent")
	}

	if len(a2.(*Agent).config.Args) != 1 || a2.(*Agent).config.Args[0] != "--flag" {
		t.Error("WithArgs didn't set args on new agent")
	}
}

func TestWithModelImmutability(t *testing.T) {
	a1 := New()
	a2 := a1.WithModel("llama3")

	if a1.model != DefaultModel {
		t.Error("WithModel modified original agent")
	}

	if a2.model != "llama3" {
		t.Error("WithModel didn't set model on new agent")
	}
}
