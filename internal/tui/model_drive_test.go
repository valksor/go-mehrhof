package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/valksor/kvelmo/internal/conductor"
)

// readyModel returns a model that has received a window size and two worktrees.
// The Model uses a pointer receiver for Update and mutates in place, so callers
// read fields directly off the returned model after each Update.
func readyModel(t *testing.T) *Model {
	t.Helper()
	m := NewModel("/tmp/proj", LayoutStacked)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(worktreeListMsg{dirs: []string{"/tmp/proj/wt1", "/tmp/proj/wt2"}})

	return &m
}

func TestModel_WindowSizeMakesReady(t *testing.T) {
	m := NewModel("/tmp/proj", LayoutStacked)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if !m.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}
	if m.width != 100 || m.height != 30 {
		t.Errorf("dimensions = %dx%d", m.width, m.height)
	}
}

func TestModel_WorktreeList(t *testing.T) {
	m := readyModel(t)
	if len(m.worktrees) != 2 {
		t.Errorf("worktrees = %d, want 2", len(m.worktrees))
	}
}

func TestModel_View_RendersInVariousStates(t *testing.T) {
	m := readyModel(t)

	if v := m.View(); v.Content == "" {
		t.Error("View should render content when ready")
	}

	m.showHelp = true
	_ = m.View()
	m.showHelp = false

	m.startMode = true
	_ = m.View()
	m.startMode = false
}

func TestModel_HandleKey_Navigation(t *testing.T) {
	m := readyModel(t)

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.active != 1 {
		t.Errorf("active after tab = %d, want 1", m.active)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.active != 0 {
		t.Errorf("active after shift+tab = %d, want 0", m.active)
	}

	before := m.showHelp
	m.Update(keyPress("?"))
	if m.showHelp == before {
		t.Error("? should toggle help")
	}
}

func TestModel_HandleSocketEvent(t *testing.T) {
	m := readyModel(t)

	m.Update(socketEventMsg{
		worktreeDir: "/tmp/proj/wt1",
		event: conductor.ConductorEvent{
			Type:    "state_changed",
			State:   conductor.StateImplementing,
			Message: "implementing now",
		},
	})

	var found bool
	for _, wt := range m.worktrees {
		if wt.Dir == "/tmp/proj/wt1" && wt.State == string(conductor.StateImplementing) {
			found = true
		}
	}
	if !found {
		t.Error("socket event did not update worktree state")
	}
}

func TestModel_ProgressMsg(t *testing.T) {
	m := readyModel(t)

	m.Update(progressMsg{
		worktreeDir: "/tmp/proj/wt1",
		percent:     0.5,
		etaSeconds:  30,
		calibrated:  true,
		active:      true,
	})

	for _, wt := range m.worktrees {
		if wt.Dir == "/tmp/proj/wt1" {
			if !wt.ProgressActive || wt.ProgressPercent != 0.5 {
				t.Errorf("progress not applied: %+v", wt)
			}
		}
	}
}

func TestModel_ErrMsg(t *testing.T) {
	m := readyModel(t)
	m.Update(errMsg{err: errors.New("boom")})
	if m.err == nil {
		t.Error("errMsg should set model.err")
	}
}

func TestModel_DisconnectedMsg(t *testing.T) {
	m := readyModel(t)
	m.Update(disconnectedMsg{worktreeDir: "/tmp/proj/wt1"})
	for _, wt := range m.worktrees {
		if wt.Dir == "/tmp/proj/wt1" && wt.State != "disconnected" {
			t.Errorf("wt1 state = %q, want disconnected", wt.State)
		}
	}
}

func TestModel_SpecAndChangelogResult(t *testing.T) {
	m := readyModel(t)

	m.Update(specResultMsg{content: "# Spec content"})
	_ = m.View()

	m.Update(changelogResultMsg{content: "## Changelog"})
	_ = m.View()
}

func TestModel_CommandResultMsg(t *testing.T) {
	m := readyModel(t)
	m.Update(commandResultMsg{output: "command output line"})
	_ = m.View()
}

func TestModel_ConnectedMsg(t *testing.T) {
	m := readyModel(t)
	_, cmd := m.Update(connectedMsg{worktreeDir: "/tmp/proj/wt1"})
	_ = cmd
}

func TestModel_Init(t *testing.T) {
	m := NewModel("/tmp/proj", LayoutStacked)
	if cmd := m.Init(); cmd == nil {
		t.Error("Init should return a discover command")
	}
}

func TestModel_QuitKey(t *testing.T) {
	m := readyModel(t)
	_, cmd := m.Update(keyPress("q"))
	if cmd == nil {
		t.Error("q should return a quit command")
	}
}

func TestRenderProgressBar(t *testing.T) {
	for _, pct := range []float64{0, 0.25, 0.5, 1.0} {
		if out := renderProgressBar(pct, 60, true); out == "" {
			t.Errorf("renderProgressBar(%f) returned empty", pct)
		}
	}
	if out := renderProgressBar(0.3, 0, false); out == "" {
		t.Error("uncalibrated progress bar should still render")
	}
}

// keyPress builds a tea.KeyPressMsg from a single-rune string.
func keyPress(s string) tea.KeyPressMsg {
	r := []rune(s)

	return tea.KeyPressMsg{Code: r[0], Text: s}
}
