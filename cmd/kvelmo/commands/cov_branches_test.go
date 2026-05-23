package commands

import (
	"strings"
	"testing"
)

// --- fork compare/select populated ---

func TestRunForkCompare_Populated(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("fork.compare", map[string]any{
		"forks": []any{
			map[string]any{
				"id": "f1", "label": "approach-a", "state": "implemented",
				"lines_added": 10, "lines_removed": 3,
				"diff_stats": map[string]any{"files": 2, "added": 10, "removed": 3},
			},
		},
	})

	out := captureStdout(t, func() {
		if err := runForkCompare(nil, nil); err != nil {
			t.Errorf("runForkCompare: %v", err)
		}
	})
	if !strings.Contains(out, "approach-a") || !strings.Contains(out, "Fork comparison") {
		t.Errorf("fork compare output:\n%s", out)
	}
}

func TestRunForkSelect_Populated(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("fork.select", map[string]any{"selected": true, "id": "f1"})

	_ = captureStdout(t, func() {
		if err := runForkSelect(nil, []string{"f1"}); err != nil {
			t.Errorf("runForkSelect: %v", err)
		}
	})
}

// --- queue list populated + JSON ---

func TestRunQueueList_Populated(t *testing.T) {
	setBoolPtr(t, &queueListJSON, false)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("queue.list", map[string]any{
		"queue": []any{
			map[string]any{"id": "q1", "source": "github:org/repo#1", "title": "First"},
		},
		"count": 1,
	})

	out := captureStdout(t, func() {
		if err := runQueueList(queueListCmd, nil); err != nil {
			t.Errorf("runQueueList: %v", err)
		}
	})
	if !strings.Contains(out, "Task queue (1)") || !strings.Contains(out, "First") {
		t.Errorf("queue list output:\n%s", out)
	}
}

func TestRunQueueList_JSON(t *testing.T) {
	setBoolPtr(t, &queueListJSON, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("queue.list", map[string]any{"queue": []any{}, "count": 0})

	out := captureStdout(t, func() {
		if err := runQueueList(queueListCmd, nil); err != nil {
			t.Errorf("runQueueList json: %v", err)
		}
	})
	if !strings.Contains(out, "queue") {
		t.Errorf("queue list json output:\n%s", out)
	}
}

// --- status verbose + full ---

func TestRunStatus_VerboseFull(t *testing.T) {
	origV, origFull, origF, origB, origA := statusVerbose, statusFull, statusFailed, statusBlocked, statusAll
	t.Cleanup(func() {
		statusVerbose, statusFull, statusFailed, statusBlocked, statusAll = origV, origFull, origF, origB, origA
	})
	statusVerbose, statusFull, statusFailed, statusBlocked, statusAll = true, true, false, false, false

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{
		"state":             "reviewing",
		"path":              "/proj",
		"pending_prompt_id": "p1",
		"task": map[string]any{
			"id": "t1", "title": "Task", "source": "github",
			"context_items": []any{
				map[string]any{"type": "file", "ref": "main.go", "label": "main"},
			},
		},
	})
	stub.SetResponse("checkpoints", map[string]any{"checkpoints": []any{
		map[string]any{"sha": "aaaa"},
	}})

	out := captureStdout(t, func() {
		if err := runStatus(StatusCmd, nil); err != nil {
			t.Errorf("runStatus verbose full: %v", err)
		}
	})
	if !strings.Contains(out, "Socket:") || !strings.Contains(out, "Context:") || !strings.Contains(out, "Checkpoints:") {
		t.Errorf("status verbose/full output:\n%s", out)
	}
	if !strings.Contains(out, "quality gate") && !strings.Contains(out, "Quality gate") {
		t.Errorf("status missing pending-prompt notice:\n%s", out)
	}
}

// --- showAllStatus with --blocked and --failed filters ---

func TestShowAllStatus_BlockedFilter(t *testing.T) {
	origB, origF, origV := statusBlocked, statusFailed, statusVerbose
	t.Cleanup(func() { statusBlocked, statusFailed, statusVerbose = origB, origF, origV })
	statusBlocked, statusFailed, statusVerbose = true, false, false

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{
			map[string]any{"path": "/p1", "state": "implementing"},            // not blocked → filtered out
			map[string]any{"path": "/p2", "state": "failed", "task_id": "t2"}, // blocked
		},
	})

	out := captureStdout(t, func() {
		if err := showAllStatus(); err != nil {
			t.Errorf("showAllStatus blocked: %v", err)
		}
	})
	if !strings.Contains(out, "/p2") {
		t.Errorf("blocked filter output:\n%s", out)
	}
}

func TestShowAllStatus_FailedNone(t *testing.T) {
	origB, origF := statusBlocked, statusFailed
	t.Cleanup(func() { statusBlocked, statusFailed = origB, origF })
	statusBlocked, statusFailed = false, true

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{map[string]any{"path": "/p1", "state": "implementing"}},
	})

	out := captureStdout(t, func() {
		if err := showAllStatus(); err != nil {
			t.Errorf("showAllStatus failed: %v", err)
		}
	})
	if !strings.Contains(out, "No failed tasks") {
		t.Errorf("failed-none output = %q", out)
	}
}

// --- list search with all filter params ---

func TestRunTaskSearch_AllFilters(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.search", map[string]any{
		"tasks": []any{
			map[string]any{"title": strings.Repeat("y", 40), "final_state": "finished", "source": strings.Repeat("z", 30), "completed_at": "2026-05-01T10:00:00Z"},
		},
		"count": 1,
	})

	for _, f := range []struct{ name, val string }{
		{"tag", "backend"},
		{"since", "2026-01-01"},
		{"until", "2026-12-31"},
		{"state", "finished"},
		{"file", "auth/"},
	} {
		if err := listSearchCmd.Flags().Set(f.name, f.val); err != nil {
			t.Fatal(err)
		}
	}
	if err := listSearchCmd.Flags().Set("limit", "5"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, n := range []string{"tag", "since", "until", "state", "file"} {
			_ = listSearchCmd.Flags().Set(n, "")
		}
		_ = listSearchCmd.Flags().Set("limit", "0")
	})

	out := captureStdout(t, func() {
		if err := runListSearchCmd(listSearchCmd, []string{"query"}); err != nil {
			t.Errorf("runListSearchCmd all filters: %v", err)
		}
	})
	if !strings.Contains(out, "...") {
		t.Errorf("task search (truncation) output:\n%s", out)
	}
}

func TestRunTaskSearch_BadSince(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	_ = startStubWorktreeSocket(t)

	if err := listSearchCmd.Flags().Set("since", "not-a-date"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listSearchCmd.Flags().Set("since", "") })

	if err := runListSearchCmd(listSearchCmd, []string{"q"}); err == nil {
		t.Fatal("expected error for bad --since")
	}
}

// --- memory search with results + outcomes populated ---

func TestRunMemorySearch_Populated(t *testing.T) {
	setBoolPtr(t, &memorySearchJSON, false)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.search", map[string]any{
		"results": []any{
			map[string]any{"content": "auth uses JWT", "score": 0.9, "task_id": "t1", "kind": "decision"},
		},
	})

	out := captureStdout(t, func() {
		if err := runMemorySearch(memorySearchCmd, []string{"auth"}); err != nil {
			t.Errorf("runMemorySearch: %v", err)
		}
	})
	if out == "" {
		t.Error("memory search populated produced no output")
	}
}

// --- export / report / audit / activity ---

func TestRunExport_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("export", map[string]any{
		"tasks":    []any{map[string]any{"id": "t1", "path": "/p", "state": "implementing"}},
		"activity": []any{},
	})

	_ = captureStdout(t, func() {
		if err := runExport(nil, nil); err != nil {
			t.Errorf("runExport: %v", err)
		}
	})
}

func TestRunReport_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("report.generate", map[string]any{"path": "/tmp/report.json", "format": "json"})

	_ = captureStdout(t, func() {
		if err := runReport(nil, nil); err != nil {
			t.Errorf("runReport: %v", err)
		}
	})
}

func TestRunExportTask_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.export", map[string]any{"id": "t1", "path": "/tmp/t1.json"})

	_ = captureStdout(t, func() {
		if err := runExportTask(nil, nil); err != nil {
			t.Errorf("runExportTask: %v", err)
		}
	})
}

// --- prompt (PS1) ---

func TestRunPrompt_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{"state": "implementing", "path": "/p"})

	// runPrompt prints a PS1 fragment; should not error with a live socket.
	_ = captureStdout(t, func() {
		_ = runPrompt(PromptCmd, nil)
	})
}

// --- rpc raw call ---

func TestRunRPC_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{"state": "implementing"})

	out := captureStdout(t, func() {
		if err := runRPC(RPCCmd, []string{"status"}); err != nil {
			t.Errorf("runRPC: %v", err)
		}
	})
	if !strings.Contains(out, "implementing") {
		t.Errorf("rpc output:\n%s", out)
	}
}

// --- catalog use with source (starts task) ---

func TestRunCatalogUse_WithSource(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t) // worktree socket for the start call
	stub.SetResponse("start", map[string]any{"state": "loaded"})

	// catalog.get is served by the GLOBAL socket; start one too.
	gstub := startStubGlobalSocket(t)
	gstub.SetResponse("catalog.get", map[string]any{"name": "bugfix", "source": "github:org/repo#1"})

	out := captureStdout(t, func() {
		if err := runCatalogUse(nil, []string{"bugfix"}); err != nil {
			t.Errorf("runCatalogUse with source: %v", err)
		}
	})
	if !strings.Contains(out, "Task started from template") {
		t.Errorf("catalog use (with source) output:\n%s", out)
	}
}

// --- config offline get/set value formatting ---

func TestConfigGetOffline(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	// No socket → offline path reads default settings.
	out := captureStdout(t, func() {
		if err := configGetOffline("workers.max"); err != nil {
			t.Errorf("configGetOffline: %v", err)
		}
	})
	if out == "" {
		t.Error("configGetOffline produced no output")
	}
}

func TestConfigShowOffline(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	out := captureStdout(t, func() {
		if err := configShowOffline(); err != nil {
			t.Errorf("configShowOffline: %v", err)
		}
	})
	if !strings.Contains(out, "workers") {
		t.Errorf("configShowOffline output:\n%s", out)
	}
}
