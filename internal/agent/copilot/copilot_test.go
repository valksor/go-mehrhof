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
