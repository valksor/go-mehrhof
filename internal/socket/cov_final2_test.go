package socket

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/activitylog"
	"github.com/valksor/kvelmo/internal/testutil"
	"github.com/valksor/kvelmo/metrics"
)

// --- global_metrics.go: handleMetricsHistory with a real store ---

func TestGlobalHandleMetricsHistory_WithStore(t *testing.T) {
	ctx := context.Background()

	prev := timeSeriesStore
	ts := metrics.NewTimeSeriesStore(metrics.New(), testutil.TempDir(t), time.Hour, 7)
	SetTimeSeriesStore(ts)
	t.Cleanup(func() { timeSeriesStore = prev })

	g := newTestGlobalSocket(t)

	t.Run("invalid params", func(t *testing.T) {
		resp, err := g.handleMetricsHistory(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleMetricsHistory() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("invalid from time", func(t *testing.T) {
		params, _ := json.Marshal(map[string]string{"from": "not-a-time"})
		resp, err := g.handleMetricsHistory(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMetricsHistory() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid from time")
		}
	})

	t.Run("invalid to time", func(t *testing.T) {
		params, _ := json.Marshal(map[string]string{"to": "not-a-time"})
		resp, err := g.handleMetricsHistory(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMetricsHistory() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid to time")
		}
	})

	t.Run("valid query with default window", func(t *testing.T) {
		resp, err := g.handleMetricsHistory(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleMetricsHistory() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if enabled, _ := result["enabled"].(bool); !enabled {
			t.Error("enabled = false, want true with a configured store")
		}
	})

	t.Run("explicit from and to", func(t *testing.T) {
		from := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
		to := time.Now().Format(time.RFC3339)
		params, _ := json.Marshal(map[string]string{"from": from, "to": to})
		resp, err := g.handleMetricsHistory(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMetricsHistory() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
	})
}

// --- global_health.go: pingWorktree against a live socket ---

func TestGlobalPingWorktree_Reachable(t *testing.T) {
	ctx := context.Background()

	repoDir := testutil.TempDir(t)
	testutil.InitGitRepo(t, repoDir)
	wtSockPath := filepath.Join(testutil.TempDir(t), "wt.sock")
	wt := NewWorktreeSocketSimple(wtSockPath, repoDir)
	wt.server.drainTimeout = 200 * time.Millisecond
	wctx, wcancel := context.WithCancel(context.Background())
	t.Cleanup(wcancel)
	go func() { _ = wt.Start(wctx) }()
	waitForSocket(ctx, t, wtSockPath)
	t.Cleanup(func() { _ = wt.Stop() })

	g := newTestGlobalSocket(t)
	if !g.pingWorktree(ctx, wtSockPath) {
		t.Error("pingWorktree() on a live socket should return true")
	}
}

// --- server.go: CleanupStaleSocket branches ---

func TestCleanupStaleSocket_Branches(t *testing.T) {
	t.Run("missing path is not stale", func(t *testing.T) {
		removed, err := CleanupStaleSocket(filepath.Join(testutil.TempDir(t), "missing.sock"))
		if err != nil {
			t.Fatalf("CleanupStaleSocket() error = %v", err)
		}
		if removed {
			t.Error("missing path should not be reported as removed")
		}
	})

	t.Run("regular file is removed", func(t *testing.T) {
		p := filepath.Join(testutil.TempDir(t), "notasocket")
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		removed, err := CleanupStaleSocket(p)
		if err != nil {
			t.Fatalf("CleanupStaleSocket() error = %v", err)
		}
		if !removed {
			t.Error("regular file should be removed and reported")
		}
	})

	t.Run("live socket is not stale", func(t *testing.T) {
		p := filepath.Join(testutil.TempDir(t), "live.sock")
		var lc net.ListenConfig
		ln, err := lc.Listen(t.Context(), "unix", p)
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer func() { _ = ln.Close() }()
		// Accept connections so the dial in CleanupStaleSocket succeeds.
		go func() {
			for {
				c, acceptErr := ln.Accept()
				if acceptErr != nil {
					return
				}
				_ = c.Close()
			}
		}()
		removed, err := CleanupStaleSocket(p)
		if err != nil {
			t.Fatalf("CleanupStaleSocket() error = %v", err)
		}
		if removed {
			t.Error("a live socket should not be removed")
		}
	})
}

// --- global_export.go: queryWorktreeCheckpoints against a live socket ---

func TestGlobalExport_ReachableWorktreeCheckpoints(t *testing.T) {
	ctx := context.Background()

	repoDir := testutil.TempDir(t)
	testutil.InitGitRepo(t, repoDir)
	wtSockPath := filepath.Join(testutil.TempDir(t), "wt.sock")
	wt := NewWorktreeSocketSimple(wtSockPath, repoDir)
	wt.server.drainTimeout = 200 * time.Millisecond
	wctx, wcancel := context.WithCancel(context.Background())
	t.Cleanup(wcancel)
	go func() { _ = wt.Start(wctx) }()
	waitForSocket(ctx, t, wtSockPath)
	t.Cleanup(func() { _ = wt.Stop() })

	g := newTestGlobalSocket(t)
	g.mu.Lock()
	g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: repoDir, State: "loaded", SocketPath: wtSockPath}
	g.mu.Unlock()

	resp, err := g.handleExport(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleExport() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["tasks"]; !ok {
		t.Error("expected tasks in export result")
	}
}

// --- worktree_stream.go: replay path on subscribe with last_seq ---

func TestWorktreeStreamSubscribe_Replay(t *testing.T) {
	repoDir := testutil.TempDir(t)
	testutil.InitGitRepo(t, repoDir)
	wtSockPath := filepath.Join(testutil.TempDir(t), "wt.sock")
	wt, err := NewWorktreeSocket(WorktreeConfig{
		WorktreePath: repoDir,
		SocketPath:   wtSockPath,
		GlobalPath:   filepath.Join(testutil.TempDir(t), "nonexistent.sock"),
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

	// Emit an event first so the replay buffer has a seq>0 entry to replay.
	wt.emitEvent("pre_event", map[string]string{"a": "1"})

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", wtSockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Subscribe with last_seq=0 — anything buffered with seq>0 replays.
	subReq := &Request{ID: "1", Method: "stream.subscribe", Params: json.RawMessage(`{"last_seq":0}`)}
	data, _ := EncodeRequest(subReq)
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	reader := bufio.NewReader(conn)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	// First line is the subscribe response; subsequent lines may be replayed
	// events. We just confirm the handshake succeeded.
	if _, err := reader.ReadBytes('\n'); err != nil {
		t.Fatalf("read subscribe response: %v", err)
	}
}

// --- global_activity.go: activityLogAdapter.Record ---

func TestActivityLogAdapterRecord(t *testing.T) {
	log, err := activitylog.New(testutil.TempDir(t), 3)
	if err != nil {
		t.Fatalf("activitylog.New: %v", err)
	}
	// Run the background writer so the recorded entry is persisted, then query
	// it back to confirm the adapter forwards the full entry.
	ctx, cancel := context.WithCancel(context.Background())
	go log.Start(ctx)

	adapter := &activityLogAdapter{log: log}
	adapter.Record(ActivityEntry{Method: "ping", DurationMs: 5, CorrelationID: "abc"})

	var entries []activitylog.Entry
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries, err = log.Query(activitylog.QueryOptions{})
		if err == nil && len(entries) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if len(entries) == 0 {
		t.Fatal("expected recorded entry to be persisted")
	}
	if entries[0].Method != "ping" {
		t.Errorf("recorded method = %q, want ping", entries[0].Method)
	}
}
