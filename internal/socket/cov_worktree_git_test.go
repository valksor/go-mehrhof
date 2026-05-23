package socket

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/testutil"
)

// gitWorktree builds a worktree socket rooted at a real git repo (with one
// commit) and wires w.repo, so the git.* handlers reach their success paths.
func gitWorktree(ctx context.Context, t *testing.T) *WorktreeSocket {
	t.Helper()
	w := newTestWorktreeSocket(ctx, t)
	dir := testutil.TempDir(t)
	testutil.InitGitRepo(t, dir)
	w.path = dir
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}
	w.repo = repo

	return w
}

func TestWorktreeHandleGitStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("no repo", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t) // repo is nil
		resp, err := w.handleGitStatus(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleGitStatus() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no repo")
		}
	})

	t.Run("clean repo", func(t *testing.T) {
		w := gitWorktree(ctx, t)
		resp, err := w.handleGitStatus(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleGitStatus() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := result["branch"]; !ok {
			t.Error("expected branch key")
		}
		if hasChanges, _ := result["has_changes"].(bool); hasChanges {
			t.Error("clean repo should have no changes")
		}
	})

	t.Run("dirty repo", func(t *testing.T) {
		w := gitWorktree(ctx, t)
		if err := os.WriteFile(filepath.Join(w.path, "new.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		resp, err := w.handleGitStatus(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleGitStatus() error = %v", err)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		if hasChanges, _ := result["has_changes"].(bool); !hasChanges {
			t.Error("dirty repo should report changes")
		}
	})
}

func TestWorktreeHandleGitDiff(t *testing.T) {
	ctx := context.Background()

	t.Run("no repo", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleGitDiff(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleGitDiff() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no repo")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := gitWorktree(ctx, t)
		resp, err := w.handleGitDiff(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleGitDiff() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("diff of staged change", func(t *testing.T) {
		w := gitWorktree(ctx, t)
		if err := os.WriteFile(filepath.Join(w.path, "README.md"), []byte("changed"), 0o644); err != nil {
			t.Fatal(err)
		}
		params, _ := json.Marshal(GitDiffParams{Cached: false})
		resp, err := w.handleGitDiff(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleGitDiff() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		if _, ok := result["diff"]; !ok {
			t.Error("expected diff key")
		}
	})
}

func TestWorktreeHandleGitDiffAgainst(t *testing.T) {
	ctx := context.Background()

	t.Run("no repo", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleGitDiffAgainst(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleGitDiffAgainst() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no repo")
		}
	})

	t.Run("missing ref", func(t *testing.T) {
		w := gitWorktree(ctx, t)
		params, _ := json.Marshal(GitDiffAgainstParams{Ref: ""})
		resp, err := w.handleGitDiffAgainst(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleGitDiffAgainst() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing ref")
		}
	})

	t.Run("diff against HEAD", func(t *testing.T) {
		w := gitWorktree(ctx, t)
		params, _ := json.Marshal(GitDiffAgainstParams{Ref: "HEAD", Stat: true})
		resp, err := w.handleGitDiffAgainst(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleGitDiffAgainst() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
	})
}

func TestWorktreeHandleGitLog(t *testing.T) {
	ctx := context.Background()

	t.Run("no repo", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleGitLog(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleGitLog() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no repo")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := gitWorktree(ctx, t)
		resp, err := w.handleGitLog(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleGitLog() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("log returns entries", func(t *testing.T) {
		w := gitWorktree(ctx, t)
		// Add a second commit so there is history.
		run := func(args ...string) {
			cmd := exec.CommandContext(ctx, "git", append([]string{"-C", w.path}, args...)...)
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@t")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		if err := os.WriteFile(filepath.Join(w.path, "a.txt"), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", ".")
		run("commit", "-m", "second")

		params, _ := json.Marshal(GitLogParams{Count: 5})
		resp, err := w.handleGitLog(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleGitLog() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		entries, ok := result["entries"].([]any)
		if !ok || len(entries) < 2 {
			t.Errorf("expected at least 2 log entries, got %v", result["entries"])
		}
	})
}
