package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- chat history: no global socket error branch ---

func TestRunChatHistory_NoSocketFill(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	if err := runChatHistory(chatHistoryCmd, nil); err == nil {
		t.Fatal("expected error when no global socket is running")
	}
}

// --- chat history: system role label + content truncation + --limit ---

func TestRunChatHistory_SystemRoleTruncateLimit(t *testing.T) {
	if err := chatHistoryCmd.Flags().Set("limit", "2"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = chatHistoryCmd.Flags().Set("limit", "0") })

	longContent := strings.Repeat("x", 600) // > 500 -> truncated in display

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.history", map[string]any{
		"messages": []any{
			map[string]any{"id": "m0", "role": "user", "content": "first", "timestamp": "2026-05-01T10:00:00Z"},
			map[string]any{"id": "m1", "role": "system", "content": "system note", "timestamp": "2026-05-01T10:00:01Z"},
			map[string]any{"id": "m2", "role": "assistant", "content": longContent, "timestamp": "2026-05-01T10:00:02Z"},
		},
		"task_id": "t1",
	})

	out := captureStdout(t, func() {
		if err := runChatHistory(chatHistoryCmd, nil); err != nil {
			t.Errorf("runChatHistory: %v", err)
		}
	})
	// limit=2 keeps the last two (System + Assistant); "first" must be dropped.
	if !strings.Contains(out, "System:") || !strings.Contains(out, "...") {
		t.Errorf("chat history (system/truncate/limit) output:\n%s", out)
	}
	if strings.Contains(out, "first") {
		t.Errorf("chat history --limit should have dropped the oldest message:\n%s", out)
	}
}

// --- logs: --json branch ---

func TestRunLogs_JSON(t *testing.T) {
	if err := LogsCmd.Flags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = LogsCmd.Flags().Set("json", "false") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.history", map[string]any{
		"messages": []any{map[string]any{"id": "m1", "role": "user", "content": "hi"}},
		"task_id":  "t1",
	})

	out := captureStdout(t, func() {
		if err := runLogs(LogsCmd, nil); err != nil {
			t.Errorf("runLogs json: %v", err)
		}
	})
	if !strings.Contains(out, "messages") {
		t.Errorf("logs json output:\n%s", out)
	}
}

// --- logs: default truncation of long content + system/user roles + limit ---

func TestRunLogs_TruncateAndRoles(t *testing.T) {
	if err := LogsCmd.Flags().Set("limit", "2"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = LogsCmd.Flags().Set("limit", "0") })

	longContent := strings.Repeat("y", 300) // > 200 -> truncated unless --full

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.history", map[string]any{
		"messages": []any{
			map[string]any{"id": "m0", "role": "user", "content": "oldest", "timestamp": "bad-timestamp"},
			map[string]any{"id": "m1", "role": "system", "content": "sys", "timestamp": "2026-05-01T10:00:01Z"},
			map[string]any{"id": "m2", "role": "assistant", "content": longContent, "timestamp": "2026-05-01T10:00:02Z"},
		},
		"task_id": "t1",
	})

	out := captureStdout(t, func() {
		if err := runLogs(LogsCmd, nil); err != nil {
			t.Errorf("runLogs truncate: %v", err)
		}
	})
	if !strings.Contains(out, "showing 2") || !strings.Contains(out, "truncated") {
		t.Errorf("logs truncate output:\n%s", out)
	}
	if !strings.Contains(out, "SYSTEM") || !strings.Contains(out, "AGENT") {
		t.Errorf("logs role labels missing:\n%s", out)
	}
}

// --- logs: --full keeps long content untruncated ---

func TestRunLogs_Full(t *testing.T) {
	if err := LogsCmd.Flags().Set("full", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = LogsCmd.Flags().Set("full", "false") })

	longContent := strings.Repeat("z", 300)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.history", map[string]any{
		"messages": []any{map[string]any{"id": "m1", "role": "assistant", "content": longContent, "timestamp": "2026-05-01T10:00:00Z"}},
		"task_id":  "t1",
	})

	out := captureStdout(t, func() {
		if err := runLogs(LogsCmd, nil); err != nil {
			t.Errorf("runLogs full: %v", err)
		}
	})
	if strings.Contains(out, "truncated") {
		t.Errorf("logs --full should not truncate:\n%s", out)
	}
}

// --- stats history: enabled but no entries ---

func TestRunStatsHistory_EnabledNoEntries(t *testing.T) {
	origH, origJSON := statsHistory, statsJSON
	t.Cleanup(func() { statsHistory, statsJSON = origH, origJSON })
	statsHistory, statsJSON = true, false

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("metrics.history", map[string]any{"enabled": true, "entries": []any{}})

	out := captureStdout(t, func() {
		if err := runStatsHistory(); err != nil {
			t.Errorf("runStatsHistory empty: %v", err)
		}
	})
	if !strings.Contains(out, "No metrics history entries yet") {
		t.Errorf("stats history empty output = %q", out)
	}
}

// --- stats history: server error ---

func TestRunStatsHistory_Error(t *testing.T) {
	origH := statsHistory
	t.Cleanup(func() { statsHistory = origH })
	statsHistory = true

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetError("metrics.history", -32000, "boom")

	if err := runStatsHistory(); err == nil {
		t.Fatal("expected error when metrics.history fails")
	}
}

// --- stats all: no global socket error ---

func TestRunStatsAll_NoSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	if err := runStatsAll(); err == nil {
		t.Fatal("expected error when no global socket for stats --all")
	}
}

// --- group status: --json branch ---

func TestRunGroupStatus_JSON(t *testing.T) {
	setBoolPtr(t, &groupStatusJSON, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.status", map[string]any{"id": "g1", "label": "L", "status": "active"})

	out := captureStdout(t, func() {
		if err := runGroupStatus(nil, []string{"g1"}); err != nil {
			t.Errorf("runGroupStatus json: %v", err)
		}
	})
	if !strings.Contains(out, "\"id\"") {
		t.Errorf("group status json output:\n%s", out)
	}
}

// --- group status: populated tasks loop ---

func TestRunGroupStatus_WithTasks(t *testing.T) {
	setBoolPtr(t, &groupStatusJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.status", map[string]any{
		"id": "g1", "label": "Group One", "status": "active",
		"tasks": []any{
			map[string]any{"task_id": "t1", "state": "implementing", "project_dir": "/p1"},
			map[string]any{"task_id": "t2", "state": "reviewing", "project_dir": "/p2"},
		},
	})

	out := captureStdout(t, func() {
		if err := runGroupStatus(nil, []string{"g1"}); err != nil {
			t.Errorf("runGroupStatus tasks: %v", err)
		}
	})
	if !strings.Contains(out, "Group One") || !strings.Contains(out, "t1") || !strings.Contains(out, "/p2") {
		t.Errorf("group status (with tasks) output:\n%s", out)
	}
}

// --- screenshots delete: failure branch (success=false) ---

func TestRunScreenshotsDelete_Failed(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.delete", map[string]any{"success": false})

	out := captureStdout(t, func() {
		if err := runScreenshotsDelete(screenshotsDeleteCmd, []string{"s9"}); err != nil {
			t.Errorf("runScreenshotsDelete failed: %v", err)
		}
	})
	if !strings.Contains(out, "Failed to delete screenshot s9") {
		t.Errorf("screenshots delete failed output = %q", out)
	}
}

// --- screenshots capture: populated metadata output ---

func TestRunScreenshotsCapture_Populated(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.capture", map[string]any{
		"screenshot": map[string]any{
			"id": "cap1", "filename": "cap1.png", "width": 1024, "height": 768,
			"size_bytes": 8192, "source": "system",
		},
	})

	out := captureStdout(t, func() {
		if err := runScreenshotsCapture(screenshotsCaptureCmd, nil); err != nil {
			t.Errorf("runScreenshotsCapture: %v", err)
		}
	})
	if !strings.Contains(out, "cap1") || !strings.Contains(out, "1024x768") {
		t.Errorf("screenshots capture output:\n%s", out)
	}
}

// --- screenshots get: --output saves the decoded image to disk ---

func TestRunScreenshotsGet_SaveToFile(t *testing.T) {
	orig := screenshotsGetOutput
	t.Cleanup(func() { screenshotsGetOutput = orig })

	shortKvelmoHome(t)
	dir := chdirToShortTemp(t)
	outFile := filepath.Join(dir, "shot.png")
	screenshotsGetOutput = outFile

	stub := startStubWorktreeSocket(t)
	// base64 of "PNGDATA"
	stub.SetResponse("screenshots.get", map[string]any{
		"screenshot": map[string]any{
			"id": "g1", "filename": "g1.png", "width": 100, "height": 100,
			"format": "png", "source": "browser", "step": "after-login", "timestamp": "2026-05-01T10:00:00Z",
		},
		"data": "UE5HREFUQQ==",
	})

	out := captureStdout(t, func() {
		if err := runScreenshotsGet(screenshotsGetCmd, []string{"g1"}); err != nil {
			t.Errorf("runScreenshotsGet save: %v", err)
		}
	})
	if !strings.Contains(out, "Saved to:") || !strings.Contains(out, "after-login") {
		t.Errorf("screenshots get save output:\n%s", out)
	}
	saved, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected saved file: %v", err)
	}
	if string(saved) != "PNGDATA" {
		t.Errorf("saved image data = %q, want PNGDATA", string(saved))
	}
}

// --- screenshots get: --output with invalid base64 -> decode error ---

func TestRunScreenshotsGet_BadBase64(t *testing.T) {
	orig := screenshotsGetOutput
	t.Cleanup(func() { screenshotsGetOutput = orig })

	shortKvelmoHome(t)
	dir := chdirToShortTemp(t)
	screenshotsGetOutput = filepath.Join(dir, "shot.png")

	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.get", map[string]any{
		"screenshot": map[string]any{"id": "g1", "filename": "g1.png"},
		"data":       "!!!not-valid-base64!!!",
	})

	if err := runScreenshotsGet(screenshotsGetCmd, []string{"g1"}); err == nil {
		t.Fatal("expected decode error for invalid base64 image data")
	}
}
