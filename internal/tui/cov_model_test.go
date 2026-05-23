package tui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/valksor/kvelmo/internal/conductor"
)

// modelWithWorktree returns a model with a single worktree (no socket server),
// so workflow keys produce commands that error when executed — we only assert
// that a command is returned, not its result.
func modelWithWorktree() *Model {
	m := NewModel("/proj", LayoutStacked)
	m.worktrees = []WorktreeState{{Dir: "/proj/wt"}}

	return &m
}

func TestHandleKeyWorkflowShortcuts(t *testing.T) {
	// Each of these keys, when chat is unfocused, should return a non-nil cmd.
	keys := []string{"p", "i", "s", "u", "r", "v", "R", "S", "f", "o", "F", "e", "U"}
	for _, k := range keys {
		t.Run("key_"+k, func(t *testing.T) {
			m := modelWithWorktree()
			m.chatInput.Blur()
			_, cmd := m.handleKey(keyPress(k))
			if cmd == nil {
				t.Errorf("key %q should return a workflow command", k)
			}
		})
	}
	t.Run("ctrl+a aborts", func(t *testing.T) {
		m := modelWithWorktree()
		m.chatInput.Blur()
		_, cmd := m.handleKey(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
		if cmd == nil {
			t.Error("ctrl+a should return abort command")
		}
	})
}

func TestHandleKeyToggles(t *testing.T) {
	t.Run("t toggles start mode", func(t *testing.T) {
		m := NewModel("/proj", LayoutStacked)
		m.chatInput.Blur()
		m.handleKey(keyPress("t"))
		if !m.startMode {
			t.Error("t should enable start mode")
		}
		if m.chatInput.Placeholder != "Enter task description..." {
			t.Errorf("placeholder = %q", m.chatInput.Placeholder)
		}
		m.handleKey(keyPress("t"))
		if m.startMode {
			t.Error("t should disable start mode")
		}
		if m.chatInput.Placeholder != testChatPlaceholder {
			t.Errorf("placeholder = %q", m.chatInput.Placeholder)
		}
	})

	t.Run("d toggles dry-run", func(t *testing.T) {
		m := NewModel("/proj", LayoutStacked)
		m.chatInput.Blur()
		m.handleKey(keyPress("d"))
		if !m.dryRun {
			t.Error("d should enable dry-run")
		}
		m.handleKey(keyPress("d"))
		if m.dryRun {
			t.Error("d should disable dry-run")
		}
	})

	t.Run("c enables changelog mode", func(t *testing.T) {
		m := NewModel("/proj", LayoutStacked)
		m.chatInput.Blur()
		m.handleKey(keyPress("c"))
		if !m.changelogMode || m.changelogFull {
			t.Errorf("c: changelogMode=%v full=%v", m.changelogMode, m.changelogFull)
		}
		if !strings.Contains(m.chatInput.Placeholder, "source..target") {
			t.Errorf("placeholder = %q", m.chatInput.Placeholder)
		}
	})

	t.Run("C enables full changelog mode", func(t *testing.T) {
		m := NewModel("/proj", LayoutStacked)
		m.chatInput.Blur()
		m.handleKey(keyPress("C"))
		if !m.changelogMode || !m.changelogFull {
			t.Errorf("C: changelogMode=%v full=%v", m.changelogMode, m.changelogFull)
		}
	})
}

func TestHandleKeyFocusedForwardsToInput(t *testing.T) {
	// When chat is focused, workflow letters are typed into the input rather than
	// triggering workflow commands.
	m := NewModel("/proj", LayoutStacked)
	m.chatInput.Focus()
	_, cmd := m.handleKey(keyPress("p"))
	_ = cmd // forwarded to textinput; may be nil or a blink cmd
	if got := m.chatInput.Value(); got != "p" {
		t.Errorf("focused input value = %q, want %q", got, "p")
	}
}

func TestHandleKeyEnterStartMode(t *testing.T) {
	m := modelWithWorktree()
	m.startMode = true
	m.chatInput.SetValue("build the feature")
	_, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter in start mode should return a command")
	}
	if m.startMode {
		t.Error("start mode should reset after enter")
	}
	if m.chatInput.Placeholder != testChatPlaceholder {
		t.Errorf("placeholder should reset, got %q", m.chatInput.Placeholder)
	}
	if m.chatInput.Value() != "" {
		t.Error("input should be cleared")
	}
}

func TestHandleKeyEnterChangelogMode(t *testing.T) {
	m := modelWithWorktree()
	m.changelogMode = true
	m.changelogFull = true
	m.chatInput.SetValue("v1..v2")
	_, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter in changelog mode should return a command")
	}
	if m.changelogMode || m.changelogFull {
		t.Error("changelog mode flags should reset after enter")
	}
}

func TestHandleKeyEnterChatMessage(t *testing.T) {
	m := modelWithWorktree()
	m.chatInput.SetValue("hi there")
	_, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter with plain text should return a chat command")
	}
	if m.chatInput.Value() != "" {
		t.Error("input cleared after send")
	}
}

func TestHandleSocketEventTypes(t *testing.T) {
	mk := func() *Model {
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: "/a"}}

		return &m
	}

	t.Run("cache_hit annotated", func(t *testing.T) {
		m := mk()
		m.handleSocketEvent(socketEventMsg{worktreeDir: "/a", event: conductor.ConductorEvent{Type: "cache_hit", Message: "reused"}})
		if len(m.worktrees[0].Output) != 1 || !strings.HasSuffix(m.worktrees[0].Output[0], "[cached]") {
			t.Errorf("output = %v", m.worktrees[0].Output)
		}
	})

	t.Run("adversarial review appends", func(t *testing.T) {
		m := mk()
		m.handleSocketEvent(socketEventMsg{worktreeDir: "/a", event: conductor.ConductorEvent{Type: "adversarial_review_started", Message: "starting"}})
		if len(m.worktrees[0].Output) != 1 {
			t.Errorf("output = %v", m.worktrees[0].Output)
		}
	})

	t.Run("state_changed clears failure class on non-failed", func(t *testing.T) {
		m := mk()
		m.worktrees[0].LastFailureClass = "hard_stop"
		m.handleSocketEvent(socketEventMsg{worktreeDir: "/a", event: conductor.ConductorEvent{Type: "state_changed", State: conductor.State(stateImplementing)}})
		if m.worktrees[0].State != stateImplementing {
			t.Errorf("state = %q", m.worktrees[0].State)
		}
		if m.worktrees[0].LastFailureClass != "" {
			t.Error("failure class should be cleared on non-failed state")
		}
	})

	t.Run("state_changed to submitted clears progress", func(t *testing.T) {
		m := mk()
		m.worktrees[0].ProgressActive = true
		m.handleSocketEvent(socketEventMsg{worktreeDir: "/a", event: conductor.ConductorEvent{Type: "state_changed", State: conductor.State(stateSubmitted)}})
		if m.worktrees[0].ProgressActive {
			t.Error("progress should be cleared when leaving active phases")
		}
	})

	t.Run("state_changed to planning keeps progress active", func(t *testing.T) {
		m := mk()
		m.worktrees[0].ProgressActive = true
		m.handleSocketEvent(socketEventMsg{worktreeDir: "/a", event: conductor.ConductorEvent{Type: "state_changed", State: conductor.State(statePlanning)}})
		if !m.worktrees[0].ProgressActive {
			t.Error("progress should stay active during planning")
		}
	})

	t.Run("phase_failure_classified sets class", func(t *testing.T) {
		m := mk()
		m.handleSocketEvent(socketEventMsg{worktreeDir: "/a", event: conductor.ConductorEvent{Type: "phase_failure_classified", FailureClass: conductor.FailureClass("recoverable")}})
		if m.worktrees[0].LastFailureClass != "recoverable" {
			t.Errorf("failure class = %q", m.worktrees[0].LastFailureClass)
		}
	})

	t.Run("autofix_attempt parses attempt/max", func(t *testing.T) {
		m := mk()
		data, _ := json.Marshal(map[string]int{"attempt": 2, "max_attempts": 3})
		m.handleSocketEvent(socketEventMsg{worktreeDir: "/a", event: conductor.ConductorEvent{Type: "autofix_attempt", Message: "fixing", Data: data}})
		if m.worktrees[0].AutoFixAttempt != 2 || m.worktrees[0].AutoFixMax != 3 {
			t.Errorf("autofix = %d/%d", m.worktrees[0].AutoFixAttempt, m.worktrees[0].AutoFixMax)
		}
	})

	t.Run("autofix_success resets counters", func(t *testing.T) {
		m := mk()
		m.worktrees[0].AutoFixAttempt = 2
		m.worktrees[0].AutoFixMax = 3
		m.handleSocketEvent(socketEventMsg{worktreeDir: "/a", event: conductor.ConductorEvent{Type: "autofix_success", Message: "done"}})
		if m.worktrees[0].AutoFixAttempt != 0 || m.worktrees[0].AutoFixMax != 0 {
			t.Errorf("autofix counters not reset: %d/%d", m.worktrees[0].AutoFixAttempt, m.worktrees[0].AutoFixMax)
		}
	})

	t.Run("worktree_provisioned appends", func(t *testing.T) {
		m := mk()
		m.handleSocketEvent(socketEventMsg{worktreeDir: "/a", event: conductor.ConductorEvent{Type: "worktree_provisioned", Message: "provisioned"}})
		if len(m.worktrees[0].Output) != 1 {
			t.Errorf("output = %v", m.worktrees[0].Output)
		}
	})

	t.Run("risk_evaluated sets level", func(t *testing.T) {
		m := mk()
		data, _ := json.Marshal(map[string]string{"level": "high"})
		m.handleSocketEvent(socketEventMsg{worktreeDir: "/a", event: conductor.ConductorEvent{Type: "risk_evaluated", Data: data}})
		if m.worktrees[0].RiskLevel != "high" {
			t.Errorf("risk = %q", m.worktrees[0].RiskLevel)
		}
	})

	t.Run("context_attached appends", func(t *testing.T) {
		m := mk()
		m.handleSocketEvent(socketEventMsg{worktreeDir: "/a", event: conductor.ConductorEvent{Type: "context_attached", Message: "added file"}})
		if len(m.worktrees[0].Output) != 1 {
			t.Errorf("output = %v", m.worktrees[0].Output)
		}
	})
}

func TestParseAutoFixHelpers(t *testing.T) {
	t.Run("attempt valid", func(t *testing.T) {
		data, _ := json.Marshal(map[string]int{"attempt": 5})
		if got := parseAutoFixAttempt(data); got != 5 {
			t.Errorf("attempt = %d, want 5", got)
		}
	})
	t.Run("attempt invalid json", func(t *testing.T) {
		if got := parseAutoFixAttempt(json.RawMessage("not json")); got != 0 {
			t.Errorf("attempt = %d, want 0", got)
		}
	})
	t.Run("max valid", func(t *testing.T) {
		data, _ := json.Marshal(map[string]int{"max_attempts": 7})
		if got := parseAutoFixMax(data); got != 7 {
			t.Errorf("max = %d, want 7", got)
		}
	})
	t.Run("max invalid json", func(t *testing.T) {
		if got := parseAutoFixMax(json.RawMessage("garbage")); got != 0 {
			t.Errorf("max = %d, want 0", got)
		}
	})
}
