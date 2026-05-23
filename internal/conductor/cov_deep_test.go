package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent/strategy"
	"github.com/valksor/kvelmo/settings"
)

func TestAutoAdvance_FullLifecycle(t *testing.T) {
	c, dir := setupExecConductor(t)
	c.machine.ForceState(StatePlanned)
	c.workUnit.Specifications = []string{"spec.md"}
	c.SetAutoAdvance(true)
	// Skip simplify and optimize so the chain is implement -> review.
	c.SetSkipPhases([]string{PhaseSimplify, PhaseOptimize})

	// Make a change so the implement job produces a checkpoint.
	if err := os.WriteFile(filepath.Join(dir, "auto.go"), []byte("package main\n\nfunc Auto() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err := c.Implement(ctx); err != nil {
		t.Fatalf("Implement error = %v", err)
	}

	// Auto-advance should carry the workflow implement -> implemented -> review.
	waitForStateExec(t, c, StateReviewing, 20*time.Second)

	// Review kicks off an async quality gate goroutine that writes under the
	// work dir. Wait for it to finish so the test's TempDir cleanup doesn't race
	// with the goroutine's writes.
	c.mu.RLock()
	gateCh := c.qualityGateCh
	c.mu.RUnlock()
	if gateCh != nil {
		select {
		case <-gateCh:
		case <-time.After(15 * time.Second):
		}
	}
	time.Sleep(200 * time.Millisecond)
}

func TestCopySpecsAndPlanToRepo(t *testing.T) {
	c, dir := setupExecConductor(t)

	// Configure spec/plan output paths and write a real spec file.
	s := c.GetEffectiveSettings()
	s.Storage.SpecOutputPath = "docs/spec.md"
	commit := true
	s.Storage.CommitSpecs = &commit
	c.cachedSettings.Store(s)

	specFile := filepath.Join(dir, "the-spec.md")
	if err := os.WriteFile(specFile, []byte("# Spec content"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.workUnit.Specifications = []string{specFile}

	c.mu.Lock()
	c.copySpecsToRepo()
	c.commitRepoSpecs(context.Background())
	c.mu.Unlock()

	// The spec should be copied to the configured repo path.
	copied := filepath.Join(dir, "docs/spec.md")
	if _, err := os.Stat(copied); err != nil {
		t.Errorf("spec not copied to repo: %v", err)
	}
}

func TestLoadPhasePoliciesFromSettings(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.PhasePolicies = map[string]string{
		"implement": "fail",
		"simplify":  "skip",
		"optimize":  "retry",
	}
	s.Workflow.Retry.MaxAttempts = 4
	s.Workflow.Retry.BackoffSeconds = 2
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if c.phasePolicies[PhaseImplement].Policy != FailurePolicyFail {
		t.Errorf("implement policy = %v, want fail", c.phasePolicies[PhaseImplement].Policy)
	}
	if c.phasePolicies[PhaseSimplify].Policy != FailurePolicySkip {
		t.Errorf("simplify policy = %v, want skip", c.phasePolicies[PhaseSimplify].Policy)
	}
	opt := c.phasePolicies[PhaseOptimize]
	if opt.Policy != FailurePolicyRetry || opt.MaxRetries != 4 || opt.RetryDelay != 2*time.Second {
		t.Errorf("optimize policy = %+v, want retry/4/2s", opt)
	}
}

func TestLoadStrategiesFromSettings(t *testing.T) {
	s := settings.DefaultSettings()
	s.Agent.Strategy = "iterative"
	s.Agent.PhaseStrategy = map[string]string{"implement": "iterative"}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if c.strategy == nil {
		t.Error("default strategy should be loaded")
	}
	c.mu.RLock()
	_, ok := c.phaseStrategies["implement"]
	c.mu.RUnlock()
	if !ok {
		t.Error("per-phase strategy override should be loaded")
	}
}

func TestSetupCanaryHarness_Disabled(t *testing.T) {
	c := newTestConductor(t)
	// Canary disabled by default → no harness, no panic.
	c.mu.Lock()
	c.setupCanaryHarness()
	c.mu.Unlock()
	if c.canaryHarness != nil {
		t.Error("canary harness should not be set when disabled")
	}
	// checkCanaryViolations with nil harness is a no-op.
	c.mu.Lock()
	c.checkCanaryViolations("output")
	c.mu.Unlock()
}

func TestSetupCanaryHarness_Enabled(t *testing.T) {
	s := settings.DefaultSettings()
	s.Security.CanaryEnabled = true
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.mu.Lock()
	c.setupCanaryHarness()
	harnessSet := c.canaryHarness != nil
	// Clean checkCanaryViolations (resets harness to nil).
	c.checkCanaryViolations("clean output")
	c.mu.Unlock()

	if !harnessSet {
		t.Error("canary harness should be set when enabled")
	}
	if c.canaryHarness != nil {
		t.Error("canary harness should be cleaned up after check")
	}
}

func TestEvaluateAndMaybeIterate_TriggersIteration(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.machine.ForceState(StateImplemented)
	c.workUnit.HasImplemented = true

	// Use the iterative strategy for implement so an unresolved marker triggers
	// iteration.
	if start, ok := strategy.Get("iterative"); ok {
		c.SetPhaseStrategy(PhaseImplement, start)
	}

	// Output containing "TODO" → strategy requests iteration. The re-submit runs
	// in a goroutine; we just assert the function reports it handled iteration.
	handled := c.evaluateAndMaybeIterate(context.Background(), EventImplementDone, "did the work but TODO: finish edge case")
	if !handled {
		t.Error("output with TODO marker should trigger iteration")
	}
	c.mu.RLock()
	count := c.iterationCount[PhaseImplement]
	c.mu.RUnlock()
	if count != 1 {
		t.Errorf("iteration count = %d, want 1", count)
	}
}

func TestEvaluateAndMaybeIterate_MaxIterations(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.machine.ForceState(StateImplemented)
	if start, ok := strategy.Get("iterative"); ok {
		c.SetPhaseStrategy(PhaseImplement, start)
	}
	c.mu.Lock()
	c.maxIterations = 1
	c.iterationCount[PhaseImplement] = 1 // already at max
	c.mu.Unlock()

	// At max iterations, the result is accepted (returns false) and the count resets.
	handled := c.evaluateAndMaybeIterate(context.Background(), EventImplementDone, "still has a TODO")
	if handled {
		t.Error("at max iterations, should accept result (return false)")
	}
}

func TestApplyRouteDecision_SkipDispatches(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.SetAutoAdvance(false) // prevent auto-advance side effects after skip
	c.machine.ForceState(StatePlanning)

	// Skip dispatches the completion event to advance past the phase.
	handled := c.applyRouteDecision(context.Background(), RouteDecision{
		Action: RouteSkip, TargetPhase: PhaseImplement, Reason: "trivial",
	}, EventPlanDone)
	if !handled {
		t.Error("RouteSkip should be handled (true)")
	}
	// Planning + plan_done -> planned.
	if c.State() != StatePlanned {
		t.Errorf("state after skip = %s, want planned", c.State())
	}
}

func TestResumeFromCheckpoint_NoTask(t *testing.T) {
	c := newTestConductor(t)
	if err := c.ResumeFromCheckpoint(context.Background(), "abc"); err == nil {
		t.Error("ResumeFromCheckpoint with no task should error")
	}
}

func TestResumeFromCheckpoint_UnknownCheckpoint(t *testing.T) {
	c, _ := setupExecConductor(t)
	// No phase metrics reference the SHA → error.
	if err := c.ResumeFromCheckpoint(context.Background(), "deadbeef"); err == nil {
		t.Error("ResumeFromCheckpoint with unknown checkpoint should error")
	}
}

func TestSelectFork_NoActiveTask(t *testing.T) {
	c := newTestConductor(t)
	if err := c.SelectFork(context.Background(), "fork-1"); err == nil {
		t.Error("SelectFork with no task should error")
	}
}

func TestSelectFork_NotFound(t *testing.T) {
	c, _, cleanup := setupForkTestRepo(t, true, 3)
	defer cleanup()
	if err := c.SelectFork(context.Background(), "nonexistent"); err == nil {
		t.Error("SelectFork with unknown fork ID should error")
	}
}

func TestSelectFork_MergesAndCleansUp(t *testing.T) {
	c, _, cleanup := setupForkTestRepo(t, true, 3)
	defer cleanup()

	ctx := context.Background()

	// Create two forks; add a commit to the winning one.
	winner, err := c.Fork(ctx, "winner")
	if err != nil {
		t.Fatalf("Fork winner: %v", err)
	}
	if _, err := c.Fork(ctx, "loser"); err != nil {
		t.Fatalf("Fork loser: %v", err)
	}

	// Commit a change in the winner worktree so the merge has content.
	newFile := filepath.Join(winner.WorktreeDir, "win.go")
	if err := os.WriteFile(newFile, []byte("package main\n\nfunc Win() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(ctx, t, "-C", winner.WorktreeDir, "add", ".")
	gitCmd(ctx, t, "-C", winner.WorktreeDir, "commit", "-m", "winning work")

	if err := c.SelectFork(ctx, winner.ID); err != nil {
		t.Fatalf("SelectFork error = %v", err)
	}

	// After selection, all forks are cleared from the work unit.
	if len(c.ListForks()) != 0 {
		t.Errorf("forks should be cleared after selection, got %d", len(c.ListForks()))
	}
}

func TestApplyFailurePolicy_SkipAdvances(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.SetAutoAdvance(false)
	c.machine.ForceState(StateSimplifying)
	// Simplify uses skip policy by default; on failure it advances past the phase.
	handled := c.applyFailurePolicy(context.Background(), EventSimplifyDone, "nothing to simplify")
	if !handled {
		t.Error("skip policy should handle the failure (true)")
	}
	// Simplifying + simplify_done -> implemented.
	if c.State() != StateImplemented {
		t.Errorf("state after skip = %s, want implemented", c.State())
	}
}

func TestUpdateTask_WithChange(t *testing.T) {
	c, dir := setupExecConductor(t)

	taskFile := filepath.Join(dir, "task.md")
	if err := os.WriteFile(taskFile, []byte("# Changed\n\nNew body content here."), 0o644); err != nil {
		t.Fatal(err)
	}
	c.workUnit.Source = &Source{Provider: "file", Reference: taskFile}
	c.workUnit.Description = "Old body content."

	changed, path, err := c.UpdateTask(context.Background())
	if err != nil {
		t.Fatalf("UpdateTask error = %v", err)
	}
	if !changed {
		t.Error("UpdateTask should report change when content differs")
	}
	if path == "" {
		t.Error("expected a delta spec path")
	}
	// The work unit description is updated and a delta spec is appended.
	wu := c.GetWorkUnit()
	if wu.Description != "New body content here." {
		t.Errorf("description not updated: %q", wu.Description)
	}
}

func TestEmitNonBlocking(t *testing.T) {
	// A conductor whose event channel is not drained must not deadlock on emit
	// because emit is best-effort (drops when full). Use a small buffer.
	c := &Conductor{
		machine:        NewMachine(),
		events:         make(chan ConductorEvent, 1),
		pendingPrompts: make(map[string]chan bool),
	}
	// Fill the buffer, then emit again — should not block.
	c.emit(ConductorEvent{Type: "first"})
	done := make(chan struct{})
	go func() {
		c.emit(ConductorEvent{Type: "second"})
		c.emit(ConductorEvent{Type: "third"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("emit blocked when channel full")
	}
}
