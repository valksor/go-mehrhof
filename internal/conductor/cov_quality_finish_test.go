package conductor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/git"
)

// mockQualityRunner implements the QualityRunner interface for tests.
type mockQualityRunner struct {
	passed   bool
	findings []string
	err      error
}

func (m *mockQualityRunner) RunGates(_ context.Context, _ string) (bool, []string, error) {
	return m.passed, m.findings, m.err
}

func TestRunStructuredQualityGates_NoRunner(t *testing.T) {
	c := newTestConductor(t)
	// No runner configured → passes.
	if err := c.runStructuredQualityGates(context.Background(), t.TempDir()); err != nil {
		t.Errorf("no runner should pass, got %v", err)
	}
}

func TestRunStructuredQualityGates_Passing(t *testing.T) {
	c := newTestConductor(t)
	c.SetQualityRunner(&mockQualityRunner{passed: true})
	if err := c.runStructuredQualityGates(context.Background(), t.TempDir()); err != nil {
		t.Errorf("passing runner should pass, got %v", err)
	}
}

func TestRunStructuredQualityGates_RunnerError(t *testing.T) {
	c := newTestConductor(t)
	c.SetQualityRunner(&mockQualityRunner{err: errors.New("runner blew up")})
	err := c.runStructuredQualityGates(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("runner error should propagate")
	}
}

func TestRunStructuredQualityGates_Findings(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "q1"})
	// git is nil → hold-the-line classification is skipped, so findings gate.
	c.SetQualityRunner(&mockQualityRunner{passed: false, findings: []string{"unused var", "no docs"}})
	err := c.runStructuredQualityGates(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("findings without review should fail the gate")
	}
}

func TestRunQualityGateAsync_NoRunner(t *testing.T) {
	// With no quality runner and no project type detected in an empty temp dir,
	// the async gate completes and caches a result on the work unit.
	// setupExecConductor already disables external review (no blocking prompt).
	c, _ := setupExecConductor(t)
	c.runQualityGateAsync()

	// Wait for the gate channel to close.
	c.mu.RLock()
	ch := c.qualityGateCh
	c.mu.RUnlock()
	if ch != nil {
		<-ch
	}

	wu := c.GetWorkUnit()
	if wu.QualityGatePassed == nil {
		t.Error("QualityGatePassed should be set after async gate completes")
	}
}

func TestCreateCheckpoint_NoGit(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "cp-1"})
	if _, err := c.CreateCheckpoint(context.Background(), "msg"); err == nil {
		t.Error("CreateCheckpoint without git should error")
	}
}

func TestCreateCheckpoint_WithGit(t *testing.T) {
	c, dir := setupExecConductor(t)

	// Make a change to commit.
	if err := os.WriteFile(filepath.Join(dir, "cp.go"), []byte("package main\n\nvar X = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sha, err := c.CreateCheckpoint(context.Background(), "checkpoint one")
	if err != nil {
		t.Fatalf("CreateCheckpoint error = %v", err)
	}
	if sha == "" {
		t.Error("expected non-empty checkpoint SHA")
	}

	wu := c.GetWorkUnit()
	found := false
	for _, c := range wu.Checkpoints {
		if c == sha {
			found = true
		}
	}
	if !found {
		t.Error("checkpoint SHA not recorded in work unit")
	}
	if meta, ok := wu.CheckpointMeta[sha]; !ok || meta.Message != "checkpoint one" {
		t.Errorf("checkpoint meta = %+v", wu.CheckpointMeta[sha])
	}
}

func TestGenerateDeltaSpecification(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	c.ForceWorkUnit(&WorkUnit{ID: "delta-1", Title: "Delta"})

	c.mu.Lock()
	path, err := c.GenerateDeltaSpecification(context.Background(), "old content\n", "old content\nnew line\n")
	c.mu.Unlock()
	if err != nil {
		t.Fatalf("GenerateDeltaSpecification error = %v", err)
	}
	if path == "" {
		t.Fatal("expected a delta spec path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read delta spec: %v", err)
	}
	if len(data) == 0 {
		t.Error("delta spec file is empty")
	}
}

func TestSaveSpecification_WithStore(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	c.ForceWorkUnit(&WorkUnit{ID: "spec-1", Title: "Spec Task"})

	path, err := c.SaveSpecification("# Specification\n\nDo the thing.")
	if err != nil {
		t.Fatalf("SaveSpecification error = %v", err)
	}
	if path == "" {
		t.Fatal("expected a spec path")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("spec file not written: %v", err)
	}

	wu := c.GetWorkUnit()
	if len(wu.Specifications) == 0 {
		t.Error("specification not added to work unit")
	}
}

func TestFinish_NoTask(t *testing.T) {
	c := newTestConductor(t)
	if _, err := c.Finish(context.Background(), FinishOptions{}); err == nil {
		t.Error("Finish with no task should error")
	}
}

func TestFinish_WrongStateWithoutForce(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "fin-1"})
	c.machine.ForceState(StateImplemented)
	if _, err := c.Finish(context.Background(), FinishOptions{}); err == nil {
		t.Error("Finish from non-submitted state without force should error")
	}
}

func TestFinish_FromSubmitted(t *testing.T) {
	c, dir := setupExecConductor(t)

	// Determine the actual default branch git created (main or master).
	base, err := c.git.DefaultBranch(context.Background())
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}

	// Create a feature branch with a commit so Finish can delete it.
	ctx := context.Background()
	gitCmd(ctx, t, "-C", dir, "checkout", "-b", "feature/finish")
	if err := os.WriteFile(filepath.Join(dir, "fin.go"), []byte("package main\n\nvar F = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(ctx, t, "-C", dir, "add", ".")
	gitCmd(ctx, t, "-C", dir, "commit", "-m", "feature work")

	// Point base branch at the actual default branch so checkout/pull succeed.
	s := c.GetEffectiveSettings()
	s.Git.BaseBranch = base
	c.cachedSettings.Store(s)

	c.workUnit.Branch = "feature/finish"
	c.machine.ForceState(StateSubmitted)

	result, err := c.Finish(ctx, FinishOptions{})
	if err != nil {
		t.Fatalf("Finish error = %v", err)
	}
	if result.PreviousBranch != "feature/finish" {
		t.Errorf("PreviousBranch = %q", result.PreviousBranch)
	}
	if result.CurrentBranch != base {
		t.Errorf("CurrentBranch = %q, want %q", result.CurrentBranch, base)
	}
	if c.State() != StateNone {
		t.Errorf("state after finish = %s, want none", c.State())
	}
	if c.WorkUnit() != nil {
		t.Error("work unit should be cleared after finish")
	}
}

func TestFinish_GitNotAvailable(t *testing.T) {
	c := newTestConductor(t) // no git repo opened
	c.ForceWorkUnit(&WorkUnit{ID: "fin-2", Branch: "feature/x"})
	c.machine.ForceState(StateSubmitted)
	// git is nil → Finish errors after the learning extraction no-ops.
	if _, err := c.Finish(context.Background(), FinishOptions{}); err == nil {
		t.Error("Finish without git should error")
	}
}

func TestEvaluateAndMaybeIterate_NonPhaseEvent(t *testing.T) {
	c := newTestConductor(t)
	// A non-phase completion event returns false immediately.
	if c.evaluateAndMaybeIterate(context.Background(), EventStart, "output") {
		t.Error("non-phase event should not trigger iteration")
	}
}

func TestEvaluateAndMaybeIterate_AcceptedOutput(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "iter-1"})
	c.mu.Lock()
	c.iterationCount[PhaseImplement] = 1
	c.mu.Unlock()

	// Default strategy accepts plain output (no iteration marker), so this
	// resets the iteration count and returns false.
	if c.evaluateAndMaybeIterate(context.Background(), EventImplementDone, "looks complete") {
		t.Error("accepted output should not trigger iteration")
	}
	c.mu.RLock()
	count := c.iterationCount[PhaseImplement]
	c.mu.RUnlock()
	if count != 0 {
		t.Errorf("iteration count should reset to 0, got %d", count)
	}
}

func TestApplyFailurePolicy_NonPhaseEvent(t *testing.T) {
	c := newTestConductor(t)
	if c.applyFailurePolicy(context.Background(), EventStart, "boom") {
		t.Error("non-phase event should not be handled by failure policy")
	}
}

func TestApplyFailurePolicy_FailPolicy(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "fp-1"})
	c.phasePolicies[PhaseImplement] = PhasePolicy{Policy: FailurePolicyFail}

	// Implement uses fail policy → returns false (caller handles default error path).
	if c.applyFailurePolicy(context.Background(), EventImplementDone, "implement failed") {
		t.Error("fail policy should return false")
	}
	// The failure is classified and stored.
	if c.LastFailureClass() == "" {
		t.Error("LastFailureClass should be set after applyFailurePolicy")
	}
}

func TestCopySpecsToRepo_NoOutputPath(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "cs-1", Specifications: []string{"/tmp/x.md"}})
	// No SpecOutputPath configured → no-op, no panic.
	c.mu.Lock()
	c.copySpecsToRepo()
	c.mu.Unlock()
}

func TestCommitRepoSpecs_NotEnabled(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "cr-1"})
	// CommitSpecs defaults to nil/false → no-op, no panic.
	c.mu.Lock()
	c.commitRepoSpecs(context.Background())
	c.mu.Unlock()
}

func TestValidateAgentCommits_NilParams(t *testing.T) {
	c := newTestConductor(t)
	// Nil params → no-op, no panic.
	c.validateAgentCommits(context.Background(), nil)
}

func TestValidateAgentCommits_WithCommits(t *testing.T) {
	c, dir := setupExecConductor(t)
	ctx := context.Background()

	// Capture current HEAD as the baseline.
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	base, err := repo.CurrentCommit(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Add a non-conforming commit.
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte("package main\n\nvar B = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(ctx, t, "-C", dir, "add", ".")
	gitCmd(ctx, t, "-C", dir, "commit", "-m", "totally unconventional message")

	params := &commitValidationParams{
		pattern:        "^(feat|fix):",
		workDir:        dir,
		lastCheckpoint: base,
		checkpointSHAs: map[string]struct{}{},
	}
	// Should not panic; emits a degraded warning for the non-conforming commit.
	c.validateAgentCommits(ctx, params)
}
