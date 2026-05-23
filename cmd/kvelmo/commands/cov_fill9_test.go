package commands

import (
	"os"
	"strings"
	"testing"
)

// feedStdin temporarily replaces os.Stdin with a pipe carrying the given input.
func feedStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, _ = w.WriteString(input)
	_ = w.Close()
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = orig; _ = r.Close() })
}

// --- CI status: success/failure/pending icons ---

func TestRunCIStatus_Icons(t *testing.T) {
	setBoolPtr(t, &ciJSON, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("ci.status", map[string]any{
		"state": "in_progress",
		"checks": []any{
			map[string]any{"name": "build", "status": "success"},
			map[string]any{"name": "test", "status": "failure"},
			map[string]any{"name": "lint", "status": "pending"},
			map[string]any{"name": "deploy", "status": "weird"},
		},
	})

	out := captureStdout(t, func() {
		if err := runCIStatus(CICmd, nil); err != nil {
			t.Errorf("runCIStatus icons: %v", err)
		}
	})
	if !strings.Contains(out, "[+] build") || !strings.Contains(out, "[-] test") || !strings.Contains(out, "[~] lint") {
		t.Errorf("ci status icons output:\n%s", out)
	}
}

// --- CI status: message short-circuit ---

func TestRunCIStatus_Message(t *testing.T) {
	setBoolPtr(t, &ciJSON, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("ci.status", map[string]any{"message": "No CI configured for this repo"})

	out := captureStdout(t, func() {
		if err := runCIStatus(CICmd, nil); err != nil {
			t.Errorf("runCIStatus message: %v", err)
		}
	})
	if !strings.Contains(out, "No CI configured") {
		t.Errorf("ci status message output = %q", out)
	}
}

// --- finish: remote branch deleted branch ---

func TestRunFinish_RemoteBranchDeleted(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.finish", map[string]any{
		"current_branch":        "main",
		"previous_branch":       "feat/x",
		"branch_deleted":        true,
		"remote_branch_deleted": true,
	})

	out := captureStdout(t, func() {
		if err := runFinish(FinishCmd, nil); err != nil {
			t.Errorf("runFinish remote-deleted: %v", err)
		}
	})
	if !strings.Contains(out, "Deleted remote branch: feat/x") {
		t.Errorf("finish remote-branch output:\n%s", out)
	}
}

// --- refresh: merged / closed next-action branches ---

func TestRunRefresh_MergedAction(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.refresh", map[string]any{
		"task_id": "t1", "branch": "feat/x", "pr_url": "http://pr/1", "pr_status": "merged",
		"commits_behind_base": 2, "action": "merged", "message": "PR merged",
	})

	out := captureStdout(t, func() {
		if err := runRefresh(RefreshCmd, nil); err != nil {
			t.Errorf("runRefresh merged: %v", err)
		}
	})
	if !strings.Contains(out, "kvelmo finish") || !strings.Contains(out, "2 commits behind") {
		t.Errorf("refresh merged output:\n%s", out)
	}
}

func TestRunRefresh_ClosedAction(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.refresh", map[string]any{
		"task_id": "t1", "branch": "feat/x", "action": "closed", "message": "PR closed",
	})

	out := captureStdout(t, func() {
		if err := runRefresh(RefreshCmd, nil); err != nil {
			t.Errorf("runRefresh closed: %v", err)
		}
	})
	if !strings.Contains(out, "kvelmo finish --force") {
		t.Errorf("refresh closed output:\n%s", out)
	}
}

// --- quality respond: --no branch ---

func TestRunQualityRespond_No(t *testing.T) {
	if err := qualityRespondCmd.Flags().Set("prompt-id", "p1"); err != nil {
		t.Fatal(err)
	}
	if err := qualityRespondCmd.Flags().Set("no", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = qualityRespondCmd.Flags().Set("no", "false")
		_ = qualityRespondCmd.Flags().Set("prompt-id", "")
	})

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("quality.respond", map[string]any{"ok": true})

	out := captureStdout(t, func() {
		if err := runQualityRespond(qualityRespondCmd, nil); err != nil {
			t.Errorf("runQualityRespond no: %v", err)
		}
	})
	if !strings.Contains(out, "Answered: no") {
		t.Errorf("quality respond --no output = %q", out)
	}
}

// --- abort / reset: decline confirmation ---

func TestRunAbort_Decline(t *testing.T) {
	if err := AbortCmd.Flags().Set("force", "false"); err != nil {
		t.Fatal(err)
	}
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	_ = startStubWorktreeSocket(t)
	feedStdin(t, "n\n")

	out := captureStdout(t, func() {
		if err := runAbort(AbortCmd, nil); err != nil {
			t.Errorf("runAbort decline: %v", err)
		}
	})
	if !strings.Contains(out, "Cancelled.") {
		t.Errorf("abort decline output = %q", out)
	}
}

func TestRunReset_Decline(t *testing.T) {
	if err := ResetCmd.Flags().Set("force", "false"); err != nil {
		t.Fatal(err)
	}
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	_ = startStubWorktreeSocket(t)
	feedStdin(t, "n\n")

	out := captureStdout(t, func() {
		if err := runReset(ResetCmd, nil); err != nil {
			t.Errorf("runReset decline: %v", err)
		}
	})
	if !strings.Contains(out, "Cancelled.") {
		t.Errorf("reset decline output = %q", out)
	}
}

// --- jobs list: worktree/created dash fallbacks ---

func TestRunJobsList_Dashes(t *testing.T) {
	setBoolPtr(t, &jobsListJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("jobs.list", map[string]any{
		"jobs": []any{
			map[string]any{"id": "j1", "type": "implement", "status": "running"},
		},
	})

	out := captureStdout(t, func() {
		if err := runJobsList(jobsListCmd, nil); err != nil {
			t.Errorf("runJobsList dashes: %v", err)
		}
	})
	if !strings.Contains(out, "j1") {
		t.Errorf("jobs list dashes output:\n%s", out)
	}
}

// --- hooks: empty list ---

func TestRunHooks_Empty(t *testing.T) {
	setBoolPtr(t, &hooksJSON, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("hooks.list", map[string]any{})

	out := captureStdout(t, func() {
		if err := runHooks(HooksCmd, nil); err != nil {
			t.Errorf("runHooks empty: %v", err)
		}
	})
	if !strings.Contains(out, "No workflow hooks configured") {
		t.Errorf("hooks empty output = %q", out)
	}
}

// --- hooks: populated with description fallback to command ---

func TestRunHooks_DescFallback(t *testing.T) {
	setBoolPtr(t, &hooksJSON, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("hooks.list", map[string]any{
		"pre_submit": []any{
			// No description -> falls back to the command string.
			map[string]any{"command": "make lint", "required": true},
		},
	})

	out := captureStdout(t, func() {
		if err := runHooks(HooksCmd, nil); err != nil {
			t.Errorf("runHooks desc-fallback: %v", err)
		}
	})
	if !strings.Contains(out, "make lint") || !strings.Contains(out, "[required]") {
		t.Errorf("hooks desc-fallback output:\n%s", out)
	}
}

// --- projects: empty registry ---

func TestRunProjects_Empty(t *testing.T) {
	setBoolPtr(t, &projectsVerbose, false)
	setBoolPtr(t, &projectsJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("projects.list", map[string]any{"projects": []any{}})

	out := captureStdout(t, func() {
		if err := runProjects(ProjectsCmd, nil); err != nil {
			t.Errorf("runProjects empty: %v", err)
		}
	})
	if !strings.Contains(out, "No projects registered") {
		t.Errorf("projects empty output = %q", out)
	}
}
