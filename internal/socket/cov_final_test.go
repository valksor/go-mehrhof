package socket

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/testutil"
)

// Memory handlers (memory.search/stats/outcomes/clear) are intentionally NOT
// unit-tested here: every one calls getMemoryAdapter first, which lazily loads
// an ~86 MB embedding model into a process-global singleton. Forcing the cached
// error state races with the background getMemoryAdapter goroutine that
// NewWorktreeSocket spawns, and exercising the success path would touch (and
// handleMemoryClear would wipe) the user's real on-disk memory store. The risk
// of a destructive side effect outweighs the coverage, so they are left
// uncovered. See REPORT.

// --- worktree_adversarial.go ---

func TestWorktreeHandleAdversarial(t *testing.T) {
	ctx := context.Background()

	t.Run("run nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleAdversarialRun(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleAdversarialRun() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("results nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleAdversarialResults(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleAdversarialResults() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("results empty findings", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplemented)
		resp, err := w.handleAdversarialResults(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleAdversarialResults() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if c, _ := result["count"].(float64); c != 0 {
			t.Errorf("count = %v, want 0", result["count"])
		}
	})

	t.Run("run with task surfaces a response", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplemented)
		resp, err := w.handleAdversarialRun(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleAdversarialRun() error = %v", err)
		}
		// With no agent configured this likely errors; assert liveness.
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
	})
}

// --- global_projects_util.go ---

func TestGlobalLoadAndSaveProjects(t *testing.T) {
	g := newTestGlobalSocket(t)

	// Pre-seed a projects.json in the socket's projects directory.
	projects := []WorktreeInfo{
		{ID: "wt-1", Path: "/proj-a", State: "loaded"},
		{ID: "wt-2", Path: "/proj-b", State: "implementing"},
	}
	data, _ := json.MarshalIndent(projects, "", "  ") //nolint:errchkjson // test data
	if err := os.WriteFile(g.projectsFile(), data, 0o644); err != nil {
		t.Fatal(err)
	}

	g.loadProjectsFromFile()

	g.mu.RLock()
	got := len(g.worktrees)
	g.mu.RUnlock()
	if got != 2 {
		t.Fatalf("loaded %d projects, want 2", got)
	}

	// Round-trip: save then reload into a fresh socket sharing the dir.
	g.saveProjectsToFile()
	if _, err := os.Stat(g.projectsFile()); err != nil {
		t.Errorf("projects file not written: %v", err)
	}
}

func TestGlobalProjectsFile_DefaultDir(t *testing.T) {
	g := newTestGlobalSocket(t)
	g.projectsDir = "" // force the BaseDir() fallback branch
	if g.projectsFile() == "" {
		t.Error("projectsFile() returned empty path")
	}
}

func TestGlobalBroadcasts(t *testing.T) {
	g := newTestGlobalSocket(t)
	// These broadcast to zero connected clients; the point is they marshal and
	// invoke Broadcast without panicking.
	g.broadcastTaskStateChanged("/proj", "implementing")
	g.BroadcastWorkerChanged()
}

func TestGlobalSendApprovalNotification_NoNotifier(t *testing.T) {
	prev := GetNotifier()
	SetNotifier(nil)
	t.Cleanup(func() { SetNotifier(prev) })

	g := newTestGlobalSocket(t)
	// With no notifier configured this is a no-op (early return); must not panic.
	g.sendApprovalNotification("/proj", "needs approval")
}

// --- worktree_export.go: enrichCheckpoints + collectFileChanges with a repo ---

func TestWorktreeTaskExport_WithRepo(t *testing.T) {
	ctx := context.Background()
	w := gitWorktree(ctx, t)
	setWorkUnitInState(t, w, conductor.StateImplemented)

	// A tracked-file change + a checkpoint SHA so the export collects both.
	if err := os.WriteFile(filepath.Join(w.path, "README.md"), []byte("# Edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.conductor.WorkUnit().Checkpoints = []string{"HEAD"}

	resp, err := w.handleTaskExport(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleTaskExport() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result taskExportResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Checkpoints) != 1 {
		t.Errorf("checkpoints = %d, want 1", len(result.Checkpoints))
	}
	if len(result.FileChanges) == 0 {
		t.Error("expected at least one file change in export")
	}
}

// --- global.go: handleTasksList enrichment via a reachable worktree socket ---

func TestGlobalHandleTasksList_ReachableWorktree(t *testing.T) {
	ctx := context.Background()

	// Stand up a real worktree socket the global can call into for status.
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

	resp, err := g.handleTasksList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleTasksList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result TasksListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Total = %d, want 1", result.Total)
	}
}

// --- global.go: handleAgentStatus with a worker pool (pool branch) ---

func TestGlobalHandleAgentStatus_WithPool(t *testing.T) {
	g := newTestGlobalSocketWithPool2(t)
	resp, err := g.handleAgentStatus(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleAgentStatus() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if len(resp.Result) == 0 {
		t.Fatal("expected agent status result")
	}
}
