package conductor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/settings"
)

// ─── adversarial review ──────────────────────────────────────────────────────

func TestRunAdversarialReview_Disabled(t *testing.T) {
	c := newTestConductor(t)
	findings, err := c.runAdversarialReview(context.Background())
	if err != nil || findings != nil {
		t.Errorf("disabled adversarial review = (%v, %v), want (nil, nil)", findings, err)
	}
}

func TestRunAdversarialReview_UnknownPersona(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.AdversarialReview = &settings.AdversarialReviewSettings{
		Enabled:  true,
		Personas: []string{"nonexistent-persona"},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = c.runAdversarialReview(context.Background())
	if err == nil {
		t.Error("expected error when all personas are unknown")
	}
}

func TestGetAdversarialFindings_Empty(t *testing.T) {
	c := newTestConductor(t)
	if got := c.GetAdversarialFindings(); got != nil {
		t.Errorf("expected nil findings, got %v", got)
	}
}

func TestGetAdversarialDiff_NoGit(t *testing.T) {
	c := newTestConductor(t)
	if got := c.getAdversarialDiff(context.Background()); got != "" {
		t.Errorf("no git should yield empty diff, got %q", got)
	}
}

func TestGetAdversarialSpec_NilInputs(t *testing.T) {
	c := newTestConductor(t)
	if got := c.getAdversarialSpec(nil, nil); got != "" {
		t.Errorf("nil inputs should yield empty spec, got %q", got)
	}
}

func TestBuildAdversarialPrompt(t *testing.T) {
	persona := DefaultPersonas()["security"]
	prompt := buildAdversarialPrompt(persona, "diff content", "spec content")
	if !strings.Contains(prompt, "security auditor") {
		t.Error("prompt missing persona instructions")
	}
	if !strings.Contains(prompt, "## Specification") || !strings.Contains(prompt, "spec content") {
		t.Error("prompt missing specification section")
	}
	if !strings.Contains(prompt, "## Changes (diff)") || !strings.Contains(prompt, "diff content") {
		t.Error("prompt missing diff section")
	}
	if !strings.Contains(prompt, "## Output Format") {
		t.Error("prompt missing output format")
	}
}

func TestBuildAdversarialPrompt_NoSpecNoDiff(t *testing.T) {
	persona := DefaultPersonas()["performance"]
	prompt := buildAdversarialPrompt(persona, "", "")
	if strings.Contains(prompt, "## Specification") {
		t.Error("should not include spec section when spec is empty")
	}
	if strings.Contains(prompt, "## Changes (diff)") {
		t.Error("should not include diff section when diff is empty")
	}
}

func TestParseAdversarialFindings_Attribution(t *testing.T) {
	output := "auth.go:42: [high] missing input validation\nmain.go:10: [low] minor style issue"
	got := parseAdversarialFindings("security", output)
	if len(got) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(got))
	}
	for _, f := range got {
		if f.ReviewerPersona != "security" {
			t.Errorf("finding not attributed to persona: %+v", f)
		}
	}
	if got[0].File != "auth.go" || got[0].Line != 42 {
		t.Errorf("file/line not parsed: %+v", got[0])
	}
}

func TestRunAdversarialReview_FullRun(t *testing.T) {
	c, _ := setupExecConductor(
		t,
		agent.Event{Type: agent.EventStream, Content: "auth.go:5: [high] sql injection risk"},
		agent.Event{Type: agent.EventComplete},
	)
	s := c.GetEffectiveSettings()
	s.Workflow.AdversarialReview = &settings.AdversarialReviewSettings{
		Enabled:  true,
		Personas: []string{"security"},
		Agent:    "mock",
	}
	c.cachedSettings.Store(s)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := c.RunAdversarialReview(ctx)
	if err != nil {
		t.Fatalf("RunAdversarialReview error = %v", err)
	}
	// The persona job ran and produced at least one finding.
	if len(results) == 0 {
		t.Error("expected adversarial findings from the security persona")
	}
	// Stored findings are retrievable.
	if len(c.GetAdversarialFindings()) != len(results) {
		t.Error("GetAdversarialFindings should return the stored result")
	}
}

// ─── consensus review ────────────────────────────────────────────────────────

func TestRunConsensusReview_Disabled(t *testing.T) {
	c := newTestConductor(t)
	_, err := c.runConsensusReview(context.Background(), "review this")
	if err == nil {
		t.Error("disabled consensus review should error")
	}
}

func TestParseConsensusFindings(t *testing.T) {
	output := "file.go:42: missing error check\n\nplain line without location\n  another.go:13:7: unused import"
	got := parseConsensusFindings("agent-a", output)
	if len(got) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(got))
	}
	if got[0].File != "file.go" || got[0].Line != 42 {
		t.Errorf("first finding file/line = %s:%d", got[0].File, got[0].Line)
	}
	if got[0].Source != "agent-a" {
		t.Errorf("source = %q", got[0].Source)
	}
	// Plain line has no file/line.
	if got[1].File != "" {
		t.Errorf("plain line should have no file, got %q", got[1].File)
	}
}

func TestRunConsensusReview_FullRun(t *testing.T) {
	c, _ := setupExecConductor(
		t,
		agent.Event{Type: agent.EventStream, Content: "core.go:1: missing nil check"},
		agent.Event{Type: agent.EventComplete},
	)
	s := c.GetEffectiveSettings()
	s.Agent.Consensus = &settings.ConsensusConfig{
		Enabled:      true,
		Agents:       []string{"mock"},
		MinAgreement: 1,
	}
	c.cachedSettings.Store(s)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := c.runConsensusReview(ctx, "review the code")
	if err != nil {
		t.Fatalf("runConsensusReview error = %v", err)
	}
	if len(results) == 0 {
		t.Error("expected consensus findings")
	}
}

// ─── router ──────────────────────────────────────────────────────────────────

func TestDefaultRouter_Route(t *testing.T) {
	r := NewDefaultRouter()
	d := r.Route(context.Background(), PhaseImplement, "output", 0)
	if d.Action != RouteAdvance {
		t.Errorf("default router action = %s, want advance", d.Action)
	}
	if d.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2", d.MaxRetries)
	}
}

func TestApplyRouteDecision_Advance(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "route-1"})
	handled := c.applyRouteDecision(context.Background(), RouteDecision{Action: RouteAdvance}, EventImplementDone)
	if handled {
		t.Error("RouteAdvance should not be handled (caller advances)")
	}
	// The decision is recorded in route history.
	if len(c.GetWorkUnit().RouteHistory) != 1 {
		t.Error("route decision not recorded")
	}
}

func TestApplyRouteDecision_RetryAtMax(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "route-2"})
	// Attempt >= MaxRetries → falls through to advance (returns false).
	handled := c.applyRouteDecision(context.Background(), RouteDecision{
		Action: RouteRetry, Attempt: 2, MaxRetries: 2,
	}, EventImplementDone)
	if handled {
		t.Error("retry at max should fall through to advance (false)")
	}
}

func TestApplyRouteDecision_RollbackNoTarget(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "route-3"})
	// Rollback without a target phase → falls through to advance (returns false).
	handled := c.applyRouteDecision(context.Background(), RouteDecision{
		Action: RouteRollback, TargetPhase: "",
	}, EventImplementDone)
	if handled {
		t.Error("rollback without target should fall through (false)")
	}
}

// ─── approve / review items ──────────────────────────────────────────────────

func TestApprove_RecordsApproval(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	c.ForceWorkUnit(&WorkUnit{ID: "appr-1"})
	if err := c.Approve("submit"); err != nil {
		t.Fatalf("Approve error = %v", err)
	}
	wu := c.GetWorkUnit()
	rec, ok := wu.Approvals["submit"]
	if !ok {
		t.Fatal("approval not recorded")
	}
	if rec.ApprovedAt.IsZero() {
		t.Error("ApprovedAt should be set")
	}
}

func TestCheckUncheckReviewItem(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	c.ForceWorkUnit(&WorkUnit{ID: "chk-1"})

	if err := c.CheckReviewItem("tests pass"); err != nil {
		t.Fatalf("CheckReviewItem error = %v", err)
	}
	// Checking twice is idempotent.
	if err := c.CheckReviewItem("tests pass"); err != nil {
		t.Fatalf("second CheckReviewItem error = %v", err)
	}
	wu := c.GetWorkUnit()
	if len(wu.ChecklistChecked) != 1 {
		t.Errorf("ChecklistChecked = %v, want 1 item", wu.ChecklistChecked)
	}

	if err := c.UncheckReviewItem("tests pass"); err != nil {
		t.Fatalf("UncheckReviewItem error = %v", err)
	}
	if len(c.GetWorkUnit().ChecklistChecked) != 0 {
		t.Error("item should be removed after uncheck")
	}
}

func TestCheckReviewItem_NoTask(t *testing.T) {
	c := newTestConductor(t)
	if err := c.CheckReviewItem("x"); err == nil {
		t.Error("CheckReviewItem with no task should error")
	}
	if err := c.UncheckReviewItem("x"); err == nil {
		t.Error("UncheckReviewItem with no task should error")
	}
}

func TestApproveRejectNode_NoScheduler(t *testing.T) {
	c := newTestConductor(t)
	if err := c.ApproveNode("node-1"); err == nil {
		t.Error("ApproveNode with no active scheduler should error")
	}
	if err := c.RejectNode("node-1"); err == nil {
		t.Error("RejectNode with no active scheduler should error")
	}
}

func TestClearAutoFixState(t *testing.T) {
	c := newTestConductor(t)
	c.mu.Lock()
	c.autoFixAttempt = 3
	c.autoFixLastErr = "boom"
	c.mu.Unlock()

	c.clearAutoFixState()

	status := c.GetAutoFixStatus()
	if status.Active {
		t.Error("auto-fix should be inactive after clear")
	}
	if status.Attempt != 0 {
		t.Errorf("attempt = %d, want 0", status.Attempt)
	}
	if status.LastError != "" {
		t.Errorf("last error = %q, want empty", status.LastError)
	}
}

func TestEmitRiskEvaluated(t *testing.T) {
	c, _ := setupExecConductor(t)
	// Should emit without panicking; the drain goroutine consumes the event.
	score := c.EvaluateRisk(context.Background())
	c.emitRiskEvaluated(score)
}
