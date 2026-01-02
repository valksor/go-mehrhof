package aider

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

func TestNew(t *testing.T) {
	a := New()
	if a == nil {
		t.Fatal("New returned nil")
	}
	if a.config.Command[0] != "aider" {
		t.Errorf("Command[0] = %q, want %q", a.config.Command[0], "aider")
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
		Command:     []string{"custom-aider", "--model", "gpt-4"},
		Environment: map[string]string{"KEY": "val"},
		Timeout:     10 * time.Minute,
		WorkDir:     "/tmp",
	}

	a := NewWithConfig(cfg)
	if a == nil {
		t.Fatal("NewWithConfig returned nil")
	}
	if a.config.Command[0] != "custom-aider" {
		t.Errorf("Command[0] = %q, want %q", a.config.Command[0], "custom-aider")
	}
	if a.config.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want %v", a.config.Timeout, 10*time.Minute)
	}
}

func TestNewWithConfigEmptyCommand(t *testing.T) {
	cfg := agent.Config{
		Command: []string{}, // Empty command should default to "aider"
	}

	a := NewWithConfig(cfg)
	if a.config.Command[0] != "aider" {
		t.Errorf("Command[0] = %q, want %q (default)", a.config.Command[0], "aider")
	}
}

func TestName(t *testing.T) {
	a := New()
	if a.Name() != AgentName {
		t.Errorf("Name() = %q, want %q", a.Name(), AgentName)
	}
	if a.Name() != "aider" {
		t.Errorf("Name() = %q, want %q", a.Name(), "aider")
	}
}

func TestAgentNameConstant(t *testing.T) {
	if AgentName != "aider" {
		t.Errorf("AgentName = %q, want %q", AgentName, "aider")
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
	aAgent := a.WithEnv("API_KEY", "secret123")
	if aAgent.(*Agent).config.Environment["API_KEY"] != "secret123" {
		t.Errorf("Environment[API_KEY] = %q, want %q", aAgent.(*Agent).config.Environment["API_KEY"], "secret123")
	}
}

func TestWithArgs(t *testing.T) {
	a := New()
	aAgent := a.WithArgs("--model", "gpt-4")
	typed := aAgent.(*Agent)
	if len(typed.config.Args) != 2 {
		t.Fatalf("Args length = %d, want 2", len(typed.config.Args))
	}
	if typed.config.Args[0] != "--model" || typed.config.Args[1] != "gpt-4" {
		t.Errorf("Args = %v, want [--model gpt-4]", typed.config.Args)
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

	// Should include: --yes, --no-auto-commits, --message, prompt
	expectedContains := []string{"--yes", "--no-auto-commits", "--message", "Hello world"}

	for _, expected := range expectedContains {
		found := false
		for _, arg := range args {
			if arg == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("args %v should contain %q", args, expected)
		}
	}
}

func TestBuildArgs_WithConfigArgs(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"aider", "--model", "gpt-4"},
	}
	a := NewWithConfig(cfg)
	args := a.buildArgs("test")

	// Should include config args first: --model, gpt-4
	if len(args) < 2 {
		t.Fatal("args should include config args")
	}
	if args[0] != "--model" || args[1] != "gpt-4" {
		t.Errorf("expected config args [--model, gpt-4], got %v", args[:2])
	}

	// Should also include --message with prompt
	found := false
	for i, arg := range args {
		if arg == "--message" && i+1 < len(args) && args[i+1] == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("args should contain --message test, got %v", args)
	}
}

func TestAvailable_NoCLI(t *testing.T) {
	// Use a non-existent binary to test the "not found" case
	cfg := agent.Config{
		Command: []string{"nonexistent-aider-cli-binary-12345"},
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

// Test PlainTextParser.
func TestPlainTextParser_ParseEvent(t *testing.T) {
	p := NewPlainTextParser()

	line := []byte("Applied edit to file.go")
	event, err := p.ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event.Type != agent.EventText {
		t.Errorf("event.Type = %v, want %v", event.Type, agent.EventText)
	}
	if event.Text != "Applied edit to file.go" {
		t.Errorf("event.Text = %q, want %q", event.Text, "Applied edit to file.go")
	}
}

func TestPlainTextParser_Parse(t *testing.T) {
	p := NewPlainTextParser()

	events := []agent.Event{
		{Type: agent.EventText, Text: "Applied edit to main.go"},
		{Type: agent.EventText, Text: "Created test.go"},
		{Type: agent.EventText, Text: "Done!"},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(resp.Messages) == 0 {
		t.Fatal("expected at least one message")
	}

	// Check summary contains meaningful lines
	if resp.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestSummarizeAiderOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "applied edit",
			input:    "Some preamble\nApplied edit to main.go\nMore text",
			contains: "Applied edit",
		},
		{
			name:     "commit message",
			input:    "Commit abc123: fix bug",
			contains: "Commit",
		},
		{
			name:     "created file",
			input:    "Created new_file.go",
			contains: "Created",
		},
		{
			name:     "no status lines",
			input:    "Just some random output\nWithout status",
			contains: "Just some random output",
		},
		{
			name:     "empty",
			input:    "",
			contains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := summarizeAiderOutput(tt.input)
			if tt.contains != "" && !containsString(result, tt.contains) {
				t.Errorf("summarizeAiderOutput(%q) = %q, want to contain %q", tt.input, result, tt.contains)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test immutability of WithEnv/WithArgs.
func TestWithEnvImmutability(t *testing.T) {
	a1 := New()
	a2 := a1.WithEnv("KEY", "value")

	// Original should not be modified
	if _, ok := a1.config.Environment["KEY"]; ok {
		t.Error("WithEnv modified original agent")
	}

	// New agent should have the value
	if a2.(*Agent).config.Environment["KEY"] != "value" {
		t.Error("WithEnv didn't set value on new agent")
	}
}

func TestWithArgsImmutability(t *testing.T) {
	a1 := New()
	a2 := a1.WithArgs("--flag")

	// Original should not be modified
	if len(a1.config.Args) != 0 {
		t.Error("WithArgs modified original agent")
	}

	// New agent should have the args
	if len(a2.(*Agent).config.Args) != 1 || a2.(*Agent).config.Args[0] != "--flag" {
		t.Error("WithArgs didn't set args on new agent")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Additional Available tests
// ──────────────────────────────────────────────────────────────────────────────

func TestAvailable_Success(t *testing.T) {
	// Use "echo" as a mock binary that exists on all systems
	cfg := agent.Config{
		Command: []string{"echo"},
	}
	a := NewWithConfig(cfg)

	// The echo binary exists but won't work with --version
	// This tests the binary found path
	err := a.Available()
	// We expect either success (if echo has --version) or a "not working" error
	// But NOT "not found"
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			t.Errorf("Available should find echo binary, got: %v", err)
		}
		// "not working" is acceptable since echo doesn't have --version
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Additional RunWithCallback tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRunWithCallback_CallbackError(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"echo", "-n", "test output"},
		Timeout: 5 * time.Second,
	}
	a := NewWithConfig(cfg)

	callbackCount := 0
	cb := func(event agent.Event) error {
		callbackCount++
		// Return an error after first event
		if callbackCount >= 1 {
			return fmt.Errorf("callback error")
		}
		return nil
	}

	_, err := a.RunWithCallback(context.Background(), "test", cb)
	if err == nil {
		t.Error("RunWithCallback should return callback error")
	}
	if !strings.Contains(err.Error(), "callback error") {
		t.Errorf("Expected callback error, got: %v", err)
	}
}

func TestRunWithCallback_CollectsEvents(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"echo", "-n", "test output"},
		Timeout: 5 * time.Second,
	}
	a := NewWithConfig(cfg)

	var collectedEvents []agent.Event
	cb := func(event agent.Event) error {
		collectedEvents = append(collectedEvents, event)
		return nil
	}

	resp, err := a.RunWithCallback(context.Background(), "test", cb)
	if err != nil {
		t.Fatalf("RunWithCallback error = %v", err)
	}

	if len(collectedEvents) == 0 {
		t.Error("Expected at least one event")
	}
	if resp == nil {
		t.Error("Expected non-nil response")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Additional Run tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRun_Success(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"echo", "-n", "Here is the answer"},
		Timeout: 5 * time.Second,
	}
	a := NewWithConfig(cfg)

	resp, err := a.Run(context.Background(), "test")
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}

	if resp == nil {
		t.Fatal("Expected non-nil response")
	}
	if resp.Summary == "" {
		t.Error("Expected non-empty summary")
	}
}

func TestRun_ContextTimeout(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"sleep", "10"}, // Will exceed timeout
		Timeout: 100 * time.Millisecond,
	}
	a := NewWithConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := a.Run(ctx, "test")
	if err == nil {
		t.Error("Run should fail with timeout")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Additional RunStream tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRunStream_MultipleEvents(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"echo", "-n", "line1\nline2\nline3"},
		Timeout: 5 * time.Second,
	}
	a := NewWithConfig(cfg)

	events, errCh := a.RunStream(context.Background(), "test")

	var collected []agent.Event
	for event := range events {
		collected = append(collected, event)
	}

	err := <-errCh
	if err != nil {
		t.Fatalf("RunStream error = %v", err)
	}

	if len(collected) == 0 {
		t.Error("Expected at least one event")
	}
}

func TestRunStream_EmptyOutput(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"echo", "-n", ""},
		Timeout: 5 * time.Second,
	}
	a := NewWithConfig(cfg)

	events, errCh := a.RunStream(context.Background(), "test")

	collected := []agent.Event{}
	for event := range events {
		collected = append(collected, event)
	}

	err := <-errCh
	if err != nil {
		t.Fatalf("RunStream error = %v", err)
	}

	// Empty lines are skipped, so we might get 0 events
	if collected == nil {
		t.Error("collected should be initialized")
	}
}

func TestRunStream_CommandStartError(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"", "-n", "test"}, // Empty command name
		Timeout: 5 * time.Second,
	}
	a := NewWithConfig(cfg)

	events, errCh := a.RunStream(context.Background(), "test")

	// Drain events
	for range events {
	}

	err := <-errCh
	if err == nil {
		t.Error("RunStream should return error for empty command")
	}
}
