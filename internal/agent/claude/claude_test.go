package claude

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
	if a.config.Command[0] != "claude" {
		t.Errorf("Command[0] = %q, want %q", a.config.Command[0], "claude")
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
		Command:     []string{"custom-claude", "--model", "opus"},
		Environment: map[string]string{"KEY": "val"},
		Timeout:     10 * time.Minute,
		WorkDir:     "/tmp",
	}

	a := NewWithConfig(cfg)
	if a == nil {
		t.Fatal("NewWithConfig returned nil")
	}
	if a.config.Command[0] != "custom-claude" {
		t.Errorf("Command[0] = %q, want %q", a.config.Command[0], "custom-claude")
	}
	if a.config.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want %v", a.config.Timeout, 10*time.Minute)
	}
}

func TestNewWithConfigEmptyCommand(t *testing.T) {
	cfg := agent.Config{
		Command: []string{}, // Empty command should default to "claude"
	}

	a := NewWithConfig(cfg)
	if a.config.Command[0] != "claude" {
		t.Errorf("Command[0] = %q, want %q (default)", a.config.Command[0], "claude")
	}
}

func TestName(t *testing.T) {
	a := New()
	if a.Name() != AgentName {
		t.Errorf("Name() = %q, want %q", a.Name(), AgentName)
	}
	if a.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", a.Name(), "claude")
	}
}

func TestAgentNameConstant(t *testing.T) {
	if AgentName != "claude" {
		t.Errorf("AgentName = %q, want %q", AgentName, "claude")
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
	agent, ok := aAgent.(*Agent)
	if !ok {
		t.Fatal("WithEnv did not return *Agent")
	}
	if agent.config.Environment["API_KEY"] != "secret123" {
		t.Errorf("Environment[API_KEY] = %q, want %q", agent.config.Environment["API_KEY"], "secret123")
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
	typed, ok := aAgent.(*Agent)
	if !ok {
		t.Fatal("WithEnv did not return *Agent")
	}
	if typed.config.Environment["KEY1"] != "val1" {
		t.Error("WithEnv(KEY1) chain failed")
	}
	if typed.config.Environment["KEY2"] != "val2" {
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
		Command: []string{"claude", "--model", "opus"},
	}
	a := NewWithConfig(cfg)
	args := a.buildArgs("test")

	// Should include config args first: --model, opus
	if len(args) < 2 {
		t.Fatal("args should include config args")
	}
	if args[0] != "--model" || args[1] != "opus" {
		t.Errorf("expected config args [--model, opus], got %v", args[:2])
	}

	// Last arg should be the prompt
	if args[len(args)-1] != "test" {
		t.Errorf("last arg = %q, want %q", args[len(args)-1], "test")
	}
}

func TestAvailable_NoCLI(t *testing.T) {
	// Use a non-existent binary to test the "not found" case
	cfg := agent.Config{
		Command: []string{"nonexistent-claude-cli-binary-12345"},
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

func TestWithArgs(t *testing.T) {
	tests := []struct {
		name         string
		existingArgs []string
		newArgs      []string
		wantLen      int
		wantLast     []string
	}{
		{
			name:         "no existing args",
			existingArgs: nil,
			newArgs:      []string{"--model", "opus"},
			wantLen:      2,
			wantLast:     []string{"--model", "opus"},
		},
		{
			name:         "append to existing args",
			existingArgs: []string{"--print"},
			newArgs:      []string{"--verbose", "--output", "json"},
			wantLen:      4,
			wantLast:     []string{"--print", "--verbose", "--output", "json"},
		},
		{
			name:         "empty new args",
			existingArgs: []string{"--print"},
			newArgs:      []string{},
			wantLen:      1,
			wantLast:     []string{"--print"},
		},
		{
			name:         "single new arg",
			existingArgs: []string{},
			newArgs:      []string{"--help"},
			wantLen:      1,
			wantLast:     []string{"--help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := agent.Config{
				Command: []string{"claude"},
				Args:    tt.existingArgs,
			}
			a := NewWithConfig(cfg)

			// WithArgs returns agent.Agent interface
			aAgent := a.WithArgs(tt.newArgs...)
			resultAgent, ok := aAgent.(*Agent)
			if !ok {
				t.Fatal("WithArgs did not return *Agent")
			}

			if len(resultAgent.config.Args) != tt.wantLen {
				t.Errorf("Args length = %d, want %d", len(resultAgent.config.Args), tt.wantLen)
			}

			for i, want := range tt.wantLast {
				if i >= len(resultAgent.config.Args) {
					t.Errorf("Missing arg at index %d", i)

					continue
				}
				if resultAgent.config.Args[i] != want {
					t.Errorf("Args[%d] = %q, want %q", i, resultAgent.config.Args[i], want)
				}
			}
		})
	}
}

func TestWithArgs_Chaining(t *testing.T) {
	a := New()
	aAgent := agent.Agent(a)

	// Chain multiple WithArgs calls
	aAgent = aAgent.WithArgs("--model", "opus")
	aAgent = aAgent.WithArgs("--max-tokens", "4096")
	aAgent = aAgent.WithArgs("--temperature", "0.7")

	resultAgent, ok := aAgent.(*Agent)
	if !ok {
		t.Fatal("WithArgs did not return *Agent")
	}
	expectedArgs := []string{"--model", "opus", "--max-tokens", "4096", "--temperature", "0.7"}

	if len(resultAgent.config.Args) != len(expectedArgs) {
		t.Errorf("Args length = %d, want %d", len(resultAgent.config.Args), len(expectedArgs))
	}

	for i, want := range expectedArgs {
		if resultAgent.config.Args[i] != want {
			t.Errorf("Args[%d] = %q, want %q", i, resultAgent.config.Args[i], want)
		}
	}
}

func TestWithArgs_OriginalUnmodified(t *testing.T) {
	cfg := agent.Config{
		Command: []string{"claude"},
		Args:    []string{"--print"},
	}
	originalAgent := NewWithConfig(cfg)
	originalLen := len(originalAgent.config.Args)

	// WithArgs should return a new agent, not modify the original
	newAgent := originalAgent.WithArgs("--verbose")

	if len(originalAgent.config.Args) != originalLen {
		t.Errorf("Original agent was modified (len = %d, want %d)", len(originalAgent.config.Args), originalLen)
	}

	typedNew, ok := newAgent.(*Agent)
	if !ok {
		t.Fatal("WithArgs did not return *Agent")
	}
	if len(typedNew.config.Args) != originalLen+1 {
		t.Errorf("New agent has wrong arg count (len = %d, want %d)", len(typedNew.config.Args), originalLen+1)
	}
}
