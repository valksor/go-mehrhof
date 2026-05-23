package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- completion: each supported shell ---

func TestCompletionCmd_Shells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			out := captureStdout(t, func() {
				if err := CompletionCmd.RunE(CompletionCmd, []string{shell}); err != nil {
					t.Errorf("completion %s: %v", shell, err)
				}
			})
			if out == "" {
				t.Errorf("completion %s produced no script", shell)
			}
		})
	}
}

// --- config validate: openai default agent (valid) -> openai display name ---

func TestRunConfigValidate_OpenAIDefault(t *testing.T) {
	setBoolPtr(t, &configValidateJSON, false)
	home := shortKvelmoHome(t)
	chdirToShortTemp(t)
	writeTempFile(t, filepath.Join(home, "kvelmo.yaml"), "version: 1\nagent:\n  default: openai\n")

	out := captureStdout(t, func() {
		if err := runConfigValidate(configValidateCmd, nil); err != nil {
			t.Errorf("runConfigValidate openai: %v", err)
		}
	})
	// agent.default=openai is in the allowed list, so no agent.default error is added.
	if strings.Contains(out, "unknown agent") {
		t.Errorf("openai should be an allowed default agent:\n%s", out)
	}
}

// --- submit --dry-run with no preview payload -> "no preview available" ---

func TestRunSubmit_DryRunNoPreview(t *testing.T) {
	if err := SubmitCmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SubmitCmd.Flags().Set("dry-run", "false") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	// No "preview" key -> falls into the "no preview available" branch.
	stub.SetResponse("submit", map[string]any{"status": "dry-run"})

	out := captureStdout(t, func() {
		if err := runSubmit(SubmitCmd, nil); err != nil {
			t.Errorf("runSubmit dry-run no-preview: %v", err)
		}
	})
	if !strings.Contains(out, "no preview available") {
		t.Errorf("submit dry-run no-preview output:\n%s", out)
	}
}

// --- showAllStatus: no active tasks (all filtered out) ---

func TestShowAllStatus_NoActiveTasks(t *testing.T) {
	setBoolPtr(t, &statusVerbose, false)
	setBoolPtr(t, &statusFailed, false)
	setBoolPtr(t, &statusBlocked, false)
	setBoolPtr(t, &statusJSON, false)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	// Only a "none"-state task, which is filtered out when not verbose.
	stub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{map[string]any{"path": "/p1", "state": "none"}},
	})

	out := captureStdout(t, func() {
		if err := showAllStatus(); err != nil {
			t.Errorf("showAllStatus no-active: %v", err)
		}
	})
	if !strings.Contains(out, "No active tasks across projects") {
		t.Errorf("showAllStatus no-active output = %q", out)
	}
}

// --- screenshots delete: success=true branch ---

func TestRunScreenshotsDelete_Success(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.delete", map[string]any{"success": true})

	out := captureStdout(t, func() {
		if err := runScreenshotsDelete(screenshotsDeleteCmd, []string{"s1"}); err != nil {
			t.Errorf("runScreenshotsDelete success: %v", err)
		}
	})
	if !strings.Contains(out, "Screenshot s1 deleted") {
		t.Errorf("screenshots delete success output = %q", out)
	}
}

// --- screenshots capture: --step flag included in params ---

func TestRunScreenshotsCapture_WithStep(t *testing.T) {
	if err := screenshotsCaptureCmd.Flags().Set("step", "after-login"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = screenshotsCaptureCmd.Flags().Set("step", "") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.capture", map[string]any{
		"screenshot": map[string]any{"id": "c1", "filename": "c1.png", "source": "browser"},
	})

	out := captureStdout(t, func() {
		if err := runScreenshotsCapture(screenshotsCaptureCmd, nil); err != nil {
			t.Errorf("runScreenshotsCapture step: %v", err)
		}
	})
	if !strings.Contains(out, "c1") {
		t.Errorf("screenshots capture step output:\n%s", out)
	}
}

// --- workers remove: failure branch (ok=false) ---

func TestRunWorkersRemove_NotRemoved(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("workers.remove", map[string]any{"ok": false})

	out := captureStdout(t, func() {
		if err := runWorkersRemove(workersRemoveCmd, []string{"w9"}); err != nil {
			t.Errorf("runWorkersRemove not-removed: %v", err)
		}
	})
	if out == "" {
		t.Error("workers remove (not removed) produced no output")
	}
}

// --- queue add: title omitted -> no Title line ---

func TestRunQueueAdd_NoTitle(t *testing.T) {
	orig := queueAddTitle
	t.Cleanup(func() { queueAddTitle = orig })
	queueAddTitle = ""

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("queue.add", map[string]any{"id": "q5", "source": "issue-5"})

	out := captureStdout(t, func() {
		if err := runQueueAdd(queueAddCmd, []string{"issue-5"}); err != nil {
			t.Errorf("runQueueAdd no-title: %v", err)
		}
	})
	if !strings.Contains(out, "Added to queue: q5") || strings.Contains(out, "Title:") {
		t.Errorf("queue add no-title output:\n%s", out)
	}
}

// --- helpers ---

func writeTempFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
