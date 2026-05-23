package socket

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/internal/testutil"
)

func TestWorktreeHandleTaskExport(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleTaskExport(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleTaskExport() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		w.path = testutil.TempDir(t)
		resp, err := w.handleTaskExport(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleTaskExport() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("no task loaded", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		w.path = testutil.TempDir(t)
		resp, err := w.handleTaskExport(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleTaskExport() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no work unit")
		}
	})

	t.Run("json export default format", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		w.path = testutil.TempDir(t)
		setWorkUnitInState(t, w, conductor.StateImplemented)

		resp, err := w.handleTaskExport(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleTaskExport() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result taskExportResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if result.Task.ID != "test-task-id" {
			t.Errorf("Task.ID = %q, want test-task-id", result.Task.ID)
		}
		if result.ExportedAt == "" {
			t.Error("ExportedAt should be set")
		}
	})

	t.Run("markdown export", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		w.path = testutil.TempDir(t)
		setWorkUnitInState(t, w, conductor.StateImplemented)

		params, _ := json.Marshal(taskExportParams{Format: "md"})
		resp, err := w.handleTaskExport(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleTaskExport() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result map[string]string
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !strings.Contains(result["markdown"], "# Task Export: Test Task") {
			t.Errorf("markdown missing title header, got:\n%s", result["markdown"])
		}
	})
}

func TestRenderTaskExportMarkdown(t *testing.T) {
	meta := taskExportMeta{
		ID:          "task-1",
		Title:       "My <Task>",
		Description: "Do the thing",
		State:       "implemented",
		Branch:      "feature/x",
		Source:      &taskExportSource{Provider: "github", Reference: "owner/repo#1"},
		Tags:        []string{"bug", "p1"},
		PRID:        "PR-9",
		CreatedAt:   time.Now().Format(time.RFC3339),
		UpdatedAt:   time.Now().Format(time.RFC3339),
	}
	specs := []taskExportSpec{{Path: "spec.md", Content: "spec body"}}
	chat := []storage.ChatMessage{{Role: "user", Content: "hi", Timestamp: "2026-01-01"}}
	checkpoints := []CheckpointInfo{{SHA: "abcdef1234567890", Message: "checkpoint one"}}
	files := []git.FileStatus{{Status: "M", Path: "main.go"}}
	reviews := []storage.Review{{Number: 1, Title: "LGTM", Status: "approved", Reviewer: "alice", Content: "looks good"}}

	md := renderTaskExportMarkdown(meta, specs, chat, checkpoints, files, reviews, "2026-05-23T00:00:00Z")

	wantSubstrings := []string{
		"# Task Export: My &lt;Task&gt;", // title is HTML-escaped
		"**Branch:** feature/x",
		"**Source:** owner/repo#1",
		"**Tags:** bug, p1",
		"**PR:** PR-9",
		"## Description",
		"## Specifications",
		"spec body",
		"## File Changes",
		"main.go",
		"## Checkpoints",
		"abcdef12", // SHA truncated to 8 chars
		"checkpoint one",
		"## Reviews",
		"LGTM",
		"alice",
		"## Chat History",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(md, want) {
			t.Errorf("rendered markdown missing %q", want)
		}
	}
}

func TestRenderTaskExportMarkdown_Minimal(t *testing.T) {
	// A bare task with no extras should still render a header without panicking.
	meta := taskExportMeta{
		ID:        "task-2",
		Title:     "Bare",
		State:     "loaded",
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	md := renderTaskExportMarkdown(meta, nil, nil, nil, nil, nil, "2026-05-23T00:00:00Z")
	if !strings.Contains(md, "# Task Export: Bare") {
		t.Errorf("missing title header: %s", md)
	}
	// Optional sections must be absent.
	if strings.Contains(md, "## Specifications") {
		t.Error("should not render Specifications section when empty")
	}
}
