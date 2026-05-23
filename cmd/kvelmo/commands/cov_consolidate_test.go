package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/socket"
)

// --- config validate: JSON + provider token configured + invalid agent ---

func TestRunConfigValidate_JSON(t *testing.T) {
	setBoolPtr(t, &configValidateJSON, true)
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	out := captureStdout(t, func() {
		_ = runConfigValidate(nil, nil)
	})
	if !strings.Contains(out, "\"valid\"") || !strings.Contains(out, "\"checks\"") {
		t.Errorf("config validate json output:\n%s", out)
	}
}

func TestRunConfigValidate_TokenConfigured(t *testing.T) {
	setBoolPtr(t, &configValidateJSON, false)
	home := shortKvelmoHome(t)
	chdirToShortTemp(t)
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("GITHUB_TOKEN=ghp_xyz123\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		_ = runConfigValidate(nil, nil)
	})
	if !strings.Contains(out, "token configured") {
		t.Errorf("config validate token-configured output:\n%s", out)
	}
}

// --- screenshots get with nested screenshot object ---

func TestRunScreenshotsGet_Nested(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.get", map[string]any{
		"screenshot": map[string]any{
			"id": "s1", "filename": "s1.png", "width": 800, "height": 600,
			"size_bytes": 4096, "format": "png", "source": "browser", "step": "login",
			"timestamp": "2026-05-01T10:00:00Z",
		},
	})

	out := captureStdout(t, func() {
		if err := runScreenshotsGet(screenshotsGetCmd, []string{"s1"}); err != nil {
			t.Errorf("runScreenshotsGet: %v", err)
		}
	})
	if !strings.Contains(out, "s1.png") || !strings.Contains(out, "800x600") || !strings.Contains(out, "login") {
		t.Errorf("screenshots get nested output:\n%s", out)
	}
}

// --- refresh with PR status ---

func TestRunRefresh_Populated(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.refresh", map[string]any{
		"task_id": "t1", "branch": "feat/x", "pr_status": "open", "pr_merged": false,
		"pr_url": "https://x/pr/1", "commits_behind_base": 2, "action": "none",
		"message": "PR still open",
	})

	out := captureStdout(t, func() {
		if err := runRefresh(RefreshCmd, nil); err != nil {
			t.Errorf("runRefresh: %v", err)
		}
	})
	if out == "" {
		t.Error("refresh populated produced no output")
	}
}

// --- finish with PR merged ---

func TestRunFinish_Populated(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.finish", map[string]any{
		"state": "finished", "branch_deleted": true, "worktree_removed": true,
	})

	out := captureStdout(t, func() {
		if err := runFinish(FinishCmd, nil); err != nil {
			t.Errorf("runFinish: %v", err)
		}
	})
	if out == "" {
		t.Error("finish populated produced no output")
	}
}

// --- status with last-failure-class variants ---

func TestRunStatus_FailureClasses(t *testing.T) {
	origF, origB, origA := statusFailed, statusBlocked, statusAll
	t.Cleanup(func() { statusFailed, statusBlocked, statusAll = origF, origB, origA })
	statusFailed, statusBlocked, statusAll = false, false, false

	for _, fc := range []string{"hard_stop", "recoverable", "degraded", "skippable", "other"} {
		shortKvelmoHome(t)
		chdirToShortTemp(t)
		stub := startStubWorktreeSocket(t)
		stub.SetResponse("status", map[string]any{
			"state": "failed", "path": "/p", "last_failure_class": fc,
		})

		out := captureStdout(t, func() {
			if err := runStatus(StatusCmd, nil); err != nil {
				t.Errorf("runStatus %s: %v", fc, err)
			}
		})
		if !strings.Contains(out, "Failure:") {
			t.Errorf("status %s output missing failure line:\n%s", fc, out)
		}
	}
}

// --- quick with context items (file) ---

func TestRunQuick_WithContextFile(t *testing.T) {
	origText, origSource, origFiles := quickText, quickSource, quickContextFiles
	t.Cleanup(func() { quickText, quickSource, quickContextFiles = origText, origSource, origFiles })

	shortKvelmoHome(t)
	dir := chdirToShortTemp(t)
	// Create a real file so validateContextItems passes.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	quickText, quickSource, quickContextFiles = "fix it", "", []string{"main.go"}

	stub := startStubWorktreeSocket(t)
	stub.SetResponse("start", map[string]any{"state": "loaded"})

	out := captureStdout(t, func() {
		if err := runQuick(QuickCmd, nil); err != nil {
			t.Errorf("runQuick with context: %v", err)
		}
	})
	if !strings.Contains(out, "Quick mode") {
		t.Errorf("quick with context output:\n%s", out)
	}
}

// --- quick context validation failure (missing file) ---

func TestRunQuick_BadContextFile(t *testing.T) {
	origText, origSource, origFiles := quickText, quickSource, quickContextFiles
	t.Cleanup(func() { quickText, quickSource, quickContextFiles = origText, origSource, origFiles })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	quickText, quickSource, quickContextFiles = "fix", "", []string{"does-not-exist.go"}

	_ = startStubWorktreeSocket(t)

	if err := runQuick(QuickCmd, nil); err == nil {
		t.Fatal("expected error for missing context file")
	}
}

// --- loadTaskViaRPC with bad context file ---

func TestLoadTaskViaRPC_BadContext(t *testing.T) {
	origFiles := startContextFiles
	t.Cleanup(func() { startContextFiles = origFiles })
	startContextFiles = []string{"/etc/passwd"} // absolute → rejected

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	_ = startStubWorktreeSocket(t)

	cwd, _ := os.Getwd()
	if err := loadTaskViaRPC(socket.WorktreeSocketPath(cwd), "empty:x"); err == nil {
		t.Fatal("expected error for absolute context path")
	}
}
