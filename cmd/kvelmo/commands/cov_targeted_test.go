package commands

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/internal/testutil"
)

// openGitRepo opens a git repository for changelog tests.
func openGitRepo(t *testing.T, dir string) *git.Repository {
	t.Helper()
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}

	return repo
}

// pipeStdin replaces os.Stdin with a pipe pre-filled with the given input.
func pipeStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.WriteString(input)
	_ = w.Close()
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = orig; _ = r.Close() })
}

// TestRunAbort_ConfirmYes covers the prompt-confirmed branch (response "y").
func TestRunAbort_ConfirmYes(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("abort", map[string]any{"status": "aborted", "state": "paused"})

	pipeStdin(t, "y\n")
	out := captureStdout(t, func() {
		if err := runAbort(AbortCmd, nil); err != nil {
			t.Errorf("runAbort confirm-yes: %v", err)
		}
	})
	if !strings.Contains(out, "Task aborted") {
		t.Errorf("abort confirm-yes output:\n%s", out)
	}
}

// TestRunReset_ConfirmYes covers the prompt-confirmed reset branch.
func TestRunReset_ConfirmYes(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("reset", map[string]any{"state": "none"})

	pipeStdin(t, "y\n")
	out := captureStdout(t, func() {
		if err := runReset(ResetCmd, nil); err != nil {
			t.Errorf("runReset confirm-yes: %v", err)
		}
	})
	if !strings.Contains(out, "Task reset") {
		t.Errorf("reset confirm-yes output:\n%s", out)
	}
}

// TestResolvePort_Explicit covers the changed-port branch.
func TestResolvePort_Explicit(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Int("port", 0, "")
	if err := cmd.Flags().Set("port", "9999"); err != nil {
		t.Fatal(err)
	}
	if got := resolvePort(cmd, 9999); got != 9999 {
		t.Errorf("resolvePort explicit = %d, want 9999", got)
	}
}

// TestRunStart_GitRepoVerbose runs start in a real git repo with --verbose so
// the git-repository and verbose branches execute; an existing socket means the
// no-task message path runs.
func TestRunStart_GitRepoVerbose(t *testing.T) {
	origV, origText, origFrom := startVerbose, startText, startFrom
	t.Cleanup(func() { startVerbose, startText, startFrom = origV, origText, origFrom })
	startVerbose, startText, startFrom = true, "", ""

	shortKvelmoHome(t)
	repo := t.TempDir()
	testutil.InitGitRepo(t, repo)
	t.Chdir(repo)

	// Start a real worktree socket at the repo path so runInBackground finds it.
	cwd, _ := os.Getwd()
	sockPath := socket.WorktreeSocketPath(cwd)
	wt := socket.NewWorktreeSocketSimple(sockPath, cwd)
	ctx := t.Context()
	go func() { _ = wt.Start(ctx) }()
	for range 200 {
		if socket.SocketExists(sockPath) {
			break
		}
	}

	out := captureStdout(t, func() {
		if err := runStart(StartCmd, nil); err != nil {
			t.Errorf("runStart git repo verbose: %v", err)
		}
	})
	if !strings.Contains(out, "Global socket:") || !strings.Contains(out, "Worktree socket:") {
		t.Errorf("start verbose output missing socket paths:\n%s", out)
	}
}

// TestRunStart_NotGitRepo covers the not-a-git-repo warning branch.
func TestRunStart_NotGitRepo(t *testing.T) {
	origText, origFrom := startText, startFrom
	t.Cleanup(func() { startText, startFrom = origText, origFrom })
	startText, startFrom = "", ""

	shortKvelmoHome(t)
	chdirToShortTemp(t) // plain dir, not a git repo

	// No socket and isTestBinary → runInBackground returns an error after the
	// warning prints. We assert the warning appeared.
	out := captureStdout(t, func() {
		_ = runStart(StartCmd, nil)
	})
	if !strings.Contains(out, "Not a git repository") {
		t.Errorf("start non-git output missing warning:\n%s", out)
	}
}

// TestGatherChangelogCommits_FullAndPlain exercises both gather paths against a
// real repo with two commits.
func TestGatherChangelogCommits_FullAndPlain(t *testing.T) {
	repo := t.TempDir()
	testutil.InitGitRepo(t, repo)
	addCommit(t, repo, "x.txt", "second")
	first := gitRevParse(t, repo, "HEAD~1")

	openRepo := openGitRepo(t, repo)
	ctx := t.Context()

	plain, err := gatherChangelogCommits(ctx, openRepo, first, "HEAD", false)
	if err != nil {
		t.Fatalf("gatherChangelogCommits plain: %v", err)
	}
	if len(plain) == 0 {
		t.Error("expected commits in plain gather")
	}

	full, err := gatherChangelogCommits(ctx, openRepo, first, "HEAD", true)
	if err != nil {
		t.Fatalf("gatherChangelogCommits full: %v", err)
	}
	if len(full) == 0 {
		t.Error("expected commits in full gather")
	}
}
