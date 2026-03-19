package conductor

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// ─── WithWorkDir ──────────────────────────────────────────────────────────────

func TestWithWorkDir(t *testing.T) {
	opts := DefaultOptions()
	WithWorkDir("/tmp/test-dir")(&opts)
	if opts.WorkDir != "/tmp/test-dir" {
		t.Errorf("WithWorkDir() = %q, want /tmp/test-dir", opts.WorkDir)
	}
}

// ─── Start ────────────────────────────────────────────────────────────────────

func TestConductorStart_WrongState(t *testing.T) {
	c, _ := New()
	c.machine.ForceState(StateLoaded)

	err := c.Start(context.Background(), "empty:test")
	if err == nil {
		t.Error("Start() from non-None state should return error")
	}
}

func TestConductorStart_InvalidRef(t *testing.T) {
	c, _ := New()
	// No colon prefix, not a URL → Parse returns error
	err := c.Start(context.Background(), "invalidref")
	if err == nil {
		t.Error("Start() with invalid ref should return error")
	}
}

func TestConductorStart_EmptyProvider(t *testing.T) {
	c, _ := New(WithWorkDir(t.TempDir()))

	err := c.Start(context.Background(), "empty:My test task for coverage")
	if err != nil {
		t.Errorf("Start() with empty provider error = %v", err)
	}
	if c.State() != StateLoaded {
		t.Errorf("Start() state = %s, want loaded", c.State())
	}
	if c.workUnit == nil {
		t.Fatal("workUnit should not be nil after successful Start()")
	}
	if c.workUnit.Title == "" {
		t.Error("workUnit.Title should not be empty")
	}
}

// ─── Optimize ────────────────────────────────────────────────────────────────

func TestConductorOptimize_NoTask(t *testing.T) {
	c, _ := New()
	_, err := c.Optimize(context.Background())
	if err == nil {
		t.Error("Optimize() with no task should return error")
	}
}

func TestConductorOptimize_WrongState(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "x", Title: "x"})
	// StateNone doesn't have EventOptimize transition
	_, err := c.Optimize(context.Background())
	if err == nil {
		t.Error("Optimize() from wrong state should return error")
	}
}

// ─── Simplify ─────────────────────────────────────────────────────────────────

func TestConductorSimplify_NoTask(t *testing.T) {
	c, _ := New()
	_, err := c.Simplify(context.Background())
	if err == nil {
		t.Error("Simplify() with no task should return error")
	}
}

func TestConductorSimplify_WrongState(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "x", Title: "x"})
	_, err := c.Simplify(context.Background())
	if err == nil {
		t.Error("Simplify() from wrong state should return error")
	}
}

// ─── Review ───────────────────────────────────────────────────────────────────

func TestConductorReview_NoTask(t *testing.T) {
	c, _ := New()
	err := c.Review(context.Background(), false)
	if err == nil {
		t.Error("Review() with no task should return error")
	}
}

func TestConductorReview_WrongState(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "x", Title: "x"})
	err := c.Review(context.Background(), false)
	if err == nil {
		t.Error("Review() from wrong state (None) should return error")
	}
}

func TestConductorReview_FromImplemented(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "r1", Title: "Review Task"})
	c.machine.ForceState(StateImplemented)

	err := c.Review(context.Background(), false)
	if err != nil {
		t.Errorf("Review() from implemented state error = %v", err)
	}
	if c.State() != StateReviewing {
		t.Errorf("after Review(): state = %s, want reviewing", c.State())
	}
}

// ─── AddReview ────────────────────────────────────────────────────────────────

func TestConductorAddReview_NoTask(t *testing.T) {
	c, _ := New()
	// workUnit is nil → early return, should not panic
	c.AddReview(true, "looks good")
}

func TestConductorAddReview_NilStore(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "ar1", Title: "Review Test"}
	c.ForceWorkUnit(wu)
	// store is nil → updates UpdatedAt then returns early
	c.AddReview(false, "needs work")
	// Verify workUnit.UpdatedAt was touched (not zero)
	if c.workUnit.UpdatedAt.IsZero() {
		t.Error("AddReview() should set UpdatedAt even with nil store")
	}
}

// ─── ListReviews ─────────────────────────────────────────────────────────────

func TestConductorListReviews_NoTask(t *testing.T) {
	c, _ := New()
	_, err := c.ListReviews()
	if err == nil {
		t.Error("ListReviews() with no task should return error")
	}
}

func TestConductorListReviews_NilStore(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "lr1", Title: "T"})
	_, err := c.ListReviews()
	if err == nil {
		t.Error("ListReviews() with nil store should return error")
	}
}

// ─── GetReview ────────────────────────────────────────────────────────────────

func TestConductorGetReview_NoTask(t *testing.T) {
	c, _ := New()
	_, err := c.GetReview(1)
	if err == nil {
		t.Error("GetReview() with no task should return error")
	}
}

func TestConductorGetReview_NilStore(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "gr1", Title: "T"})
	_, err := c.GetReview(1)
	if err == nil {
		t.Error("GetReview() with nil store should return error")
	}
}

// ─── Abandon ─────────────────────────────────────────────────────────────────

func TestConductorAbandon_NoTask(t *testing.T) {
	c, _ := New()
	err := c.Abandon(context.Background(), false)
	if err != nil {
		t.Errorf("Abandon() with no task error = %v", err)
	}
	if c.State() != StateNone {
		t.Errorf("after Abandon: state = %s, want none", c.State())
	}
}

func TestConductorAbandon_WithTask(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "ab1", Title: "Abandon Me"}
	c.ForceWorkUnit(wu)
	c.machine.ForceState(StateLoaded)

	err := c.Abandon(context.Background(), true) // keepBranch=true skips git delete
	if err != nil {
		t.Errorf("Abandon() error = %v", err)
	}
	if c.workUnit != nil {
		t.Error("Abandon() should clear workUnit")
	}
	if c.State() != StateNone {
		t.Errorf("after Abandon: state = %s, want none", c.State())
	}
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestConductorDelete_WrongState(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "d1", Title: "T"})
	c.machine.ForceState(StateLoaded)

	err := c.Delete(context.Background(), false)
	if err == nil {
		t.Error("Delete() from non-terminal state should return error")
	}
}

func TestConductorDelete_FromNone(t *testing.T) {
	c, _ := New()
	// StateNone is allowed, workUnit is nil so no git/store ops
	err := c.Delete(context.Background(), false)
	if err != nil {
		t.Errorf("Delete() from StateNone error = %v", err)
	}
}

func TestConductorDelete_FromSubmitted(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "d2", Title: "Done"}
	c.ForceWorkUnit(wu)
	c.machine.ForceState(StateSubmitted)

	err := c.Delete(context.Background(), false)
	if err != nil {
		t.Errorf("Delete() from submitted state error = %v", err)
	}
	if c.workUnit != nil {
		t.Error("Delete() should clear workUnit")
	}
}

// ─── UpdateTask ───────────────────────────────────────────────────────────────

func TestConductorUpdateTask_NoTask(t *testing.T) {
	c, _ := New()
	_, _, err := c.UpdateTask(context.Background())
	if err == nil {
		t.Error("UpdateTask() with no task should return error")
	}
}

// ─── Undo ─────────────────────────────────────────────────────────────────────

func TestConductorUndo_NoTask(t *testing.T) {
	c, _ := New()
	err := c.Undo(context.Background())
	if err == nil {
		t.Error("Undo() with no task should return error")
	}
}

func TestConductorUndo_NoCheckpoints(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "u1", Title: "T"})
	err := c.Undo(context.Background())
	if err == nil {
		t.Error("Undo() with no checkpoints should return error")
	}
}

func TestConductorUndo_NoGit(t *testing.T) {
	c, _ := New()
	// Undo requires at least 2 checkpoints: one to save for redo, one to reset to
	wu := &WorkUnit{
		ID:          "u2",
		Title:       "T",
		Checkpoints: []string{"checkpoint_A_abc", "checkpoint_B_xyz"},
	}
	c.ForceWorkUnit(wu)
	c.machine.ForceState(StateLoaded)

	err := c.Undo(context.Background())
	if err != nil {
		t.Errorf("Undo() without git should succeed: %v", err)
	}
	// After undo: checkpoints should be [A], having popped B
	if len(c.workUnit.Checkpoints) != 1 {
		t.Errorf("Undo() should remove one checkpoint, got %d", len(c.workUnit.Checkpoints))
	}
	if c.workUnit.Checkpoints[0] != "checkpoint_A_abc" {
		t.Errorf("Undo() should keep checkpoint A, got %s", c.workUnit.Checkpoints[0])
	}
	// Redo stack should contain B
	if len(c.workUnit.RedoStack) != 1 {
		t.Error("Undo() should add to redo stack")
	}
	if c.workUnit.RedoStack[0] != "checkpoint_B_xyz" {
		t.Errorf("Undo() should save checkpoint B to redo, got %s", c.workUnit.RedoStack[0])
	}
}

func TestConductorUndo_SingleCheckpoint(t *testing.T) {
	c, _ := New()
	// With only 1 checkpoint, undo should fail (need at least 2)
	wu := &WorkUnit{ID: "u3", Title: "T", Checkpoints: []string{"only_one"}}
	c.ForceWorkUnit(wu)
	c.machine.ForceState(StateLoaded)

	err := c.Undo(context.Background())
	if err == nil {
		t.Error("Undo() with single checkpoint should return error")
	}
}

// ─── Redo ─────────────────────────────────────────────────────────────────────

func TestConductorRedo_NoTask(t *testing.T) {
	c, _ := New()
	err := c.Redo(context.Background())
	if err == nil {
		t.Error("Redo() with no task should return error")
	}
}

func TestConductorRedo_NoRedoStack(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "r1", Title: "T"})
	err := c.Redo(context.Background())
	if err == nil {
		t.Error("Redo() with no redo stack should return error")
	}
}

func TestConductorRedo_NoGit(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "r2", Title: "T", RedoStack: []string{"def45678def45678"}}
	c.ForceWorkUnit(wu)
	c.machine.ForceState(StateLoaded)

	err := c.Redo(context.Background())
	if err != nil {
		t.Errorf("Redo() without git should succeed: %v", err)
	}
	if len(c.workUnit.RedoStack) != 0 {
		t.Error("Redo() should remove from redo stack")
	}
	if len(c.workUnit.Checkpoints) != 1 {
		t.Error("Redo() should add to checkpoints")
	}
}

// ─── GotoCheckpoint ───────────────────────────────────────────────────────────

func TestConductorGotoCheckpoint_NoTask(t *testing.T) {
	c, _ := New()
	err := c.GotoCheckpoint(context.Background(), "abc12345abc12345")
	if err == nil {
		t.Error("GotoCheckpoint() with no task should return error")
	}
}

func TestConductorGotoCheckpoint_NotFound(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{ID: "g1", Title: "T", Checkpoints: []string{"abc12345abc12345"}}
	c.ForceWorkUnit(wu)

	err := c.GotoCheckpoint(context.Background(), "notfound12345678")
	if err == nil {
		t.Error("GotoCheckpoint() with non-existent SHA should return error")
	}
}

func TestConductorGotoCheckpoint_NoGit(t *testing.T) {
	c, _ := New()
	wu := &WorkUnit{
		ID:          "g2",
		Title:       "T",
		Checkpoints: []string{"abc12345abc12345", "def45678def45678"},
	}
	c.ForceWorkUnit(wu)

	err := c.GotoCheckpoint(context.Background(), "abc12345abc12345")
	if err != nil {
		t.Errorf("GotoCheckpoint() without git should succeed: %v", err)
	}
	// "def45678def45678" should be in redo stack now
	if len(c.workUnit.RedoStack) != 1 {
		t.Errorf("GotoCheckpoint: redo stack = %v, want 1 item", c.workUnit.RedoStack)
	}
	if len(c.workUnit.Checkpoints) != 1 {
		t.Errorf("GotoCheckpoint: checkpoints = %v, want 1 item", c.workUnit.Checkpoints)
	}
}

// ─── logVerbosef ─────────────────────────────────────────────────────────────

func TestConductorLogVerbosef_Verbose(t *testing.T) {
	var buf bytes.Buffer
	c, _ := New(WithVerbose(true), WithStdout(&buf))
	c.logVerbosef("hello %s", "world")
	if buf.String() != "hello world\n" {
		t.Errorf("logVerbosef output = %q, want %q", buf.String(), "hello world\n")
	}
}

func TestConductorLogVerbosef_NotVerbose(t *testing.T) {
	var buf bytes.Buffer
	c, _ := New(WithVerbose(false), WithStdout(&buf))
	c.logVerbosef("hello %s", "world")
	if buf.String() != "" {
		t.Errorf("logVerbosef when not verbose = %q, want empty", buf.String())
	}
}

// ─── buildPRDescription ───────────────────────────────────────────────────────

func TestBuildPRDescription_EmptyDescription(t *testing.T) {
	result := buildPRDescription("", 0, 0, "", nil, nil)
	if !strings.Contains(result, "## Summary") {
		t.Error("buildPRDescription missing Summary section")
	}
}

func TestBuildPRDescription_Basic(t *testing.T) {
	result := buildPRDescription("Do something important", 0, 0, "", nil, nil)
	if !strings.Contains(result, "Do something important") {
		t.Error("buildPRDescription missing description")
	}
	if !strings.Contains(result, "## Summary") {
		t.Error("buildPRDescription missing Summary section")
	}
}

func TestBuildPRDescription_WithSpecs(t *testing.T) {
	result := buildPRDescription("desc", 1, 0, "", nil, nil)
	if !strings.Contains(result, "## Implementation") {
		t.Error("buildPRDescription with specs missing Implementation section")
	}
}

func TestBuildPRDescription_WithCheckpoints(t *testing.T) {
	result := buildPRDescription("desc", 0, 2, "", nil, nil)
	if !strings.Contains(result, "## Checkpoints") {
		t.Error("buildPRDescription with checkpoints missing Checkpoints section")
	}
	if !strings.Contains(result, "2 checkpoint") {
		t.Error("buildPRDescription with checkpoints missing count")
	}
}

// ─── buildSimplifyPrompt ─────────────────────────────────────────────────────

func TestBuildSimplifyPrompt(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{Title: "Simplify task", Description: "Make it simpler"})
	prompt := c.buildSimplifyPrompt()
	if !strings.Contains(prompt, "Simplify task") {
		t.Error("buildSimplifyPrompt missing task title")
	}
	if !strings.Contains(prompt, "simplify") {
		t.Error("buildSimplifyPrompt missing 'simplify'")
	}
}

// ─── buildOptimizePrompt ─────────────────────────────────────────────────────

func TestBuildOptimizePrompt(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{Title: "Optimize task", Description: "Make it faster"})
	prompt := c.buildOptimizePrompt()
	if !strings.Contains(prompt, "Optimize task") {
		t.Error("buildOptimizePrompt missing task title")
	}
	if !strings.Contains(prompt, "optimize") {
		t.Error("buildOptimizePrompt missing 'optimize'")
	}
}
