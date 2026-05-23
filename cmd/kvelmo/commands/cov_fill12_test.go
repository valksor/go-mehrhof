package commands

import (
	"strings"
	"testing"
)

// --- files search: --json branch ---

func TestRunFilesSearch_JSONBranch(t *testing.T) {
	orig := filesSearchJSON
	t.Cleanup(func() { filesSearchJSON = orig })
	filesSearchJSON = true

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("files.search", map[string]any{"files": []any{"a.go"}})

	out := captureStdout(t, func() {
		if err := runFilesSearch(filesSearchCmd, []string{"a"}); err != nil {
			t.Errorf("runFilesSearch json: %v", err)
		}
	})
	if !strings.Contains(out, "files") {
		t.Errorf("files search json output:\n%s", out)
	}
}

// --- files list: --json branch ---

func TestRunFilesList_JSONBranch(t *testing.T) {
	origJSON, origExt, origDepth := filesListJSON, filesListExt, filesListDepth
	t.Cleanup(func() { filesListJSON, filesListExt, filesListDepth = origJSON, origExt, origDepth })
	filesListJSON, filesListExt, filesListDepth = true, nil, 0

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("files.list", map[string]any{"files": []any{"main.go"}})

	out := captureStdout(t, func() {
		if err := runFilesList(filesListCmd, nil); err != nil {
			t.Errorf("runFilesList json: %v", err)
		}
	})
	if !strings.Contains(out, "files") {
		t.Errorf("files list json output:\n%s", out)
	}
}

// --- audit: empty user-id (-> "-") and entry with error (-> "ERR") ---

func TestRunAudit_UserDashAndError(t *testing.T) {
	setBoolPtr(t, &auditJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("export", map[string]any{
		"tasks": []any{map[string]any{"id": "t1", "state": "implementing"}},
		"activity": []any{
			// No user_id -> "-"; with error -> "ERR" appended.
			map[string]any{"timestamp": "2026-05-01T10:00:00Z", "method": "ping", "duration_ms": 7, "error": "boom"},
		},
	})

	out := captureStdout(t, func() {
		if err := runAudit(AuditCmd, nil); err != nil {
			t.Errorf("runAudit user-dash/error: %v", err)
		}
	})
	if !strings.Contains(out, "ERR") {
		t.Errorf("audit user-dash/error output:\n%s", out)
	}
}

// --- memory stats: embedder field present ---

func TestRunMemoryStats_Embedder(t *testing.T) {
	setBoolPtr(t, &memoryStatsJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.stats", map[string]any{
		"total_documents": 5,
		"embedder":        "openai-text-embed-3",
	})

	out := captureStdout(t, func() {
		if err := runMemoryStats(memoryStatsCmd, nil); err != nil {
			t.Errorf("runMemoryStats embedder: %v", err)
		}
	})
	if !strings.Contains(out, "Embedder:") || !strings.Contains(out, "openai-text-embed-3") {
		t.Errorf("memory stats embedder output:\n%s", out)
	}
}

// --- diff: showDiffAgainst --json branch ---

func TestRunDiff_JSONAgainst(t *testing.T) {
	setBoolPtr(t, &diffJSON, true)
	t.Cleanup(func() { diffJSON = false })
	if err := DiffCmd.Flags().Set("stat", "false"); err != nil {
		t.Fatal(err)
	}

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("checkpoints", map[string]any{
		"checkpoints": []any{map[string]any{"sha": "a1"}, map[string]any{"sha": "b2"}},
	})
	stub.SetResponse("git.diff_against", map[string]any{"diff": "patch"})

	out := captureStdout(t, func() {
		if err := runDiff(DiffCmd, nil); err != nil {
			t.Errorf("runDiff json against: %v", err)
		}
	})
	if !strings.Contains(out, "diff") {
		t.Errorf("diff json-against output:\n%s", out)
	}
}
