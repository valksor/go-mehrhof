package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/valksor/kvelmo/pkg/conductor"
)

func TestNewModel(t *testing.T) {
	t.Run("sets cwd and layout", func(t *testing.T) {
		m := NewModel("/tmp/proj", LayoutStacked)
		if m.cwd != "/tmp/proj" {
			t.Errorf("cwd = %q, want %q", m.cwd, "/tmp/proj")
		}
		if m.layout != LayoutStacked {
			t.Errorf("layout = %d, want %d", m.layout, LayoutStacked)
		}
	})

	t.Run("dashboard layout", func(t *testing.T) {
		m := NewModel("/tmp/proj", LayoutDashboard)
		if m.layout != LayoutDashboard {
			t.Errorf("layout = %d, want %d", m.layout, LayoutDashboard)
		}
	})

	t.Run("initial state", func(t *testing.T) {
		m := NewModel("/tmp/proj", LayoutStacked)
		if m.ready {
			t.Error("ready should be false initially")
		}
		if m.showHelp {
			t.Error("showHelp should be false initially")
		}
		if m.active != 0 {
			t.Errorf("active = %d, want 0", m.active)
		}
		if len(m.worktrees) != 0 {
			t.Errorf("worktrees should be empty, got %d", len(m.worktrees))
		}
		if m.err != nil {
			t.Errorf("err should be nil, got %v", m.err)
		}
		if m.ctx == nil {
			t.Error("ctx should not be nil")
		}
		if m.cancel == nil {
			t.Error("cancel should not be nil")
		}
		if m.msgs == nil {
			t.Error("msgs channel should not be nil")
		}
	})

	t.Run("chat input placeholder", func(t *testing.T) {
		m := NewModel("/tmp/proj", LayoutStacked)
		if m.chatInput.Placeholder != "Chat with agent..." {
			t.Errorf("placeholder = %q, want %q", m.chatInput.Placeholder, "Chat with agent...")
		}
	})
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{name: "shorter than n", s: "abc", n: 6, want: "abc   "},
		{name: "exact length", s: "abc", n: 3, want: "abc"},
		{name: "longer than n", s: "abcdef", n: 3, want: "abc"},
		{name: "empty string", s: "", n: 4, want: "    "},
		{name: "zero width", s: "abc", n: 0, want: ""},
		{name: "single char pad", s: "ab", n: 3, want: "ab "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := padRight(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("padRight(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
			if len(got) != tt.n {
				t.Errorf("padRight(%q, %d) length = %d, want %d", tt.s, tt.n, len(got), tt.n)
			}
		})
	}
}

func TestOutputHeight(t *testing.T) {
	tests := []struct {
		name       string
		height     int
		layout     Layout
		worktrees  int
		wantHeight int
	}{
		{
			name:       "stacked single worktree",
			height:     24,
			layout:     LayoutStacked,
			worktrees:  1,
			wantHeight: 21, // 24 - 3 (status + chat divider + chat input)
		},
		{
			name:       "stacked multiple worktrees",
			height:     24,
			layout:     LayoutStacked,
			worktrees:  2,
			wantHeight: 20, // 24 - 3 - 1 (tab bar)
		},
		{
			name:       "dashboard single worktree",
			height:     24,
			layout:     LayoutDashboard,
			worktrees:  1,
			wantHeight: 18, // 24 - 3 - 3 (workers)
		},
		{
			name:       "dashboard multiple worktrees",
			height:     24,
			layout:     LayoutDashboard,
			worktrees:  3,
			wantHeight: 17, // 24 - 3 - 1 - 3
		},
		{
			name:       "minimum height clamped to 1",
			height:     2,
			layout:     LayoutDashboard,
			worktrees:  5,
			wantHeight: 1,
		},
		{
			name:       "zero height clamped to 1",
			height:     0,
			layout:     LayoutStacked,
			worktrees:  0,
			wantHeight: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel("/tmp", tt.layout)
			m.height = tt.height
			for i := range tt.worktrees {
				m.worktrees = append(m.worktrees, WorktreeState{Dir: fmt.Sprintf("/tmp/wt%d", i)})
			}
			got := m.outputHeight()
			if got != tt.wantHeight {
				t.Errorf("outputHeight() = %d, want %d", got, tt.wantHeight)
			}
		})
	}
}

func TestHasWorktree(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	m.worktrees = []WorktreeState{
		{Dir: "/home/user/proj"},
		{Dir: "/home/user/proj-wt1"},
	}

	tests := []struct {
		dir  string
		want bool
	}{
		{"/home/user/proj", true},
		{"/home/user/proj-wt1", true},
		{"/home/user/other", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			if got := m.hasWorktree(tt.dir); got != tt.want {
				t.Errorf("hasWorktree(%q) = %v, want %v", tt.dir, got, tt.want)
			}
		})
	}
}

func TestActiveWorktree(t *testing.T) {
	t.Run("nil when empty", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		if wt := m.activeWorktree(); wt != nil {
			t.Errorf("activeWorktree() = %v, want nil", wt)
		}
	})

	t.Run("returns active worktree", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		m.worktrees = []WorktreeState{
			{Dir: "/a"},
			{Dir: "/b"},
		}
		m.active = 1
		wt := m.activeWorktree()
		if wt == nil {
			t.Fatal("activeWorktree() = nil, want non-nil")
		}
		if wt.Dir != "/b" {
			t.Errorf("activeWorktree().Dir = %q, want %q", wt.Dir, "/b")
		}
	})
}

func TestHandleKey(t *testing.T) {
	t.Run("q quits", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		_, cmd := m.handleKey(tea.KeyPressMsg{Code: 'q', Text: "q"})
		if cmd == nil {
			t.Fatal("expected quit command, got nil")
		}
		// Execute the cmd and check it produces a QuitMsg
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("expected QuitMsg, got %T", msg)
		}
	})

	t.Run("ctrl+c quits", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		_, cmd := m.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		if cmd == nil {
			t.Fatal("expected quit command, got nil")
		}
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("expected QuitMsg, got %T", msg)
		}
	})

	t.Run("? toggles help", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		if m.showHelp {
			t.Fatal("showHelp should be false initially")
		}
		m.handleKey(tea.KeyPressMsg{Code: '?', Text: "?"})
		if !m.showHelp {
			t.Error("showHelp should be true after pressing ?")
		}
		m.handleKey(tea.KeyPressMsg{Code: '?', Text: "?"})
		if m.showHelp {
			t.Error("showHelp should be false after pressing ? again")
		}
	})

	t.Run("tab cycles forward", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		m.worktrees = []WorktreeState{
			{Dir: "/a"},
			{Dir: "/b"},
			{Dir: "/c"},
		}
		if m.active != 0 {
			t.Fatalf("active = %d, want 0", m.active)
		}
		m.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
		if m.active != 1 {
			t.Errorf("after tab: active = %d, want 1", m.active)
		}
		m.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
		if m.active != 2 {
			t.Errorf("after tab: active = %d, want 2", m.active)
		}
		m.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
		if m.active != 0 {
			t.Errorf("after tab (wrap): active = %d, want 0", m.active)
		}
	})

	t.Run("shift+tab cycles backward", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		m.worktrees = []WorktreeState{
			{Dir: "/a"},
			{Dir: "/b"},
			{Dir: "/c"},
		}
		m.handleKey(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		if m.active != 2 {
			t.Errorf("after shift+tab: active = %d, want 2", m.active)
		}
		m.handleKey(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		if m.active != 1 {
			t.Errorf("after shift+tab: active = %d, want 1", m.active)
		}
	})

	t.Run("tab with no worktrees does nothing", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		m.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
		if m.active != 0 {
			t.Errorf("active = %d, want 0", m.active)
		}
	})

	t.Run("enter with empty chat does nothing", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		_, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd != nil {
			t.Errorf("expected nil cmd for empty chat, got non-nil")
		}
	})
}

func TestHandleWorktreeList(t *testing.T) {
	t.Run("adds new worktrees", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		msg := worktreeListMsg{dirs: []string{"/a", "/b"}}
		m.handleWorktreeList(msg)
		if len(m.worktrees) != 2 {
			t.Fatalf("worktrees = %d, want 2", len(m.worktrees))
		}
		if m.worktrees[0].Dir != "/a" {
			t.Errorf("worktrees[0].Dir = %q, want %q", m.worktrees[0].Dir, "/a")
		}
		if m.worktrees[1].Dir != "/b" {
			t.Errorf("worktrees[1].Dir = %q, want %q", m.worktrees[1].Dir, "/b")
		}
	})

	t.Run("does not duplicate existing worktrees", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: "/a"}}
		msg := worktreeListMsg{dirs: []string{"/a", "/b"}}
		m.handleWorktreeList(msg)
		if len(m.worktrees) != 2 {
			t.Fatalf("worktrees = %d, want 2", len(m.worktrees))
		}
	})

	t.Run("empty list adds nothing", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		msg := worktreeListMsg{dirs: []string{}}
		m.handleWorktreeList(msg)
		if len(m.worktrees) != 0 {
			t.Errorf("worktrees = %d, want 0", len(m.worktrees))
		}
	})
}

func TestHandleSocketEvent(t *testing.T) {
	t.Run("job_output appends to correct worktree", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		m.worktrees = []WorktreeState{
			{Dir: "/a"},
			{Dir: "/b"},
		}
		msg := socketEventMsg{
			worktreeDir: "/b",
			event: conductor.ConductorEvent{
				Type:    "job_output",
				Message: "hello world",
			},
		}
		m.handleSocketEvent(msg)
		if len(m.worktrees[1].Output) != 1 {
			t.Fatalf("output lines = %d, want 1", len(m.worktrees[1].Output))
		}
		if m.worktrees[1].Output[0] != "hello world" {
			t.Errorf("output = %q, want %q", m.worktrees[1].Output[0], "hello world")
		}
		// /a should be unchanged
		if len(m.worktrees[0].Output) != 0 {
			t.Errorf("worktree /a should have no output, got %d", len(m.worktrees[0].Output))
		}
	})

	t.Run("state_changed updates state", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		m.worktrees = []WorktreeState{
			{Dir: "/a"},
		}
		msg := socketEventMsg{
			worktreeDir: "/a",
			event: conductor.ConductorEvent{
				Type:  "state_changed",
				State: conductor.State("implementing"),
			},
		}
		m.handleSocketEvent(msg)
		if m.worktrees[0].State != "implementing" {
			t.Errorf("state = %q, want %q", m.worktrees[0].State, "implementing")
		}
	})

	t.Run("unknown worktree is ignored", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		m.worktrees = []WorktreeState{
			{Dir: "/a"},
		}
		msg := socketEventMsg{
			worktreeDir: "/unknown",
			event: conductor.ConductorEvent{
				Type:    "job_output",
				Message: "nope",
			},
		}
		m.handleSocketEvent(msg)
		if len(m.worktrees[0].Output) != 0 {
			t.Errorf("output should be empty, got %d lines", len(m.worktrees[0].Output))
		}
	})

	t.Run("multiple outputs accumulate", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		m.worktrees = []WorktreeState{
			{Dir: "/a"},
		}
		for _, line := range []string{"line1", "line2", "line3"} {
			msg := socketEventMsg{
				worktreeDir: "/a",
				event: conductor.ConductorEvent{
					Type:    "job_output",
					Message: line,
				},
			}
			m.handleSocketEvent(msg)
		}
		if len(m.worktrees[0].Output) != 3 {
			t.Fatalf("output lines = %d, want 3", len(m.worktrees[0].Output))
		}
		if m.worktrees[0].Output[2] != "line3" {
			t.Errorf("output[2] = %q, want %q", m.worktrees[0].Output[2], "line3")
		}
	})
}

func TestUpdateWindowSize(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	m.Update(msg)
	if !m.ready {
		t.Error("ready should be true after WindowSizeMsg")
	}
	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
	if m.output.Width() != 120 {
		t.Errorf("output.Width = %d, want 120", m.output.Width())
	}
	if m.chatInput.Width() != 118 {
		t.Errorf("chatInput.Width = %d, want 118", m.chatInput.Width())
	}
}

func TestUpdateDisconnectedMsg(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	m.worktrees = []WorktreeState{
		{Dir: "/a", State: "implementing"},
		{Dir: "/b", State: "planning"},
	}

	msg := disconnectedMsg{worktreeDir: "/a", err: errors.New("socket closed")}
	m.Update(msg)

	if m.worktrees[0].State != "disconnected" {
		t.Errorf("state = %q, want %q", m.worktrees[0].State, "disconnected")
	}
	// /b should be unchanged
	if m.worktrees[1].State != "planning" {
		t.Errorf("state = %q, want %q", m.worktrees[1].State, "planning")
	}
}

func TestUpdateDisconnectedMsgUnknown(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	m.worktrees = []WorktreeState{
		{Dir: "/a", State: "implementing"},
	}

	msg := disconnectedMsg{worktreeDir: "/unknown", err: errors.New("gone")}
	m.Update(msg)

	// Should not crash, state should be unchanged
	if m.worktrees[0].State != "implementing" {
		t.Errorf("state = %q, want %q", m.worktrees[0].State, "implementing")
	}
}

func TestUpdateErrMsg(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	testErr := errors.New("something broke")
	m.Update(errMsg{err: testErr})
	if !errors.Is(m.err, testErr) {
		t.Errorf("err = %v, want %v", m.err, testErr)
	}
}

func TestViewNotReady(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	v := m.View()
	if v.Content != "Loading...\n" {
		t.Errorf("View().Content = %q, want %q", v.Content, "Loading...\n")
	}
}

func TestViewShowsHelp(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	m.ready = true
	m.width = 80
	m.height = 24
	m.showHelp = true
	v := m.View()
	if !strings.Contains(v.Content, "Keybindings") {
		t.Error("help view should contain 'Keybindings'")
	}
	if !strings.Contains(v.Content, "q/ctrl+c") {
		t.Error("help view should contain 'q/ctrl+c'")
	}
	if !strings.Contains(v.Content, "tab/shift+tab") {
		t.Error("help view should contain 'tab/shift+tab'")
	}
}

func TestViewNoWorktrees(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	m.ready = true
	m.width = 80
	m.height = 24
	v := m.View()
	if !strings.Contains(v.Content, "no active tasks") {
		t.Error("View with no worktrees should contain 'no active tasks'")
	}
}

func TestViewWithWorktree(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	m.ready = true
	m.width = 80
	m.height = 24
	m.worktrees = []WorktreeState{
		{Dir: "/home/user/myproj", State: "implementing"},
	}
	v := m.View()
	if !strings.Contains(v.Content, "myproj") {
		t.Error("View should contain project name 'myproj'")
	}
	if !strings.Contains(v.Content, "implementing") {
		t.Error("View should contain state 'implementing'")
	}
}

func TestViewWithMultipleWorktrees(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	m.ready = true
	m.width = 80
	m.height = 24
	m.worktrees = []WorktreeState{
		{Dir: "/home/user/proj-a", State: "planning"},
		{Dir: "/home/user/proj-b", State: "implementing"},
	}
	m.active = 0
	v := m.View()
	// Tab bar should show both project names
	if !strings.Contains(v.Content, "proj-a") {
		t.Error("View should contain 'proj-a'")
	}
	if !strings.Contains(v.Content, "proj-b") {
		t.Error("View should contain 'proj-b'")
	}
}

func TestViewDashboardLayout(t *testing.T) {
	m := NewModel("/tmp", LayoutDashboard)
	m.ready = true
	m.width = 80
	m.height = 24
	m.worktrees = []WorktreeState{
		{
			Dir:   "/home/user/proj",
			State: "implementing",
			Workers: []WorkerInfo{
				{Name: "worker-1", State: "working", JobID: "job-42"},
				{Name: "worker-2", State: "available"},
			},
		},
	}
	v := m.View()
	if !strings.Contains(v.Content, "Workers") {
		t.Error("dashboard layout should show Workers section")
	}
	if !strings.Contains(v.Content, "worker-1") {
		t.Error("dashboard layout should show worker-1")
	}
	if !strings.Contains(v.Content, "worker-2") {
		t.Error("dashboard layout should show worker-2")
	}
}

func TestViewDashboardNoWorkers(t *testing.T) {
	m := NewModel("/tmp", LayoutDashboard)
	m.ready = true
	m.width = 80
	m.height = 24
	m.worktrees = []WorktreeState{
		{Dir: "/home/user/proj", State: "loaded"},
	}
	v := m.View()
	if !strings.Contains(v.Content, "no workers") {
		t.Error("dashboard with no workers should show '(no workers)'")
	}
}

func TestRenderStatusBarWithActiveWorker(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	m.width = 80
	m.worktrees = []WorktreeState{
		{
			Dir:   "/home/user/proj",
			State: "implementing",
			Workers: []WorkerInfo{
				{Name: "agent-claude", State: "working", JobID: "j1"},
			},
		},
	}
	bar := m.renderStatusBar()
	if !strings.Contains(bar, "agent-claude") {
		t.Error("status bar should show active worker name")
	}
}

func TestRenderStatusBarEmptyState(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	m.width = 80
	m.worktrees = []WorktreeState{
		{Dir: "/home/user/proj", State: ""},
	}
	bar := m.renderStatusBar()
	if !strings.Contains(bar, "none") {
		t.Error("status bar should show 'none' for empty state")
	}
}

func TestSyncViewport(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	m.worktrees = []WorktreeState{
		{Dir: "/a", Output: []string{"line1", "line2", "line3"}},
	}
	m.active = 0
	m.output.SetWidth(80)
	m.output.SetHeight(10)
	m.syncViewport()

	content := m.output.View()
	if !strings.Contains(content, "line1") {
		t.Error("viewport should contain 'line1'")
	}
}

func TestLayoutConstants(t *testing.T) {
	if LayoutStacked != 0 {
		t.Errorf("LayoutStacked = %d, want 0", LayoutStacked)
	}
	if LayoutDashboard != 1 {
		t.Errorf("LayoutDashboard = %d, want 1", LayoutDashboard)
	}
}

func TestWaitForMsg(t *testing.T) {
	t.Run("receives message from channel", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		go func() {
			m.msgs <- errMsg{err: errors.New("test")}
		}()
		cmd := waitForMsg(m.ctx, m.msgs)
		msg := cmd()
		if _, ok := msg.(errMsg); !ok {
			t.Errorf("expected errMsg, got %T", msg)
		}
	})

	t.Run("returns nil on context cancel", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		m.cancel()
		cmd := waitForMsg(m.ctx, m.msgs)
		msg := cmd()
		if msg != nil {
			t.Errorf("expected nil on canceled context, got %T", msg)
		}
	})

	t.Run("returns nil on closed channel", func(t *testing.T) {
		m := NewModel("/tmp", LayoutStacked)
		close(m.msgs)
		cmd := waitForMsg(m.ctx, m.msgs)
		msg := cmd()
		if msg != nil {
			t.Errorf("expected nil on closed channel, got %T", msg)
		}
	})
}

func TestUpdateConnectedMsg(t *testing.T) {
	m := NewModel("/tmp", LayoutStacked)
	m.worktrees = []WorktreeState{{Dir: "/a", State: "loaded"}}

	_, cmd := m.Update(connectedMsg{worktreeDir: "/a"})
	// connectedMsg should not change state and return nil cmd
	if cmd != nil {
		t.Error("connectedMsg should return nil cmd")
	}
	if m.worktrees[0].State != "loaded" {
		t.Errorf("state should remain 'loaded', got %q", m.worktrees[0].State)
	}
}
