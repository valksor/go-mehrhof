package socket

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/pkg/git"
)

func newTestWorktreeSocketWithRepo(t *testing.T) *WorktreeSocket {
	t.Helper()
	w := newTestWorktreeSocket(t)

	// Create a temp git repo with commits.
	ctx := context.Background()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	// First commit (will be our "source" ref).
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "Initial commit")
	run("tag", "v0.1.0")

	// Add a few categorized commits.
	if err := os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "Add new feature")

	if err := os.WriteFile(filepath.Join(dir, "fix.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "Fix broken handler")

	if err := os.WriteFile(filepath.Join(dir, "refactor.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "Update dependency versions")

	run("tag", "v0.2.0")

	repo, err := git.Open(dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}
	w.repo = repo

	return w
}

func TestHandleChangelogGenerate_Basic(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithRepo(t)

	params := mustMarshal(t, map[string]any{
		"source": "v0.1.0",
		"target": "v0.2.0",
	})
	req := &Request{ID: "1", Method: "changelog.generate", Params: params}
	resp, err := w.handleChangelogGenerate(ctx, req)
	if err != nil {
		t.Fatalf("handleChangelogGenerate() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleChangelogGenerate() returned error: %s", resp.Error.Message)
	}

	var result struct {
		Entries  []changelogEntry `json:"entries"`
		Markdown string           `json:"markdown"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result.Entries))
	}
	if result.Markdown == "" {
		t.Error("markdown should not be empty")
	}

	// Verify categories.
	cats := map[string]bool{}
	for _, e := range result.Entries {
		cats[e.Category] = true
	}
	if !cats["Added"] {
		t.Error("expected Added category")
	}
	if !cats["Fixed"] {
		t.Error("expected Fixed category")
	}
	if !cats["Changed"] {
		t.Error("expected Changed category")
	}
}

func TestHandleChangelogGenerate_Full(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithRepo(t)

	params := mustMarshal(t, map[string]any{
		"source": "v0.1.0",
		"target": "v0.2.0",
		"full":   true,
	})
	req := &Request{ID: "1", Method: "changelog.generate", Params: params}
	resp, err := w.handleChangelogGenerate(ctx, req)
	if err != nil {
		t.Fatalf("handleChangelogGenerate() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleChangelogGenerate() returned error: %s", resp.Error.Message)
	}

	var result struct {
		Entries  []changelogEntry `json:"entries"`
		Markdown string           `json:"markdown"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result.Entries))
	}
}

func TestHandleChangelogGenerate_NoRepo(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	params := mustMarshal(t, map[string]any{
		"source": "v0.1.0",
		"target": "v0.2.0",
	})
	req := &Request{ID: "1", Method: "changelog.generate", Params: params}
	resp, err := w.handleChangelogGenerate(ctx, req)
	if err != nil {
		t.Fatalf("handleChangelogGenerate() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error when no repo")
	}
}

func TestHandleChangelogGenerate_MissingParams(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithRepo(t)

	req := &Request{ID: "1", Method: "changelog.generate"}
	resp, err := w.handleChangelogGenerate(ctx, req)
	if err != nil {
		t.Fatalf("handleChangelogGenerate() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error when source/target missing")
	}
}
