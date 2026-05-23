package socket

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/internal/testutil"
	"github.com/valksor/kvelmo/settings"
)

// gitWorktreeWithConductor builds a worktree socket whose conductor is wired to
// a real git repository, so lifecycle transitions that need git (start) can run.
func gitWorktreeWithConductor(t *testing.T) *WorktreeSocket {
	t.Helper()
	dir := testutil.TempDir(t)
	testutil.InitGitRepo(t, dir)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}
	providers := provider.NewRegistry(settings.DefaultSettings())
	cond := conductor.NewConductor(conductor.ConductorConfig{
		Repo:         repo,
		Providers:    providers,
		WorktreePath: dir,
	})
	w := &WorktreeSocket{
		server:    NewServer(""),
		path:      dir,
		conductor: cond,
		repo:      repo,
		streams:   make(map[string]chan []byte),
	}

	return w
}

// --- worktree_phase.go: handleStart success path ---

func TestWorktreeHandleStart_Success(t *testing.T) {
	ctx := context.Background()
	w := gitWorktreeWithConductor(t)

	params, _ := json.Marshal(StartParams{Source: "empty:Fix the login button"}) //nolint:errchkjson // test data
	resp, err := w.handleStart(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleStart() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleStart() returned error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["status"] != "started" {
		t.Errorf("status = %v, want started", result["status"])
	}
	if w.conductor.State() != conductor.StateLoaded {
		t.Errorf("state after start = %s, want loaded", w.conductor.State())
	}
}

func TestWorktreeHandleStart_WithSkipPhases(t *testing.T) {
	ctx := context.Background()
	w := gitWorktreeWithConductor(t)

	params, _ := json.Marshal(StartParams{ //nolint:errchkjson // test data
		Source:     "empty:Add a feature",
		SkipPhases: []string{"simplify", "optimize"},
	})
	resp, err := w.handleStart(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleStart() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleStart() returned error: %s", resp.Error.Message)
	}
	if sp := w.conductor.SkipPhases(); len(sp) != 2 {
		t.Errorf("skip phases = %v, want 2 entries", sp)
	}
}

// --- worktree_queue.go: handleQueueReorder ---

func TestWorktreeHandleQueueReorder(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, _ := w.handleQueueReorder(ctx, &Request{ID: "1", Params: json.RawMessage(`{}`)})
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, _ := w.handleQueueReorder(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing id", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(map[string]any{"id": "", "position": 1})
		resp, _ := w.handleQueueReorder(ctx, &Request{ID: "1", Params: params})
		if resp.Error == nil {
			t.Fatal("expected error response for missing id")
		}
	})

	t.Run("invalid position", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(map[string]any{"id": "q-1", "position": 0})
		resp, _ := w.handleQueueReorder(ctx, &Request{ID: "1", Params: params})
		if resp.Error == nil {
			t.Fatal("expected error response for invalid position")
		}
	})

	t.Run("nonexistent queue id", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(map[string]any{"id": "no-such-queue-item", "position": 1})
		resp, _ := w.handleQueueReorder(ctx, &Request{ID: "1", Params: params})
		if resp.Error == nil {
			t.Fatal("expected error response reordering a nonexistent queue item")
		}
	})
}

// --- global_batch.go: action dispatched to a reachable worktree ---

func TestGlobalHandleBatch_ReachableWorktree(t *testing.T) {
	ctx := context.Background()

	// A live worktree socket with an active (non-none) state so the batch loop
	// passes the idle-skip guard and dispatches the action RPC.
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
	setWorkUnitInState(t, wt, conductor.StateImplemented)
	wctx, wcancel := context.WithCancel(context.Background())
	t.Cleanup(wcancel)
	go func() { _ = wt.Start(wctx) }()
	waitForSocket(ctx, t, wtSockPath)
	t.Cleanup(func() { _ = wt.Stop() })

	g := newTestGlobalSocket(t)
	g.mu.Lock()
	g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: repoDir, State: "implemented", SocketPath: wtSockPath}
	g.mu.Unlock()

	params, _ := json.Marshal(BatchParams{Action: "review"}) //nolint:errchkjson // test data
	resp, err := g.handleBatch(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBatch() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result struct {
		Results []BatchResultItem `json:"results"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// The action was dispatched to the live worktree (success or RPC-level
	// error, but the loop body reached the dispatch and recorded a result).
	if len(result.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(result.Results))
	}
	if result.Results[0].State != "implemented" {
		t.Errorf("result state = %q, want implemented", result.Results[0].State)
	}
}

// --- global_projects_util.go: GetOrCreateWorktreeSocket ---

func TestGlobalGetOrCreateWorktreeSocket(t *testing.T) {
	g := newTestGlobalSocket(t)
	dir := testutil.TempDir(t)
	testutil.InitGitRepo(t, dir)

	first, err := g.GetOrCreateWorktreeSocket(dir)
	if err != nil {
		t.Fatalf("GetOrCreateWorktreeSocket() error = %v", err)
	}
	if first == nil {
		t.Fatal("expected a worktree socket")
	}

	// The socket is started in a background goroutine. Wait for it to be
	// reachable before registering Stop, so Stop does not race the in-flight
	// Server.Start that writes the listener field (a pre-existing data race in
	// Server.Start/initiateShutdown — see REPORT).
	waitForSocket(t.Context(), t, WorktreeSocketPath(dir))
	t.Cleanup(func() { _ = g.Stop() })

	// A second call for the same path returns the cached socket.
	second, err := g.GetOrCreateWorktreeSocket(dir)
	if err != nil {
		t.Fatalf("GetOrCreateWorktreeSocket() second call error = %v", err)
	}
	if second == nil {
		t.Fatal("expected cached worktree socket on second call")
	}
}
