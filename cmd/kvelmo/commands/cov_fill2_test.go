package commands

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/socket"
)

// --- showAllStatus: --json branch ---

func TestShowAllStatus_JSON(t *testing.T) {
	setBoolPtr(t, &statusJSON, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{map[string]any{"path": "/p1", "state": "implementing"}},
	})

	out := captureStdout(t, func() {
		if err := showAllStatus(); err != nil {
			t.Errorf("showAllStatus json: %v", err)
		}
	})
	if !strings.Contains(out, "\"tasks\"") {
		t.Errorf("showAllStatus json output:\n%s", out)
	}
}

// --- showAllStatus: --failed filter with no matches ---

func TestShowAllStatus_FailedFilterEmpty(t *testing.T) {
	setBoolPtr(t, &statusFailed, true)
	setBoolPtr(t, &statusVerbose, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{map[string]any{"path": "/p1", "state": "implementing"}},
	})

	out := captureStdout(t, func() {
		if err := showAllStatus(); err != nil {
			t.Errorf("showAllStatus failed-filter: %v", err)
		}
	})
	if !strings.Contains(out, "No failed tasks") {
		t.Errorf("showAllStatus failed-filter output = %q", out)
	}
}

// --- showAllStatus: --blocked filter with no matches ---

func TestShowAllStatus_BlockedFilterEmpty(t *testing.T) {
	setBoolPtr(t, &statusBlocked, true)
	setBoolPtr(t, &statusVerbose, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{map[string]any{"path": "/p1", "state": "implementing"}},
	})

	out := captureStdout(t, func() {
		if err := showAllStatus(); err != nil {
			t.Errorf("showAllStatus blocked-filter: %v", err)
		}
	})
	if !strings.Contains(out, "No blocked tasks") {
		t.Errorf("showAllStatus blocked-filter output = %q", out)
	}
}

// --- showAllStatus: long-path truncation + empty taskDisplay (em-dash) ---

func TestShowAllStatus_LongPathAndEmptyTitle(t *testing.T) {
	setBoolPtr(t, &statusVerbose, false)
	setBoolPtr(t, &statusFailed, false)
	setBoolPtr(t, &statusBlocked, false)
	setBoolPtr(t, &statusJSON, false)

	longPath := "/very/long/project/path/that/definitely/exceeds/forty/characters/deeply/nested"
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{
			// No title, no id -> taskDisplay becomes the em-dash placeholder.
			map[string]any{"path": longPath, "state": "implementing"},
		},
	})

	out := captureStdout(t, func() {
		if err := showAllStatus(); err != nil {
			t.Errorf("showAllStatus long path: %v", err)
		}
	})
	if !strings.Contains(out, "...") || !strings.Contains(out, "—") {
		t.Errorf("showAllStatus long-path output:\n%s", out)
	}
}

// --- runForkSelect: friendly message when label is present ---

func TestRunForkSelect_WithLabel(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("fork.select", map[string]any{"label": "Alt approach"})

	out := captureStdout(t, func() {
		if err := runForkSelect(nil, []string{"f1"}); err != nil {
			t.Errorf("runForkSelect: %v", err)
		}
	})
	if !strings.Contains(out, "Alt approach") || !strings.Contains(out, "f1") {
		t.Errorf("fork select (labelled) output = %q", out)
	}
}

func TestRunForkSelect_Error(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("fork.select", -32000, "boom")

	if err := runForkSelect(nil, []string{"f1"}); err == nil {
		t.Fatal("expected error when fork.select fails")
	}
}

// --- runReview: fix-mode happy path (no --wait, so no job streaming) ---

func TestRunReview_FixMode(t *testing.T) {
	setBoolPtr(t, &reviewWait, false)
	if err := ReviewCmd.Flags().Set("fix", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ReviewCmd.Flags().Set("fix", "false") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	// No job_id in the response, so the command does not block on a job stream;
	// this exercises the fix-mode print plus the "Use status" follow-up line.
	stub.SetResponse("review", map[string]any{"status": "queued"})

	out := captureStdout(t, func() {
		if err := runReview(ReviewCmd, nil); err != nil {
			t.Errorf("runReview fix: %v", err)
		}
	})
	if !strings.Contains(out, "Fix mode enabled") || !strings.Contains(out, "check progress") {
		t.Errorf("review fix-mode output:\n%s", out)
	}
}

func TestRunReview_Error(t *testing.T) {
	setBoolPtr(t, &reviewWait, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("review", -32000, "boom")

	if err := runReview(ReviewCmd, nil); err == nil {
		t.Fatal("expected error when review RPC fails")
	}
}

// --- runReviewAdversarialResults: error branch ---

func TestRunReviewAdversarialResults_Error(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("adversarial.results", -32000, "boom")

	if err := runReviewAdversarialResults(nil, nil); err == nil {
		t.Fatal("expected error when adversarial.results fails")
	}
}

func TestRunReviewAdversarialResults_OK(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("adversarial.results", map[string]any{"results": []any{}})

	out := captureStdout(t, func() {
		if err := runReviewAdversarialResults(nil, nil); err != nil {
			t.Errorf("runReviewAdversarialResults: %v", err)
		}
	})
	if !strings.Contains(out, "results") {
		t.Errorf("adversarial results output = %q", out)
	}
}

// --- runExport: server error branch ---

func TestRunExport_Error(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetError("export", -32000, "boom")

	if err := runExport(ExportCmd, nil); err == nil {
		t.Fatal("expected error when export RPC fails")
	}
}

// --- runExport: CSV format branch ---

func TestRunExport_CSV(t *testing.T) {
	orig := exportFormat
	t.Cleanup(func() { exportFormat = orig })
	exportFormat = "csv"

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("export", map[string]any{
		"tasks": []any{map[string]any{"id": "t1", "path": "/p", "state": "finished"}},
		"activity": []any{
			map[string]any{"timestamp": "2026-05-01T10:00:00Z", "method": "ping", "duration_ms": 5, "user_id": "u1"},
		},
	})

	out := captureStdout(t, func() {
		if err := runExport(ExportCmd, nil); err != nil {
			t.Errorf("runExport csv: %v", err)
		}
	})
	if !strings.Contains(out, "# Tasks") || !strings.Contains(out, "# Activity") {
		t.Errorf("export csv output:\n%s", out)
	}
}

// --- runExplain: no global socket error branch ---

func TestRunExplain_NoSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	if err := runExplain(ExplainCmd, nil); err == nil {
		t.Fatal("expected error when no global socket is running")
	}
}

// --- runExplain: custom --prompt, no --wait ---

func TestRunExplain_CustomPrompt(t *testing.T) {
	setBoolPtr(t, &explainWait, false)
	if err := ExplainCmd.Flags().Set("prompt", "why did you do that"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ExplainCmd.Flags().Set("prompt", "") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.send", map[string]any{"job_id": "j1", "status": "queued"})

	out := captureStdout(t, func() {
		if err := runExplain(ExplainCmd, nil); err != nil {
			t.Errorf("runExplain custom prompt: %v", err)
		}
	})
	if !strings.Contains(out, "Explain request sent") || !strings.Contains(out, "check progress") {
		t.Errorf("explain output:\n%s", out)
	}
}

// --- changelogViaSocket: complete result with markdown ---

func TestChangelogViaSocket_Markdown(t *testing.T) {
	setBoolPtr(t, &changelogJSON, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("changelog.generate", map[string]any{"markdown": "## Changes\n- did stuff\n"})

	out := captureStdout(t, func() {
		if err := changelogViaSocket("v1", "v2", ""); err != nil {
			t.Errorf("changelogViaSocket markdown: %v", err)
		}
	})
	if !strings.Contains(out, "## Changes") {
		t.Errorf("changelog markdown output:\n%s", out)
	}
}

// --- changelogViaSocket: empty markdown -> "No commits" branch ---

func TestChangelogViaSocket_Empty(t *testing.T) {
	setBoolPtr(t, &changelogJSON, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("changelog.generate", map[string]any{"markdown": ""})

	out := captureStdout(t, func() {
		if err := changelogViaSocket("v1", "v2", "note text"); err != nil {
			t.Errorf("changelogViaSocket empty: %v", err)
		}
	})
	if !strings.Contains(out, "No commits between v1 and v2") {
		t.Errorf("changelog empty output = %q", out)
	}
}

// --- changelogViaSocket: --json branch ---

func TestChangelogViaSocket_JSON(t *testing.T) {
	setBoolPtr(t, &changelogJSON, true)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("changelog.generate", map[string]any{"markdown": "x"})

	out := captureStdout(t, func() {
		if err := changelogViaSocket("v1", "v2", ""); err != nil {
			t.Errorf("changelogViaSocket json: %v", err)
		}
	})
	if !strings.Contains(out, "markdown") {
		t.Errorf("changelog json output:\n%s", out)
	}
}

// --- changelogViaSocket: server error ---

func TestChangelogViaSocket_Error(t *testing.T) {
	setBoolPtr(t, &changelogJSON, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("changelog.generate", -32000, "boom")

	if err := changelogViaSocket("v1", "v2", ""); err == nil {
		t.Fatal("expected error when changelog.generate fails")
	}
}

// --- diagnose: offline path with a stale (non-responding) global socket ---

func TestRunDiagnose_OfflineStaleSocket(t *testing.T) {
	origJSON, origHealth := diagnoseJSON, diagnoseHealth
	t.Cleanup(func() { diagnoseJSON, diagnoseHealth = origJSON, origHealth })
	diagnoseJSON, diagnoseHealth = false, false

	shortKvelmoHome(t)
	chdirToShortTemp(t)

	// Create a real socket file that nothing is listening on: SocketExists()
	// reports true (it is a socket), but NewClient() cannot connect, so
	// socketStatus becomes "stale".
	globalPath := socket.GlobalSocketPath()
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	addr, err := net.ResolveUnixAddr("unix", globalPath)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		t.Fatal(err)
	}
	ln.SetUnlinkOnClose(false) // leave the socket file on disk after close
	_ = ln.Close()             // nothing listens now -> connect fails (stale)
	t.Cleanup(func() { _ = os.Remove(globalPath) })

	out := captureStdout(t, func() {
		if err := runDiagnose(DiagnoseCmd, nil); err != nil {
			t.Errorf("runDiagnose offline stale: %v", err)
		}
	})
	if !strings.Contains(out, "stale") {
		t.Errorf("diagnose offline stale output:\n%s", out)
	}
}

// --- runDiagnoseViaRPC: --json branch ---

func TestRunDiagnoseViaRPC_JSONServer(t *testing.T) {
	origJSON := diagnoseJSON
	t.Cleanup(func() { diagnoseJSON = origJSON })
	diagnoseJSON = true

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("system.diagnose", map[string]any{
		"checks":        []any{map[string]any{"name": "git", "status": "ok"}},
		"global_socket": "running",
		"providers":     []any{},
	})

	out := captureStdout(t, func() {
		if err := runDiagnose(DiagnoseCmd, nil); err != nil {
			t.Errorf("runDiagnose via rpc json: %v", err)
		}
	})
	if !strings.Contains(out, "global_socket") {
		t.Errorf("diagnose via rpc json output:\n%s", out)
	}
}

// --- runDiagnoseViaRPC: warning status + claude/codex display names, no issues ---

func TestRunDiagnoseViaRPC_WarningNoIssues(t *testing.T) {
	origJSON := diagnoseJSON
	t.Cleanup(func() { diagnoseJSON = origJSON })
	diagnoseJSON = false

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("system.diagnose", map[string]any{
		"checks": []any{
			map[string]any{"name": "claude", "status": "warning"},
			map[string]any{"name": "codex", "status": "warn"},
			map[string]any{"name": "git", "status": "pass", "detail": "2.40"},
		},
		"global_socket": "running",
		"providers": []any{
			map[string]any{"name": "GitHub", "configured": true},
		},
		// no "issues" -> "All checks passed!" branch
	})

	out := captureStdout(t, func() {
		if err := runDiagnose(DiagnoseCmd, nil); err != nil {
			t.Errorf("runDiagnose via rpc warning: %v", err)
		}
	})
	if !strings.Contains(out, "Claude CLI") || !strings.Contains(out, "All checks passed") {
		t.Errorf("diagnose via rpc warning output:\n%s", out)
	}
}
