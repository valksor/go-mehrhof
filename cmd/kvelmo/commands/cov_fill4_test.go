package commands

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/settings"
)

// --- detectExistingToken: project scope path ---

func TestDetectExistingToken_ProjectScope(t *testing.T) {
	shortKvelmoHome(t)
	dir := chdirToShortTemp(t)

	// Write the token into the project .env so the ScopeProject branch resolves
	// the project env path.
	envPath := settings.ProjectEnvPath(dir)
	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("GITHUB_TOKEN=ghp_projscope999\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ts := detectExistingToken("GITHUB_TOKEN", settings.ScopeProject, dir)
	if ts == nil {
		t.Fatal("expected to detect project-scope token")
	}
	if !strings.Contains(ts.Source, dir) {
		t.Errorf("project token source = %q, want path under %s", ts.Source, dir)
	}
}

// --- confirmOverride: read error (immediate EOF) returns false ---

func TestConfirmOverride_ReadError(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("")) // immediate EOF -> ReadString returns error
	cmd.SetOut(&bytes.Buffer{})

	if confirmOverride(cmd) {
		t.Error("confirmOverride should be false on read error/EOF")
	}
}

// --- runProviderLogin: empty token -> Cancelled ---

func TestRunProviderLogin_EmptyTokenCancelled(t *testing.T) {
	shortKvelmoHome(t)

	r, w, _ := os.Pipe()
	_, _ = w.WriteString("\n") // empty token line
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin; _ = r.Close() })

	loginCmd := findProviderLogin(GitHubCmd)
	var buf strings.Builder
	loginCmd.SetOut(&buf)

	if err := runProviderLogin("github")(loginCmd, nil); err != nil {
		t.Errorf("runProviderLogin empty token: %v", err)
	}
	if !strings.Contains(buf.String(), "Cancelled.") {
		t.Errorf("expected Cancelled output:\n%s", buf.String())
	}
}

// --- runProviderLogin: existing token, override declined ---

func TestRunProviderLogin_ExistingDeclineOverrideFill(t *testing.T) {
	home := shortKvelmoHome(t)
	// Pre-populate a token so detectExistingToken finds it.
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("GITHUB_TOKEN=ghp_existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Stdin is irrelevant; override answer comes from cmd.InOrStdin().
	loginCmd := findProviderLogin(GitHubCmd)
	var buf strings.Builder
	loginCmd.SetOut(&buf)
	loginCmd.SetIn(strings.NewReader("n\n")) // decline override

	if err := runProviderLogin("github")(loginCmd, nil); err != nil {
		t.Errorf("runProviderLogin decline override: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "already configured") || !strings.Contains(out, "Cancelled.") {
		t.Errorf("expected decline-override output:\n%s", out)
	}
}

// --- runProviderLogin: --project scope, validation skipped, token saved ---

func TestRunProviderLogin_ProjectScopeSaved(t *testing.T) {
	shortKvelmoHome(t)
	dir := chdirToShortTemp(t)

	// Use Jira: testProviderToken returns errValidationSkipped (no network).
	r, w, _ := os.Pipe()
	_, _ = w.WriteString("jira_token_12345\n")
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin; _ = r.Close() })

	loginCmd := findProviderLogin(JiraCmd)
	if err := loginCmd.Flags().Set("project", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = loginCmd.Flags().Set("project", "false") })
	var buf strings.Builder
	loginCmd.SetOut(&buf)

	if err := runProviderLogin(provider.NameJira)(loginCmd, nil); err != nil {
		t.Errorf("runProviderLogin --project: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "skipped") || !strings.Contains(out, "Token saved") {
		t.Errorf("project-scope login output:\n%s", out)
	}
	// Token persisted to the project .env.
	data, err := os.ReadFile(settings.ProjectEnvPath(dir))
	if err != nil {
		t.Fatalf("read project .env: %v", err)
	}
	if !strings.Contains(string(data), "JIRA_TOKEN") {
		t.Errorf("project .env missing JIRA_TOKEN: %s", data)
	}
}

// --- runProviderLogin: validation failure (stub 401) still saves token ---

func TestRunProviderLogin_ValidationFailureSaves(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	withProviderURL(t, provider.NameGitHub, srv.URL)

	shortKvelmoHome(t)

	r, w, _ := os.Pipe()
	_, _ = w.WriteString("ghp_badtoken00000\n")
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin; _ = r.Close() })

	loginCmd := findProviderLogin(GitHubCmd)
	var buf strings.Builder
	loginCmd.SetOut(&buf)

	if err := runProviderLogin("github")(loginCmd, nil); err != nil {
		t.Errorf("runProviderLogin validation failure: %v", err)
	}
	if !strings.Contains(buf.String(), "validation failed") {
		t.Errorf("expected validation-failed warning:\n%s", buf.String())
	}
}

// --- runCleanup: stale global socket found (dry-run) ---

func TestRunCleanup_StaleDryRun(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	staleSocketFile(t, socket.GlobalSocketPath())

	if err := CleanupCmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CleanupCmd.Flags().Set("dry-run", "false") })

	out := captureStdout(t, func() {
		if err := runCleanup(CleanupCmd, nil); err != nil {
			t.Errorf("runCleanup stale dry-run: %v", err)
		}
	})
	if !strings.Contains(out, "Found 1 stale socket") || !strings.Contains(out, "dry-run") {
		t.Errorf("cleanup stale dry-run output:\n%s", out)
	}
}

// --- runCleanup: stale global socket removed with --force ---

func TestRunCleanup_StaleForceRemove(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	path := socket.GlobalSocketPath()
	staleSocketFile(t, path)

	if err := CleanupCmd.Flags().Set("force", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CleanupCmd.Flags().Set("force", "false") })

	out := captureStdout(t, func() {
		if err := runCleanup(CleanupCmd, nil); err != nil {
			t.Errorf("runCleanup stale force: %v", err)
		}
	})
	if !strings.Contains(out, "Removed") || !strings.Contains(out, "Cleaned up") {
		t.Errorf("cleanup stale force output:\n%s", out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("stale socket file should have been removed")
	}
}

// --- runGroupAdd: server error ---

func TestRunGroupAdd_Error(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetError("taskgroup.add", -32000, "boom")

	if err := runGroupAdd(nil, []string{"g1", "t1"}); err == nil {
		t.Fatal("expected error when taskgroup.add fails")
	}
}

// --- runStatsAll: iterates worktree sockets of registered projects ---

func TestRunStatsAll_WithWorktree(t *testing.T) {
	shortKvelmoHome(t)
	dir := chdirToShortTemp(t)

	// Global socket lists one project whose path has a live worktree socket.
	gstub := startStubGlobalSocket(t)
	gstub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{map[string]any{"path": dir, "state": "implementing"}},
	})

	// Worktree socket for that project path returns archived task history.
	wstub := startStubWorktreeSocket(t)
	wstub.SetResponse("task.history", map[string]any{
		"tasks": []any{
			map[string]any{
				"id": "t1", "title": "Done", "final_state": "finished",
				"started_at": "2026-05-01T10:00:00Z", "completed_at": "2026-05-01T10:30:00Z",
			},
		},
	})

	out := captureStdout(t, func() {
		if err := runStatsAll(); err != nil {
			t.Errorf("runStatsAll with worktree: %v", err)
		}
	})
	if out == "" {
		t.Error("runStatsAll produced no output")
	}
}

// staleSocketFile creates a real socket file at path with no listener, so
// SocketExists reports true but connections fail (a stale socket).
func staleSocketFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	addr, err := net.ResolveUnixAddr("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		t.Fatal(err)
	}
	ln.SetUnlinkOnClose(false)
	_ = ln.Close()
	t.Cleanup(func() { _ = os.Remove(path) })
}
