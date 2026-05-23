package conductor

import (
	"context"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/eventlog"
	"github.com/valksor/kvelmo/internal/findings"
	"github.com/valksor/kvelmo/internal/storage"
)

func TestStateSetters_DoNotPanic(t *testing.T) {
	c := newTestConductor(t)

	c.SetTaskGroupChecker(nil)
	c.SetQualityRunner(nil)
	c.SetMetricsRecorder(nil)
	c.SetNotifier(nil)
	c.SetStore(nil)
	c.SetMemoryIndexer(nil)
	c.SetStrategy(nil)
	c.SetPhaseStrategy("plan", nil) // clearing a non-existent override is a no-op

	if c.EventLog() != nil {
		t.Error("EventLog should be nil before SetEventLog")
	}
}

func TestSetEventLog_AndEmit(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	log, err := eventlog.New(dir)
	if err != nil {
		t.Fatalf("open eventlog: %v", err)
	}
	t.Cleanup(func() { _ = log.Close() })

	c.SetEventLog(log)
	if c.EventLog() != log {
		t.Error("EventLog() should return the configured log")
	}

	// emitEventLog with nil log is a no-op (set a fresh conductor).
	c2 := newTestConductor(t)
	c2.emitEventLog(eventlog.Entry{Type: eventlog.EventPhaseStarted, Phase: "plan"})

	// emitEventLog with a real log appends without error.
	c.emitEventLog(eventlog.Entry{Type: eventlog.EventPhaseStarted, Phase: "plan"})
}

func TestSearchTaskHistory(t *testing.T) {
	// Nil store → nil result, no error.
	c := newTestConductor(t)
	got, err := c.SearchTaskHistory(storage.SearchOptions{})
	if err != nil || got != nil {
		t.Errorf("SearchTaskHistory with nil store = (%v, %v), want (nil, nil)", got, err)
	}

	// With store → returns archived tasks.
	dir := t.TempDir()
	c2 := newConductorWithStore(t, dir)
	c2.ForceWorkUnit(&WorkUnit{
		ID:        "search-1",
		Title:     "Searchable",
		Source:    &Source{Provider: "github", Reference: "github:owner/repo#7"},
		CreatedAt: time.Now(),
	})
	c2.archiveTask("finished")

	results, err := c2.SearchTaskHistory(storage.SearchOptions{})
	if err != nil {
		t.Fatalf("SearchTaskHistory error = %v", err)
	}
	found := false
	for _, r := range results {
		if r.ID == "search-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("archived task not found in search results: %+v", results)
	}
}

func TestSuggestions_NilStore(t *testing.T) {
	c := newTestConductor(t)
	skips, agents := c.Suggestions()
	if skips != nil || agents != nil {
		t.Errorf("Suggestions with nil store = (%v, %v), want (nil, nil)", skips, agents)
	}
}

func TestSuggestions_WithStore(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	// Empty archive → empty suggestions, no error.
	skips, agents := c.Suggestions()
	if len(skips) != 0 || len(agents) != 0 {
		t.Errorf("expected empty suggestions, got skips=%v agents=%v", skips, agents)
	}
}

func TestLastFailureClass(t *testing.T) {
	c := newTestConductor(t)
	if c.LastFailureClass() != "" {
		t.Errorf("LastFailureClass should start empty, got %q", c.LastFailureClass())
	}

	c.mu.Lock()
	c.lastFailureClass = FailureClassHardStop
	c.mu.Unlock()

	if c.LastFailureClass() != FailureClassHardStop {
		t.Errorf("LastFailureClass = %q, want hard_stop", c.LastFailureClass())
	}
}

func TestNeedsRecovery_RecoveryState(t *testing.T) {
	c := newTestConductor(t)

	// No work unit → no recovery.
	if c.NeedsRecovery() {
		t.Error("NeedsRecovery should be false with no work unit")
	}
	if c.RecoveryState() != "" {
		t.Error("RecoveryState should be empty with no work unit")
	}

	// Stable state → no recovery.
	c.ForceWorkUnit(&WorkUnit{ID: "rec-1"})
	c.machine.ForceState(StatePlanned)
	if c.NeedsRecovery() {
		t.Error("NeedsRecovery should be false in planned (stable) state")
	}
	if c.RecoveryState() != "" {
		t.Errorf("RecoveryState should be empty in stable state, got %q", c.RecoveryState())
	}

	// Active phase state → recovery needed.
	c.machine.ForceState(StateImplementing)
	if !c.NeedsRecovery() {
		t.Error("NeedsRecovery should be true in implementing state")
	}
	if c.RecoveryState() != string(StateImplementing) {
		t.Errorf("RecoveryState = %q, want implementing", c.RecoveryState())
	}
}

func TestSignalProgress_GetProgressEstimate(t *testing.T) {
	c := newTestConductor(t)

	// No estimator → nil estimate, SignalProgress is a safe no-op.
	c.SignalProgress()
	if c.GetProgressEstimate() != nil {
		t.Error("GetProgressEstimate should be nil with no active phase")
	}

	// Init an estimator and confirm Get returns a value.
	c.ForceWorkUnit(&WorkUnit{ID: "prog-1"})
	c.mu.Lock()
	c.initProgressEstimator(PhaseImplement)
	c.mu.Unlock()

	c.SignalProgress()
	est := c.GetProgressEstimate()
	if est == nil {
		t.Fatal("GetProgressEstimate should be non-nil after initProgressEstimator")
	}
}

func TestResolveNextPhase(t *testing.T) {
	c := newTestConductor(t)

	tests := []struct {
		name      string
		completed Event
		skip      []string
		want      string
	}{
		{"plan done -> implement", EventPlanDone, nil, PhaseImplement},
		{"implement done -> simplify", EventImplementDone, nil, PhaseSimplify},
		{"implement done skip simplify -> optimize", EventImplementDone, []string{PhaseSimplify}, PhaseOptimize},
		{"implement done skip simplify+optimize -> review", EventImplementDone, []string{PhaseSimplify, PhaseOptimize}, PhaseReview},
		{"implement done skip all -> none", EventImplementDone, []string{PhaseSimplify, PhaseOptimize, PhaseReview}, ""},
		{"simplify done -> optimize", EventSimplifyDone, nil, PhaseOptimize},
		{"optimize done -> review", EventOptimizeDone, nil, PhaseReview},
		{"review done -> none", EventReviewDone, nil, ""},
		{"unrelated event -> none", EventStart, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.resolveNextPhase(tt.completed, tt.skip); got != tt.want {
				t.Errorf("resolveNextPhase(%s, %v) = %q, want %q", tt.completed, tt.skip, got, tt.want)
			}
		})
	}
}

func TestMessagesToFindings(t *testing.T) {
	got := messagesToFindings([]string{"lint failed", "vet failed"})
	if len(got) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(got))
	}
	if got[0].Message != "lint failed" {
		t.Errorf("message = %q", got[0].Message)
	}
	if got[0].Severity != findings.SeverityHigh {
		t.Errorf("severity = %v, want high", got[0].Severity)
	}
	if got[0].Source != "quality_runner" {
		t.Errorf("source = %q", got[0].Source)
	}

	if len(messagesToFindings(nil)) != 0 {
		t.Error("nil messages should yield empty findings")
	}
}

func TestApplyFailureClassification_Disabled(t *testing.T) {
	c := newTestConductor(t)
	input := []findings.Finding{{Message: "x", Severity: findings.SeverityHigh}}
	// Default settings have classification disabled → returns input unchanged.
	got := c.applyFailureClassification(context.Background(), input)
	if len(got) != 1 {
		t.Errorf("classification disabled should return input unchanged, got %d", len(got))
	}
}

func TestEvaluateRisk_NoTask(t *testing.T) {
	c := newTestConductor(t)
	score := c.EvaluateRisk(context.Background())
	if score.Level != "low" {
		t.Errorf("EvaluateRisk with no task = %q, want low", score.Level)
	}
}

func TestEvaluateRisk_WithTaskNoGit(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "risk-1"})
	// No git repo → risk evaluated from empty input (low).
	score := c.EvaluateRisk(context.Background())
	if score.Level == "" {
		t.Error("EvaluateRisk should return a level")
	}
}

func TestClassifyFindings_NoGit(t *testing.T) {
	c := newTestConductor(t)
	// git is nil → classification skipped, input returned as-is.
	input := []findings.Finding{{Message: "issue", Severity: findings.SeverityHigh}}
	got := c.classifyFindings(context.Background(), input)
	if len(got) != 1 {
		t.Errorf("classifyFindings with no git should return input unchanged, got %d", len(got))
	}
}
