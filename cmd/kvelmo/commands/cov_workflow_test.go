package commands

import (
	"strings"
	"testing"
)

// TestRunPlan_WithSocket drives the plan happy path against a stub worktree
// socket and asserts the job-id confirmation is printed.
func TestRunPlan_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("plan", map[string]any{"job_id": "job-plan-1"})

	out := captureStdout(t, func() {
		if err := runPlan(PlanCmd, nil); err != nil {
			t.Errorf("runPlan: %v", err)
		}
	})
	if !strings.Contains(out, "status") {
		t.Errorf("plan output missing status hint: %q", out)
	}
}

// TestRunPlan_JSON asserts --json prints the raw response.
func TestRunPlan_JSON(t *testing.T) {
	orig := planJSON
	t.Cleanup(func() { planJSON = orig })
	planJSON = true

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("plan", map[string]any{"job_id": "job-json-1"})

	out := captureStdout(t, func() {
		if err := runPlan(PlanCmd, nil); err != nil {
			t.Errorf("runPlan json: %v", err)
		}
	})
	if !strings.Contains(out, "job-json-1") {
		t.Errorf("plan --json output missing job id: %q", out)
	}
}

// TestRunPlan_ServerError drives the error branch.
func TestRunPlan_ServerError(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("plan", -32000, "boom")

	err := runPlan(PlanCmd, nil)
	if err == nil {
		t.Fatal("expected error when plan call fails")
	}
	if !strings.Contains(err.Error(), "plan call") {
		t.Errorf("error = %q, want 'plan call'", err.Error())
	}
}

func TestRunImplement_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("implement", map[string]any{"job_id": "job-impl-1"})

	out := captureStdout(t, func() {
		if err := runImplement(ImplementCmd, nil); err != nil {
			t.Errorf("runImplement: %v", err)
		}
	})
	if !strings.Contains(out, "status") {
		t.Errorf("implement output missing status hint: %q", out)
	}
}

func TestRunImplement_ServerError(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("implement", -32000, "boom")

	if err := runImplement(ImplementCmd, nil); err == nil {
		t.Fatal("expected error when implement call fails")
	}
}

func TestRunSimplify_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("simplify", map[string]any{"job_id": "job-simp-1"})

	out := captureStdout(t, func() {
		if err := runSimplify(SimplifyCmd, nil); err != nil {
			t.Errorf("runSimplify: %v", err)
		}
	})
	if out == "" {
		t.Error("simplify produced no output")
	}
}

func TestRunOptimize_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("optimize", map[string]any{"job_id": "job-opt-1"})

	out := captureStdout(t, func() {
		if err := runOptimize(OptimizeCmd, nil); err != nil {
			t.Errorf("runOptimize: %v", err)
		}
	})
	if out == "" {
		t.Error("optimize produced no output")
	}
}

// TestRunSubmit drives the PR-url happy path and dry-run preview.
func TestRunSubmit_PRCreated(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("submit", map[string]any{
		"url":   "https://example.com/pr/1",
		"title": "My PR",
	})

	out := captureStdout(t, func() {
		if err := runSubmit(SubmitCmd, nil); err != nil {
			t.Errorf("runSubmit: %v", err)
		}
	})
	if !strings.Contains(out, "https://example.com/pr/1") {
		t.Errorf("submit output missing PR url: %q", out)
	}
	if !strings.Contains(out, "My PR") {
		t.Errorf("submit output missing PR title: %q", out)
	}
}

func TestRunSubmit_DryRunPreview(t *testing.T) {
	if err := SubmitCmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SubmitCmd.Flags().Set("dry-run", "false") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("submit", map[string]any{
		"preview": map[string]any{
			"title":       "Preview Title",
			"branch":      "feature/x",
			"base_branch": "main",
			"body":        "body text",
		},
	})

	out := captureStdout(t, func() {
		if err := runSubmit(SubmitCmd, nil); err != nil {
			t.Errorf("runSubmit dry-run: %v", err)
		}
	})
	if !strings.Contains(out, "PR Preview (dry-run)") {
		t.Errorf("submit dry-run output missing preview header: %q", out)
	}
	if !strings.Contains(out, "Preview Title") {
		t.Errorf("submit dry-run missing title: %q", out)
	}
}

func TestRunSubmit_ServerError(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("submit", -32000, "boom")

	if err := runSubmit(SubmitCmd, nil); err == nil {
		t.Fatal("expected error when submit call fails")
	}
}

// TestRunUndoRedo drives undo/redo happy + error.
func TestRunUndo_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("undo", map[string]any{"state": "planning"})

	out := captureStdout(t, func() {
		if err := runUndo(UndoCmd, nil); err != nil {
			t.Errorf("runUndo: %v", err)
		}
	})
	if !strings.Contains(out, "Undo:") {
		t.Errorf("undo output = %q", out)
	}
}

func TestRunUndo_ServerError(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("undo", -32000, "boom")

	if err := runUndo(UndoCmd, nil); err == nil {
		t.Fatal("expected error from undo")
	}
}

func TestRunRedo_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("redo", map[string]any{"state": "implementing"})

	out := captureStdout(t, func() {
		if err := runRedo(RedoCmd, nil); err != nil {
			t.Errorf("runRedo: %v", err)
		}
	})
	if !strings.Contains(out, "Redo:") {
		t.Errorf("redo output = %q", out)
	}
}

func TestRunAbort_WithSocket(t *testing.T) {
	if err := AbortCmd.Flags().Set("force", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = AbortCmd.Flags().Set("force", "false") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("abort", map[string]any{"status": "aborted", "state": "paused"})

	out := captureStdout(t, func() {
		if err := runAbort(AbortCmd, nil); err != nil {
			t.Errorf("runAbort: %v", err)
		}
	})
	if !strings.Contains(out, "Task aborted") {
		t.Errorf("abort output = %q", out)
	}
}

func TestRunReset_WithSocket(t *testing.T) {
	if err := ResetCmd.Flags().Set("force", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ResetCmd.Flags().Set("force", "false") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("reset", map[string]any{"state": "none"})

	out := captureStdout(t, func() {
		if err := runReset(ResetCmd, nil); err != nil {
			t.Errorf("runReset: %v", err)
		}
	})
	if !strings.Contains(out, "Task reset") {
		t.Errorf("reset output = %q", out)
	}
}

func TestRunStop_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("stop", map[string]any{"ok": true})

	_ = captureStdout(t, func() {
		if err := runStop(StopCmd, nil); err != nil {
			t.Errorf("runStop: %v", err)
		}
	})
}

func TestRunAbandon_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("abandon", map[string]any{"ok": true})

	_ = captureStdout(t, func() {
		if err := runAbandon(AbandonCmd, nil); err != nil {
			t.Errorf("runAbandon: %v", err)
		}
	})
}

func TestRunDelete_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("delete", map[string]any{"deleted": true})

	_ = captureStdout(t, func() {
		if err := runDelete(DeleteCmd, nil); err != nil {
			t.Errorf("runDelete: %v", err)
		}
	})
}

func TestRunUpdate_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("update", map[string]any{"updated": false})

	_ = captureStdout(t, func() {
		if err := runUpdate(UpdateCmd, nil); err != nil {
			t.Errorf("runUpdate: %v", err)
		}
	})
}

// TestRunStatus_WithSocket exercises the formatted status output.
func TestRunStatus_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{
		"path":  "/proj",
		"state": "implementing",
		"task": map[string]any{
			"id":     "t1",
			"title":  "Fix bug",
			"source": "github",
		},
		"active_job_id":      "job-1",
		"queue_depth":        2,
		"last_error":         "transient",
		"last_failure_class": "recoverable",
		"skip_phases":        []string{"simplify"},
	})

	out := captureStdout(t, func() {
		if err := runStatus(StatusCmd, nil); err != nil {
			t.Errorf("runStatus: %v", err)
		}
	})
	for _, want := range []string{"Fix bug", "github", "job-1", "Queue: 2", "transient", "auto-retry", "simplify"} {
		if !strings.Contains(out, want) {
			t.Errorf("status output missing %q in:\n%s", want, out)
		}
	}
}

// TestRunStatus_MutualExclusion verifies the flag-validation error path.
func TestRunStatus_MutualExclusion(t *testing.T) {
	origF, origB := statusFailed, statusBlocked
	t.Cleanup(func() { statusFailed, statusBlocked = origF, origB })
	statusFailed, statusBlocked = true, true

	if err := runStatus(StatusCmd, nil); err == nil {
		t.Fatal("expected error for --failed + --blocked")
	}
}

func TestRunStatus_ServerError(t *testing.T) {
	origF, origB, origA := statusFailed, statusBlocked, statusAll
	t.Cleanup(func() { statusFailed, statusBlocked, statusAll = origF, origB, origA })
	statusFailed, statusBlocked, statusAll = false, false, false

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("status", -32000, "boom")

	if err := runStatus(StatusCmd, nil); err == nil {
		t.Fatal("expected error from status call")
	}
}

// TestRunRecap_WithSocket exercises printRecap via the recap command.
func TestRunRecap_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("recap", map[string]any{
		"state": "implementing",
		"task": map[string]any{
			"title":  "Add feature",
			"source": "linear",
			"branch": "feat/x",
		},
		"tags":             []string{"backend"},
		"last_activity":    "wrote code",
		"checkpoint_count": 3,
		"last_checkpoint": map[string]any{
			"sha":     "deadbeefcafebabe",
			"message": "impl",
		},
		"files_changed": []any{
			map[string]any{"status": "added", "path": "a.go"},
			map[string]any{"status": "modified", "path": "b.go"},
		},
		"next_action": "run review",
	})

	out := captureStdout(t, func() {
		if err := runRecap(RecapCmd, nil); err != nil {
			t.Errorf("runRecap: %v", err)
		}
	})
	for _, want := range []string{"Add feature", "feat/x", "backend", "Checkpoints: 3", "Files changed", "run review"} {
		if !strings.Contains(out, want) {
			t.Errorf("recap output missing %q in:\n%s", want, out)
		}
	}
}

func TestRunRecap_ServerError(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("recap", -32000, "boom")

	if err := runRecap(RecapCmd, nil); err == nil {
		t.Fatal("expected error from recap")
	}
}

// TestRunRetry_NotFailed asserts retry refuses when the task isn't failed.
func TestRunRetry_NotFailed(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{"state": "implementing"})

	err := runRetry(RetryCmd, nil)
	if err == nil {
		t.Fatal("expected error when task not in failed state")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("error = %q, want mention of failed state", err.Error())
	}
}

// TestRunRetry_HappyPath drives reset + phase re-run for a failed task.
func TestRunRetry_HappyPath(t *testing.T) {
	origPhase := retryPhase
	t.Cleanup(func() { retryPhase = origPhase })
	retryPhase = phaseImplement

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{"state": "failed", "last_error": "implement boom"})
	stub.SetResponse("reset", map[string]any{"status": "ok", "state": "planned"})
	stub.SetResponse("implement", map[string]any{"job_id": "retry-job-1"})

	out := captureStdout(t, func() {
		if err := runRetry(RetryCmd, nil); err != nil {
			t.Errorf("runRetry: %v", err)
		}
	})
	if !strings.Contains(out, "status") {
		t.Errorf("retry output missing progress hint: %q", out)
	}
}
