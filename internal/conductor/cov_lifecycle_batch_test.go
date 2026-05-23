package conductor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/ciwatch"
	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/settings"
)

// fakeMergeProvider extends fakeSubmitProvider with approve/merge support.
type fakeMergeProvider struct {
	fakeSubmitProvider

	approveCalls int
	mergeCalls   int
	prStatus     *provider.PRStatus
}

func (f *fakeMergeProvider) ApprovePR(_ context.Context, _ string, _ string) error {
	f.approveCalls++

	return nil
}

func (f *fakeMergeProvider) MergePR(_ context.Context, _ string, _ string) error {
	f.mergeCalls++

	return nil
}

func (f *fakeMergeProvider) GetPRStatus(_ context.Context, _ string) (*provider.PRStatus, error) {
	if f.prStatus != nil {
		return f.prStatus, nil
	}

	return &provider.PRStatus{State: "open", Merged: false}, nil
}

func TestStart_FromFileProvider(t *testing.T) {
	c, dir := setupExecConductor(t)
	// Disable worktree isolation and branch creation to keep the test simple.
	s := c.GetEffectiveSettings()
	noWorktree := false
	noBranch := false
	s.Workflow.UseWorktreeIsolation = &noWorktree
	s.Git.CreateBranch = &noBranch
	c.cachedSettings.Store(s)

	// Reset to None and clear the pre-seeded work unit so Start can run.
	c.workUnit = nil
	c.machine.Reset()

	// Write a task file.
	taskFile := filepath.Join(dir, "task.md")
	if err := os.WriteFile(taskFile, []byte("# My Task\n\nImplement the widget."), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := c.Start(context.Background(), "file:"+taskFile); err != nil {
		t.Fatalf("Start error = %v", err)
	}
	if c.State() != StateLoaded {
		t.Errorf("state after start = %s, want loaded", c.State())
	}
	wu := c.GetWorkUnit()
	if wu == nil || wu.Title != "My Task" {
		t.Errorf("work unit not loaded correctly: %+v", wu)
	}
	if wu.Source == nil || wu.Source.Provider != "file" {
		t.Errorf("source provider = %v, want file", wu.Source)
	}
}

func TestStart_WrongState(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.machine.ForceState(StatePlanned)
	if err := c.Start(context.Background(), "file:/tmp/x.md"); err == nil {
		t.Error("Start from non-none state should error")
	}
}

func TestStart_ParseError(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.workUnit = nil
	c.machine.Reset()
	if err := c.Start(context.Background(), "this-is-not-a-valid-source-ref-format"); err == nil {
		t.Error("Start with unparseable source should error")
	}
}

func TestUpdateTask_NoTask(t *testing.T) {
	c := newTestConductor(t)
	if _, _, err := c.UpdateTask(context.Background()); err == nil {
		t.Error("UpdateTask with no task should error")
	}
}

func TestUpdateTask_NoChange(t *testing.T) {
	c, dir := setupExecConductor(t)

	// Write a task file and load it.
	taskFile := filepath.Join(dir, "task.md")
	if err := os.WriteFile(taskFile, []byte("# Same\n\nUnchanged body."), 0o644); err != nil {
		t.Fatal(err)
	}
	// UpdateTask passes Reference directly to the provider's FetchTask, so the
	// reference must be the bare file path (no "file:" scheme prefix).
	c.workUnit.Source = &Source{Provider: "file", Reference: taskFile}
	c.workUnit.Description = "Unchanged body."

	changed, path, err := c.UpdateTask(context.Background())
	if err != nil {
		t.Fatalf("UpdateTask error = %v", err)
	}
	if changed {
		t.Error("UpdateTask should report no change when content matches")
	}
	if path != "" {
		t.Errorf("no delta path expected, got %q", path)
	}
}

func TestRefresh_NoTask(t *testing.T) {
	c := newTestConductor(t)
	if _, err := c.Refresh(context.Background()); err == nil {
		t.Error("Refresh with no task should error")
	}
}

func TestRefresh_OpenPR(t *testing.T) {
	// Use the github provider name so the registry's GetPRStatus can resolve it.
	// The unauthenticated github provider returns an error, which Refresh logs
	// and continues — exercising the open-PR branch (no merge).
	c, _ := setupExecConductor(t)
	c.workUnit.Source = &Source{Provider: "github", Reference: "github:owner/repo#1"}

	result, err := c.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh error = %v", err)
	}
	if result.TaskID != c.GetWorkUnit().ID {
		t.Errorf("TaskID = %q, want %q", result.TaskID, c.GetWorkUnit().ID)
	}
	// PR status lookup failed (no token), so the branch is not merged.
	if result.PRMerged {
		t.Error("PR should not be reported merged when status lookup fails")
	}
}

func TestApprovePR_FlowAndGuards(t *testing.T) {
	c, _ := setupExecConductor(t)
	fp := &fakeMergeProvider{fakeSubmitProvider: fakeSubmitProvider{name: "fake"}}
	c.providers.Register(fp)
	c.workUnit.Source = &Source{Provider: "fake", Reference: "fake/repo#1"}

	// Not submitted yet → error.
	c.machine.ForceState(StateImplemented)
	if err := c.ApprovePR(context.Background(), "lgtm"); err == nil {
		t.Error("ApprovePR from non-submitted state should error")
	}

	// Submitted but no PRID → error.
	c.machine.ForceState(StateSubmitted)
	if err := c.ApprovePR(context.Background(), "lgtm"); err == nil {
		t.Error("ApprovePR without PR ID should error")
	}

	// With PRID → succeeds.
	c.workUnit.PRID = "fake/repo#1"
	if err := c.ApprovePR(context.Background(), "lgtm"); err != nil {
		t.Fatalf("ApprovePR error = %v", err)
	}
	if fp.approveCalls != 1 {
		t.Errorf("ApprovePR called %d times, want 1", fp.approveCalls)
	}
}

func TestMergePR_Flow(t *testing.T) {
	c, _ := setupExecConductor(t)
	fp := &fakeMergeProvider{fakeSubmitProvider: fakeSubmitProvider{name: "fake"}}
	c.providers.Register(fp)
	c.workUnit.Source = &Source{Provider: "fake", Reference: "fake/repo#1"}
	c.workUnit.PRID = "fake/repo#1"
	c.machine.ForceState(StateSubmitted)

	if err := c.MergePR(context.Background(), "squash"); err != nil {
		t.Fatalf("MergePR error = %v", err)
	}
	if fp.mergeCalls != 1 {
		t.Errorf("MergePR called %d times, want 1", fp.mergeCalls)
	}
}

func TestMergePR_NotSubmitted(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "m1", Source: &Source{Provider: "github"}})
	c.machine.ForceState(StateImplemented)
	if err := c.MergePR(context.Background(), "rebase"); err == nil {
		t.Error("MergePR from non-submitted state should error")
	}
}

func TestRunPreGuardrails_NoConfig(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "g1"})
	// No guardrails configured for plan → nil.
	if err := c.runPreGuardrails(context.Background(), PhasePlan); err != nil {
		t.Errorf("no guardrails should pass, got %v", err)
	}
}

func TestRunPreGuardrails_UnknownGuardrail(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.PhaseGuardrails = map[string]settings.GuardrailConfig{
		PhasePlan: {Pre: []string{"nonexistent-guardrail"}},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.ForceWorkUnit(&WorkUnit{ID: "g2"})
	// Unknown guardrail is skipped → no blocking error.
	if err := c.runPreGuardrails(context.Background(), PhasePlan); err != nil {
		t.Errorf("unknown guardrail should be skipped, got %v", err)
	}
}

func TestRunPostTransitionHooks_RequiredFailure(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Hooks = settings.HooksSettings{
		"post_implement": {{Command: "exit 1", Required: true}},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.RunPostTransitionHooks(context.Background(), EventImplement); err == nil {
		t.Error("required post-hook failure should error")
	}
}

func TestRunPostTransitionHooks_OptionalFailure(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Hooks = settings.HooksSettings{
		"post_submit": {{Command: "exit 1", Required: false}},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Optional failure is logged, not returned.
	if err := c.RunPostTransitionHooks(context.Background(), EventSubmit); err != nil {
		t.Errorf("optional post-hook failure should not error, got %v", err)
	}
}

func TestRunPostTransitionHooks_Success(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Hooks = settings.HooksSettings{
		"post_plan": {{Command: "true", Required: true, Description: "noop"}},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.RunPostTransitionHooks(context.Background(), EventPlan); err != nil {
		t.Errorf("successful post-hook should not error, got %v", err)
	}
}

func TestRunPostTransitionHooks_NoHooks(t *testing.T) {
	c := newTestConductor(t)
	if err := c.RunPostTransitionHooks(context.Background(), EventPlan); err != nil {
		t.Errorf("no hooks configured should pass, got %v", err)
	}
}

func TestHookDescription(t *testing.T) {
	if got := hookDescription(settings.TransitionHook{Command: "make test", Description: "Run tests"}); got != "Run tests" {
		t.Errorf("hookDescription = %q, want 'Run tests'", got)
	}
	if got := hookDescription(settings.TransitionHook{Command: "make build"}); got != "make build" {
		t.Errorf("hookDescription fallback = %q, want 'make build'", got)
	}
}

func TestExtractCILogs_FallbackSummary(t *testing.T) {
	c := newTestConductor(t)
	status := &ciwatch.Status{
		State: "failure",
		Checks: []ciwatch.Check{
			{Name: "build", Status: "failure", URL: "https://ci/build"},
			{Name: "test", Status: "success"},
		},
	}
	// No LogsFetcher → falls back to a summary of failed checks.
	got := c.extractCILogs(context.Background(), nil, "pr#1", status)
	if !strings.Contains(got, "build") || !strings.Contains(got, "FAILED") {
		t.Errorf("expected failed-check summary, got %q", got)
	}
}

func TestBuildCIFixPrompt(t *testing.T) {
	prompt := buildCIFixPrompt("Fix login", "Login is broken", "test failed: TestLogin", 2)
	for _, want := range []string{"Fix login", "Login is broken", "attempt 2", "test failed: TestLogin", "CI Failure"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
