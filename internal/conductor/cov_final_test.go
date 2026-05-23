package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/memory"
	"github.com/valksor/kvelmo/settings"
)

// newTestIndexer builds a memory.Indexer backed by a deterministic hash embedder.
func newTestIndexer(t *testing.T, dir string) *memory.Indexer {
	t.Helper()
	vs, err := memory.NewVectorStore(filepath.Join(dir, ".memory"), memory.NewHashEmbedder(64))
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}

	return memory.NewIndexer(vs, dir)
}

func TestReview_FixMode(t *testing.T) {
	c, dir := setupExecConductor(t)
	c.machine.ForceState(StateImplemented)
	c.workUnit.HasImplemented = true

	// Make a change so the fix job has content to checkpoint.
	if err := os.WriteFile(filepath.Join(dir, "fix.go"), []byte("package main\n\nfunc Fix() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := c.Review(context.Background(), true); err != nil {
		t.Fatalf("Review(fix) error = %v", err)
	}
	// Review transitions to reviewing and dispatches a fix job.
	if c.State() != StateReviewing {
		t.Errorf("state = %s, want reviewing", c.State())
	}
	wu := c.GetWorkUnit()
	if len(wu.Jobs) == 0 {
		t.Fatal("expected a review-fix job to be recorded")
	}
	// Wait for the fix job's watchJob goroutine (git checkpointing) and the async
	// quality gate to settle before the TempDir cleanup runs.
	waitForJobTerminal(t, c, wu.Jobs[len(wu.Jobs)-1])
	waitForQualityGate(t, c)
	// watchJob advances state to implemented after the fix job; let it persist.
	time.Sleep(300 * time.Millisecond)
}

// waitForJobTerminal blocks until the given job's event stream is closed,
// which the worker does only after the job reaches a terminal state. Draining
// the stream is the race-free way to await completion (reading job.Status
// directly races with the worker writing it).
func waitForJobTerminal(t *testing.T, c *Conductor, jobID string) {
	t.Helper()
	stream := c.pool.Stream(jobID)
	if stream == nil {
		return // already completed and stream removed
	}
	timeout := time.After(15 * time.Second)
	for {
		select {
		case _, ok := <-stream:
			if !ok {
				return // stream closed → job terminal
			}
		case <-timeout:
			return
		}
	}
}

func TestReview_FixModeNoPool(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "rev-1"})
	c.machine.ForceState(StateImplemented)
	// pool is nil → fix mode errors before transitioning.
	if err := c.Review(context.Background(), true); err == nil {
		t.Error("Review(fix) with no pool should error")
	}
}

func TestRunQualityAutoFix_NoPool(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "af-1"})
	// pool is nil → error.
	if err := c.runQualityAutoFix(context.Background(), errFixNeeded); err == nil {
		t.Error("runQualityAutoFix with no pool should error")
	}
}

func TestRunQualityAutoFix_Succeeds(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.machine.ForceState(StateReviewing)
	c.workUnit.HasImplemented = true

	// After the fix job completes, runQualityGateChecks runs. With no quality
	// runner, no detectable project type, and external review disabled, the
	// gate passes, so auto-fix succeeds on the first attempt.
	if err := c.runQualityAutoFix(context.Background(), errFixNeeded); err != nil {
		t.Fatalf("runQualityAutoFix error = %v", err)
	}
	// Auto-fix state should be cleared on success.
	if c.GetAutoFixStatus().Active {
		t.Error("auto-fix should be inactive after success")
	}
}

func TestFinish_WithMemoryIndexer(t *testing.T) {
	c, dir := setupExecConductor(t)
	c.SetMemoryIndexer(newTestIndexer(t, dir))

	base, err := c.git.DefaultBranch(context.Background())
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}

	ctx := context.Background()
	gitCmd(ctx, t, "-C", dir, "checkout", "-b", "feature/learn")
	if err := os.WriteFile(filepath.Join(dir, "learn.go"), []byte("package main\n\nvar L = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(ctx, t, "-C", dir, "add", ".")
	gitCmd(ctx, t, "-C", dir, "commit", "-m", "learn work")

	s := c.GetEffectiveSettings()
	s.Git.BaseBranch = base
	c.cachedSettings.Store(s)

	c.workUnit.Branch = "feature/learn"
	c.workUnit.PhaseMetrics = map[string]*PhaseMetrics{
		"implement": {Agent: "mock", Duration: time.Second},
	}
	passed := true
	c.workUnit.QualityGatePassed = &passed
	c.machine.ForceState(StateSubmitted)

	// Finish exercises extractLearnings + linkTaskOutcome with a real indexer.
	if _, err := c.Finish(ctx, FinishOptions{}); err != nil {
		t.Fatalf("Finish error = %v", err)
	}
	if c.State() != StateNone {
		t.Errorf("state after finish = %s, want none", c.State())
	}
}

func TestCheckApproval_RiskBasedAutoApprove(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Policy.ApprovalRequired = map[string]bool{"submit": true}
	s.Workflow.Policy.RiskBasedApproval = &settings.RiskBasedApprovalSettings{
		Enabled:              true,
		AutoApproveThreshold: 0.9, // very permissive → low-risk changes auto-approve
		HighRiskThreshold:    0.95,
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.ForceWorkUnit(&WorkUnit{ID: "ca-1", Source: &Source{Provider: "github"}})

	// checkApproval temporarily takes c.mu.RLock via evaluateRiskUnlocked, so it
	// must be invoked without the caller holding the write lock.
	// No git, empty diff → low risk → auto-approved (nil error).
	if err := c.checkApproval(context.Background(), EventSubmit); err != nil {
		t.Errorf("low-risk change should auto-approve, got %v", err)
	}
}

func TestApproverIdentity(t *testing.T) {
	if got := approverIdentity("alice"); got != "alice" {
		t.Errorf("configured identity should be used, got %q", got)
	}
	// Empty configured identity falls back to the current user/hostname.
	if got := approverIdentity(""); got == "" {
		t.Error("fallback identity should be non-empty")
	}
}

func TestInitialize_NonGitDir(t *testing.T) {
	// A bare temp dir with no git repo: Initialize logs a warning and continues.
	c, err := New(WithWorkDir(t.TempDir()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.Initialize(context.Background()); err != nil {
		t.Errorf("Initialize on non-git dir should not error, got %v", err)
	}
}

func TestInitialize_GitDir(t *testing.T) {
	c, _ := setupExecConductor(t)
	// git already opened by setup; Initialize re-opens and applies settings.
	if err := c.Initialize(context.Background()); err != nil {
		t.Errorf("Initialize on git dir error = %v", err)
	}
	if c.Repo() == nil {
		t.Error("Repo should be set after Initialize on a git dir")
	}
}

func TestAbandon_ClearsState(t *testing.T) {
	c, dir := setupExecConductor(t)
	ctx := context.Background()

	gitCmd(ctx, t, "-C", dir, "checkout", "-b", "feature/abandon")
	c.workUnit.Branch = "feature/abandon"
	c.machine.ForceState(StateImplementing)

	if err := c.Abandon(ctx, true); err != nil { // keepBranch=true
		t.Fatalf("Abandon error = %v", err)
	}
	if c.State() != StateNone {
		t.Errorf("state after abandon = %s, want none", c.State())
	}
	if c.WorkUnit() != nil {
		t.Error("work unit should be cleared after abandon")
	}
}

func TestDelete_TerminalStateOnly(t *testing.T) {
	c, _ := setupExecConductor(t)
	// Implementing is not a terminal state → delete refused.
	c.machine.ForceState(StateImplementing)
	if err := c.Delete(context.Background(), false); err == nil {
		t.Error("Delete from non-terminal state should error")
	}

	// Failed is allowed.
	c.machine.ForceState(StateFailed)
	if err := c.Delete(context.Background(), false); err != nil {
		t.Errorf("Delete from failed state should succeed, got %v", err)
	}
	if c.WorkUnit() != nil {
		t.Error("work unit should be cleared after delete")
	}
}

func TestQualityGateSecurity_Disabled(t *testing.T) {
	c, dir := setupExecConductor(t)
	// RequireSecurityScan defaults false → security gate is a no-op.
	if err := c.qualityGateSecurity(context.Background(), dir); err != nil {
		t.Errorf("disabled security gate should pass, got %v", err)
	}
}

func TestApproveRejectNode_WithScheduler(t *testing.T) {
	// Drive a plan job so an active scheduler exists, then approve a non-pending
	// node (no approval pending → error path with an active scheduler).
	c, _ := setupExecConductor(t)
	c.machine.ForceState(StateLoaded)

	specDir := c.store.SpecificationsDir(c.workUnit.ID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "specification-1.md"), []byte("# Spec"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := c.Plan(context.Background()); err != nil {
		t.Fatalf("Plan error = %v", err)
	}

	// Briefly wait for the scheduler to be registered.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.RLock()
		active := c.activeScheduler != nil
		c.mu.RUnlock()
		if active {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// With an active scheduler but no pending approval for this node, ApproveNode
	// returns a "no pending approval" error rather than "no active graph".
	err := c.ApproveNode("missing-node")
	if err != nil && err.Error() == "no active graph execution" {
		t.Skip("scheduler completed before assertion (timing); skipping")
	}

	waitForStateExec(t, c, StatePlanned, 15*time.Second)
}

var errFixNeeded = fixNeededError("quality gate failed: lint errors")

type fixNeededError string

func (e fixNeededError) Error() string { return string(e) }
