package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/socket"
)

// --- stats (3 paths) ---

func TestRunStatsProject_WithSocket(t *testing.T) {
	origH, origA := statsHistory, statsAll
	t.Cleanup(func() { statsHistory, statsAll = origH, origA })
	statsHistory, statsAll = false, false

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.history", map[string]any{
		"tasks": []any{
			map[string]any{"id": "t1", "title": "Done", "final_state": "finished", "started_at": "2026-05-01T10:00:00Z", "completed_at": "2026-05-01T10:30:00Z"},
		},
	})

	out := captureStdout(t, func() {
		if err := runStats(StatsCmd, nil); err != nil {
			t.Errorf("runStats project: %v", err)
		}
	})
	if out == "" {
		t.Error("stats project produced no output")
	}
}

func TestRunStatsHistory_Enabled(t *testing.T) {
	origH := statsHistory
	t.Cleanup(func() { statsHistory = origH })
	statsHistory = true

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("metrics.history", map[string]any{
		"enabled": true,
		"entries": []any{
			map[string]any{"timestamp": "2026-05-01T10:00:00Z", "rpc_requests": 10, "jobs_completed": 2, "rpc_errors": 0},
		},
	})

	out := captureStdout(t, func() {
		if err := runStatsHistory(); err != nil {
			t.Errorf("runStatsHistory: %v", err)
		}
	})
	if !strings.Contains(out, "Metrics history: 1 entries") {
		t.Errorf("stats history output:\n%s", out)
	}
}

func TestRunStatsHistory_Disabled(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("metrics.history", map[string]any{"enabled": false, "entries": []any{}})

	out := captureStdout(t, func() {
		if err := runStatsHistory(); err != nil {
			t.Errorf("runStatsHistory: %v", err)
		}
	})
	if !strings.Contains(out, "not enabled") {
		t.Errorf("stats history disabled output = %q", out)
	}
}

func TestRunStatsAll_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.list", map[string]any{"tasks": []any{}})

	out := captureStdout(t, func() {
		if err := runStatsAll(); err != nil {
			t.Errorf("runStatsAll: %v", err)
		}
	})
	if out == "" {
		t.Error("stats all produced no output")
	}
}

// --- showAllStatus rich rows ---

func TestShowAllStatus_RichRows(t *testing.T) {
	origV := statusVerbose
	t.Cleanup(func() { statusVerbose = origV })
	statusVerbose = false

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{
			map[string]any{"path": "/p1", "state": "implementing", "task_title": "Build", "source": "github"},
			map[string]any{"path": "/p2", "state": "failed", "task_id": "t2", "last_error": "boom"},
		},
	})

	out := captureStdout(t, func() {
		if err := showAllStatus(); err != nil {
			t.Errorf("showAllStatus: %v", err)
		}
	})
	if !strings.Contains(out, "PROJECT") || !strings.Contains(out, "FAIL") {
		t.Errorf("showAllStatus output:\n%s", out)
	}
}

func TestIsBlockedTaskAndFlag(t *testing.T) {
	failed := socket.TaskListSummary{State: "failed"}
	if !isBlockedTask(failed) {
		t.Error("failed task should be blocked")
	}
	if statusFlag(failed) != "FAIL" {
		t.Errorf("statusFlag(failed) = %q", statusFlag(failed))
	}

	prompt := socket.TaskListSummary{State: "reviewing", PendingPromptID: "p1"}
	if !isBlockedTask(prompt) {
		t.Error("prompting task should be blocked")
	}
	if statusFlag(prompt) != "PROMPT" {
		t.Errorf("statusFlag(prompt) = %q", statusFlag(prompt))
	}

	warn := socket.TaskListSummary{State: "implementing", LastError: "x"}
	if statusFlag(warn) != "WARN" {
		t.Errorf("statusFlag(warn) = %q", statusFlag(warn))
	}

	hard := socket.TaskListSummary{State: "implementing", LastFailureClass: "hard_stop"}
	if !isBlockedTask(hard) || statusFlag(hard) != "BLOCK" {
		t.Errorf("hard_stop task flag = %q", statusFlag(hard))
	}
}

// --- chat history ---

func TestRunChatHistory_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.history", map[string]any{
		"messages": []any{
			map[string]any{"id": "m1", "role": "user", "content": "hello", "timestamp": "2026-05-01T10:00:00Z"},
			map[string]any{"id": "m2", "role": "assistant", "content": "hi there", "timestamp": "2026-05-01T10:00:05Z"},
		},
		"task_id": "t1",
	})

	out := captureStdout(t, func() {
		if err := runChatHistory(chatHistoryCmd, nil); err != nil {
			t.Errorf("runChatHistory: %v", err)
		}
	})
	if !strings.Contains(out, "hello") || !strings.Contains(out, "hi there") {
		t.Errorf("chat history output:\n%s", out)
	}
}

// --- logs ---

func TestRunLogs_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.history", map[string]any{
		"messages": []any{
			map[string]any{"id": "m1", "role": "assistant", "content": "did some work", "timestamp": "2026-05-01T10:00:00Z"},
		},
		"task_id": "t1",
	})

	out := captureStdout(t, func() {
		if err := runLogs(LogsCmd, nil); err != nil {
			t.Errorf("runLogs: %v", err)
		}
	})
	if !strings.Contains(out, "Activity log") || !strings.Contains(out, "did some work") {
		t.Errorf("logs output:\n%s", out)
	}
}

func TestRunLogs_Empty(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.history", map[string]any{"messages": []any{}, "task_id": "t1"})

	out := captureStdout(t, func() {
		if err := runLogs(LogsCmd, nil); err != nil {
			t.Errorf("runLogs: %v", err)
		}
	})
	if !strings.Contains(out, "No activity logs") {
		t.Errorf("logs empty output = %q", out)
	}
}

// --- catalog ---

func TestRunCatalogUse_NoSource(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("catalog.get", map[string]any{"name": "bugfix", "description": "fix bugs"})

	out := captureStdout(t, func() {
		if err := runCatalogUse(nil, []string{"bugfix"}); err != nil {
			t.Errorf("runCatalogUse: %v", err)
		}
	})
	if !strings.Contains(out, "no source") {
		t.Errorf("catalog use (no source) output = %q", out)
	}
}

func TestRunCatalogList_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("catalog.list", map[string]any{"items": []any{}})

	_ = captureStdout(t, func() {
		if err := runCatalogList(nil, nil); err != nil {
			t.Errorf("runCatalogList: %v", err)
		}
	})
}

// --- group ---

func TestRunGroupCreate_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.create", map[string]any{"id": "g1", "label": "My Group"})

	out := captureStdout(t, func() {
		if err := runGroupCreate(nil, []string{"My Group"}); err != nil {
			t.Errorf("runGroupCreate: %v", err)
		}
	})
	if !strings.Contains(out, "Created group g1") {
		t.Errorf("group create output = %q", out)
	}
}

func TestRunGroupList_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.list", map[string]any{"groups": []any{}})

	_ = captureStdout(t, func() {
		if err := runGroupList(nil, nil); err != nil {
			t.Errorf("runGroupList: %v", err)
		}
	})
}

// --- codegraph ---

func TestRunCodegraphIndex_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.index", map[string]any{"indexed": 12, "files": 3})

	_ = captureStdout(t, func() {
		if err := runCodegraphIndex(CodegraphCmd, nil); err != nil {
			t.Errorf("runCodegraphIndex: %v", err)
		}
	})
}

// --- screenshots ---

func TestRunScreenshotsList_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.list", map[string]any{"screenshots": []any{}})

	_ = captureStdout(t, func() {
		if err := runScreenshotsList(screenshotsListCmd, nil); err != nil {
			t.Errorf("runScreenshotsList: %v", err)
		}
	})
}

// --- browser (global socket) ---

func TestRunBrowserNavigate_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.navigate", map[string]any{"url": "https://example.com", "title": "Example"})

	out := captureStdout(t, func() {
		if err := runBrowserNavigate(BrowserCmd, []string{"https://example.com"}); err != nil {
			t.Errorf("runBrowserNavigate: %v", err)
		}
	})
	if !strings.Contains(out, "example.com") {
		t.Errorf("browser navigate output = %q", out)
	}
}

func TestRunBrowserSnapshot_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.snapshot", map[string]any{"snapshot": "<html></html>"})

	out := captureStdout(t, func() {
		if err := runBrowserSnapshot(BrowserCmd, nil); err != nil {
			t.Errorf("runBrowserSnapshot: %v", err)
		}
	})
	if !strings.Contains(out, "<html>") {
		t.Errorf("browser snapshot output = %q", out)
	}
}

func TestRunBrowserClick_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.click", map[string]any{"ok": true})

	_ = captureStdout(t, func() {
		if err := runBrowserClick(BrowserCmd, []string{"Submit"}); err != nil {
			t.Errorf("runBrowserClick: %v", err)
		}
	})
}

func TestRunBrowserConfig_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.config.get", map[string]any{
		"headless": true, "browser": "chromium", "profile": "default", "timeout": 30,
	})

	out := captureStdout(t, func() {
		if err := runBrowserConfig(BrowserCmd, nil); err != nil {
			t.Errorf("runBrowserConfig: %v", err)
		}
	})
	if !strings.Contains(out, "headless = true") || !strings.Contains(out, "chromium") {
		t.Errorf("browser config output:\n%s", out)
	}
}

func TestRunBrowserConfigSet_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.config.set", map[string]any{"headless": false})

	out := captureStdout(t, func() {
		if err := runBrowserConfigSet(BrowserCmd, []string{"headless", "false"}); err != nil {
			t.Errorf("runBrowserConfigSet: %v", err)
		}
	})
	if !strings.Contains(out, "headless = false") {
		t.Errorf("browser config set output = %q", out)
	}
}

func TestRunBrowserConfigSet_WrongArgs(t *testing.T) {
	shortKvelmoHome(t)
	_ = startStubGlobalSocket(t)

	if err := runBrowserConfigSet(BrowserCmd, []string{"only-one"}); err == nil {
		t.Fatal("expected error for wrong arg count")
	}
}

func TestRunBrowserStatus_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.status", map[string]any{"running": false})

	_ = captureStdout(t, func() {
		if err := runBrowserStatus(BrowserCmd, nil); err != nil {
			t.Errorf("runBrowserStatus: %v", err)
		}
	})
}

func TestRunBrowserInstall_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.install", map[string]any{"installed": true})

	_ = captureStdout(t, func() {
		if err := runBrowserInstall(BrowserCmd, nil); err != nil {
			t.Errorf("runBrowserInstall: %v", err)
		}
	})
}

// --- backup / restore / export / report ---

func TestRunSecurityScan_Findings(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("security.scan", map[string]any{"findings": []any{}})

	_ = captureStdout(t, func() {
		if err := runSecurityScan(SecurityCmd, nil); err != nil {
			t.Errorf("runSecurityScan: %v", err)
		}
	})
}

// --- memory outcomes ---

func TestRunMemoryOutcomes_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.outcomes", map[string]any{"outcomes": []any{}})

	_ = captureStdout(t, func() {
		if err := runMemoryOutcomes(nil, nil); err != nil {
			t.Errorf("runMemoryOutcomes: %v", err)
		}
	})
}

// --- review list / risk ---

func TestRunReviewList_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("review.list", map[string]any{"reviews": []any{}})

	_ = captureStdout(t, func() {
		if err := runReviewList(ReviewCmd, nil); err != nil {
			t.Errorf("runReviewList: %v", err)
		}
	})
}

func TestRunReviewRisk_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("risk.evaluate", map[string]any{"score": 0.3, "level": "low"})

	_ = captureStdout(t, func() {
		if err := runReviewRisk(nil, nil); err != nil {
			t.Errorf("runReviewRisk: %v", err)
		}
	})
}

// --- quick happy path ---

func TestRunQuick_WithSocket(t *testing.T) {
	origText, origSource := quickText, quickSource
	t.Cleanup(func() { quickText, quickSource = origText, origSource })
	quickText, quickSource = "", ""

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("start", map[string]any{"task_id": "t1", "state": "loaded"})

	out := captureStdout(t, func() {
		if err := runQuick(QuickCmd, []string{"fix", "typo"}); err != nil {
			t.Errorf("runQuick: %v", err)
		}
	})
	if !strings.Contains(out, "Quick mode") {
		t.Errorf("quick output:\n%s", out)
	}
}

// --- provision preview (start --provision-preview) ---

func TestRunProvisionPreview_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("provision.preview", map[string]any{"actions": []any{}})

	out := captureStdout(t, func() {
		if err := runProvisionPreview(); err != nil {
			t.Errorf("runProvisionPreview: %v", err)
		}
	})
	if !strings.Contains(out, "actions") {
		t.Errorf("provision preview output = %q", out)
	}
}

// --- cleanup ---

func TestRunCleanup_NoStale(t *testing.T) {
	shortKvelmoHome(t)
	if err := CleanupCmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CleanupCmd.Flags().Set("dry-run", "false") })

	out := captureStdout(t, func() {
		if err := runCleanup(CleanupCmd, nil); err != nil {
			t.Errorf("runCleanup: %v", err)
		}
	})
	if !strings.Contains(out, "No stale sockets") {
		t.Errorf("cleanup output = %q", out)
	}
}

// --- batch ---

func TestRunBatch_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.batch", map[string]any{
		"results": []any{map[string]any{"id": "t1", "status": "ok"}},
	})

	_ = captureStdout(t, func() {
		if err := runBatch(BatchCmd, []string{"status"}); err != nil {
			t.Errorf("runBatch: %v", err)
		}
	})
}

// --- upgrade pure helpers ---

func TestResolveGitHubToken(t *testing.T) {
	home := shortKvelmoHome(t)
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("GITHUB_TOKEN=ghp_fromenv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := resolveGitHubToken(); got != "ghp_fromenv" {
		t.Errorf("resolveGitHubToken() = %q, want ghp_fromenv", got)
	}
}

// --- list history filter routes to search ---

func TestRunListHistoryCmd_FilterRoutesToSearch(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.search", map[string]any{"tasks": []any{}, "count": 0})

	if err := listHistoryCmd.Flags().Set("state", "finished"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listHistoryCmd.Flags().Set("state", "") })

	out := captureStdout(t, func() {
		if err := runListHistoryCmd(listHistoryCmd, nil); err != nil {
			t.Errorf("runListHistoryCmd: %v", err)
		}
	})
	if !strings.Contains(out, "No matching archived tasks") {
		t.Errorf("list history (filtered) output = %q", out)
	}
}
