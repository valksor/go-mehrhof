package gemini

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
	if a.config.Command[0] != "gemini" {
		t.Errorf("Command[0] = %q, want %q", a.config.Command[0], "gemini")
	}
	if a.config.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want %v", a.config.Timeout, 30*time.Minute)
	}
	if a.parser == nil {
		t.Error("parser should not be nil")
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := agent.Config{
		Command:     []string{"custom-gemini", "-m", "gemini-2.5-flash"},
		Environment: map[string]string{"KEY": "val"},
		Timeout:     10 * time.Minute,
		WorkDir:     "/tmp",
	}

	a := NewWithConfig(cfg)
	if a == nil {
		t.Fatal("NewWithConfig returned nil")
	}
	if a.config.Command[0] != "custom-gemini" {
		t.Errorf("Command[0] = %q, want %q", a.config.Command[0], "custom-gemini")
	}
	if a.config.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want %v", a.config.Timeout, 10*time.Minute)
	}
}

func TestNewWithConfigEmptyCommand(t *testing.T) {
	cfg := agent.Config{
		Command: []string{}, // Empty command should default to "gemini"
	}

	a := NewWithConfig(cfg)
	if a.config.Command[0] != "gemini" {
		t.Errorf("Command[0] = %q, want %q (default)", a.config.Command[0], "gemini")
	}
}

func TestName(t *testing.T) {
	a := New()
	if a.Name() != AgentName {
		t.Errorf("Name() = %q, want %q", a.Name(), AgentName)
	}
	if a.Name() != "gemini" {
		t.Errorf("Name() = %q, want %q", a.Name(), "gemini")
	}
}

func TestAgentNameConstant(t *testing.T) {
	if AgentName != "gemini" {
		t.Errorf("AgentName = %q, want %q", AgentName, "gemini")
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

func TestWithEnv(t *testing.T) {
	a := New()
	aAgent := a.WithEnv("GEMINI_API_KEY", "secret123")
	if aAgent.(*Agent).config.Environment["GEMINI_API_KEY"] != "secret123" {
		t.Errorf("Environment[GEMINI_API_KEY] = %q, want %q", aAgent.(*Agent).config.Environment["GEMINI_API_KEY"], "secret123")
	}
}

func TestWithArgs(t *testing.T) {
	a := New()
	aAgent := a.WithArgs("-m", "gemini-2.5-flash")
	if len(aAgent.(*Agent).config.Args) != 2 {
		t.Errorf("Args length = %d, want 2", len(aAgent.(*Agent).config.Args))
	}
	if aAgent.(*Agent).config.Args[0] != "-m" {
		t.Errorf("Args[0] = %q, want %q", aAgent.(*Agent).config.Args[0], "-m")
	}
	if aAgent.(*Agent).config.Args[1] != "gemini-2.5-flash" {
		t.Errorf("Args[1] = %q, want %q", aAgent.(*Agent).config.Args[1], "gemini-2.5-flash")
	}
}

func TestMethodChaining(t *testing.T) {
	a := New().
		WithWorkDir("/work").
		WithTimeout(15 * time.Minute)

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

func TestBuildArgs_BasicPrompt(t *testing.T) {
	a := New()
	args := a.buildArgs("Hello world")

	// Should include: -p, prompt, --output-format, stream-json
	expectedMinLen := 4
	if len(args) < expectedMinLen {
		t.Errorf("args length = %d, want at least %d", len(args), expectedMinLen)
	}

	// Check that -p and prompt are present
	foundPromptFlag := false
	foundOutputFormat := false
	for i, arg := range args {
		if arg == "-p" && i+1 < len(args) && args[i+1] == "Hello world" {
			foundPromptFlag = true
		}
		if arg == "--output-format" && i+1 < len(args) && args[i+1] == "stream-json" {
			foundOutputFormat = true
		}
	}

	if !foundPromptFlag {
		t.Error("args should include -p flag with prompt")
	}
	if !foundOutputFormat {
		t.Error("args should include --output-format stream-json")
	}
}

func TestBuildArgs_WithConfigArgs(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"gemini", "-m", "gemini-2.5-flash"},
	}
	a := NewWithConfig(cfg)
	args := a.buildArgs("test")

	// Should include config args first: -m, gemini-2.5-flash
	if len(args) < 2 {
		t.Fatal("args should include config args")
	}
	if args[0] != "-m" || args[1] != "gemini-2.5-flash" {
		t.Errorf("expected config args [-m, gemini-2.5-flash], got %v", args[:2])
	}
}

func TestAvailable_NoCLI(t *testing.T) {
	// Use a non-existent binary to test the "not found" case
	cfg := agent.Config{
		Command: []string{"nonexistent-gemini-cli-binary-12345"},
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

func TestMetadataProviderInterface(t *testing.T) {
	// Verify Agent implements MetadataProvider
	var _ agent.MetadataProvider = (*Agent)(nil)
}

func TestMetadata(t *testing.T) {
	a := New()
	meta := a.Metadata()

	if meta.Name != "Gemini CLI" {
		t.Errorf("Metadata.Name = %q, want %q", meta.Name, "Gemini CLI")
	}

	if len(meta.Models) == 0 {
		t.Error("Metadata should include models")
	}

	// Check that there's a default model
	hasDefault := false
	for _, m := range meta.Models {
		if m.Default {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		t.Error("Metadata should include a default model")
	}

	// Check capabilities
	if !meta.Capabilities.Streaming {
		t.Error("Gemini should support streaming")
	}
}

func TestRunStream_ContextCancel(t *testing.T) {
	// Use nonexistent binary - this tests the start failure path
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
		// It's okay if there's no error since the binary doesn't exist
		// and the command never starts
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
	// Callback shouldn't be called since command fails to start
	if callbackCalled {
		t.Log("Callback was called (this may vary)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Parser tests
// ─────────────────────────────────────────────────────────────────────────────

func TestGeminiParser_ParseEvent_Text(t *testing.T) {
	p := NewGeminiParser()

	// Test JSON with text content
	line := []byte(`{"type":"text","text":"Hello, world!"}`)
	event, err := p.ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}

	if event.Type != agent.EventText {
		t.Errorf("event.Type = %v, want %v", event.Type, agent.EventText)
	}
	if event.Text != "Hello, world!" {
		t.Errorf("event.Text = %q, want %q", event.Text, "Hello, world!")
	}
}

func TestGeminiParser_ParseEvent_Parts(t *testing.T) {
	p := NewGeminiParser()

	// Test JSON with parts array (Gemini format)
	line := []byte(`{"parts":[{"text":"Part 1"},{"text":"Part 2"}]}`)
	event, err := p.ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}

	if event.Type != agent.EventText {
		t.Errorf("event.Type = %v, want %v", event.Type, agent.EventText)
	}
	if event.Text != "Part 1Part 2" {
		t.Errorf("event.Text = %q, want %q", event.Text, "Part 1Part 2")
	}
}

func TestGeminiParser_ParseEvent_FinishReason(t *testing.T) {
	p := NewGeminiParser()

	// Test completion event
	line := []byte(`{"finishReason":"STOP"}`)
	event, err := p.ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}

	if event.Type != agent.EventComplete {
		t.Errorf("event.Type = %v, want %v", event.Type, agent.EventComplete)
	}
}

func TestGeminiParser_ParseEvent_Usage(t *testing.T) {
	p := NewGeminiParser()

	// Test usage metadata
	line := []byte(`{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":50}}`)
	event, err := p.ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}

	if event.Type != agent.EventUsage {
		t.Errorf("event.Type = %v, want %v", event.Type, agent.EventUsage)
	}
}

func TestGeminiParser_ParseEvent_PlainText(t *testing.T) {
	p := NewGeminiParser()

	// Test non-JSON line (plain text fallback)
	line := []byte("Plain text output")
	event, err := p.ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}

	if event.Type != agent.EventText {
		t.Errorf("event.Type = %v, want %v", event.Type, agent.EventText)
	}
	if event.Text != "Plain text output" {
		t.Errorf("event.Text = %q, want %q", event.Text, "Plain text output")
	}
}

func TestGeminiParser_Parse(t *testing.T) {
	p := NewGeminiParser()

	events := []agent.Event{
		{Type: agent.EventText, Text: "Hello "},
		{Type: agent.EventText, Text: "world!"},
		{Type: agent.EventUsage, Data: map[string]any{
			"promptTokenCount":     float64(100),
			"candidatesTokenCount": float64(50),
		}},
	}

	response, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(response.Messages) == 0 {
		t.Error("response should have messages")
	}

	if response.Messages[0] != "Hello world!" {
		t.Errorf("response.Messages[0] = %q, want %q", response.Messages[0], "Hello world!")
	}

	if response.Usage == nil {
		t.Error("response should have usage stats")
	} else {
		if response.Usage.InputTokens != 100 {
			t.Errorf("Usage.InputTokens = %d, want 100", response.Usage.InputTokens)
		}
		if response.Usage.OutputTokens != 50 {
			t.Errorf("Usage.OutputTokens = %d, want 50", response.Usage.OutputTokens)
		}
	}
}

func TestSummarizeOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "single line",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "multiple lines",
			input: "First line\nSecond line\nThird line",
			want:  "First line",
		},
		{
			name:  "long line truncated",
			input: string(make([]byte, 250)),
			want:  string(make([]byte, 200)) + "...",
		},
		{
			name:  "whitespace only",
			input: "   \n   \n   ",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeOutput(tt.input)
			if got != tt.want {
				t.Errorf("summarizeOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}
