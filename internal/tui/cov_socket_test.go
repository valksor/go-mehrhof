package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/internal/testutil"
)

func TestGitRoot(t *testing.T) {
	t.Run("returns repo root", func(t *testing.T) {
		dir := testutil.TempDir(t)
		testutil.InitGitRepo(t, dir)
		got := gitRoot(context.Background(), dir)
		if got == "" {
			t.Fatal("gitRoot returned empty for a real repo")
		}
		// The returned path must be a git toplevel; running git in it should agree.
		sub := gitRoot(context.Background(), got)
		if sub != got {
			t.Errorf("root of root = %q, want stable %q", sub, got)
		}
	})

	t.Run("non-git dir returns input", func(t *testing.T) {
		dir := testutil.TempDir(t)
		got := gitRoot(context.Background(), dir)
		if got != dir {
			t.Errorf("gitRoot(non-git) = %q, want %q", got, dir)
		}
	})
}

func TestDiscoverWorktrees(t *testing.T) {
	t.Run("no global socket falls back to cwd", func(t *testing.T) {
		setupKvelmoHome(t)
		// No global server running.
		msg := discoverWorktrees("/proj/x")()
		list, ok := msg.(worktreeListMsg)
		if !ok {
			t.Fatalf("type = %T", msg)
		}
		if len(list.dirs) != 1 || list.dirs[0] != "/proj/x" {
			t.Errorf("dirs = %v, want [/proj/x]", list.dirs)
		}
	})

	t.Run("global socket present but task paths differ from cwd root", func(t *testing.T) {
		setupKvelmoHome(t)
		s := serveGlobal(t)
		// tasks.list returns a task whose path is a non-git dir → its gitRoot is
		// itself, which won't match cwd's gitRoot, so falls back to [cwd].
		s.on("tasks.list", map[string]any{"tasks": []map[string]any{{"path": "/some/other/dir"}}})
		list, ok := discoverWorktrees("/proj/cwd")().(worktreeListMsg)
		if !ok {
			t.Fatal("expected worktreeListMsg")
		}
		if len(list.dirs) != 1 || list.dirs[0] != "/proj/cwd" {
			t.Errorf("dirs = %v, want fallback [/proj/cwd]", list.dirs)
		}
	})

	t.Run("global socket present matching cwd root", func(t *testing.T) {
		setupKvelmoHome(t)
		// Create a real git repo so cwd and the task path share a git root.
		repo := testutil.TempDir(t)
		testutil.InitGitRepo(t, repo)
		s := serveGlobal(t)
		s.on("tasks.list", map[string]any{"tasks": []map[string]any{{"path": repo}}})
		list, ok := discoverWorktrees(repo)().(worktreeListMsg)
		if !ok {
			t.Fatal("expected worktreeListMsg")
		}
		if len(list.dirs) != 1 {
			t.Fatalf("dirs = %v", list.dirs)
		}
		if gitRoot(context.Background(), list.dirs[0]) != gitRoot(context.Background(), repo) {
			t.Errorf("dir %q does not share root with %q", list.dirs[0], repo)
		}
	})

	t.Run("tasks with empty path are skipped", func(t *testing.T) {
		setupKvelmoHome(t)
		s := serveGlobal(t)
		s.on("tasks.list", map[string]any{"tasks": []map[string]any{{"path": ""}}})
		list, ok := discoverWorktrees("/proj/cwd")().(worktreeListMsg)
		if !ok {
			t.Fatal("expected worktreeListMsg")
		}
		if len(list.dirs) != 1 || list.dirs[0] != "/proj/cwd" {
			t.Errorf("dirs = %v, want fallback", list.dirs)
		}
	})
}

func TestSubscribeWorktree(t *testing.T) {
	t.Run("no socket returns disconnected", func(t *testing.T) {
		setupKvelmoHome(t)
		ch := make(chan tea.Msg, 4)
		msg := subscribeWorktree(context.Background(), "/proj/no-sock", ch)()
		dc, ok := msg.(disconnectedMsg)
		if !ok {
			t.Fatalf("type = %T", msg)
		}
		if dc.worktreeDir != "/proj/no-sock" {
			t.Errorf("dir = %q", dc.worktreeDir)
		}
	})

	t.Run("subscribe error response disconnects", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/sub-err"
		path := socket.WorktreeSocketPath(dir)
		ln := mustListen(t, path)
		// Server responds to stream.subscribe with an RPC error.
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
			sc := bufio.NewScanner(conn)
			if !sc.Scan() {
				return
			}
			var req socket.Request
			_ = json.Unmarshal(sc.Bytes(), &req)
			resp := socket.NewErrorResponse(req.ID, socket.ErrCodeInternal, "no stream")
			out, _ := socket.EncodeResponse(resp)
			_, _ = conn.Write(out)
		}()

		ch := make(chan tea.Msg, 4)
		msg := subscribeWorktree(context.Background(), dir, ch)()
		if _, ok := msg.(disconnectedMsg); !ok {
			t.Fatalf("expected disconnectedMsg on subscribe error, got %T", msg)
		}
	})

	t.Run("successful subscribe streams events", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/sub-ok"
		path := socket.WorktreeSocketPath(dir)
		ln := mustListen(t, path)
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
			sc := bufio.NewScanner(conn)
			if !sc.Scan() {
				return
			}
			var req socket.Request
			_ = json.Unmarshal(sc.Bytes(), &req)
			// OK response to subscribe.
			ok := &socket.Response{ID: req.ID, Result: json.RawMessage(`{}`)}
			out, _ := socket.EncodeResponse(ok)
			_, _ = conn.Write(out)
			// Then stream one event line (NDJSON, no id).
			ev, _ := json.Marshal(conductor.ConductorEvent{Type: "job_output", Message: "streamed"})
			_, _ = conn.Write(append(ev, '\n'))
			// Hold the connection open briefly so the reader goroutine processes.
			time.Sleep(50 * time.Millisecond)
		}()

		ctx := t.Context()
		ch := make(chan tea.Msg, 4)
		msg := subscribeWorktree(ctx, dir, ch)()
		if _, ok := msg.(connectedMsg); !ok {
			t.Fatalf("expected connectedMsg, got %T", msg)
		}
		// Drain the streamed event from the channel.
		select {
		case got := <-ch:
			ev, ok := got.(socketEventMsg)
			if !ok {
				t.Fatalf("channel msg type = %T", got)
			}
			if ev.event.Message != "streamed" {
				t.Errorf("event message = %q", ev.event.Message)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for streamed event")
		}
	})
}

func TestStartProgressPolling(t *testing.T) {
	t.Run("polls and reports progress for active state", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/prog"
		path := socket.WorktreeSocketPath(dir)
		ln := mustListen(t, path)
		// Server answers status.get with an active state, then progress.get.
		go progressServer(ln, "implementing", map[string]any{
			"active":      true,
			"percent":     50.0,
			"eta_seconds": 30,
			"calibrated":  true,
		})

		ctx := t.Context()
		ch := make(chan tea.Msg, 4)
		// Speed up: the production ticker is 3s. We just wait for one tick.
		startProgressPolling(ctx, dir, ch)
		select {
		case got := <-ch:
			pm, ok := got.(progressMsg)
			if !ok {
				t.Fatalf("type = %T", got)
			}
			if !pm.active || pm.percent != 50.0 || pm.etaSeconds != 30 {
				t.Errorf("progress = %+v", pm)
			}
		case <-time.After(6 * time.Second):
			t.Fatal("timed out waiting for progress msg")
		}
	})

	t.Run("skips polling for inactive state", func(t *testing.T) {
		setupKvelmoHome(t)
		dir := "/proj/prog-inactive"
		path := socket.WorktreeSocketPath(dir)
		ln := mustListen(t, path)
		// status.get returns a terminal state -> progress.get should not be called,
		// and no progressMsg should be emitted.
		go progressServer(ln, "submitted", nil)

		ctx := t.Context()
		ch := make(chan tea.Msg, 4)
		startProgressPolling(ctx, dir, ch)
		select {
		case got := <-ch:
			t.Fatalf("expected no progress msg for inactive state, got %T", got)
		case <-time.After(4 * time.Second):
			// Expected: nothing emitted within one tick window.
		}
	})
}

// mustListen binds a unix socket at path and registers cleanup.
func mustListen(t *testing.T, path string) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatalf("listen %s: %v", path, err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	return ln
}

// progressServer accepts connections and answers status.get with the given
// state and progress.get with the given payload (if non-nil).
func progressServer(ln net.Listener, state string, progress map[string]any) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer func() { _ = c.Close() }()
			sc := bufio.NewScanner(c)
			for sc.Scan() {
				var req socket.Request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				var result any
				switch req.Method {
				case "status.get":
					result = map[string]any{"state": state}
				case "progress.get":
					result = progress
				default:
					result = map[string]any{}
				}
				resp, _ := socket.NewResultResponse(req.ID, result)
				out, _ := socket.EncodeResponse(resp)
				if _, err := c.Write(out); err != nil {
					return
				}
			}
		}(conn)
	}
}
