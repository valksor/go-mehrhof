package copilot

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

	if a.mode != ModeSuggest {
		t.Errorf("default mode = %q, want %q", a.mode, ModeSuggest)
	}

	if a.target != TargetShell {
		t.Errorf("default target = %q, want %q", a.target, TargetShell)
	}
}

func TestWithMode(t *testing.T) {
	a := New()
	b := a.WithMode(ModeExplain)

	// Original should be unchanged
	if a.mode != ModeSuggest {
		t.Error("WithMode modified original agent")
	}

	// New agent should have new mode
	if b.mode != ModeExplain {
		t.Errorf("WithMode() mode = %q, want %q", b.mode, ModeExplain)
	}
}

func TestWithTarget(t *testing.T) {
	a := New()
	b := a.WithTarget(TargetGit)

	if a.target != TargetShell {
		t.Error("WithTarget modified original agent")
	}

	if b.target != TargetGit {
		t.Errorf("WithTarget() target = %q, want %q", b.target, TargetGit)
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

func TestWithArgs(t *testing.T) {
	a := New()
	b := a.WithArgs("--mode", "explain").(*Agent)

	if len(a.config.Args) != 0 {
		t.Error("WithArgs modified original agent")
	}

	if len(b.config.Args) != 2 {
		t.Errorf("WithArgs() args len = %d, want 2", len(b.config.Args))
	}

	if b.config.Args[0] != "--mode" || b.config.Args[1] != "explain" {
		t.Errorf("WithArgs() args = %v, want [--mode explain]", b.config.Args)
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

func TestBuildArgs(t *testing.T) {
	tests := []struct {
		name   string
		mode   Mode
		target TargetType
		prompt string
		want   []string
	}{
		{
			name:   "suggest mode with shell target",
			mode:   ModeSuggest,
			target: TargetShell,
			prompt: "list all files",
			want:   []string{"copilot", "suggest", "-t", "shell", "list all files"},
		},
		{
			name:   "suggest mode with git target",
			mode:   ModeSuggest,
			target: TargetGit,
			prompt: "show recent commits",
			want:   []string{"copilot", "suggest", "-t", "git", "show recent commits"},
		},
		{
			name:   "suggest mode with gh target",
			mode:   ModeSuggest,
			target: TargetGH,
			prompt: "list my repos",
			want:   []string{"copilot", "suggest", "-t", "gh", "list my repos"},
		},
		{
			name:   "explain mode",
			mode:   ModeExplain,
			target: TargetShell, // Should be ignored in explain mode
			prompt: "git rebase -i HEAD~3",
			want:   []string{"copilot", "explain", "git rebase -i HEAD~3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New()
			a.mode = tt.mode
			a.target = tt.target

			got := a.buildArgs(tt.prompt)

			if len(got) != len(tt.want) {
				t.Errorf("buildArgs() len = %d, want %d", len(got), len(tt.want))
				return
			}

			for i, arg := range got {
				if arg != tt.want[i] {
					t.Errorf("buildArgs()[%d] = %q, want %q", i, arg, tt.want[i])
				}
			}
		})
	}
}

func TestMetadata(t *testing.T) {
	a := New()
	meta := a.Metadata()

	if meta.Name != "GitHub Copilot CLI" {
		t.Errorf("Metadata().Name = %q, want %q", meta.Name, "GitHub Copilot CLI")
	}

	if meta.Capabilities.Streaming {
		t.Error("Metadata().Capabilities.Streaming should be false")
	}

	if meta.Capabilities.ToolUse {
		t.Error("Metadata().Capabilities.ToolUse should be false")
	}

	if meta.Capabilities.MultiTurn {
		t.Error("Metadata().Capabilities.MultiTurn should be false")
	}
}

func TestPlainTextParser_ParseEvent(t *testing.T) {
	p := NewPlainTextParser()

	event, err := p.ParseEvent([]byte("ls -la"))
	if err != nil {
		t.Fatalf("ParseEvent() error = %v", err)
	}

	if event.Type != agent.EventText {
		t.Errorf("ParseEvent() type = %v, want %v", event.Type, agent.EventText)
	}

	if event.Text != "ls -la" {
		t.Errorf("ParseEvent() text = %q, want %q", event.Text, "ls -la")
	}
}

func TestPlainTextParser_Parse(t *testing.T) {
	p := NewPlainTextParser()

	events := []agent.Event{
		{Type: agent.EventText, Text: "First line"},
		{Type: agent.EventText, Text: "Second line"},
		{Type: agent.EventText, Text: "Third line"},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(resp.Messages) != 1 {
		t.Errorf("Parse() messages len = %d, want 1", len(resp.Messages))
	}

	expectedMsg := "First line\nSecond line\nThird line"
	if resp.Messages[0] != expectedMsg {
		t.Errorf("Parse() message = %q, want %q", resp.Messages[0], expectedMsg)
	}

	if resp.Summary != "First line" {
		t.Errorf("Parse() summary = %q, want %q", resp.Summary, "First line")
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
			input: "ls -la",
			want:  "ls -la",
		},
		{
			name:  "multiple lines",
			input: "First\nSecond\nThird",
			want:  "First",
		},
		{
			name:  "skip suggestion prefix",
			input: "Suggestion: ls -la\nThis lists files",
			want:  "This lists files",
		},
		{
			name:  "skip command prefix",
			input: "Command: git status\nShows current status",
			want:  "Shows current status",
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

func TestAgentInterface(t *testing.T) {
	// Verify Agent implements agent.Agent
	var _ agent.Agent = (*Agent)(nil)

	// Verify Agent implements MetadataProvider
	var _ agent.MetadataProvider = (*Agent)(nil)
}

func TestNewWithConfig(t *testing.T) {
	cfg := agent.Config{
		Command:    []string{"custom-gh", "copilot"},
		Timeout:    10 * time.Minute,
		RetryCount: 5,
		RetryDelay: 2 * time.Second,
		WorkDir:    "/custom/dir",
		Args:       []string{"--mode", "explain"},
		Environment: map[string]string{
			"TEST_VAR": "test_value",
		},
	}

	a := NewWithConfig(cfg)

	if a.Name() != AgentName {
		t.Errorf("Name() = %q, want %q", a.Name(), AgentName)
	}

	if a.config.Command[0] != "custom-gh" {
		t.Errorf("Command[0] = %q, want %q", a.config.Command[0], "custom-gh")
	}

	if a.config.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want 10m", a.config.Timeout)
	}

	if a.config.WorkDir != "/custom/dir" {
		t.Errorf("WorkDir = %q, want %q", a.config.WorkDir, "/custom/dir")
	}

	if len(a.config.Args) != 2 {
		t.Errorf("Args len = %d, want 2", len(a.config.Args))
	}
}

func TestNewWithConfig_EmptyCommand(t *testing.T) {
	cfg := agent.Config{
		Command: []string{},
	}

	a := NewWithConfig(cfg)

	if a.config.Command[0] != "gh" {
		t.Errorf("Command[0] = %q, want %q (default)", a.config.Command[0], "gh")
	}
}

func TestSetParser(t *testing.T) {
	a := New()
	mockParser := &mockParser{}

	a.SetParser(mockParser)

	if a.parser != mockParser {
		t.Error("SetParser() did not set parser")
	}
}

func TestSetMode(t *testing.T) {
	a := New()

	a.SetMode(ModeExplain)

	if a.mode != ModeExplain {
		t.Errorf("SetMode() mode = %q, want %q", a.mode, ModeExplain)
	}
}

func TestSetTarget(t *testing.T) {
	a := New()

	a.SetTarget(TargetGH)

	if a.target != TargetGH {
		t.Errorf("SetTarget() target = %q, want %q", a.target, TargetGH)
	}
}

func TestBuildArgs_ConfigOverrides(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		prompt string
		want   string // Check for mode in result
	}{
		{
			name:   "mode override via --mode",
			args:   []string{"--mode", "explain"},
			prompt: "test prompt",
			want:   "explain",
		},
		{
			name:   "mode override via -m",
			args:   []string{"-m", "explain"},
			prompt: "test prompt",
			want:   "explain",
		},
		{
			name:   "target override via --target",
			args:   []string{"--target", "git"},
			prompt: "test prompt",
			want:   "suggest", // mode should still be suggest
		},
		{
			name:   "target override via -t",
			args:   []string{"-t", "gh"},
			prompt: "test prompt",
			want:   "suggest",
		},
		{
			name:   "both mode and target override",
			args:   []string{"--mode", "explain", "--target", "git"},
			prompt: "test prompt",
			want:   "explain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New()
			a.config.Args = tt.args

			got := a.buildArgs(tt.prompt)

			// Find the mode in args
			foundMode := ""
			for i, arg := range got {
				if arg == "suggest" || arg == "explain" {
					foundMode = arg
					break
				}
				if i > 0 && got[i-1] == "copilot" && (arg == "suggest" || arg == "explain") {
					foundMode = arg
					break
				}
			}

			if foundMode != tt.want {
				t.Errorf("buildArgs() mode = %q, want %q, args = %v", foundMode, tt.want, got)
			}
		})
	}
}

func TestBuildArgs_ConfigOverrides_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		prompt string
		// Should keep defaults for unrecognized values
		wantMode   Mode
		wantTarget TargetType
	}{
		{
			name:       "invalid mode value keeps default",
			args:       []string{"--mode", "invalid"},
			prompt:     "test",
			wantMode:   ModeSuggest,
			wantTarget: TargetShell,
		},
		{
			name:       "invalid target value keeps default",
			args:       []string{"--target", "invalid"},
			prompt:     "test",
			wantMode:   ModeSuggest,
			wantTarget: TargetShell,
		},
		{
			name:       "mode flag without value keeps default",
			args:       []string{"--mode"},
			prompt:     "test",
			wantMode:   ModeSuggest,
			wantTarget: TargetShell,
		},
		{
			name:       "target flag at end without value",
			args:       []string{"--target"},
			prompt:     "test",
			wantMode:   ModeSuggest,
			wantTarget: TargetShell,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New()
			a.config.Args = tt.args

			got := a.buildArgs(tt.prompt)

			// Check that defaults are used
			foundMode := a.mode
			foundTarget := a.target

			// Parse mode from result
			for i, arg := range got {
				if i > 0 && got[i-1] == "copilot" {
					if arg == "suggest" || arg == "explain" {
						foundMode = Mode(arg)
					}
				}
			}

			if foundMode != tt.wantMode {
				t.Errorf("buildArgs() mode = %q, want %q", foundMode, tt.wantMode)
			}

			if foundTarget != tt.wantTarget {
				t.Errorf("buildArgs() target = %q, want %q", foundTarget, tt.wantTarget)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	registry := agent.NewRegistry()

	err := Register(registry)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Verify the agent is registered
	got, err := registry.Get(AgentName)
	if err != nil {
		t.Error("Register() did not register the agent")
	}

	if got.Name() != AgentName {
		t.Errorf("Registered agent name = %q, want %q", got.Name(), AgentName)
	}
}

func TestRegister_Duplicate(t *testing.T) {
	registry := agent.NewRegistry()

	// First registration should succeed
	err := Register(registry)
	if err != nil {
		t.Fatalf("First Register() error = %v", err)
	}

	// Second registration should fail
	err = Register(registry)
	if err == nil {
		t.Error("Second Register() should return error for duplicate")
	}
}

func TestPlainTextParser_Parse_EmptyEvents(t *testing.T) {
	p := NewPlainTextParser()

	resp, err := p.Parse([]agent.Event{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(resp.Messages) != 0 {
		t.Errorf("Parse() messages len = %d, want 0", len(resp.Messages))
	}

	if resp.Summary != "" {
		t.Errorf("Parse() summary = %q, want empty", resp.Summary)
	}
}

func TestPlainTextParser_Parse_WithEmptyText(t *testing.T) {
	p := NewPlainTextParser()

	events := []agent.Event{
		{Type: agent.EventText, Text: ""},
		{Type: agent.EventText, Text: "   "},
		{Type: agent.EventText, Text: "actual content"},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should have 1 non-empty message
	if len(resp.Messages) != 1 {
		t.Errorf("Parse() messages len = %d, want 1", len(resp.Messages))
	}

	if resp.Summary != "actual content" {
		t.Errorf("Parse() summary = %q, want %q", resp.Summary, "actual content")
	}
}

func TestWithEnv_Chaining(t *testing.T) {
	a := New()
	a.config.Environment = map[string]string{"EXISTING": "value"}

	b := a.WithEnv("NEW_KEY", "new_value").(*Agent)

	// Original should not have new key
	if _, ok := a.config.Environment["NEW_KEY"]; ok {
		t.Error("WithEnv modified original agent")
	}

	// Original should still have existing key
	if a.config.Environment["EXISTING"] != "value" {
		t.Error("WithEnv removed existing keys from original")
	}

	// New agent should have both keys
	if b.config.Environment["EXISTING"] != "value" {
		t.Error("WithEnv did not copy existing keys")
	}

	if b.config.Environment["NEW_KEY"] != "new_value" {
		t.Errorf("WithEnv() NEW_KEY = %q, want %q", b.config.Environment["NEW_KEY"], "new_value")
	}
}

func TestWithArgs_Chaining(t *testing.T) {
	a := New()
	a.config.Args = []string{"--existing"}

	b := a.WithArgs("--new1", "--new2").(*Agent)

	// Original should be unchanged
	if len(a.config.Args) != 1 {
		t.Error("WithArgs modified original agent")
	}

	// New agent should have both existing and new args
	if len(b.config.Args) != 3 {
		t.Errorf("WithArgs() args len = %d, want 3", len(b.config.Args))
	}

	if b.config.Args[0] != "--existing" {
		t.Errorf("WithArgs() args[0] = %q, want %q", b.config.Args[0], "--existing")
	}
}

// mockParser is a simple mock for testing SetParser
type mockParser struct{}

func (m *mockParser) ParseEvent(line []byte) (agent.Event, error) {
	return agent.Event{Type: agent.EventText, Text: string(line)}, nil
}

func (m *mockParser) Parse(events []agent.Event) (*agent.Response, error) {
	return &agent.Response{Messages: []string{"mock"}}, nil
}
