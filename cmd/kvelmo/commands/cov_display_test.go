package commands

import (
	"strings"
	"testing"
)

// setBoolPtr temporarily sets a *bool and restores it after the test.
func setBoolPtr(t *testing.T, p *bool, v bool) {
	t.Helper()
	orig := *p
	t.Cleanup(func() { *p = orig })
	*p = v
}

// --- codegraph populated + JSON ---

func TestRunCodegraphSearch_Results(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.search", map[string]any{
		"symbols": []any{
			map[string]any{"name": "HandleRequest", "kind": "func", "file": "h.go", "line": 12, "package": "web"},
		},
	})

	out := captureStdout(t, func() {
		if err := runCodegraphSearch(nil, []string{"Handle"}); err != nil {
			t.Errorf("runCodegraphSearch: %v", err)
		}
	})
	if !strings.Contains(out, "HandleRequest") || !strings.Contains(out, "Found 1 symbol") {
		t.Errorf("codegraph search results output:\n%s", out)
	}
}

func TestRunCodegraphSearch_JSON(t *testing.T) {
	setBoolPtr(t, &codegraphJSON, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.search", map[string]any{"symbols": []any{
		map[string]any{"name": "Foo", "kind": "func", "file": "f.go", "line": 1, "package": "p"},
	}})

	out := captureStdout(t, func() {
		if err := runCodegraphSearch(nil, []string{"Foo"}); err != nil {
			t.Errorf("runCodegraphSearch json: %v", err)
		}
	})
	if !strings.Contains(out, "Foo") {
		t.Errorf("codegraph search json output:\n%s", out)
	}
}

func TestRunCodegraphCallers_Results(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.callers", map[string]any{
		"callers": []any{
			map[string]any{"name": "main", "file": "main.go", "line": 5},
		},
	})

	out := captureStdout(t, func() {
		if err := runCodegraphCallers(nil, []string{"Foo"}); err != nil {
			t.Errorf("runCodegraphCallers: %v", err)
		}
	})
	if out == "" {
		t.Error("codegraph callers produced no output")
	}
}

func TestRunCodegraphDeps_Results(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.deps", map[string]any{
		"deps": []any{"internal/socket", "meta"},
	})

	out := captureStdout(t, func() {
		if err := runCodegraphDeps(nil, []string{"internal/web"}); err != nil {
			t.Errorf("runCodegraphDeps: %v", err)
		}
	})
	if out == "" {
		t.Error("codegraph deps produced no output")
	}
}

func TestRunCodegraphStats_Populated(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.stats", map[string]any{
		"symbols": 100, "files": 10, "edges": 250, "func": 40,
	})

	out := captureStdout(t, func() {
		if err := runCodegraphStats(nil, nil); err != nil {
			t.Errorf("runCodegraphStats: %v", err)
		}
	})
	if !strings.Contains(out, "Breakdown") {
		t.Errorf("codegraph stats output:\n%s", out)
	}
}

// --- security findings populated ---

func TestRunSecurityScan_WithFindings(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("security.scan", map[string]any{
		"findings": []any{
			map[string]any{"severity": "high", "type": "secret", "file": "a.go", "line": 4, "message": "hardcoded token"},
		},
		"count":    1,
		"scanners": []string{"secrets"},
	})

	out := captureStdout(t, func() {
		if err := runSecurityScan(SecurityCmd, nil); err != nil {
			t.Errorf("runSecurityScan: %v", err)
		}
	})
	if !strings.Contains(out, "Found 1 issue") || !strings.Contains(out, "hardcoded token") {
		t.Errorf("security findings output:\n%s", out)
	}
}

func TestRunSecurityScan_JSON(t *testing.T) {
	setBoolPtr(t, &securityScanJSON, true)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("security.scan", map[string]any{"findings": []any{}, "count": 0, "scanners": []string{"secrets"}})

	out := captureStdout(t, func() {
		if err := runSecurityScan(SecurityCmd, nil); err != nil {
			t.Errorf("runSecurityScan json: %v", err)
		}
	})
	if !strings.Contains(out, "scanners") {
		t.Errorf("security json output:\n%s", out)
	}
}

// --- memory search populated + JSON ---

func TestRunMemorySearch_Results(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.search", map[string]any{
		"results": []any{
			map[string]any{"content": "remembered fact", "score": 0.95, "source": "task-1"},
		},
	})

	out := captureStdout(t, func() {
		if err := runMemorySearch(memorySearchCmd, []string{"fact"}); err != nil {
			t.Errorf("runMemorySearch: %v", err)
		}
	})
	if out == "" {
		t.Error("memory search produced no output")
	}
}

func TestRunMemorySearch_JSON(t *testing.T) {
	setBoolPtr(t, &memorySearchJSON, true)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.search", map[string]any{"results": []any{}})

	out := captureStdout(t, func() {
		if err := runMemorySearch(memorySearchCmd, []string{"fact"}); err != nil {
			t.Errorf("runMemorySearch json: %v", err)
		}
	})
	if !strings.Contains(out, "results") {
		t.Errorf("memory search json output:\n%s", out)
	}
}

// --- list with tasks + JSON ---

func TestRunTaskHistory_JSON(t *testing.T) {
	setBoolPtr(t, &listHistoryJSON, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.history", map[string]any{
		"tasks": []any{
			map[string]any{"title": "T", "final_state": "finished", "source": "github", "completed_at": "2026-05-01T10:00:00Z"},
		},
		"count": 1,
	})

	out := captureStdout(t, func() {
		if err := runTaskHistory(true); err != nil {
			t.Errorf("runTaskHistory json: %v", err)
		}
	})
	if !strings.Contains(out, "\"count\": 1") {
		t.Errorf("history json output:\n%s", out)
	}
}

func TestRunListProjects_JSON(t *testing.T) {
	setBoolPtr(t, &statusJSON, false)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{
			map[string]any{"id": "long-id-that-exceeds-thirty-characters-for-truncation", "state": "implementing", "task_title": strings.Repeat("x", 40), "path": "/p"},
		},
	})

	out := captureStdout(t, func() {
		if err := runListProjects(); err != nil {
			t.Errorf("runListProjects: %v", err)
		}
	})
	if !strings.Contains(out, "...") {
		t.Errorf("list projects (truncated title) output:\n%s", out)
	}
}

// --- review list/view populated ---

func TestRunReviewList_Populated(t *testing.T) {
	setBoolPtr(t, &reviewListJSON, false)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("review.list", map[string]any{
		"reviews": []any{
			map[string]any{"id": 1, "verdict": "approved", "created_at": "2026-05-01T10:00:00Z"},
		},
	})

	out := captureStdout(t, func() {
		if err := runReviewList(ReviewCmd, nil); err != nil {
			t.Errorf("runReviewList: %v", err)
		}
	})
	if out == "" {
		t.Error("review list produced no output")
	}
}

func TestRunReviewView_Content(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("review.view", map[string]any{"id": 1, "content": "looks good"})

	out := captureStdout(t, func() {
		if err := runReviewView(reviewViewCmd, []string{"1"}); err != nil {
			t.Errorf("runReviewView: %v", err)
		}
	})
	if !strings.Contains(out, "looks good") {
		t.Errorf("review view output:\n%s", out)
	}
}

// --- screenshots list populated + JSON ---

func TestRunScreenshotsList_Populated(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.list", map[string]any{
		"screenshots": []any{
			map[string]any{"id": "s1", "path": "/tmp/s1.png", "captured_at": "2026-05-01T10:00:00Z", "width": 800, "height": 600},
		},
	})

	out := captureStdout(t, func() {
		if err := runScreenshotsList(screenshotsListCmd, nil); err != nil {
			t.Errorf("runScreenshotsList: %v", err)
		}
	})
	if out == "" {
		t.Error("screenshots list produced no output")
	}
}

// --- group status populated ---

func TestRunGroupStatus_Populated(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.status", map[string]any{
		"id":    "g1",
		"label": "My Group",
		"members": []any{
			map[string]any{"task_id": "t1", "project_dir": "/p1", "state": "submitted"},
		},
	})

	out := captureStdout(t, func() {
		if err := runGroupStatus(nil, []string{"g1"}); err != nil {
			t.Errorf("runGroupStatus: %v", err)
		}
	})
	if out == "" {
		t.Error("group status produced no output")
	}
}

func TestRunGroupList_JSON(t *testing.T) {
	setBoolPtr(t, &groupListJSON, true)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.list", map[string]any{"groups": []any{
		map[string]any{"id": "g1", "label": "My Group"},
	}})

	out := captureStdout(t, func() {
		if err := runGroupList(nil, nil); err != nil {
			t.Errorf("runGroupList json: %v", err)
		}
	})
	if !strings.Contains(out, "g1") {
		t.Errorf("group list json output:\n%s", out)
	}
}

// --- backup list populated ---

func TestRunBackupList_Populated(t *testing.T) {
	setBoolPtr(t, &backupListJSON, false)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("backup.list", map[string]any{
		"backups": []any{
			map[string]any{"path": "/b/backup-1.tar.gz", "size": 2048, "created_at": "2026-05-01T10:00:00Z"},
		},
	})

	out := captureStdout(t, func() {
		if err := runBackupList(nil, nil); err != nil {
			t.Errorf("runBackupList: %v", err)
		}
	})
	if out == "" {
		t.Error("backup list produced no output")
	}
}

// --- catalog list populated ---

func TestRunCatalogList_Populated(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("catalog.list", map[string]any{
		"items": []any{
			map[string]any{"name": "bugfix", "description": "fix a bug", "category": "dev"},
		},
	})

	out := captureStdout(t, func() {
		if err := runCatalogList(nil, nil); err != nil {
			t.Errorf("runCatalogList: %v", err)
		}
	})
	if out == "" {
		t.Error("catalog list produced no output")
	}
}

// --- diff stat path (showRegularDiff with --stat) ---

func TestRunDiff_StatNoCheckpoints(t *testing.T) {
	setBoolPtr(t, &diffJSON, false)
	if err := DiffCmd.Flags().Set("stat", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = DiffCmd.Flags().Set("stat", "false") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("checkpoints", map[string]any{"checkpoints": []any{}})
	stub.SetResponse("git.diff_against", map[string]any{"diff": " a.go | 2 +-"})

	out := captureStdout(t, func() {
		if err := runDiff(DiffCmd, nil); err != nil {
			t.Errorf("runDiff stat: %v", err)
		}
	})
	if !strings.Contains(out, "a.go") {
		t.Errorf("diff stat output = %q", out)
	}
}

func TestRunDiff_SingleCheckpoint(t *testing.T) {
	setBoolPtr(t, &diffJSON, false)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("checkpoints", map[string]any{"checkpoints": []any{
		map[string]any{"sha": "aaaa1111"},
	}})
	stub.SetResponse("git.diff_against", map[string]any{"diff": ""})

	out := captureStdout(t, func() {
		if err := runDiff(DiffCmd, nil); err != nil {
			t.Errorf("runDiff single checkpoint: %v", err)
		}
	})
	if !strings.Contains(out, "No changes since last checkpoint") {
		t.Errorf("diff single checkpoint output = %q", out)
	}
}

// --- batch with results ---

func TestRunBatch_Results(t *testing.T) {
	setBoolPtr(t, &batchJSON, false)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.batch", map[string]any{
		"action": "status",
		"total":  2,
		"results": []any{
			map[string]any{"path": "/p1", "state": "implementing", "success": true},
			map[string]any{"path": "/p2", "state": "failed", "success": false, "error": "boom"},
		},
	})

	out := captureStdout(t, func() {
		if err := runBatch(BatchCmd, []string{"status"}); err != nil {
			t.Errorf("runBatch: %v", err)
		}
	})
	if !strings.Contains(out, "/p1") || !strings.Contains(out, "boom") {
		t.Errorf("batch results output:\n%s", out)
	}
}

// --- recordings list JSON ---

func TestRunRecordingsList_JSON(t *testing.T) {
	setBoolPtr(t, &recordingsOutputJSON, true)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("recordings.list", map[string]any{
		"recordings": []any{
			map[string]any{"path": "/r/rec.jsonl", "job_id": "job-1", "agent": "claude", "lines": 5, "started_at": "2026-05-01T10:00:00Z"},
		},
	})

	out := captureStdout(t, func() {
		if err := runRecordingsList(recordingsListCmd, nil); err != nil {
			t.Errorf("runRecordingsList json: %v", err)
		}
	})
	if !strings.Contains(out, "job-1") {
		t.Errorf("recordings list json output:\n%s", out)
	}
}

// --- checkpoints JSON ---

func TestRunCheckpoints_JSON(t *testing.T) {
	setBoolPtr(t, &checkpointsJSON, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("checkpoints", map[string]any{"checkpoints": []any{
		map[string]any{"sha": "aaaa1111bbbb", "message": "plan"},
	}})

	out := captureStdout(t, func() {
		if err := runCheckpoints(CheckpointsCmd, nil); err != nil {
			t.Errorf("runCheckpoints json: %v", err)
		}
	})
	if !strings.Contains(out, "aaaa1111bbbb") {
		t.Errorf("checkpoints json output:\n%s", out)
	}
}

// --- status JSON ---

func TestRunStatus_JSON(t *testing.T) {
	origJSON, origF, origB, origA := statusJSON, statusFailed, statusBlocked, statusAll
	t.Cleanup(func() { statusJSON, statusFailed, statusBlocked, statusAll = origJSON, origF, origB, origA })
	statusJSON, statusFailed, statusBlocked, statusAll = true, false, false, false

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{"state": "implementing", "path": "/p"})

	out := captureStdout(t, func() {
		if err := runStatus(StatusCmd, nil); err != nil {
			t.Errorf("runStatus json: %v", err)
		}
	})
	if !strings.Contains(out, "implementing") {
		t.Errorf("status json output:\n%s", out)
	}
}
