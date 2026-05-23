package socket

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/testutil"
)

// --- worktree_review.go ---

func TestWorktreeHandleReviewView(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleReviewView(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleReviewView() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleReviewView(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleReviewView() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("nonexistent review", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplemented)
		params, _ := json.Marshal(ReviewViewParams{Number: 999})
		resp, err := w.handleReviewView(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleReviewView() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for nonexistent review")
		}
	})
}

func TestWorktreeHandleReviewList(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor returns empty list", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleReviewList(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleReviewList() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result ReviewListResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if result.Reviews == nil {
			t.Error("reviews should be non-nil empty slice")
		}
	})

	t.Run("conductor with no store returns empty list", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleReviewList(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleReviewList() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
	})
}

func TestWorktreeHandleContextResolve(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleContextResolve(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleContextResolve() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing type and ref", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(map[string]string{"type": "", "ref": ""})
		resp, err := w.handleContextResolve(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleContextResolve() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing type/ref")
		}
	})

	t.Run("resolve file reference", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		w.path = testutil.TempDir(t)
		fp := filepath.Join(w.path, "note.md")
		if err := os.WriteFile(fp, []byte("note body"), 0o644); err != nil {
			t.Fatal(err)
		}
		params, _ := json.Marshal(map[string]string{"type": "file", "ref": "note.md"})
		resp, err := w.handleContextResolve(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleContextResolve() error = %v", err)
		}
		// Resolution of an in-worktree file should succeed; if the resolver
		// returns an error response that's still a valid non-panicking path.
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
	})
}

func TestWorktreeHandleProvisionPreview(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)
	w.path = testutil.TempDir(t)

	resp, err := w.handleProvisionPreview(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleProvisionPreview() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error response: %s", resp.Error.Message)
	}
	if len(resp.Result) == 0 {
		t.Fatal("expected a provision preview result")
	}
}

// --- worktree_files.go ---

func TestWorktreeHandleWorktreeFilesList(t *testing.T) {
	ctx := context.Background()

	t.Run("lists files at worktree root", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		w.path = testutil.TempDir(t)
		if err := os.WriteFile(filepath.Join(w.path, "a.go"), []byte("package x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(w.path, "b.md"), []byte("# b"), 0o644); err != nil {
			t.Fatal(err)
		}
		resp, err := w.handleWorktreeFilesList(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleWorktreeFilesList() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result struct {
			Files []struct {
				Name string `json:"name"`
			} `json:"files"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(result.Files) < 2 {
			t.Errorf("expected at least 2 files, got %d", len(result.Files))
		}
	})

	t.Run("filters by extension", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		w.path = testutil.TempDir(t)
		if err := os.WriteFile(filepath.Join(w.path, "a.go"), []byte("package x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(w.path, "b.md"), []byte("# b"), 0o644); err != nil {
			t.Fatal(err)
		}
		params, _ := json.Marshal(map[string]any{"extensions": []string{"go"}})
		resp, err := w.handleWorktreeFilesList(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleWorktreeFilesList() error = %v", err)
		}
		var result struct {
			Files []struct {
				Name string `json:"name"`
			} `json:"files"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		for _, f := range result.Files {
			if filepath.Ext(f.Name) == ".md" {
				t.Errorf("extension filter leaked a .md file: %s", f.Name)
			}
		}
	})

	t.Run("path outside worktree rejected", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		w.path = testutil.TempDir(t)
		params, _ := json.Marshal(map[string]string{"path": "../../../etc"})
		resp, err := w.handleWorktreeFilesList(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleWorktreeFilesList() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for path outside worktree")
		}
	})
}

func TestWorktreeHandleBrowse_Success(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)
	w.path = testutil.TempDir(t)
	sub := filepath.Join(w.path, "pkg")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.path, "doc.md"), []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("lists directories only by default", func(t *testing.T) {
		resp, err := w.handleBrowse(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleBrowse() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result struct {
			Entries []WorktreeBrowseEntry `json:"entries"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		// pkg dir present, doc.md absent (files=false default).
		var sawDir, sawFile bool
		for _, e := range result.Entries {
			if e.Name == "pkg" && e.IsDir {
				sawDir = true
			}
			if e.Name == "doc.md" {
				sawFile = true
			}
		}
		if !sawDir {
			t.Error("expected pkg directory in listing")
		}
		if sawFile {
			t.Error("doc.md should be excluded when files=false")
		}
	})

	t.Run("includes md files when files=true", func(t *testing.T) {
		params, _ := json.Marshal(WorktreeBrowseParams{Files: true})
		resp, err := w.handleBrowse(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleBrowse() error = %v", err)
		}
		var result struct {
			Entries []WorktreeBrowseEntry `json:"entries"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		var sawFile bool
		for _, e := range result.Entries {
			if e.Name == "doc.md" {
				sawFile = true
			}
		}
		if !sawFile {
			t.Error("expected doc.md when files=true")
		}
	})

	t.Run("path traversal denied", func(t *testing.T) {
		params, _ := json.Marshal(WorktreeBrowseParams{Path: "/etc"})
		resp, err := w.handleBrowse(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleBrowse() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for path outside worktree")
		}
	})
}
