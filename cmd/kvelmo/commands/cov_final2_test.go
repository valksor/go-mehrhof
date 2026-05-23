package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/provider"
)

// --- explain ---

func TestRunExplain_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.send", map[string]any{"job_id": "ex-1", "status": "queued"})

	out := captureStdout(t, func() {
		if err := runExplain(ExplainCmd, nil); err != nil {
			t.Errorf("runExplain: %v", err)
		}
	})
	if !strings.Contains(out, "Explain request sent") {
		t.Errorf("explain output:\n%s", out)
	}
}

// --- screenshots capture ---

func TestRunScreenshotsCapture_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.capture", map[string]any{
		"screenshot": map[string]any{
			"id": "s1", "filename": "s1.png", "width": 1024, "height": 768,
			"size_bytes": 8192, "source": "browser",
		},
	})

	out := captureStdout(t, func() {
		if err := runScreenshotsCapture(screenshotsCaptureCmd, nil); err != nil {
			t.Errorf("runScreenshotsCapture: %v", err)
		}
	})
	if !strings.Contains(out, "Screenshot captured: s1") {
		t.Errorf("screenshots capture output:\n%s", out)
	}
}

func TestRunScreenshotsDelete_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.delete", map[string]any{"deleted": true})

	out := captureStdout(t, func() {
		if err := runScreenshotsDelete(screenshotsDeleteCmd, []string{"s1"}); err != nil {
			t.Errorf("runScreenshotsDelete: %v", err)
		}
	})
	if out == "" {
		t.Error("screenshots delete produced no output")
	}
}

// --- chat send / stop / clear ---

func TestRunChatSend_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.send", map[string]any{"job_id": "c-1", "status": "queued"})

	_ = captureStdout(t, func() {
		if err := runChatSend(chatSendCmd, []string{"hello agent"}); err != nil {
			t.Errorf("runChatSend: %v", err)
		}
	})
}

func TestRunChatStop_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.stop", map[string]any{"stopped": true})

	_ = captureStdout(t, func() {
		if err := runChatStop(chatStopCmd, nil); err != nil {
			t.Errorf("runChatStop: %v", err)
		}
	})
}

func TestRunChatClear_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("chat.clear", map[string]any{"cleared": 3})

	_ = captureStdout(t, func() {
		if err := runChatClear(chatClearCmd, nil); err != nil {
			t.Errorf("runChatClear: %v", err)
		}
	})
}

// --- browse ---

func TestRunBrowse_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("browse", map[string]any{"items": []any{}})

	_ = captureStdout(t, func() {
		_ = runBrowse(BrowseCmd, nil)
	})
}

// --- discover populated ---

func TestRunDiscover_Populated(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("discovery.scan", map[string]any{
		"commands": []string{"make test", "make build"},
		"count":    2,
	})

	out := captureStdout(t, func() {
		if err := runDiscover(DiscoverCmd, nil); err != nil {
			t.Errorf("runDiscover: %v", err)
		}
	})
	if !strings.Contains(out, "make test") {
		t.Errorf("discover populated output:\n%s", out)
	}
}

// --- memory stats populated ---

func TestRunMemoryStats_Populated(t *testing.T) {
	setBoolPtr(t, &memoryStatsJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.stats", map[string]any{
		"count": 42, "size_bytes": 8192, "entries_by_kind": map[string]any{"decision": 10, "outcome": 32},
	})

	out := captureStdout(t, func() {
		if err := runMemoryStats(memoryStatsCmd, nil); err != nil {
			t.Errorf("runMemoryStats: %v", err)
		}
	})
	if out == "" {
		t.Error("memory stats populated produced no output")
	}
}

// --- loadEffectiveOffline with project config ---

func TestLoadEffectiveOffline_WithProject(t *testing.T) {
	shortKvelmoHome(t)
	dir := chdirToShortTemp(t)
	// Write a project config so the project-merge branch runs.
	projDir := filepath.Join(dir, ".valksor")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "kvelmo.yaml"), []byte("workers:\n  max: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	eff, err := loadEffectiveOffline()
	if err != nil {
		t.Fatalf("loadEffectiveOffline: %v", err)
	}
	if eff == nil {
		t.Fatal("expected non-nil effective settings")
	}
}

// --- group status with members display ---

func TestRunGroupStatus_Members(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.status", map[string]any{
		"id":    "g1",
		"label": "Group One",
		"members": []any{
			map[string]any{"task_id": "t1", "project_dir": "/p1", "state": "submitted", "branch": "feat/a"},
			map[string]any{"task_id": "t2", "project_dir": "/p2", "state": "implementing"},
		},
	})

	out := captureStdout(t, func() {
		if err := runGroupStatus(nil, []string{"g1"}); err != nil {
			t.Errorf("runGroupStatus: %v", err)
		}
	})
	if out == "" {
		t.Error("group status members produced no output")
	}
}

// --- provider login token-prefix warning ---

func TestRunProviderLogin_TokenPrefixWarning(t *testing.T) {
	shortKvelmoHome(t)

	// GitHub config has TokenPrefix "ghp_"; pipe a token WITHOUT that prefix to
	// trigger the prefix-warning branch. Skip network validation by pointing at
	// a stub that returns 200.
	withProviderURL(t, provider.NameGitHub, "http://127.0.0.1:1/unused")

	r, w, _ := os.Pipe()
	_, _ = w.WriteString("wrongprefix_token\n")
	_ = w.Close()
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = orig; _ = r.Close() })

	loginCmd := findProviderLogin(GitHubCmd)
	var buf strings.Builder
	loginCmd.SetOut(&buf)

	_ = runProviderLogin("github")(loginCmd, nil)
	if !strings.Contains(buf.String(), "doesn't start with expected prefix") {
		t.Errorf("provider login prefix-warning output:\n%s", buf.String())
	}
}
