package socket

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/worker"
)

// --- worktree_changelog.go: AI path via worker pool ---

func TestWorktreeHandleChangelogGenerate_ViaAgent(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithRepo(t)
	// Wire a worker pool so handleChangelogGenerate takes the changelogViaAgent
	// branch (which gathers commits and submits a job).
	w.pool = worker.NewPool(worker.PoolConfig{MaxWorkers: 1})

	t.Run("no repo", func(t *testing.T) {
		bare := newTestWorktreeSocket(ctx, t) // repo nil
		bare.pool = w.pool
		resp, err := bare.handleChangelogGenerate(ctx, &Request{ID: "1", Params: mustMarshal(t, map[string]string{"source": "v0.1.0", "target": "v0.2.0"})})
		if err != nil {
			t.Fatalf("handleChangelogGenerate() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no repo")
		}
	})

	t.Run("missing source/target", func(t *testing.T) {
		resp, err := w.handleChangelogGenerate(ctx, &Request{ID: "1", Params: mustMarshal(t, map[string]string{"source": "", "target": ""})})
		if err != nil {
			t.Fatalf("handleChangelogGenerate() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing source/target")
		}
	})

	t.Run("submits changelog job", func(t *testing.T) {
		resp, err := w.handleChangelogGenerate(ctx, &Request{ID: "1", Params: mustMarshal(t, map[string]any{"source": "v0.1.0", "target": "v0.2.0"})})
		if err != nil {
			t.Fatalf("handleChangelogGenerate() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		// Either a job was submitted (status=generating, job_id set) or there
		// were no commits in range (markdown=""). Both are valid AI-path
		// outcomes; assert the call produced a structured result.
		if result["status"] == nil && result["markdown"] == nil {
			t.Errorf("unexpected changelog result shape: %v", result)
		}
	})

	t.Run("full mode submits job", func(t *testing.T) {
		resp, err := w.handleChangelogGenerate(ctx, &Request{ID: "1", Params: mustMarshal(t, map[string]any{"source": "v0.1.0", "target": "v0.2.0", "full": true})})
		if err != nil {
			t.Fatalf("handleChangelogGenerate() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
	})
}

// --- worktree_recap.go: handleRecap with a real repo + checkpoint ---

func TestWorktreeHandleRecap_WithRepoAndCheckpoint(t *testing.T) {
	ctx := context.Background()
	w := gitWorktree(ctx, t)
	setWorkUnitInState(t, w, conductor.StateImplemented)

	// Modify a tracked file so DiffFilesWithStatus reports a working-tree change.
	if err := os.WriteFile(filepath.Join(w.path, "README.md"), []byte("# Changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Record HEAD as a checkpoint so the enrichment + LastActivity path runs.
	headSHA := currentHeadSHA(t, w.path)
	w.conductor.WorkUnit().Checkpoints = []string{headSHA}
	w.conductor.WorkUnit().Tags = []string{"feature"}

	resp, err := w.handleRecap(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRecap() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result RecapResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.CheckpointCount != 1 {
		t.Errorf("CheckpointCount = %d, want 1", result.CheckpointCount)
	}
	if result.LastCheckpoint == nil {
		t.Fatal("expected LastCheckpoint to be set")
	}
	if result.LastCheckpoint.Message == "" {
		t.Error("expected enriched checkpoint message")
	}
	if len(result.FilesChanged) == 0 {
		t.Error("expected FilesChanged to include the working-tree change")
	}
}

func currentHeadSHA(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}

	return string(out[:len(out)-1]) // strip trailing newline
}

// --- worktree_phase.go: dry-run preview branches ---

func TestWorktreePhaseHandlers_DryRun(t *testing.T) {
	ctx := context.Background()
	// DryRun toggles conductor dry-run mode then attempts the transition. From
	// the none state every transition still errors, but the dry-run set/reset
	// branch is exercised.
	cases := []struct {
		name    string
		params  any
		handler func(*WorktreeSocket) func(context.Context, *Request) (*Response, error)
	}{
		{"plan", PlanParams{DryRun: true}, func(w *WorktreeSocket) func(context.Context, *Request) (*Response, error) { return w.handlePlan }},
		{"implement", ImplementParams{DryRun: true}, func(w *WorktreeSocket) func(context.Context, *Request) (*Response, error) { return w.handleImplement }},
		{"optimize", OptimizeParams{DryRun: true}, func(w *WorktreeSocket) func(context.Context, *Request) (*Response, error) { return w.handleOptimize }},
		{"simplify", SimplifyParams{DryRun: true}, func(w *WorktreeSocket) func(context.Context, *Request) (*Response, error) { return w.handleSimplify }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWorktreeSocket(ctx, t)
			resp, err := tc.handler(w)(ctx, &Request{ID: "1", Params: mustMarshal(t, tc.params)})
			if err != nil {
				t.Fatalf("handle%s() error = %v", tc.name, err)
			}
			// Dry-run mode must always be restored to its prior value (false).
			if w.conductor.DryRunEnabled() {
				t.Errorf("handle%s() left dry-run enabled after returning", tc.name)
			}
			if resp == nil {
				t.Fatalf("handle%s() returned nil response", tc.name)
			}
		})
	}
}
