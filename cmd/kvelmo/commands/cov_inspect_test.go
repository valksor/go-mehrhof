package commands

import (
	"strings"
	"testing"
)

// --- Worktree-socket inspection commands ---

func TestRunEventlog_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("eventlog.query", map[string]any{
		"entries": []any{
			map[string]any{"timestamp": "2026-05-19T10:00:00Z", "type": "phase_started", "phase": "plan", "message": "started"},
		},
		"total": 1,
	})

	out := captureStdout(t, func() {
		if err := runEventlog(EventlogCmd, nil); err != nil {
			t.Errorf("runEventlog: %v", err)
		}
	})
	if !strings.Contains(out, "phase_started") || !strings.Contains(out, "[plan]") {
		t.Errorf("eventlog output:\n%s", out)
	}
}

func TestRunEventlog_Empty(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("eventlog.query", map[string]any{"entries": []any{}, "total": 0})

	out := captureStdout(t, func() {
		if err := runEventlog(EventlogCmd, nil); err != nil {
			t.Errorf("runEventlog: %v", err)
		}
	})
	if !strings.Contains(out, "No lifecycle events") {
		t.Errorf("eventlog empty output = %q", out)
	}
}

func TestRunEventlog_ServerError(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("eventlog.query", -32000, "boom")

	if err := runEventlog(EventlogCmd, nil); err == nil {
		t.Fatal("expected error from eventlog")
	}
}

func TestRunCIStatus_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("ci.status", map[string]any{"runs": []any{}})

	_ = captureStdout(t, func() {
		if err := runCIStatus(CICmd, nil); err != nil {
			t.Errorf("runCIStatus: %v", err)
		}
	})
}

func TestRunHooks_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("hooks.list", map[string]any{"hooks": []any{}})

	_ = captureStdout(t, func() {
		if err := runHooks(HooksCmd, nil); err != nil {
			t.Errorf("runHooks: %v", err)
		}
	})
}

func TestRunHooks_ServerError(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("hooks.list", -32000, "boom")

	if err := runHooks(HooksCmd, nil); err == nil {
		t.Fatal("expected error from hooks")
	}
}

func TestRunDiscover_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("discovery.scan", map[string]any{"commands": []any{}})

	_ = captureStdout(t, func() {
		if err := runDiscover(DiscoverCmd, nil); err != nil {
			t.Errorf("runDiscover: %v", err)
		}
	})
}

func TestRunCacheStatsClear_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("cache.stats", map[string]any{"hits": 3, "misses": 1})
	stub.SetResponse("cache.clear", map[string]any{"cleared": 4})

	_ = captureStdout(t, func() {
		if err := runCacheStats(nil, nil); err != nil {
			t.Errorf("runCacheStats: %v", err)
		}
	})
	_ = captureStdout(t, func() {
		if err := runCacheClear(nil, nil); err != nil {
			t.Errorf("runCacheClear: %v", err)
		}
	})
}

func TestRunQuality_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("autofix.status", map[string]any{"attempts": 0})

	_ = captureStdout(t, func() {
		if err := runQuality(QualityCmd, nil); err != nil {
			t.Errorf("runQuality: %v", err)
		}
	})
}

func TestRunQualityFailclass_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("failclass.stats", map[string]any{"flaky": 0, "genuine": 0})

	_ = captureStdout(t, func() {
		if err := runQualityFailclass(nil, nil); err != nil {
			t.Errorf("runQualityFailclass: %v", err)
		}
	})
}

// --- Codegraph (worktree) ---

func TestRunCodegraphStats_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.stats", map[string]any{"symbols": 42, "files": 5})

	_ = captureStdout(t, func() {
		if err := runCodegraphStats(nil, nil); err != nil {
			t.Errorf("runCodegraphStats: %v", err)
		}
	})
}

func TestRunCodegraphSearch_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.search", map[string]any{"results": []any{}})

	_ = captureStdout(t, func() {
		if err := runCodegraphSearch(nil, []string{"HandleRequest"}); err != nil {
			t.Errorf("runCodegraphSearch: %v", err)
		}
	})
}

func TestRunCodegraphCallers_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.callers", map[string]any{"callers": []any{}})

	_ = captureStdout(t, func() {
		if err := runCodegraphCallers(nil, []string{"Foo"}); err != nil {
			t.Errorf("runCodegraphCallers: %v", err)
		}
	})
}

func TestRunCodegraphDeps_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.deps", map[string]any{"deps": []any{}})

	_ = captureStdout(t, func() {
		if err := runCodegraphDeps(nil, []string{"internal/socket"}); err != nil {
			t.Errorf("runCodegraphDeps: %v", err)
		}
	})
}

// --- Fork (worktree) ---

func TestRunForkCreate_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("fork.create", map[string]any{"id": "f1", "branch": "fork/f1"})

	_ = captureStdout(t, func() {
		if err := runForkCreate(nil, []string{"alt-approach"}); err != nil {
			t.Errorf("runForkCreate: %v", err)
		}
	})
}

func TestRunForkList_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("fork.list", map[string]any{"forks": []any{
		map[string]any{"id": "f1", "label": "alt", "branch": "fork/f1", "state": "planning", "checkpoint_sha": "deadbeefcafe"},
	}})

	out := captureStdout(t, func() {
		if err := runForkList(nil, nil); err != nil {
			t.Errorf("runForkList: %v", err)
		}
	})
	if !strings.Contains(out, "f1") {
		t.Errorf("fork list output:\n%s", out)
	}
}

// --- Git (worktree) ---

func TestRunGitStatus_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("git.status", map[string]any{
		"branch":      "feat/x",
		"has_changes": true,
		"files":       []string{"a.go", "b.go"},
	})

	out := captureStdout(t, func() {
		if err := runGitStatus(gitStatusCmd, nil); err != nil {
			t.Errorf("runGitStatus: %v", err)
		}
	})
	if !strings.Contains(out, "feat/x") || !strings.Contains(out, "a.go") {
		t.Errorf("git status output:\n%s", out)
	}
}

func TestRunGitDiff_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("git.diff", map[string]any{"diff": "+added line"})

	out := captureStdout(t, func() {
		if err := runGitDiff(gitDiffCmd, nil); err != nil {
			t.Errorf("runGitDiff: %v", err)
		}
	})
	if !strings.Contains(out, "+added line") {
		t.Errorf("git diff output = %q", out)
	}
}

func TestRunGitLog_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("git.log", map[string]any{"entries": []any{
		map[string]any{"sha": "abcdef123456", "message": "first commit"},
	}})

	out := captureStdout(t, func() {
		if err := runGitLog(gitLogCmd, nil); err != nil {
			t.Errorf("runGitLog: %v", err)
		}
	})
	if !strings.Contains(out, "first commit") {
		t.Errorf("git log output = %q", out)
	}
}

// --- Diff command (checkpoints-aware) ---

func TestRunDiff_NoCheckpoints(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("checkpoints", map[string]any{"checkpoints": []any{}})
	stub.SetResponse("git.diff", map[string]any{"diff": "+regular diff"})

	out := captureStdout(t, func() {
		if err := runDiff(DiffCmd, nil); err != nil {
			t.Errorf("runDiff: %v", err)
		}
	})
	if !strings.Contains(out, "+regular diff") {
		t.Errorf("diff (no checkpoints) output = %q", out)
	}
}

func TestRunDiff_AgainstCheckpoint(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("checkpoints", map[string]any{"checkpoints": []any{
		map[string]any{"sha": "aaaa1111"},
		map[string]any{"sha": "bbbb2222"},
	}})
	stub.SetResponse("git.diff_against", map[string]any{"diff": "+checkpoint diff"})

	out := captureStdout(t, func() {
		if err := runDiff(DiffCmd, nil); err != nil {
			t.Errorf("runDiff: %v", err)
		}
	})
	if !strings.Contains(out, "+checkpoint diff") {
		t.Errorf("diff (vs checkpoint) output = %q", out)
	}
}

// --- Files (worktree) ---

func TestRunFilesList_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("files.list", map[string]any{"files": []any{}})

	_ = captureStdout(t, func() {
		if err := runFilesList(filesListCmd, nil); err != nil {
			t.Errorf("runFilesList: %v", err)
		}
	})
}

func TestRunFilesSearch_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("files.search", map[string]any{"results": []any{}})

	_ = captureStdout(t, func() {
		if err := runFilesSearch(filesSearchCmd, []string{"query"}); err != nil {
			t.Errorf("runFilesSearch: %v", err)
		}
	})
}

// --- Policy (worktree) ---

func TestRunPolicyCheck_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("policy.check", map[string]any{"ok": true, "violations": []any{}})

	_ = captureStdout(t, func() {
		if err := runPolicyCheck(nil, nil); err != nil {
			t.Errorf("runPolicyCheck: %v", err)
		}
	})
}

// --- Update / finish / refresh ---

func TestRunFinish_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.finish", map[string]any{"state": "finished"})

	_ = captureStdout(t, func() {
		if err := runFinish(FinishCmd, nil); err != nil {
			t.Errorf("runFinish: %v", err)
		}
	})
}

func TestRunRefresh_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.refresh", map[string]any{"updated": false})

	_ = captureStdout(t, func() {
		if err := runRefresh(RefreshCmd, nil); err != nil {
			t.Errorf("runRefresh: %v", err)
		}
	})
}

// --- Global-socket commands ---

func TestRunMemoryStats_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.stats", map[string]any{"count": 7})

	_ = captureStdout(t, func() {
		if err := runMemoryStats(memoryStatsCmd, nil); err != nil {
			t.Errorf("runMemoryStats: %v", err)
		}
	})
}

func TestRunMemorySearch_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.search", map[string]any{"results": []any{}})

	_ = captureStdout(t, func() {
		if err := runMemorySearch(memorySearchCmd, []string{"auth"}); err != nil {
			t.Errorf("runMemorySearch: %v", err)
		}
	})
}

func TestRunActivity_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	_ = stub

	out := captureStdout(t, func() {
		if err := runActivity(ActivityCmd, nil); err != nil {
			t.Errorf("runActivity: %v", err)
		}
	})
	if !strings.Contains(out, "ping") {
		t.Errorf("activity output:\n%s", out)
	}
}

func TestRunAudit_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	_ = startStubGlobalSocket(t)

	_ = captureStdout(t, func() {
		if err := runAudit(AuditCmd, nil); err != nil {
			t.Errorf("runAudit: %v", err)
		}
	})
}

func TestRunSecurityScan_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("security.scan", map[string]any{"findings": []any{}})

	_ = captureStdout(t, func() {
		if err := runSecurityScan(SecurityCmd, nil); err != nil {
			t.Errorf("runSecurityScan: %v", err)
		}
	})
}

func TestRunConfigCheck_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("config.check", map[string]any{"ok": true, "issues": []any{}})

	_ = captureStdout(t, func() {
		if err := runConfigCheck(nil, nil); err != nil {
			t.Errorf("runConfigCheck: %v", err)
		}
	})
}

func TestRunNotifyTest_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("notify.test", map[string]any{"sent": 2, "message": "ok"})

	_ = captureStdout(t, func() {
		if err := runNotifyTest(nil, nil); err != nil {
			t.Errorf("runNotifyTest: %v", err)
		}
	})
}

func TestRunBackupList_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("backup.list", map[string]any{"backups": []any{}})

	_ = captureStdout(t, func() {
		if err := runBackupList(nil, nil); err != nil {
			t.Errorf("runBackupList: %v", err)
		}
	})
}

func TestRunJobsList_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("jobs.list", map[string]any{"jobs": []any{
		map[string]any{"id": "j1", "type": "plan", "status": "running", "worktree_id": "wt1", "created_at": "2026-05-19T10:00:00Z"},
	}})

	out := captureStdout(t, func() {
		if err := runJobsList(jobsListCmd, nil); err != nil {
			t.Errorf("runJobsList: %v", err)
		}
	})
	if !strings.Contains(out, "j1") {
		t.Errorf("jobs list output:\n%s", out)
	}
}

func TestRunJobsGet_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("jobs.get", map[string]any{
		"id": "j1", "type": "implement", "status": "completed", "worktree_id": "wt1",
		"created_at": "2026-05-19T10:00:00Z", "completed_at": "2026-05-19T10:05:00Z",
	})

	out := captureStdout(t, func() {
		if err := runJobsGet(jobsGetCmd, []string{"j1"}); err != nil {
			t.Errorf("runJobsGet: %v", err)
		}
	})
	if !strings.Contains(out, "implement") {
		t.Errorf("jobs get output:\n%s", out)
	}
}

// TestCompleteJobIDs exercises the shell-completion helper with a live socket.
func TestCompleteJobIDs_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("jobs.list", map[string]any{"jobs": []any{
		map[string]any{"id": "job-aaa"},
		map[string]any{"id": "job-bbb"},
	}})

	ids, _ := completeJobIDs(nil, nil, "")
	if len(ids) != 2 {
		t.Errorf("completeJobIDs returned %v, want 2 ids", ids)
	}
}
