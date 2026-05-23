package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/internal/testutil"
)

// setupKvelmoHome points KVELMO_HOME at a short temp dir and creates the
// worktrees subdirectory so socket.WorktreeSocketPath/GlobalSocketPath resolve
// to bindable paths.
func setupKvelmoHome(t *testing.T) {
	t.Helper()
	home := testutil.TempDir(t)
	t.Setenv("KVELMO_HOME", home)
	if err := socket.EnsureDir(); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
}

// serveWorktree starts a stub server on the socket path that the TUI will
// resolve for worktreeDir.
func serveWorktree(t *testing.T, worktreeDir string) *stubServer {
	t.Helper()

	return newStubServerAt(t, socket.WorktreeSocketPath(worktreeDir))
}

// serveGlobal starts a stub server on the global socket path.
func serveGlobal(t *testing.T) *stubServer {
	t.Helper()

	return newStubServerAt(t, socket.GlobalSocketPath())
}

func TestExecuteWorktreeCommand(t *testing.T) {
	setupKvelmoHome(t)
	dir := "/proj/wt1"

	t.Run("known command", func(t *testing.T) {
		s := serveWorktree(t, dir)
		s.on("status", map[string]any{"state": "loaded"})
		out, err := executeWorktreeCommand(context.Background(), dir, "/status", "", false)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if out != "State: loaded" {
			t.Errorf("out = %q", out)
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		serveWorktree(t, dir)
		out, err := executeWorktreeCommand(context.Background(), dir, "/nope", "", false)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if out != "Unknown command: /nope" {
			t.Errorf("out = %q", out)
		}
	})

	t.Run("connect failure", func(t *testing.T) {
		// No server listening on this worktree's path.
		_, err := executeWorktreeCommand(context.Background(), "/proj/no-server", "/status", "", false)
		if err == nil {
			t.Fatal("expected connect error, got nil")
		}
		if !strings.Contains(err.Error(), "connect") {
			t.Errorf("err = %v, want connect failure", err)
		}
	})
}

func TestExecuteGlobalCommand(t *testing.T) {
	t.Run("known command", func(t *testing.T) {
		setupKvelmoHome(t)
		s := serveGlobal(t)
		s.on("jobs.list", map[string]any{"jobs": []any{}})
		out, err := executeGlobalCommand(context.Background(), "/jobs", "")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if out != "No jobs." {
			t.Errorf("out = %q", out)
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		setupKvelmoHome(t)
		serveGlobal(t)
		out, err := executeGlobalCommand(context.Background(), "/nope", "")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if out != "Unknown global command: /nope" {
			t.Errorf("out = %q", out)
		}
	})

	t.Run("connect failure", func(t *testing.T) {
		setupKvelmoHome(t)
		// No global server listening.
		_, err := executeGlobalCommand(context.Background(), "/jobs", "")
		if err == nil {
			t.Fatal("expected connect error, got nil")
		}
		if !strings.Contains(err.Error(), "global socket") {
			t.Errorf("err = %v, want global socket failure", err)
		}
	})
}

func TestExecuteCommand(t *testing.T) {
	t.Run("global path", func(t *testing.T) {
		setupKvelmoHome(t)
		s := serveGlobal(t)
		s.on("metrics", map[string]any{"jobs": 1})
		m := NewModel("/proj", LayoutStacked)
		cmd, _ := parseSlashCommand("/stats")
		teaCmd := m.executeCommand(cmd, "")
		msg := teaCmd()
		res, ok := msg.(commandResultMsg)
		if !ok {
			t.Fatalf("msg type = %T", msg)
		}
		if !strings.Contains(res.output, "jobs") {
			t.Errorf("output = %q", res.output)
		}
	})

	t.Run("global error wrapped", func(t *testing.T) {
		setupKvelmoHome(t)
		// No server -> connect error surfaced as "Error: ..." output.
		m := NewModel("/proj", LayoutStacked)
		cmd, _ := parseSlashCommand("/stats")
		teaCmd := m.executeCommand(cmd, "")
		res, ok := teaCmd().(commandResultMsg)
		if !ok {
			t.Fatalf("type = %T", teaCmd())
		}
		if !strings.HasPrefix(res.output, "Error:") {
			t.Errorf("output = %q, want Error prefix", res.output)
		}
	})

	t.Run("worktree path", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/wtX"
		s := serveWorktree(t, dir)
		s.on("status", map[string]any{"state": "planning", "title": "T"})
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: dir}}
		cmd, _ := parseSlashCommand("/status")
		res, ok := m.executeCommand(cmd, "")().(commandResultMsg)
		if !ok {
			t.Fatal("wrong msg type")
		}
		if !strings.Contains(res.output, "planning") {
			t.Errorf("output = %q", res.output)
		}
	})

	t.Run("worktree command with no active worktree", func(t *testing.T) {
		setupKvelmoHome(t)
		m := NewModel("/proj", LayoutStacked)
		cmd, _ := parseSlashCommand("/status")
		res, ok := m.executeCommand(cmd, "")().(commandResultMsg)
		if !ok {
			t.Fatal("wrong msg type")
		}
		if res.output != "No active worktree." {
			t.Errorf("output = %q", res.output)
		}
	})

	t.Run("worktree error wrapped", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/no-srv"
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: dir}}
		cmd, _ := parseSlashCommand("/status")
		res, ok := m.executeCommand(cmd, "")().(commandResultMsg)
		if !ok {
			t.Fatal("wrong msg type")
		}
		if !strings.HasPrefix(res.output, "Error:") {
			t.Errorf("output = %q, want Error prefix", res.output)
		}
	})
}

func TestModelSendChatMessage(t *testing.T) {
	t.Run("nil when no worktree", func(t *testing.T) {
		m := NewModel("/proj", LayoutStacked)
		if cmd := m.sendChatMessage("hi"); cmd != nil {
			t.Error("expected nil cmd with no worktree")
		}
	})
	t.Run("sends and returns nil msg", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/chat"
		s := serveWorktree(t, dir)
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: dir}}
		msg := m.sendChatMessage("hello")()
		if msg != nil {
			t.Errorf("expected nil msg on success, got %T %v", msg, msg)
		}
		if c := s.calls(); len(c) != 1 || c[0].method != "chat.send" {
			t.Errorf("calls = %+v", c)
		}
	})
	t.Run("error on connect failure", func(t *testing.T) {
		setupKvelmoHome(t)
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: "/proj/no-srv"}}
		msg := m.sendChatMessage("hello")()
		if _, ok := msg.(errMsg); !ok {
			t.Errorf("expected errMsg, got %T", msg)
		}
	})
}

func TestModelSendStartTask(t *testing.T) {
	t.Run("nil when no worktree", func(t *testing.T) {
		m := NewModel("/proj", LayoutStacked)
		if cmd := m.sendStartTask("desc"); cmd != nil {
			t.Error("expected nil cmd")
		}
	})
	t.Run("sends start", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/start"
		s := serveWorktree(t, dir)
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: dir}}
		if msg := m.sendStartTask("build it")(); msg != nil {
			t.Errorf("expected nil msg, got %T", msg)
		}
		if c := s.calls(); len(c) != 1 || c[0].method != "start" {
			t.Errorf("calls = %+v", c)
		}
	})
	t.Run("error on connect failure", func(t *testing.T) {
		setupKvelmoHome(t)
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: "/proj/no-srv"}}
		if _, ok := m.sendStartTask("x")().(errMsg); !ok {
			t.Error("expected errMsg")
		}
	})
}

func TestModelSendWorkflowCmd(t *testing.T) {
	t.Run("nil when no worktree", func(t *testing.T) {
		m := NewModel("/proj", LayoutStacked)
		if cmd := m.sendWorkflowCmd("plan"); cmd != nil {
			t.Error("expected nil cmd")
		}
	})
	t.Run("sends without dry-run", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/wf"
		s := serveWorktree(t, dir)
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: dir}}
		if msg := m.sendWorkflowCmd(phasePlan)(); msg != nil {
			t.Errorf("expected nil, got %T", msg)
		}
		if c := s.calls(); len(c) != 1 || c[0].method != phasePlan {
			t.Errorf("calls = %+v", c)
		}
	})
	t.Run("sends with dry-run params", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/wf2"
		s := serveWorktree(t, dir)
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: dir}}
		m.dryRun = true
		m.sendWorkflowCmd(phaseSubmit)()
		c := s.calls()
		if len(c) != 1 {
			t.Fatalf("calls = %+v", c)
		}
		if !strings.Contains(string(c[0].params), "dry_run") {
			t.Errorf("params = %s, want dry_run", c[0].params)
		}
	})
	t.Run("error on connect failure", func(t *testing.T) {
		setupKvelmoHome(t)
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: "/proj/no-srv"}}
		if _, ok := m.sendWorkflowCmd("plan")().(errMsg); !ok {
			t.Error("expected errMsg")
		}
	})
}

func TestModelFetchSpec(t *testing.T) {
	t.Run("nil when no worktree", func(t *testing.T) {
		m := NewModel("/proj", LayoutStacked)
		if cmd := m.fetchSpec(); cmd != nil {
			t.Error("expected nil cmd")
		}
	})
	t.Run("returns spec content", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/spec"
		s := serveWorktree(t, dir)
		s.on("show.spec", map[string]any{"specifications": []map[string]any{{"content": "Spec A"}, {"content": "Spec B"}}})
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: dir}}
		msg := m.fetchSpec()()
		res, ok := msg.(specResultMsg)
		if !ok {
			t.Fatalf("type = %T", msg)
		}
		if !strings.Contains(res.content, "Spec A") || !strings.Contains(res.content, "Spec B") {
			t.Errorf("content = %q", res.content)
		}
	})
	t.Run("empty spec yields placeholder", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/spec2"
		s := serveWorktree(t, dir)
		s.on("show.spec", map[string]any{"specifications": []any{}})
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: dir}}
		res, ok := m.fetchSpec()().(specResultMsg)
		if !ok {
			t.Fatal("wrong msg type")
		}
		if res.content != "No specification available." {
			t.Errorf("content = %q", res.content)
		}
	})
	t.Run("error on connect failure", func(t *testing.T) {
		setupKvelmoHome(t)
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: "/proj/no-srv"}}
		if _, ok := m.fetchSpec()().(errMsg); !ok {
			t.Error("expected errMsg")
		}
	})
}

func TestModelFetchChangelog(t *testing.T) {
	t.Run("nil when no worktree", func(t *testing.T) {
		m := NewModel("/proj", LayoutStacked)
		if cmd := m.fetchChangelog("v1..v2", false); cmd != nil {
			t.Error("expected nil cmd")
		}
	})
	t.Run("invalid range", func(t *testing.T) {
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: "/proj/cl"}}
		msg := m.fetchChangelog("badrange", false)()
		if _, ok := msg.(errMsg); !ok {
			t.Errorf("expected errMsg for bad range, got %T", msg)
		}
	})
	t.Run("returns markdown", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/cl2"
		s := serveWorktree(t, dir)
		s.on("changelog.generate", map[string]any{"markdown": "## Log"})
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: dir}}
		res, ok := m.fetchChangelog("v1..v2 a note", true)().(changelogResultMsg)
		if !ok {
			t.Fatal("wrong type")
		}
		if res.content != "## Log" {
			t.Errorf("content = %q", res.content)
		}
		if c := s.calls(); len(c) != 1 || !strings.Contains(string(c[0].params), "note") {
			t.Errorf("expected note in params: %+v", c)
		}
	})
	t.Run("empty markdown placeholder", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/cl3"
		s := serveWorktree(t, dir)
		s.on("changelog.generate", map[string]any{})
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: dir}}
		res, ok := m.fetchChangelog("v1..v2", false)().(changelogResultMsg)
		if !ok {
			t.Fatal("wrong msg type")
		}
		if res.content != "No commits between v1 and v2" {
			t.Errorf("content = %q", res.content)
		}
	})
	t.Run("error on connect failure", func(t *testing.T) {
		setupKvelmoHome(t)
		m := NewModel("/proj", LayoutStacked)
		m.worktrees = []WorktreeState{{Dir: "/proj/no-srv"}}
		if _, ok := m.fetchChangelog("v1..v2", false)().(errMsg); !ok {
			t.Error("expected errMsg")
		}
	})
}

// TestEnterDispatchesSlashCommand verifies the Enter key path routes a slash
// command through executeCommand and produces a commandResultMsg.
func TestEnterDispatchesSlashCommand(t *testing.T) {
	setupKvelmoHome(t)
	s := serveGlobal(t)
	s.on("metrics", map[string]any{"ok": true})
	m := NewModel("/proj", LayoutStacked)
	m.chatInput.SetValue("/stats")
	_, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command from enter+slash")
	}
	if _, ok := cmd().(commandResultMsg); !ok {
		t.Errorf("expected commandResultMsg, got %T", cmd())
	}
	if m.chatInput.Value() != "" {
		t.Error("chat input should be cleared after enter")
	}
}
