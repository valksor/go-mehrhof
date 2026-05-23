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

// mcpWorktree builds a worktree socket rooted at a real temp dir, with a work
// unit in the given state, so MCP handlers (which read/write within the
// worktree) have a valid root.
func mcpWorktree(ctx context.Context, t *testing.T, state conductor.State) *WorktreeSocket {
	t.Helper()
	w := newTestWorktreeSocket(ctx, t)
	w.path = testutil.TempDir(t)
	setWorkUnitInState(t, w, state)

	return w
}

func TestWorktreeHandleMCPTaskGet(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleMCPTaskGet(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleMCPTaskGet() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("no task loaded", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleMCPTaskGet(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleMCPTaskGet() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no work unit")
		}
	})

	t.Run("returns task metadata", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateImplemented)
		resp, err := w.handleMCPTaskGet(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleMCPTaskGet() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result MCPTaskGetResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if result.ID != "test-task-id" {
			t.Errorf("ID = %q, want test-task-id", result.ID)
		}
		if result.State != string(conductor.StateImplemented) {
			t.Errorf("State = %q, want %q", result.State, conductor.StateImplemented)
		}
	})
}

func TestWorktreeHandleMCPSpecifications(t *testing.T) {
	ctx := context.Background()

	t.Run("no task loaded", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleMCPSpecifications(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleMCPSpecifications() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no work unit")
		}
	})

	t.Run("lists spec paths without content", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateImplemented)
		resp, err := w.handleMCPSpecifications(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleMCPSpecifications() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result MCPSpecificationsResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(result.Specifications) != 1 {
			t.Fatalf("specifications = %v, want 1", result.Specifications)
		}
		if result.Specifications[0].Content != "" {
			t.Error("content should be empty when include_content is false")
		}
	})

	t.Run("includes content when requested", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateImplemented)
		specPath := filepath.Join(w.path, "spec.md")
		if err := os.WriteFile(specPath, []byte("# Real spec"), 0o644); err != nil {
			t.Fatal(err)
		}
		w.conductor.WorkUnit().Specifications = []string{specPath}

		params, _ := json.Marshal(map[string]bool{"include_content": true})
		resp, err := w.handleMCPSpecifications(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPSpecifications() error = %v", err)
		}
		var result MCPSpecificationsResult
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Specifications) != 1 || result.Specifications[0].Content != "# Real spec" {
			t.Errorf("specifications = %+v, want content '# Real spec'", result.Specifications)
		}
	})
}

func TestWorktreeHandleMCPFileRead(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		resp, err := w.handleMCPFileRead(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleMCPFileRead() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing path", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		params, _ := json.Marshal(MCPFileReadParams{Path: ""})
		resp, err := w.handleMCPFileRead(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPFileRead() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing path")
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		params, _ := json.Marshal(MCPFileReadParams{Path: "../../../etc/passwd"})
		resp, err := w.handleMCPFileRead(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPFileRead() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for path traversal")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		params, _ := json.Marshal(MCPFileReadParams{Path: "missing.txt"})
		resp, err := w.handleMCPFileRead(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPFileRead() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for nonexistent file")
		}
	})

	t.Run("directory rejected", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		sub := filepath.Join(w.path, "subdir")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		params, _ := json.Marshal(MCPFileReadParams{Path: "subdir"})
		resp, err := w.handleMCPFileRead(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPFileRead() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for directory path")
		}
	})

	t.Run("reads file content", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		fp := filepath.Join(w.path, "hello.txt")
		if err := os.WriteFile(fp, []byte("hello world"), 0o644); err != nil {
			t.Fatal(err)
		}
		params, _ := json.Marshal(MCPFileReadParams{Path: "hello.txt"})
		resp, err := w.handleMCPFileRead(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPFileRead() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result MCPFileReadResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if result.Content != "hello world" {
			t.Errorf("Content = %q, want 'hello world'", result.Content)
		}
		if result.Size != 11 {
			t.Errorf("Size = %d, want 11", result.Size)
		}
	})
}

func TestWorktreeHandleMCPArtifactsSave(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleMCPArtifactsSave(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleMCPArtifactsSave() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		resp, err := w.handleMCPArtifactsSave(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleMCPArtifactsSave() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing kind or content", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		params, _ := json.Marshal(MCPArtifactsSaveParams{Kind: "", Content: ""})
		resp, err := w.handleMCPArtifactsSave(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPArtifactsSave() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing kind/content")
		}
	})

	t.Run("unsupported kind surfaces error", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		params, _ := json.Marshal(MCPArtifactsSaveParams{Kind: "nonsense", Content: "x"})
		resp, err := w.handleMCPArtifactsSave(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPArtifactsSave() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for unsupported artifact kind")
		}
	})
}

func TestWorktreeHandleMCPCheckpoint(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleMCPCheckpoint(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleMCPCheckpoint() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		resp, err := w.handleMCPCheckpoint(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleMCPCheckpoint() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing message", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		params, _ := json.Marshal(MCPCheckpointParams{Message: ""})
		resp, err := w.handleMCPCheckpoint(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPCheckpoint() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing message")
		}
	})

	t.Run("no repo surfaces error", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateLoaded)
		params, _ := json.Marshal(MCPCheckpointParams{Message: "checkpoint"})
		resp, err := w.handleMCPCheckpoint(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPCheckpoint() error = %v", err)
		}
		// No git repo wired into the conductor: creating a checkpoint must fail
		// with an error response rather than panic.
		if resp.Error == nil {
			t.Fatal("expected error response when no git repository available")
		}
	})
}

func TestWorktreeHandleMCPSignalComplete(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleMCPSignalComplete(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleMCPSignalComplete() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateImplementing)
		resp, err := w.handleMCPSignalComplete(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleMCPSignalComplete() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing phase", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateImplementing)
		params, _ := json.Marshal(MCPSignalCompleteParams{Phase: ""})
		resp, err := w.handleMCPSignalComplete(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPSignalComplete() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing phase")
		}
	})

	t.Run("unknown phase", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateImplementing)
		params, _ := json.Marshal(MCPSignalCompleteParams{Phase: "bogus"})
		resp, err := w.handleMCPSignalComplete(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPSignalComplete() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for unknown phase")
		}
	})

	t.Run("valid phase acknowledged", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateImplementing)
		params, _ := json.Marshal(MCPSignalCompleteParams{Phase: conductor.PhaseImplement})
		resp, err := w.handleMCPSignalComplete(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPSignalComplete() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result MCPSignalResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !result.OK {
			t.Error("OK = false, want true")
		}
	})
}

func TestWorktreeHandleMCPSignalFailure(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleMCPSignalFailure(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleMCPSignalFailure() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateImplementing)
		resp, err := w.handleMCPSignalFailure(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleMCPSignalFailure() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("acknowledged", func(t *testing.T) {
		w := mcpWorktree(ctx, t, conductor.StateImplementing)
		params, _ := json.Marshal(MCPSignalFailureParams{Phase: "implement", Reason: "boom"})
		resp, err := w.handleMCPSignalFailure(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleMCPSignalFailure() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %s", resp.Error.Message)
		}
		var result MCPSignalResult
		_ = json.Unmarshal(resp.Result, &result)
		if !result.OK {
			t.Error("OK = false, want true")
		}
	})
}

func TestPhaseToCompletionEvent(t *testing.T) {
	cases := []struct {
		phase   string
		wantErr bool
	}{
		{conductor.PhasePlan, false},
		{conductor.PhaseImplement, false},
		{conductor.PhaseSimplify, false},
		{conductor.PhaseOptimize, false},
		{conductor.PhaseReview, false},
		{"unknown", true},
		{"", true},
	}
	for _, tc := range cases {
		t.Run(tc.phase, func(t *testing.T) {
			ev, err := phaseToCompletionEvent(tc.phase)
			if tc.wantErr {
				if err == nil {
					t.Errorf("phaseToCompletionEvent(%q) expected error", tc.phase)
				}

				return
			}
			if err != nil {
				t.Errorf("phaseToCompletionEvent(%q) unexpected error: %v", tc.phase, err)
			}
			if ev == "" {
				t.Errorf("phaseToCompletionEvent(%q) returned empty event", tc.phase)
			}
		})
	}
}

func TestResolveWithinWorktree(t *testing.T) {
	t.Run("no path configured", func(t *testing.T) {
		w := nilConductorWorktree()
		if _, err := w.resolveWithinWorktree("x"); err == nil {
			t.Fatal("expected error when worktree path not configured")
		}
	})

	t.Run("resolves valid relative path", func(t *testing.T) {
		w := newTestWorktreeSocket(t.Context(), t)
		w.path = testutil.TempDir(t)
		fp := filepath.Join(w.path, "f.txt")
		if err := os.WriteFile(fp, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := w.resolveWithinWorktree("f.txt")
		if err != nil {
			t.Fatalf("resolveWithinWorktree() error = %v", err)
		}
		if got == "" {
			t.Error("expected non-empty resolved path")
		}
	})
}
