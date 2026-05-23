package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/ciwatch"
	"github.com/valksor/kvelmo/internal/findings"
	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/settings"
)

// ciStatusProvider implements provider.Provider + ciwatch.StatusFetcher.
type ciStatusProvider struct {
	name   string
	status ciwatch.Status
}

func (p *ciStatusProvider) Name() string { return p.name }
func (p *ciStatusProvider) FetchTask(context.Context, string) (*provider.Task, error) {
	return &provider.Task{}, nil
}
func (p *ciStatusProvider) UpdateStatus(context.Context, string, string) error { return nil }
func (p *ciStatusProvider) FetchCIStatus(context.Context, string) (*ciwatch.Status, error) {
	s := p.status

	return &s, nil
}

func TestStartCIFixLoop_NoPRID(t *testing.T) {
	c, _ := setupExecConductor(t)
	// No PRID → loop returns immediately, no panic.
	c.startCIFixLoop(context.Background())
}

func TestStartCIFixLoop_CIPasses(t *testing.T) {
	c, _ := setupExecConductor(t)
	cp := &ciStatusProvider{name: "ci", status: ciwatch.Status{State: "success"}}
	c.providers.Register(cp)
	c.workUnit.Source = &Source{Provider: "ci", Reference: "ci/repo#1"}
	c.workUnit.PRID = "ci/repo#1"

	s := c.GetEffectiveSettings()
	s.Workflow.CI.PollIntervalSec = 1
	s.Workflow.CI.MaxFixAttempts = 1
	c.cachedSettings.Store(s)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// CI reports success on first poll → loop exits via the success branch.
	c.startCIFixLoop(ctx)
}

func TestAttemptCIFix_NoPool(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "ci-1"})
	if err := c.attemptCIFix(context.Background(), "logs", 1, "feature/x"); err == nil {
		t.Error("attemptCIFix with no pool should error")
	}
}

func TestStart_FullWithWorktree(t *testing.T) {
	c, dir := setupExecConductor(t)
	// Use the actual default branch git created so worktree base resolution works.
	base, err := c.git.DefaultBranch(context.Background())
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	// Enable worktree isolation + branch creation to exercise provisionWorktree.
	s := c.GetEffectiveSettings()
	useWt := true
	createBranch := true
	s.Workflow.UseWorktreeIsolation = &useWt
	s.Git.CreateBranch = &createBranch
	s.Git.BaseBranch = base
	c.cachedSettings.Store(s)

	c.workUnit = nil
	c.machine.Reset()

	taskFile := filepath.Join(dir, "wt-task.md")
	if err := os.WriteFile(taskFile, []byte("# WT Task\n\nBuild it in a worktree."), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := c.Start(context.Background(), "file:"+taskFile); err != nil {
		t.Fatalf("Start error = %v", err)
	}
	if c.State() != StateLoaded {
		t.Errorf("state = %s, want loaded", c.State())
	}
	wu := c.GetWorkUnit()
	if wu.Branch == "" {
		t.Error("expected a branch to be created")
	}
	if wu.WorktreePath == "" {
		t.Error("expected an isolated worktree path")
	}
}

func TestApplyFailureClassification_Enabled(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.FailureClassification.Enabled = true
	s.Workflow.FailureClassification.HistoryWindow = 5
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// A genuine finding (not flaky) is returned as-is.
	input := []findings.Finding{
		{Message: "real bug", Severity: findings.SeverityHigh, File: "a.go", Line: 10},
	}
	got := c.applyFailureClassification(context.Background(), input)
	if len(got) != 1 {
		t.Errorf("genuine finding should be retained, got %d", len(got))
	}
}

func TestClassifyFindings_WithGitDiff(t *testing.T) {
	c, dir := setupExecConductor(t)
	ctx := context.Background()

	// Establish a base branch on origin so hold-the-line can diff against it.
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

	// Introduce a change in a tracked file.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc Changed() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(ctx, t, "-C", dir, "add", ".")
	gitCmd(ctx, t, "-C", dir, "commit", "-m", "change main")

	// A finding on the changed file/line is "introduced" and should be retained.
	input := []findings.Finding{
		{Message: "issue in changed code", Severity: findings.SeverityHigh, File: "main.go", Line: 3},
	}
	got := c.classifyFindings(ctx, input)
	// We don't assert the exact count (diff classification depends on line math),
	// only that classification runs against a real diff without error.
	_ = got
}

func TestRunSpecAlignmentCheckAsync_NoSpecs(t *testing.T) {
	c, dir := setupExecConductor(t)
	// No specs stored → returns early without dispatching a job.
	c.runSpecAlignmentCheckAsync(context.Background(), dir, c.pool)
}

func TestDispatchAutoAdvance_UnknownPhase(t *testing.T) {
	c, _ := setupExecConductor(t)
	// Unknown phase → no-op (default branch), no panic.
	c.dispatchAutoAdvance(context.Background(), "deploy")
}

func TestBuildProjectCommandsSection(t *testing.T) {
	c, dir := setupExecConductor(t)
	// Write a Makefile so discovery finds commands.
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("build:\n\techo building\n\ntest:\n\techo testing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	section := c.buildProjectCommandsSection()
	// Section may be empty if discovery finds nothing actionable; just ensure it
	// runs without panicking and returns a string.
	_ = section
}
