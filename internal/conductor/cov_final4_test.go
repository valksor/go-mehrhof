package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/internal/codegraph"
	"github.com/valksor/kvelmo/internal/graph"
	"github.com/valksor/kvelmo/internal/varpool"
	"github.com/valksor/kvelmo/internal/worker"
	"github.com/valksor/kvelmo/settings"
)

func TestPartialResults_RoundTrip(t *testing.T) {
	c, _ := setupExecConductor(t)

	// No cached results initially.
	if got := c.loadPartialResults(EventImplementDone); got != nil {
		t.Errorf("expected nil before any save, got %v", got)
	}

	// Manually seed a partial-results entry and confirm load round-trips.
	c.mu.Lock()
	c.varPool.SetScoped(varpool.ScopeSystem, partialResultsKey(PhaseImplement),
		`{"node-a":"output A","node-b":"output B"}`, "test")
	c.mu.Unlock()

	c.mu.Lock()
	got := c.loadPartialResults(EventImplementDone)
	c.mu.Unlock()
	if len(got) != 2 || got["node-a"] != "output A" {
		t.Errorf("loadPartialResults = %v, want 2 entries", got)
	}

	// Clear removes the entry.
	c.mu.Lock()
	c.clearPartialResults(EventImplementDone)
	cleared := c.loadPartialResults(EventImplementDone)
	c.mu.Unlock()
	if cleared != nil {
		t.Errorf("expected nil after clear, got %v", cleared)
	}
}

func TestSavePartialResults_FromScheduler(t *testing.T) {
	c, dir := setupExecConductor(t)

	// Build a real phase graph + scheduler and mark a node done with a result.
	g := buildPhaseGraph(worker.JobTypePlan, "planning", "prompt", dir)
	sched := graph.NewScheduler(g, c.pool)

	c.mu.Lock()
	c.savePartialResults(sched, EventPlanDone)
	c.mu.Unlock()

	// Without any completed nodes, savePartialResults stores nothing — loading
	// returns nil. This exercises the empty-completed branch.
	c.mu.Lock()
	got := c.loadPartialResults(EventPlanDone)
	c.mu.Unlock()
	if got != nil {
		t.Errorf("expected no partial results for an unrun scheduler, got %v", got)
	}
}

func TestPlan_GraphFailureSavesPartialAndRollsBack(t *testing.T) {
	// Agent errors → the plan graph fails → handleGraphCompletion error path
	// runs savePartialResults + applyFailurePolicy. Plan uses retry policy by
	// default, so force fail-fast for a deterministic rollback.
	c, _ := setupExecConductor(t, agent.Event{Type: agent.EventError, Error: "plan boom"})
	c.machine.ForceState(StateLoaded)
	c.phasePolicies[PhasePlan] = PhasePolicy{Policy: FailurePolicyFail}

	if _, err := c.Plan(context.Background()); err != nil {
		t.Fatalf("Plan submit error = %v", err)
	}

	// On failure the machine rolls back from planning to loaded.
	waitForStateExec(t, c, StateLoaded, 15*time.Second)
}

func TestQualityGateExternalReview_AlwaysMode(t *testing.T) {
	c, dir := setupExecConductor(t)
	s := c.GetEffectiveSettings()
	// Use "true" so LookPath resolves to /usr/bin/true and the command succeeds.
	s.Workflow.ExternalReview.Mode = settings.ExternalReviewAlways
	s.Workflow.ExternalReview.Command = "true"
	c.cachedSettings.Store(s)

	if err := c.qualityGateExternalReview(context.Background(), dir); err != nil {
		t.Errorf("external review with 'true' command should pass, got %v", err)
	}
}

func TestQualityGateExternalReview_PathRejected(t *testing.T) {
	c, dir := setupExecConductor(t)
	s := c.GetEffectiveSettings()
	s.Workflow.ExternalReview.Mode = settings.ExternalReviewAlways
	s.Workflow.ExternalReview.Command = "../bin/evil"
	c.cachedSettings.Store(s)

	if err := c.qualityGateExternalReview(context.Background(), dir); err == nil {
		t.Error("external review command with path separators should be rejected")
	}
}

func TestQualityGateExternalReview_ToolNotFound(t *testing.T) {
	c, dir := setupExecConductor(t)
	s := c.GetEffectiveSettings()
	s.Workflow.ExternalReview.Mode = settings.ExternalReviewAlways
	s.Workflow.ExternalReview.Command = "definitely-not-a-real-binary-xyz"
	c.cachedSettings.Store(s)

	// Tool not found → skipped (nil), not an error.
	if err := c.qualityGateExternalReview(context.Background(), dir); err != nil {
		t.Errorf("missing tool should be skipped, got %v", err)
	}
}

func TestGetAdversarialDiffAndSpec_WithGitAndSpec(t *testing.T) {
	c, dir := setupExecConductor(t)
	ctx := context.Background()

	// Set up an origin so getAdversarialDiff can diff against origin/<base>.
	remote := t.TempDir()
	gitCmd(ctx, t, "init", "--bare", remote)
	gitCmd(ctx, t, "-C", dir, "remote", "add", "origin", remote)
	base, err := c.git.DefaultBranch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(ctx, t, "-C", dir, "push", "-u", "origin", base)
	s := c.GetEffectiveSettings()
	s.Git.BaseBranch = base
	c.cachedSettings.Store(s)

	// Add a change so the diff is non-empty.
	if err := os.WriteFile(filepath.Join(dir, "adv.go"), []byte("package main\n\nfunc Adv() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(ctx, t, "-C", dir, "add", ".")
	gitCmd(ctx, t, "-C", dir, "commit", "-m", "adv change")

	diff := c.getAdversarialDiff(ctx)
	if diff == "" {
		t.Error("expected a non-empty adversarial diff")
	}

	// Store a spec so getAdversarialSpec returns content.
	if _, err := c.SaveSpecification("# Spec\n\nThe spec body."); err != nil {
		t.Fatal(err)
	}
	spec := c.getAdversarialSpec(c.GetWorkUnit(), c.store)
	if spec == "" {
		t.Error("expected non-empty adversarial spec content")
	}
}

func TestResolveSymbol_CodegraphHit(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Write a Go file with a symbol and index it into a codegraph.
	if err := os.WriteFile(filepath.Join(dir, "svc.go"), []byte("package svc\n\nfunc Handler() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := codegraph.New(ctx, filepath.Join(dir, "graph.db"))
	if err != nil {
		t.Fatalf("codegraph.New: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })
	if err := g.IndexDirectory(ctx, dir); err != nil {
		t.Fatalf("IndexDirectory: %v", err)
	}

	r := &ContextResolver{WorktreeRoot: dir, Graph: g}
	resolved, err := r.Resolve(ctx, ContextItem{Type: ContextTypeSymbol, Ref: "Handler"})
	if err != nil {
		t.Fatalf("resolve symbol error = %v", err)
	}
	if resolved.Content == "" {
		t.Error("expected codegraph symbol result content")
	}
}
