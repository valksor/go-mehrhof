package conductor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/codegraph"
	"github.com/valksor/kvelmo/internal/git"
)

func TestDetectPRTemplate(t *testing.T) {
	t.Run("no template returns empty", func(t *testing.T) {
		dir := t.TempDir()
		if got := detectPRTemplate(dir); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("finds .github template", func(t *testing.T) {
		dir := t.TempDir()
		ghDir := filepath.Join(dir, ".github")
		if err := os.MkdirAll(ghDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "## Summary\n\n## Testing\n"
		if err := os.WriteFile(filepath.Join(ghDir, "PULL_REQUEST_TEMPLATE.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := detectPRTemplate(dir); got != content {
			t.Errorf("got %q, want %q", got, content)
		}
	})
}

func TestLoadRecordingsFlat(t *testing.T) {
	// No phase metrics → empty result.
	if got := loadRecordingsFlat(nil); got != nil {
		t.Errorf("nil metrics should yield nil, got %v", got)
	}

	// Metrics with empty recording path → skipped.
	pm := map[string]*PhaseMetrics{
		"plan":      {RecordingPath: ""},
		"implement": {RecordingPath: "/nonexistent/recording.jsonl"},
	}
	if got := loadRecordingsFlat(pm); got != nil {
		t.Errorf("missing recordings should yield nil, got %v", got)
	}
}

func TestPreviewSubmit_NoTask(t *testing.T) {
	c := newTestConductor(t)
	_, err := c.PreviewSubmit(context.Background())
	if err == nil {
		t.Error("PreviewSubmit with no task should error")
	}
}

func TestPreviewSubmit_WithTask(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	c.ForceWorkUnit(&WorkUnit{
		ID:             "prev-1",
		Title:          "Preview Task",
		Description:    "Add a preview feature",
		Branch:         "feature/preview",
		Specifications: []string{"spec.md"},
		Checkpoints:    []string{"sha1"},
		Source:         &Source{Provider: "github", Reference: "owner/repo#1", URL: "https://example.com"},
	})

	preview, err := c.PreviewSubmit(context.Background())
	if err != nil {
		t.Fatalf("PreviewSubmit error = %v", err)
	}
	if preview.Branch != "feature/preview" {
		t.Errorf("branch = %q", preview.Branch)
	}
	if !strings.Contains(preview.Body, "## Summary") {
		t.Errorf("body missing summary section:\n%s", preview.Body)
	}
	if !strings.Contains(preview.Body, "Add a preview feature") {
		t.Errorf("body missing description:\n%s", preview.Body)
	}
	if preview.Checkpoints != 1 {
		t.Errorf("checkpoints = %d, want 1", preview.Checkpoints)
	}
	if preview.Specifications != 1 {
		t.Errorf("specifications = %d, want 1", preview.Specifications)
	}
	// Default PR title pattern interpolates the title.
	if !strings.Contains(preview.Title, "Preview Task") {
		t.Errorf("title = %q", preview.Title)
	}
}

func TestSaveArtifact_NoTask(t *testing.T) {
	c := newTestConductor(t)
	if _, err := c.SaveArtifact(context.Background(), "plan", "content"); err == nil {
		t.Error("SaveArtifact with no task should error")
	}
}

func TestSaveArtifact_UnsupportedKind(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	c.ForceWorkUnit(&WorkUnit{ID: "art-1", Title: "T"})
	if _, err := c.SaveArtifact(context.Background(), "bogus", "content"); err == nil {
		t.Error("SaveArtifact with unsupported kind should error")
	}
}

func TestSaveArtifact_PlanAndImplementation(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	c.ForceWorkUnit(&WorkUnit{ID: "art-2", Title: "T"})

	ctx := context.Background()
	planPath, err := c.SaveArtifact(ctx, "plan", "# Plan\n\nDo it.")
	if err != nil {
		t.Fatalf("SaveArtifact plan error = %v", err)
	}
	if !strings.Contains(planPath, "plan-1.md") {
		t.Errorf("plan path = %q, want plan-1.md", planPath)
	}
	if _, err := os.Stat(planPath); err != nil {
		t.Errorf("plan file not written: %v", err)
	}

	// A second plan artifact gets the next sequence number.
	planPath2, err := c.SaveArtifact(ctx, "plan", "# Plan 2")
	if err != nil {
		t.Fatalf("second SaveArtifact plan error = %v", err)
	}
	if !strings.Contains(planPath2, "plan-2.md") {
		t.Errorf("second plan path = %q, want plan-2.md", planPath2)
	}

	implPath, err := c.SaveArtifact(ctx, "implementation_summary", "# Impl")
	if err != nil {
		t.Fatalf("SaveArtifact impl error = %v", err)
	}
	if !strings.Contains(implPath, "implementation-1.md") {
		t.Errorf("impl path = %q", implPath)
	}
}

func TestFinalizePhase_NoTask(t *testing.T) {
	c := newTestConductor(t)
	if err := c.FinalizePhase(context.Background(), EventImplementDone); err == nil {
		t.Error("FinalizePhase with no task should error")
	}
}

func TestFinalizePhase_PlanWithoutSpecErrors(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	c.ForceWorkUnit(&WorkUnit{ID: "fin-1", Title: "T"})
	// EventPlanDone requires a spec file; none exists → error.
	if err := c.FinalizePhase(context.Background(), EventPlanDone); err == nil {
		t.Error("FinalizePhase(EventPlanDone) without spec should error")
	}
}

func TestStop_NotStoppableState(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "stop-1"})
	c.machine.ForceState(StatePlanned) // planned has no stop transition
	if err := c.Stop(context.Background()); err == nil {
		t.Error("Stop from a non-stoppable state should error")
	}
}

func TestStop_FromImplementing(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "stop-2"})
	c.machine.ForceState(StateImplementing)

	if err := c.Stop(context.Background()); err != nil {
		t.Fatalf("Stop from implementing error = %v", err)
	}
	// Implementing -> Planned per the stop transition table.
	if c.State() != StatePlanned {
		t.Errorf("state after stop = %s, want planned", c.State())
	}
	wu := c.GetWorkUnit()
	if wu.CancelledBy != "user" {
		t.Errorf("CancelledBy = %q, want user", wu.CancelledBy)
	}
}

func TestCheckRequiredPhases(t *testing.T) {
	c := newTestConductor(t)
	wu := &WorkUnit{ID: "req-1"}
	c.ForceWorkUnit(wu)

	// Seed history with implement_done but not review_done.
	c.machine.RestoreHistory([]HistoryEntry{
		{From: StatePlanned, To: StateImplementing, Event: EventImplement},
		{From: StateImplementing, To: StateImplemented, Event: EventImplementDone},
	})

	c.mu.Lock()
	defer c.mu.Unlock()

	// Implement satisfied → no error.
	if err := c.checkRequiredPhases([]string{PhaseImplement}); err != nil {
		t.Errorf("implement required should pass: %v", err)
	}
	// Review not completed → error.
	if err := c.checkRequiredPhases([]string{PhaseReview}); err == nil {
		t.Error("review required should fail when not completed")
	}
	// Unknown phase is silently skipped.
	if err := c.checkRequiredPhases([]string{"deploy"}); err != nil {
		t.Errorf("unknown phase should be skipped: %v", err)
	}
}

func TestCheckSensitivePaths(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "sens-1"})

	c.mu.Lock()
	defer c.mu.Unlock()

	// No sensitive paths configured → no error.
	if err := c.checkSensitivePaths(nil, []git.FileStatus{{Path: "a.go"}}); err != nil {
		t.Errorf("no sensitive paths should pass: %v", err)
	}

	// Sensitive path matched, no review done → error.
	changed := []git.FileStatus{{Path: "config/prod/secrets.yaml", Status: "modified"}}
	if err := c.checkSensitivePaths([]string{"config/"}, changed); err == nil {
		t.Error("sensitive path change without review should error")
	}

	// Glob pattern match.
	changedGo := []git.FileStatus{{Path: "auth.go", Status: "modified"}}
	if err := c.checkSensitivePaths([]string{"*.go"}, changedGo); err == nil {
		t.Error("glob match without review should error")
	}

	// Non-matching path → no error.
	if err := c.checkSensitivePaths([]string{"secrets/*"}, changedGo); err != nil {
		t.Errorf("non-matching path should pass: %v", err)
	}
}

func TestCheckSensitivePaths_ReviewDoneSkips(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "sens-2"})
	c.machine.RestoreHistory([]HistoryEntry{
		{From: StateImplemented, To: StateReviewing, Event: EventReview},
		{From: StateReviewing, To: StateImplemented, Event: EventReviewDone},
	})

	c.mu.Lock()
	defer c.mu.Unlock()

	changed := []git.FileStatus{{Path: "config/prod/secrets.yaml", Status: "modified"}}
	// Review was completed → sensitive path check passes.
	if err := c.checkSensitivePaths([]string{"config/"}, changed); err != nil {
		t.Errorf("review done should bypass sensitive path check: %v", err)
	}
}

func TestFormatSymbolResults(t *testing.T) {
	r := &ContextResolver{}
	symbols := []codegraph.Symbol{
		{Name: "Foo", Kind: "func", File: "foo.go", Line: 10, Package: "main"},
		{Name: "Bar", Kind: "type", File: "bar.go", Line: 20, Package: "pkg"},
	}
	got, err := r.formatSymbolResults("Foo", symbols, "")
	if err != nil {
		t.Fatalf("formatSymbolResults error = %v", err)
	}
	if got.Label != "Symbol: Foo" {
		t.Errorf("label = %q", got.Label)
	}
	if !strings.Contains(got.Content, "func Foo") || !strings.Contains(got.Content, "foo.go:10") {
		t.Errorf("content missing symbol details:\n%s", got.Content)
	}

	// Custom label override.
	got2, _ := r.formatSymbolResults("Foo", symbols, "Custom")
	if got2.Label != "Custom" {
		t.Errorf("custom label not used: %q", got2.Label)
	}
}

func TestResolve_UnknownType(t *testing.T) {
	r := &ContextResolver{WorktreeRoot: t.TempDir()}
	_, err := r.Resolve(context.Background(), ContextItem{Type: ContextType("bogus"), Ref: "x"})
	if err == nil {
		t.Error("unknown context type should error")
	}
}

func TestResolveCommitAndBranch(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	for _, args := range [][]string{
		{"init", dir},
		{"-C", dir, "config", "user.email", "test@test.com"},
		{"-C", dir, "config", "user.name", "Test User"},
	} {
		gitCmd(ctx, t, args...)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-C", dir, "add", "."},
		{"-C", dir, "commit", "-m", "first commit"},
	} {
		gitCmd(ctx, t, args...)
	}

	repo, err := git.Open(dir)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	r := &ContextResolver{WorktreeRoot: dir, Repo: repo}

	// Commit resolution.
	commit, err := r.Resolve(ctx, ContextItem{Type: ContextTypeCommit, Ref: "HEAD"})
	if err != nil {
		t.Fatalf("resolve commit error = %v", err)
	}
	if !strings.Contains(commit.Content, "first commit") {
		t.Errorf("commit content missing message:\n%s", commit.Content)
	}

	// Branch resolution.
	branch, err := r.Resolve(ctx, ContextItem{Type: ContextTypeBranch, Ref: "HEAD"})
	if err != nil {
		t.Fatalf("resolve branch error = %v", err)
	}
	if !strings.Contains(branch.Content, "Latest commit") {
		t.Errorf("branch content missing latest commit:\n%s", branch.Content)
	}
}

func TestResolveCommitBranch_NoRepo(t *testing.T) {
	r := &ContextResolver{WorktreeRoot: t.TempDir()}
	if _, err := r.Resolve(context.Background(), ContextItem{Type: ContextTypeCommit, Ref: "HEAD"}); err == nil {
		t.Error("resolve commit without repo should error")
	}
	if _, err := r.Resolve(context.Background(), ContextItem{Type: ContextTypeBranch, Ref: "main"}); err == nil {
		t.Error("resolve branch without repo should error")
	}
}
