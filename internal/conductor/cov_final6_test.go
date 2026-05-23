package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRecordingsFlat_WithRecording(t *testing.T) {
	dir := t.TempDir()
	recPath := filepath.Join(dir, "rec.jsonl")

	// A recording file: header line, then record lines (JSONL).
	content := `{"job_id":"j1","agent":"mock","started_at":"2026-01-01T00:00:00Z"}
{"timestamp":"2026-01-01T00:00:01Z","job_id":"j1","direction":"out","type":"tool_use","event":{"content":"Edit","data":{"file_path":"main.go"}}}
{"timestamp":"2026-01-01T00:00:02Z","job_id":"j1","direction":"out","type":"text","event":{"content":"done"}}
`
	if err := os.WriteFile(recPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	pm := map[string]*PhaseMetrics{
		"implement": {RecordingPath: recPath},
	}
	flat := loadRecordingsFlat(pm)
	if len(flat) != 2 {
		t.Fatalf("expected 2 flattened records, got %d", len(flat))
	}
	if flat[0]["type"] != "tool_use" {
		t.Errorf("first record type = %v", flat[0]["type"])
	}
	if flat[0]["tool"] != "Edit" {
		t.Errorf("first record tool = %v", flat[0]["tool"])
	}
	if flat[0]["file"] != "main.go" {
		t.Errorf("first record file = %v", flat[0]["file"])
	}
}

func TestCommitSubTaskWork(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", dir},
		{"-C", dir, "config", "user.email", "t@t.com"},
		{"-C", dir, "config", "user.name", "T"},
	} {
		gitCmd(ctx, t, args...)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(ctx, t, "-C", dir, "add", ".")
	gitCmd(ctx, t, "-C", dir, "commit", "-m", "init")

	// No changes → no-op, returns nil.
	if err := commitSubTaskWork(ctx, dir, "noop"); err != nil {
		t.Errorf("commitSubTaskWork with no changes should return nil, got %v", err)
	}

	// With a change → commits.
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n\nvar B = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := commitSubTaskWork(ctx, dir, "add b"); err != nil {
		t.Fatalf("commitSubTaskWork error = %v", err)
	}
}

func TestCommitSubTaskWork_BadDir(t *testing.T) {
	if err := commitSubTaskWork(context.Background(), t.TempDir(), "msg"); err == nil {
		t.Error("commitSubTaskWork on a non-git dir should error")
	}
}

func TestApplyRouteDecision_RetryDispatches(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.SetAutoAdvance(false)
	c.machine.ForceState(StateImplemented)

	// Retry below max → handled (true); re-submit happens in a goroutine.
	handled := c.applyRouteDecision(context.Background(), RouteDecision{
		Action: RouteRetry, Attempt: 0, MaxRetries: 2, Reason: "needs work",
	}, EventImplementDone)
	if !handled {
		t.Error("RouteRetry below max should be handled (true)")
	}
	// Allow the re-submit goroutine to run (best-effort; just ensure no panic).
	time.Sleep(100 * time.Millisecond)
}

func TestApplyRouteDecision_RollbackDispatches(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.SetAutoAdvance(false)
	c.machine.ForceState(StateImplementing)
	c.workUnit.Specifications = []string{"spec.md"}

	// Rollback with a target → dispatches error then re-runs target phase.
	handled := c.applyRouteDecision(context.Background(), RouteDecision{
		Action: RouteRollback, TargetPhase: PhasePlan, Reason: "wrong approach",
	}, EventImplementDone)
	if !handled {
		t.Error("RouteRollback with target should be handled (true)")
	}
	time.Sleep(100 * time.Millisecond)
}

func TestDispatchAutoAdvance_Review(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.machine.ForceState(StateImplemented)
	c.workUnit.HasImplemented = true

	// Review path: dispatchAutoAdvance(review) calls c.Review which transitions
	// to reviewing.
	c.dispatchAutoAdvance(context.Background(), PhaseReview)
	if c.State() != StateReviewing {
		t.Errorf("state after auto-advance review = %s, want reviewing", c.State())
	}
}

func TestBuildProjectCommandsSection_WithMakefile(t *testing.T) {
	c, dir := setupExecConductor(t)
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("build:\n\t@echo build\n\ntest:\n\t@echo test\n\nlint:\n\t@echo lint\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Should run discovery and return a (possibly non-empty) section without panic.
	_ = c.buildProjectCommandsSection()
}
