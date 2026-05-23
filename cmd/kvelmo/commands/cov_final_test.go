package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- config validate (offline, full body) ---

func TestRunConfigValidate_Offline(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	out := captureStdout(t, func() {
		_ = runConfigValidate(nil, nil)
	})
	if !strings.Contains(out, "Settings") {
		t.Errorf("config validate output:\n%s", out)
	}
}

// --- agent status with checks + JSON ---

func TestRunAgentStatus_Checks(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("agent.status", map[string]any{
		"agent_available": false,
		"simulation_mode": true,
		"checks": []any{
			map[string]any{"name": "claude", "status": "ok", "detail": "found"},
			map[string]any{"name": "codex", "status": "fail", "detail": "missing"},
		},
	})

	out := captureStdout(t, func() {
		if err := runAgentStatus(AgentCmd, nil); err != nil {
			t.Errorf("runAgentStatus: %v", err)
		}
	})
	if !strings.Contains(out, "simulation mode") || !strings.Contains(out, "claude") {
		t.Errorf("agent status output:\n%s", out)
	}
}

func TestRunAgentStatus_JSON(t *testing.T) {
	setBoolPtr(t, &agentStatusJSON, true)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("agent.status", map[string]any{"agent_available": true})

	out := captureStdout(t, func() {
		if err := runAgentStatus(AgentCmd, nil); err != nil {
			t.Errorf("runAgentStatus json: %v", err)
		}
	})
	if !strings.Contains(out, "agent_available") {
		t.Errorf("agent status json output:\n%s", out)
	}
}

// --- ci with checks + JSON ---

func TestRunCIStatus_Checks(t *testing.T) {
	setBoolPtr(t, &ciJSON, false)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("ci.status", map[string]any{
		"state": "running",
		"checks": []any{
			map[string]any{"name": "build", "status": "passed", "url": "https://ci/1"},
		},
	})

	out := captureStdout(t, func() {
		if err := runCIStatus(CICmd, nil); err != nil {
			t.Errorf("runCIStatus: %v", err)
		}
	})
	if !strings.Contains(out, "CI Status") || !strings.Contains(out, "build") {
		t.Errorf("ci status output:\n%s", out)
	}
}

func TestRunCIStatus_JSON(t *testing.T) {
	setBoolPtr(t, &ciJSON, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("ci.status", map[string]any{"state": "passed", "checks": []any{}})

	out := captureStdout(t, func() {
		if err := runCIStatus(CICmd, nil); err != nil {
			t.Errorf("runCIStatus json: %v", err)
		}
	})
	if !strings.Contains(out, "state") {
		t.Errorf("ci status json output:\n%s", out)
	}
}

// --- hooks populated + JSON ---

func TestRunHooks_Populated(t *testing.T) {
	setBoolPtr(t, &hooksJSON, false)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("hooks.list", map[string]any{
		"pre_submit": []any{
			map[string]any{"command": "make test", "description": "run tests", "required": true},
		},
	})

	out := captureStdout(t, func() {
		if err := runHooks(HooksCmd, nil); err != nil {
			t.Errorf("runHooks: %v", err)
		}
	})
	if !strings.Contains(out, "pre_submit") || !strings.Contains(out, "required") {
		t.Errorf("hooks output:\n%s", out)
	}
}

func TestRunHooks_JSON(t *testing.T) {
	setBoolPtr(t, &hooksJSON, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("hooks.list", map[string]any{})

	out := captureStdout(t, func() {
		if err := runHooks(HooksCmd, nil); err != nil {
			t.Errorf("runHooks json: %v", err)
		}
	})
	if out == "" {
		t.Error("hooks json produced no output")
	}
}

// --- policy violations + JSON ---

func TestRunPolicyCheck_Violations(t *testing.T) {
	setBoolPtr(t, &policyJSON, false)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("policy.check", map[string]any{
		"violations": []any{
			map[string]any{"severity": "error", "rule": "tests", "message": "no tests"},
			map[string]any{"severity": "warning", "rule": "docs", "message": "no docs"},
		},
	})

	out := captureStdout(t, func() {
		if err := runPolicyCheck(nil, nil); err != nil {
			t.Errorf("runPolicyCheck: %v", err)
		}
	})
	if !strings.Contains(out, "Policy violations (2)") || !strings.Contains(out, "no tests") {
		t.Errorf("policy violations output:\n%s", out)
	}
}

func TestRunPolicyCheck_JSON(t *testing.T) {
	setBoolPtr(t, &policyJSON, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("policy.check", map[string]any{"violations": []any{}})

	out := captureStdout(t, func() {
		if err := runPolicyCheck(nil, nil); err != nil {
			t.Errorf("runPolicyCheck json: %v", err)
		}
	})
	if !strings.Contains(out, "violations") {
		t.Errorf("policy json output:\n%s", out)
	}
}

// --- eventlog JSON ---

func TestRunEventlog_JSON(t *testing.T) {
	setBoolPtr(t, &eventlogJSON, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("eventlog.query", map[string]any{"entries": []any{}, "total": 0})

	out := captureStdout(t, func() {
		if err := runEventlog(EventlogCmd, nil); err != nil {
			t.Errorf("runEventlog json: %v", err)
		}
	})
	if !strings.Contains(out, "entries") {
		t.Errorf("eventlog json output:\n%s", out)
	}
}

// --- recap with all fields ---

func TestPrintRecap_AllFields(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("recap", map[string]any{
		"state": "failed",
		"task": map[string]any{
			"title": "Big task", "source": "github", "branch": "feat/big",
		},
		"tags":             []string{"backend", "urgent"},
		"last_activity":    "5m ago",
		"checkpoint_count": 12,
		"last_checkpoint":  map[string]any{"sha": "abcdef1234567890", "message": "implement"},
		"files_changed": []any{
			map[string]any{"status": "added", "path": "a.go"},
			map[string]any{"status": "modified", "path": "b.go"},
			map[string]any{"status": "deleted", "path": "c.go"},
			map[string]any{"status": "renamed", "path": "d.go"},
			map[string]any{"status": "added", "path": "e.go"},
			map[string]any{"status": "added", "path": "f.go"},
			map[string]any{"status": "added", "path": "g.go"},
			map[string]any{"status": "added", "path": "h.go"},
			map[string]any{"status": "added", "path": "i.go"},
			map[string]any{"status": "added", "path": "j.go"},
			map[string]any{"status": "added", "path": "k.go"},
		},
		"phase_metrics": map[string]any{
			"plan":      map[string]any{},
			"implement": map[string]any{},
		},
		"last_error":  "build failed",
		"next_action": "fix the build",
	})

	out := captureStdout(t, func() {
		if err := runRecap(RecapCmd, nil); err != nil {
			t.Errorf("runRecap: %v", err)
		}
	})
	for _, want := range []string{"Big task", "feat/big", "backend, urgent", "5m ago", "Checkpoints: 12", "and 1 more", "Phases completed", "build failed", "fix the build"} {
		if !strings.Contains(out, want) {
			t.Errorf("recap missing %q in:\n%s", want, out)
		}
	}
}

func TestRunRecap_JSON(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("recap", map[string]any{"state": "planning", "next_action": "implement"})

	if err := RecapCmd.Flags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = RecapCmd.Flags().Set("json", "false") })

	out := captureStdout(t, func() {
		if err := runRecap(RecapCmd, nil); err != nil {
			t.Errorf("runRecap json: %v", err)
		}
	})
	if !strings.Contains(out, "next_action") {
		t.Errorf("recap json output:\n%s", out)
	}
}

// --- screenshots get / capture ---

func TestRunScreenshotsGet_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.get", map[string]any{
		"id": "s1", "path": "/tmp/s1.png", "captured_at": "2026-05-01T10:00:00Z",
	})

	_ = captureStdout(t, func() {
		if err := runScreenshotsGet(screenshotsGetCmd, []string{"s1"}); err != nil {
			t.Errorf("runScreenshotsGet: %v", err)
		}
	})
}

func TestRunScreenshotsList_Items(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.list", map[string]any{
		"screenshots": []any{
			map[string]any{"id": "s1", "path": "/tmp/s1.png", "label": "home", "captured_at": "2026-05-01T10:00:00Z"},
		},
	})

	out := captureStdout(t, func() {
		if err := runScreenshotsList(screenshotsListCmd, nil); err != nil {
			t.Errorf("runScreenshotsList: %v", err)
		}
	})
	if out == "" {
		t.Error("screenshots list (items) produced no output")
	}
}

// --- activity populated ---

func TestRunActivity_Entries(t *testing.T) {
	shortKvelmoHome(t)
	_ = startStubGlobalSocket(t) // default activity.query has 2 entries

	out := captureStdout(t, func() {
		if err := runActivity(ActivityCmd, nil); err != nil {
			t.Errorf("runActivity: %v", err)
		}
	})
	if !strings.Contains(out, "ping") || !strings.Contains(out, "tasks.list") {
		t.Errorf("activity output:\n%s", out)
	}
}

// --- checklist check/uncheck ---

func TestRunChecklist_Check(t *testing.T) {
	if err := ChecklistCmd.Flags().Set("check", "tests pass"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ChecklistCmd.Flags().Set("check", "") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("review.checklist.check", map[string]any{"ok": true})

	out := captureStdout(t, func() {
		if err := runChecklist(ChecklistCmd, nil); err != nil {
			t.Errorf("runChecklist check: %v", err)
		}
	})
	if !strings.Contains(out, "Checked: tests pass") {
		t.Errorf("checklist check output = %q", out)
	}
}

func TestRunChecklist_Uncheck(t *testing.T) {
	if err := ChecklistCmd.Flags().Set("uncheck", "docs"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ChecklistCmd.Flags().Set("uncheck", "") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("review.checklist.uncheck", map[string]any{"ok": true})

	out := captureStdout(t, func() {
		if err := runChecklist(ChecklistCmd, nil); err != nil {
			t.Errorf("runChecklist uncheck: %v", err)
		}
	})
	if !strings.Contains(out, "Unchecked: docs") {
		t.Errorf("checklist uncheck output = %q", out)
	}
}

// --- chat history with limit ---

func TestRunChatHistory_Limit(t *testing.T) {
	if err := chatHistoryCmd.Flags().Set("limit", "1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = chatHistoryCmd.Flags().Set("limit", "50") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.history", map[string]any{
		"messages": []any{
			map[string]any{"id": "m1", "role": "user", "content": "first"},
			map[string]any{"id": "m2", "role": "assistant", "content": "second"},
		},
		"task_id": "t1",
	})

	out := captureStdout(t, func() {
		if err := runChatHistory(chatHistoryCmd, nil); err != nil {
			t.Errorf("runChatHistory: %v", err)
		}
	})
	if out == "" {
		t.Error("chat history (limit) produced no output")
	}
}

// --- browser status running ---

func TestRunBrowserStatus_Running(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.status", map[string]any{
		"running": true, "url": "https://example.com", "title": "Example", "browser": "chromium",
	})

	out := captureStdout(t, func() {
		if err := runBrowserStatus(BrowserCmd, nil); err != nil {
			t.Errorf("runBrowserStatus: %v", err)
		}
	})
	if out == "" {
		t.Error("browser status (running) produced no output")
	}
}

// --- provider login: existing token + override declined ---

func TestRunProviderLogin_ExistingTokenDeclineOverride(t *testing.T) {
	home := shortKvelmoHome(t)
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("GITHUB_TOKEN=ghp_existing12345\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	loginCmd := findProviderLogin(GitHubCmd)
	if loginCmd == nil {
		t.Fatal("github login subcommand missing")
	}
	// Pipe "n\n" so confirmOverride returns false → command prints Cancelled.
	loginCmd.SetIn(strings.NewReader("n\n"))

	var buf strings.Builder
	loginCmd.SetOut(&buf)

	if err := runProviderLogin("github")(loginCmd, nil); err != nil {
		t.Errorf("runProviderLogin decline: %v", err)
	}
	if !strings.Contains(buf.String(), "already configured") || !strings.Contains(buf.String(), "Cancelled") {
		t.Errorf("provider login decline output:\n%s", buf.String())
	}
}

// --- group submit / remove / add ---

func TestRunGroupSubmit_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.submit", map[string]any{"submitted": true, "count": 2})

	_ = captureStdout(t, func() {
		if err := runGroupSubmit(nil, []string{"g1"}); err != nil {
			t.Errorf("runGroupSubmit: %v", err)
		}
	})
}

func TestRunGroupRemove_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.remove", map[string]any{"removed": true})

	_ = captureStdout(t, func() {
		if err := runGroupRemove(nil, []string{"g1"}); err != nil {
			t.Errorf("runGroupRemove: %v", err)
		}
	})
}

// --- restore with socket ---

func TestRunRestore_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("backup.restore", map[string]any{"restored": true, "tasks": 3})

	_ = captureStdout(t, func() {
		if err := runRestore(nil, []string{"/tmp/b.tar.gz"}); err != nil {
			t.Errorf("runRestore: %v", err)
		}
	})
}

// --- catalog add ---

func TestRunCatalogAdd_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("catalog.import", map[string]any{"imported": true, "name": "tmpl"})

	dir := t.TempDir()
	f := filepath.Join(dir, "tmpl.yaml")
	if err := os.WriteFile(f, []byte("name: tmpl\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_ = captureStdout(t, func() {
		if err := runCatalogAdd(nil, []string{f}); err != nil {
			t.Errorf("runCatalogAdd: %v", err)
		}
	})
}
