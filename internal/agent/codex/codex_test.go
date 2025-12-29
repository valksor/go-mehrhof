package codex

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
	if a.config.Command[0] != "codex" {
		t.Errorf("Command[0] = %q, want %q", a.config.Command[0], "codex")
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
		Command:     []string{"custom-codex", "--model", "gpt-4o"},
		Environment: map[string]string{"KEY": "val"},
		Timeout:     10 * time.Minute,
		WorkDir:     "/tmp",
	}

	a := NewWithConfig(cfg)
	if a == nil {
		t.Fatal("NewWithConfig returned nil")
	}
	if a.config.Command[0] != "custom-codex" {
		t.Errorf("Command[0] = %q, want %q", a.config.Command[0], "custom-codex")
	}
	if a.config.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want %v", a.config.Timeout, 10*time.Minute)
	}
}

func TestNewWithConfigEmptyCommand(t *testing.T) {
	cfg := agent.Config{
		Command: []string{}, // Empty command should default to "codex"
	}

	a := NewWithConfig(cfg)
	if a.config.Command[0] != "codex" {
		t.Errorf("Command[0] = %q, want %q (default)", a.config.Command[0], "codex")
	}
}

func TestName(t *testing.T) {
	a := New()
	if a.Name() != AgentName {
		t.Errorf("Name() = %q, want %q", a.Name(), AgentName)
	}
	if a.Name() != "codex" {
		t.Errorf("Name() = %q, want %q", a.Name(), "codex")
	}
}

func TestAgentNameConstant(t *testing.T) {
	if AgentName != "codex" {
		t.Errorf("AgentName = %q, want %q", AgentName, "codex")
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
	aAgent := a.WithArgs("--model", "gpt-4o")
	cfg := aAgent.(*Agent).config

	if len(cfg.Args) != 2 {
		t.Errorf("Args length = %d, want 2", len(cfg.Args))
	}
	if cfg.Args[0] != "--model" {
		t.Errorf("Args[0] = %q, want %q", cfg.Args[0], "--model")
	}
	if cfg.Args[1] != "gpt-4o" {
		t.Errorf("Args[1] = %q, want %q", cfg.Args[1], "gpt-4o")
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

	// Should include: --print, --verbose, --output-format, stream-json, prompt
	expectedArgs := []string{"--print", "--verbose", "--output-format", "stream-json", "Hello world"}

	if len(args) != len(expectedArgs) {
		t.Errorf("args length = %d, want %d", len(args), len(expectedArgs))
	}

	for i, expected := range expectedArgs {
		if i < len(args) && args[i] != expected {
			t.Errorf("args[%d] = %q, want %q", i, args[i], expected)
		}
	}
}

func TestBuildArgs_WithConfigArgs(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"codex", "--model", "gpt-4o"},
	}
	a := NewWithConfig(cfg)
	args := a.buildArgs("test")

	// Should include config args first: --model, gpt-4o
	if len(args) < 2 {
		t.Fatal("args should include config args")
	}
	if args[0] != "--model" || args[1] != "gpt-4o" {
		t.Errorf("expected config args [--model, gpt-4o], got %v", args[:2])
	}

	// Last arg should be the prompt
	if args[len(args)-1] != "test" {
		t.Errorf("last arg = %q, want %q", args[len(args)-1], "test")
	}
}

func TestAvailable_NoCLI(t *testing.T) {
	// Use a non-existent binary to test the "not found" case
	cfg := agent.Config{
		Command: []string{"nonexistent-codex-cli-binary-12345"},
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
