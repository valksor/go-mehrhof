package socket

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/testutil"
)

// TestWorktreeSocketEndToEnd builds a fully-wired worktree socket via
// NewWorktreeSocket (exercising registerHandlers + setupEventForwarding), runs
// it against a live global socket (exercising Start + registerWithGlobal), and
// drives several RPCs over a real connection before a clean Stop.
func TestWorktreeSocketEndToEnd(t *testing.T) {
	repoDir := testutil.TempDir(t)
	testutil.InitGitRepo(t, repoDir)

	// Live global socket so registerWithGlobal has something to register against.
	globalPath := testutil.TempSocketPath(t)
	global := NewGlobalSocket(globalPath)
	gctx, gcancel := context.WithCancel(context.Background())
	t.Cleanup(gcancel)
	go func() { _ = global.Start(gctx) }()
	waitForSocket(t.Context(), t, globalPath)
	t.Cleanup(func() { _ = global.Stop() })

	wtSockPath := filepath.Join(testutil.TempDir(t), "wt.sock")
	wt, err := NewWorktreeSocket(WorktreeConfig{
		WorktreePath: repoDir,
		SocketPath:   wtSockPath,
		GlobalPath:   globalPath,
	})
	if err != nil {
		t.Fatalf("NewWorktreeSocket: %v", err)
	}
	// Keep Stop's drain short.
	wt.server.drainTimeout = 200 * time.Millisecond

	wctx, wcancel := context.WithCancel(context.Background())
	t.Cleanup(wcancel)
	go func() { _ = wt.Start(wctx) }()
	waitForSocket(t.Context(), t, wtSockPath)
	t.Cleanup(func() { _ = wt.Stop() })

	client, err := NewClient(wtSockPath, WithRetry(10, 20*time.Millisecond, 500*time.Millisecond))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	t.Run("ping", func(t *testing.T) {
		resp, err := client.Call(ctx, "ping", nil)
		if err != nil {
			t.Fatalf("ping: %v", err)
		}
		var result map[string]string
		_ = json.Unmarshal(resp.Result, &result)
		if result["status"] != "ok" {
			t.Errorf("ping status = %q, want ok", result["status"])
		}
	})

	t.Run("status", func(t *testing.T) {
		resp, err := client.Call(ctx, "status", nil)
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		var status StatusResult
		if err := json.Unmarshal(resp.Result, &status); err != nil {
			t.Fatalf("unmarshal status: %v", err)
		}
		if status.State != StateNone {
			t.Errorf("state = %q, want none", status.State)
		}
	})

	t.Run("strategy.list", func(t *testing.T) {
		resp, err := client.Call(ctx, "strategy.list", nil)
		if err != nil {
			t.Fatalf("strategy.list: %v", err)
		}
		var strategies []any
		_ = json.Unmarshal(resp.Result, &strategies)
		if len(strategies) == 0 {
			t.Error("expected at least one strategy")
		}
	})

	t.Run("git.status", func(t *testing.T) {
		resp, err := client.Call(ctx, "git.status", nil)
		if err != nil {
			t.Fatalf("git.status: %v", err)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal git.status: %v", err)
		}
		if _, ok := result["branch"]; !ok {
			t.Error("expected branch in git.status result")
		}
	})

	t.Run("checkpoints", func(t *testing.T) {
		resp, err := client.Call(ctx, "checkpoints", nil)
		if err != nil {
			t.Fatalf("checkpoints: %v", err)
		}
		if resp.Result == nil {
			t.Error("expected non-nil checkpoints result")
		}
	})

	// The global socket should have learned about this worktree via
	// registerWithGlobal. Allow a brief moment for the async registration.
	t.Run("registered with global", func(t *testing.T) {
		gclient, err := NewClient(globalPath)
		if err != nil {
			t.Fatalf("global client: %v", err)
		}
		defer func() { _ = gclient.Close() }()

		var found bool
		for range 20 {
			resp, callErr := gclient.Call(ctx, "projects.list", nil)
			if callErr == nil {
				var list ProjectListResult
				if json.Unmarshal(resp.Result, &list) == nil {
					for _, p := range list.Projects {
						if p.Path == wt.path {
							found = true

							break
						}
					}
				}
			}
			if found {
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
		if !found {
			t.Error("worktree was not registered with the global socket")
		}
	})
}

// TestWorktreeStreamSubscribeAndEmit drives the streaming path: it subscribes
// over a raw connection, then emits an event and reads it back.
func TestWorktreeStreamSubscribeAndEmit(t *testing.T) {
	repoDir := testutil.TempDir(t)
	testutil.InitGitRepo(t, repoDir)

	wtSockPath := filepath.Join(testutil.TempDir(t), "wt.sock")
	wt, err := NewWorktreeSocket(WorktreeConfig{
		WorktreePath: repoDir,
		SocketPath:   wtSockPath,
		GlobalPath:   filepath.Join(testutil.TempDir(t), "nonexistent-global.sock"),
	})
	if err != nil {
		t.Fatalf("NewWorktreeSocket: %v", err)
	}
	wt.server.drainTimeout = 200 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = wt.Start(ctx) }()
	waitForSocket(ctx, t, wtSockPath)
	t.Cleanup(func() { _ = wt.Stop() })

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", wtSockPath)
	if err != nil {
		t.Fatalf("dial worktree: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Subscribe.
	subReq := &Request{ID: "1", Method: "stream.subscribe"}
	data, _ := EncodeRequest(subReq)
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	reader := bufio.NewReader(conn)
	if _, err := reader.ReadBytes('\n'); err != nil {
		t.Fatalf("read subscribe response: %v", err)
	}

	// The subscriber channel registers server-side after the subscribe response,
	// so re-emit on a short cadence until the event is delivered. This avoids a
	// fixed sleep that can race a loaded CI scheduler.
	overall := time.Now().Add(3 * time.Second)
	var line []byte
	for {
		wt.emitEvent("test_event", map[string]string{"hello": "world"})
		_ = conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
		b, err := reader.ReadBytes('\n')
		if err == nil {
			line = b

			break
		}
		if time.Now().After(overall) {
			t.Fatalf("read emitted event: %v", err)
		}
	}
	var event map[string]any
	if err := json.Unmarshal(line, &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event["type"] != "test_event" {
		t.Errorf("event type = %v, want test_event", event["type"])
	}
}

// waitForSocket blocks until a Unix socket at path accepts a connection.
func waitForSocket(ctx context.Context, t *testing.T, path string) {
	t.Helper()
	var dialer net.Dialer
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := dialer.DialContext(ctx, "unix", path)
		if err == nil {
			_ = c.Close()

			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("socket %s did not become reachable", path)
}
