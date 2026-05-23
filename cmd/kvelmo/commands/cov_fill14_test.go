package commands

import (
	"strings"
	"testing"
)

// --- security: finding with a suggestion -> suggestion line ---

func TestRunSecurityScan_Suggestion(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("security.scan", map[string]any{
		"findings": []any{
			map[string]any{
				"severity": "high", "type": "secret", "file": "a.go", "line": 4,
				"message": "hardcoded token", "suggestion": "use a secret manager",
			},
		},
		"count": 1,
	})

	out := captureStdout(t, func() {
		if err := runSecurityScan(SecurityCmd, nil); err != nil {
			t.Errorf("runSecurityScan suggestion: %v", err)
		}
	})
	if !strings.Contains(out, "use a secret manager") {
		t.Errorf("security suggestion output:\n%s", out)
	}
}

// --- submit: malformed --section warning (no '=' separator) ---

func TestRunSubmit_MalformedSection(t *testing.T) {
	if err := SubmitCmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatal(err)
	}
	if err := SubmitCmd.Flags().Set("section", "NoEqualsSign"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = SubmitCmd.Flags().Set("dry-run", "false")
		_ = SubmitCmd.Flags().Set("section", "")
	})

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("submit", map[string]any{"status": "dry-run"})

	// Malformed section warning goes to stderr; assert the command still completes.
	out := captureStdout(t, func() {
		if err := runSubmit(SubmitCmd, nil); err != nil {
			t.Errorf("runSubmit malformed section: %v", err)
		}
	})
	if !strings.Contains(out, "no preview available") {
		t.Errorf("submit malformed-section output:\n%s", out)
	}
}

// --- backup: explicit output path argument ---

func TestRunBackup_OutputPath(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("backup.create", map[string]any{"path": "/tmp/b.bk"})

	out := captureStdout(t, func() {
		if err := runBackup(BackupCmd, []string{"/tmp/b.bk"}); err != nil {
			t.Errorf("runBackup output-path: %v", err)
		}
	})
	if out == "" {
		t.Error("backup with output path produced no output")
	}
}

// --- browse: positional path argument + populated entries ---

func TestRunBrowse_PathArg(t *testing.T) {
	setBoolPtr(t, &browseFiles, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("browse", map[string]any{"entries": []any{"a/", "b/"}})

	out := captureStdout(t, func() {
		if err := runBrowse(BrowseCmd, []string{"subdir"}); err != nil {
			t.Errorf("runBrowse path-arg: %v", err)
		}
	})
	if !strings.Contains(out, "a/") {
		t.Errorf("browse path-arg output:\n%s", out)
	}
}

// --- files list: positional path argument ---

func TestRunFilesList_PathArg(t *testing.T) {
	origJSON := filesListJSON
	t.Cleanup(func() { filesListJSON = origJSON })
	filesListJSON = false

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("files.list", map[string]any{"files": []any{"x.go"}})

	out := captureStdout(t, func() {
		if err := runFilesList(filesListCmd, []string{"internal"}); err != nil {
			t.Errorf("runFilesList path-arg: %v", err)
		}
	})
	if !strings.Contains(out, "x.go") {
		t.Errorf("files list path-arg output:\n%s", out)
	}
}

// --- memory clear: cleared ok branch ---

func TestRunMemoryClear_OK(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.clear", map[string]any{"ok": true})

	out := captureStdout(t, func() {
		if err := runMemoryClear(memoryClearCmd, nil); err != nil {
			t.Errorf("runMemoryClear ok: %v", err)
		}
	})
	if !strings.Contains(out, "Memory store cleared") {
		t.Errorf("memory clear ok output = %q", out)
	}
}

// --- activity: empty entries ---

func TestRunActivity_Empty(t *testing.T) {
	setBoolPtr(t, &activityJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("activity.query", map[string]any{"entries": []any{}, "count": 0, "enabled": true})

	out := captureStdout(t, func() {
		if err := runActivity(ActivityCmd, nil); err != nil {
			t.Errorf("runActivity empty: %v", err)
		}
	})
	if !strings.Contains(out, "No activity entries found") {
		t.Errorf("activity empty output = %q", out)
	}
}

// --- quality respond: --yes branch ---

func TestRunQualityRespond_Yes(t *testing.T) {
	if err := qualityRespondCmd.Flags().Set("prompt-id", "p2"); err != nil {
		t.Fatal(err)
	}
	if err := qualityRespondCmd.Flags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = qualityRespondCmd.Flags().Set("yes", "false")
		_ = qualityRespondCmd.Flags().Set("prompt-id", "")
	})

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("quality.respond", map[string]any{"ok": true})

	out := captureStdout(t, func() {
		if err := runQualityRespond(qualityRespondCmd, nil); err != nil {
			t.Errorf("runQualityRespond yes: %v", err)
		}
	})
	if !strings.Contains(out, "Answered: yes") {
		t.Errorf("quality respond --yes output = %q", out)
	}
}

// --- prompt: empty/none state -> no output ---

func TestRunPrompt_EmptyState(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{"state": "none"})

	out := captureStdout(t, func() {
		if err := runPrompt(PromptCmd, nil); err != nil {
			t.Errorf("runPrompt empty-state: %v", err)
		}
	})
	if out != "" {
		t.Errorf("prompt empty-state should produce no output, got %q", out)
	}
}
