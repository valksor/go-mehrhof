package commands

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
)

// TestServeSelftestChecks exercises the non-blocking startup health checks.
func TestServeSelftestChecks(t *testing.T) {
	runStartupChecks()
	checkProviderTokens()
}

// TestCheckProviderTokens_WithEnvFile exercises the path where at least one
// provider token is configured via .env.
func TestCheckProviderTokens_WithEnvFile(t *testing.T) {
	home := shortKvelmoHome(t)
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("GITHUB_TOKEN=ghp_xxx\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	checkProviderTokens()
}

// TestWaitForJob_NoSocket asserts a missing socket surfaces a dial error.
func TestWaitForJob_NoSocket(t *testing.T) {
	err := waitForJob(filepath.Join(t.TempDir(), "no.sock"), "job-1")
	if err == nil {
		t.Error("expected error for missing socket")
	}
}

// TestConnectWorktree_NoSocket exercises the no-socket branch.
func TestConnectWorktree_NoSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	_, _, err := connectWorktree()
	if err == nil {
		t.Error("expected error when no worktree socket running")
	}
}

// TestConnectWorktree_WithSocket exercises the success path.
func TestConnectWorktree_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	_ = startStubWorktreeSocket(t)

	client, cleanup, err := connectWorktree()
	if err != nil {
		t.Fatalf("connectWorktree: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
	cleanup()
}

// TestCompleteExistingTags_NoSocket exercises the early-error path.
func TestCompleteExistingTags_NoSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	tags, _ := completeExistingTags()
	if tags != nil {
		t.Errorf("expected nil tags, got %v", tags)
	}
}

// TestCompleteExistingTags_WithSocket runs through the success path.
func TestCompleteExistingTags_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.tag", map[string]any{"tags": []string{"bug", "ui"}})

	tags, _ := completeExistingTags()
	if len(tags) != 2 {
		t.Errorf("tags = %v, want 2 entries", tags)
	}
}

// TestRunStart_MutualExclusion verifies --text and --from are rejected together.
func TestRunStart_MutualExclusion(t *testing.T) {
	origText, origFrom := startText, startFrom
	t.Cleanup(func() { startText, startFrom = origText, origFrom })

	startText = "fix the bug"
	startFrom = "file:task.md"

	err := runStart(StartCmd, nil)
	if err == nil {
		t.Fatal("expected error for --text + --from")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q", err.Error())
	}
}

// TestTestProviderToken_InvalidToken exercises the provider login token test
// with an obviously-bad token. The call returns an error (likely network or
// HTTP 401) but still executes the function body.
func TestTestProviderToken_InvalidToken(t *testing.T) {
	// Point the GitHub validation endpoint at a stub returning 401 so this
	// exercises the invalid-token path without a real network call.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	withProviderURL(t, "github", srv.URL)

	if err := testProviderToken("github", "fake-token-that-will-fail"); err == nil {
		t.Error("expected an error for an invalid token")
	}
}

func TestTestProviderToken_UnknownProvider(t *testing.T) {
	// Unknown providers return nil (skip validation) by design.
	if err := testProviderToken("not-a-real-provider", "x"); err != nil {
		t.Errorf("unknown provider should return nil, got %v", err)
	}
}

func TestTestProviderToken_SkippedProviders(t *testing.T) {
	// Jira and AzureDevOps return errValidationSkipped — the function exits
	// before the network call but the body still executes the switch arm.
	for _, p := range []string{"jira", "azuredevops"} {
		_ = testProviderToken(p, "any-token")
	}
}

// TestRunProviderTest_UnknownProvider verifies error path for unknown name.
func TestRunProviderTest_UnknownProvider(t *testing.T) {
	cmd := &cobra.Command{}
	err := runProviderTest(cmd, []string{"not-a-real-provider"})
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

// TestRunProviderTest_NoToken exercises the path where no token is configured.
func TestRunProviderTest_NoToken(t *testing.T) {
	shortKvelmoHome(t)
	cmd := &cobra.Command{}
	_ = runProviderTest(cmd, []string{"github"})
}

// TestShowAllStatus_NoSocket calls the function with no global socket.
func TestShowAllStatus_NoSocket(t *testing.T) {
	shortKvelmoHome(t)
	_ = showAllStatus()
}

// TestShowAllStatus_WithSocket runs through with a stub backend.
func TestShowAllStatus_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("projects.list", []any{
		map[string]any{"path": "/p", "state": "implementing"},
	})

	_ = showAllStatus()
}

// TestReplaceOlderSocket_DifferentVersion verifies the replacement path runs.
func TestReplaceOlderSocket_DifferentVersion(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("ping", map[string]string{
		"version": "0.0.1-old",
		"commit":  "deadbeef",
	})

	_ = replaceOlderSocket(t.Context(), socket.GlobalSocketPath())
}

// chdirToShortTemp creates a short temp dir and cds into it for the test.
func chdirToShortTemp(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "kvcwd-") //nolint:usetesting // short path needed
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Chdir(dir)

	return dir
}
