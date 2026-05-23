package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/internal/testutil"
)

// --- memory search with results display (Total set) ---

func TestRunMemorySearch_Display(t *testing.T) {
	setBoolPtr(t, &memorySearchJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.search", map[string]any{
		"results": []any{
			map[string]any{"id": "m1", "task_id": "t1", "type": "decision", "content": strings.Repeat("x", 250), "score": 0.95},
			map[string]any{"id": "m2", "task_id": "t2", "type": "outcome", "content": "short", "score": 0.80},
		},
		"total": 2,
	})

	out := captureStdout(t, func() {
		if err := runMemorySearch(memorySearchCmd, []string{"auth"}); err != nil {
			t.Errorf("runMemorySearch: %v", err)
		}
	})
	if !strings.Contains(out, "Found 2 result(s)") || !strings.Contains(out, "...") {
		t.Errorf("memory search display output:\n%s", out)
	}
}

// --- config validate: invalid agent default ---

func TestRunConfigValidate_InvalidAgent(t *testing.T) {
	setBoolPtr(t, &configValidateJSON, false)
	home := shortKvelmoHome(t)
	chdirToShortTemp(t)
	// Global config with an invalid agent.default triggers the validity check.
	if err := os.WriteFile(filepath.Join(home, "kvelmo.yaml"), []byte("agent:\n  default: bogusagent\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		_ = runConfigValidate(nil, nil)
	})
	if !strings.Contains(out, "agent.default") {
		t.Errorf("config validate invalid-agent output:\n%s", out)
	}
}

// --- provider login: validation failed (network error) ---

func TestRunProviderLogin_ValidationFailed(t *testing.T) {
	shortKvelmoHome(t)
	// Point at an unreachable URL so testProviderToken returns a connection
	// error, exercising the "validation failed" warning branch.
	withProviderURL(t, provider.NameGitHub, "http://127.0.0.1:1/nope")

	r, w, _ := os.Pipe()
	_, _ = w.WriteString("ghp_sometoken12345\n")
	_ = w.Close()
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = orig; _ = r.Close() })

	loginCmd := findProviderLogin(GitHubCmd)
	var buf strings.Builder
	loginCmd.SetOut(&buf)

	_ = runProviderLogin("github")(loginCmd, nil)
	if !strings.Contains(buf.String(), "validation failed") {
		t.Errorf("provider login validation-failed output:\n%s", buf.String())
	}
}

// --- cleanup: orphaned git worktree (real, removed) ---

func TestRunCleanup_OrphanedWorktree(t *testing.T) {
	if err := CleanupCmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatal(err)
	}
	if err := CleanupCmd.Flags().Set("git", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = CleanupCmd.Flags().Set("dry-run", "false")
		_ = CleanupCmd.Flags().Set("git", "false")
	})

	shortKvelmoHome(t)
	repo := t.TempDir()
	testutil.InitGitRepo(t, repo)

	// Add a linked worktree, then delete its directory to orphan it.
	wtDir := filepath.Join(repo, "..", "wt-orphan")
	runGit(t, repo, "worktree", "add", wtDir, "-b", "orphan-branch")
	if err := os.RemoveAll(wtDir); err != nil {
		t.Fatal(err)
	}

	t.Chdir(repo)

	out := captureStdout(t, func() {
		if err := runCleanup(CleanupCmd, nil); err != nil {
			t.Errorf("runCleanup orphaned: %v", err)
		}
	})
	if !strings.Contains(out, "orphaned git worktree") {
		t.Errorf("cleanup orphaned output:\n%s", out)
	}
}

// --- diagnose offline with provider tokens configured ---

func TestRunDiagnose_OfflineWithTokens(t *testing.T) {
	origJSON, origHealth := diagnoseJSON, diagnoseHealth
	t.Cleanup(func() { diagnoseJSON, diagnoseHealth = origJSON, origHealth })
	diagnoseJSON, diagnoseHealth = false, false

	home := shortKvelmoHome(t)
	chdirToShortTemp(t)
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("GITHUB_TOKEN=ghp_abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := runDiagnose(DiagnoseCmd, nil); err != nil {
			t.Errorf("runDiagnose offline tokens: %v", err)
		}
	})
	if !strings.Contains(out, "configured") {
		t.Errorf("diagnose offline tokens output:\n%s", out)
	}
}

// --- changelog via socket: job_id path (streams) and empty result ---

func TestRunChangelog_SocketEmptyResult(t *testing.T) {
	origJSON := changelogJSON
	t.Cleanup(func() { changelogJSON = origJSON })
	changelogJSON = false

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("changelog.generate", map[string]any{"markdown": ""})

	out := captureStdout(t, func() {
		if err := runChangelog(ChangelogCmd, []string{"v1", "v2"}); err != nil {
			t.Errorf("runChangelog empty: %v", err)
		}
	})
	if !strings.Contains(out, "No commits between") {
		t.Errorf("changelog empty-result output:\n%s", out)
	}
}

func TestRunChangelog_SocketJSON(t *testing.T) {
	origJSON := changelogJSON
	t.Cleanup(func() { changelogJSON = origJSON })
	changelogJSON = true

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("changelog.generate", map[string]any{"markdown": "## Added"})

	out := captureStdout(t, func() {
		if err := runChangelog(ChangelogCmd, []string{"v1", "v2"}); err != nil {
			t.Errorf("runChangelog json: %v", err)
		}
	})
	if !strings.Contains(out, "markdown") {
		t.Errorf("changelog json output:\n%s", out)
	}
}
