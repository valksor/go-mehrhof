package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/ciwatch"
	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/settings"
)

func TestWithDryRun(t *testing.T) {
	c, err := New(WithWorkDir(t.TempDir()), WithDryRun(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !c.DryRunEnabled() {
		t.Error("WithDryRun(true) should enable dry-run mode")
	}
}

func TestQualityGateSecurity_Enabled(t *testing.T) {
	c, dir := setupExecConductor(t)
	s := c.GetEffectiveSettings()
	s.Workflow.Policy.RequireSecurityScan = true
	c.cachedSettings.Store(s)

	// A clean temp repo should produce no blocking security findings → passes.
	if err := c.qualityGateSecurity(context.Background(), dir); err != nil {
		t.Errorf("clean repo security scan should pass, got %v", err)
	}
}

func TestRunQualityGate_AutoFixDisabled(t *testing.T) {
	// With no quality runner, no project type, and external review disabled,
	// runQualityGate (auto-fix disabled by default) passes.
	c, dir := setupExecConductor(t)
	c.worktree = dir
	if err := c.runQualityGate(context.Background()); err != nil {
		t.Errorf("clean quality gate should pass, got %v", err)
	}
}

func TestCopyPlanToRepo(t *testing.T) {
	c, dir := setupExecConductor(t)

	// Configure a plan output path and store a plan.
	s := c.GetEffectiveSettings()
	s.Storage.PlanOutputPath = "docs/plan.md"
	c.cachedSettings.Store(s)

	planStore := storage.NewPlanStore(c.store)
	plan, err := planStore.CreatePlan(c.workUnit.ID, "plan-1", "build the thing")
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	if err := planStore.AppendPlanHistory(c.workUnit.ID, plan.ID, "assistant", "step one"); err != nil {
		t.Fatalf("AppendPlanHistory: %v", err)
	}

	c.mu.Lock()
	c.copyPlanToRepo()
	c.mu.Unlock()

	if _, err := os.Stat(filepath.Join(dir, "docs/plan.md")); err != nil {
		t.Errorf("plan not copied to repo: %v", err)
	}
}

func TestRunSpecAlignmentCheckAsync_FullDispatch(t *testing.T) {
	c, dir := setupExecConductor(t)
	ctx := context.Background()

	// Establish origin so a diff is available.
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

	// Store a spec and create a diff.
	if _, err := c.SaveSpecification("# Spec\n\nRequirement: add the function."); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "align.go"), []byte("package main\n\nfunc Align() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(ctx, t, "-C", dir, "add", ".")
	gitCmd(ctx, t, "-C", dir, "commit", "-m", "align change")

	// Dispatches a spec-alignment job and starts watchSpecAlignmentJob.
	c.runSpecAlignmentCheckAsync(ctx, dir, c.pool)

	// Wait for the job to be recorded.
	deadline := time.Now().Add(5 * time.Second)
	var jobID string
	for time.Now().Before(deadline) {
		jobs := c.GetWorkUnit().Jobs
		if len(jobs) > 0 {
			jobID = jobs[len(jobs)-1]

			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if jobID == "" {
		t.Fatal("spec alignment job was not dispatched")
	}

	// Wait (race-free) for the job's stream to close so the watch goroutine
	// finishes writing under .valksor/work before the test's TempDir cleanup.
	if stream := c.pool.Stream(jobID); stream != nil {
		timeout := time.After(10 * time.Second)
	drain:
		for {
			select {
			case _, ok := <-stream:
				if !ok {
					break drain
				}
			case <-timeout:
				break drain
			}
		}
	}
	// Give the watch goroutine a moment to flush its review write.
	time.Sleep(300 * time.Millisecond)
}

func TestLoadVarPool(t *testing.T) {
	c, dir := setupExecConductor(t)

	// Persist a varpool to disk, then load it via the work unit's VarPoolPath.
	c.varPool.SetScoped("sys", "k", "v", "test")
	vpPath := filepath.Join(dir, "varpool.json")
	if err := c.varPool.Save(vpPath); err != nil {
		t.Fatalf("Save varpool: %v", err)
	}

	c.workUnit.VarPoolPath = vpPath
	c.mu.Lock()
	c.loadVarPool()
	c.mu.Unlock()

	if got := c.varPool.GetScopedString("sys", "k"); got != "v" {
		t.Errorf("varpool value not loaded: %q", got)
	}
}

func TestLoadVarPool_NoPath(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "vp-1"}) // no VarPoolPath
	c.mu.Lock()
	c.loadVarPool() // no-op, no panic
	c.mu.Unlock()
}

func TestRunPreGuardrails_Blocking(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.PhaseGuardrails = map[string]settings.GuardrailConfig{
		PhaseImplement: {Pre: []string{"require-spec"}},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// No specifications → require-spec produces a blocking finding.
	c.ForceWorkUnit(&WorkUnit{ID: "g-block"})
	if err := c.runPreGuardrails(context.Background(), PhaseImplement); err == nil {
		t.Error("require-spec guardrail should block when no specs exist")
	}
}

func TestRunPostGuardrails(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.PhaseGuardrails = map[string]settings.GuardrailConfig{
		PhaseImplement: {Post: []string{"require-spec"}},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.ForceWorkUnit(&WorkUnit{ID: "g-post"})

	c.mu.Lock()
	defer c.mu.Unlock()
	// Post-guardrails are informational; with no specs require-spec yields a finding.
	got := c.runPostGuardrails(context.Background(), PhaseImplement)
	if len(got) == 0 {
		t.Error("expected post-guardrail findings from require-spec with no specs")
	}
}

func TestRunPostGuardrails_NoConfig(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "g-noconf"})
	c.mu.Lock()
	defer c.mu.Unlock()
	if got := c.runPostGuardrails(context.Background(), PhasePlan); got != nil {
		t.Errorf("no guardrails should yield nil, got %v", got)
	}
}

// ciLogsProvider implements Provider + StatusFetcher + LogsFetcher.
type ciLogsProvider struct {
	ciStatusProvider

	logs string
}

func (p *ciLogsProvider) FetchCILogs(context.Context, string) (string, error) {
	return p.logs, nil
}

func TestExtractCILogs_WithLogsFetcher(t *testing.T) {
	c := newTestConductor(t)
	fetcher := &ciLogsProvider{logs: "compile error: undefined symbol"}
	status := &ciwatch.Status{State: "failure"}

	got := c.extractCILogs(context.Background(), fetcher, "pr#1", status)
	if got != "compile error: undefined symbol" {
		t.Errorf("expected fetched logs, got %q", got)
	}
}

func TestRejectNode_WithScheduler(t *testing.T) {
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

	err := c.RejectNode("missing-node")
	if err != nil && err.Error() == "no active graph execution" {
		t.Skip("scheduler completed before assertion (timing)")
	}

	waitForStateExec(t, c, StatePlanned, 15*time.Second)
}

var _ provider.Provider = (*ciLogsProvider)(nil)
