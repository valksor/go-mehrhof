package conductor

import (
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/varpool"
	"github.com/valksor/kvelmo/settings"
)

func TestPhaseToScope(t *testing.T) {
	tests := []struct {
		phase string
		want  string
	}{
		{PhasePlan, varpool.ScopePlan},
		{PhaseImplement, varpool.ScopeImplement},
		{PhaseSimplify, varpool.ScopeSimplify},
		{PhaseOptimize, varpool.ScopeOptimize},
		{PhaseReview, varpool.ScopeReview},
		{"custom", "custom"},
	}
	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			if got := phaseToScope(tt.phase); got != tt.want {
				t.Errorf("phaseToScope(%q) = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}

func TestPhaseFromEvent(t *testing.T) {
	tests := []struct {
		event Event
		want  string
	}{
		{EventPlanDone, string(EventPlan)},
		{EventImplementDone, string(EventImplement)},
		{EventSimplifyDone, string(EventSimplify)},
		{EventOptimizeDone, string(EventOptimize)},
		{EventReviewDone, ""}, // review has no in-progress phase mapping here
		{EventStart, ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			if got := phaseFromEvent(tt.event); got != tt.want {
				t.Errorf("phaseFromEvent(%s) = %q, want %q", tt.event, got, tt.want)
			}
		})
	}
}

func TestPhaseLabelFromEvent(t *testing.T) {
	tests := []struct {
		event Event
		want  string
	}{
		{EventPlanDone, PhasePlan},
		{EventImplementDone, PhaseImplement},
		{EventSimplifyDone, PhaseSimplify},
		{EventOptimizeDone, PhaseOptimize},
		{EventReviewDone, PhaseReview},
		{EventStart, "start"}, // falls through to raw string
	}
	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			if got := phaseLabelFromEvent(tt.event); got != tt.want {
				t.Errorf("phaseLabelFromEvent(%s) = %q, want %q", tt.event, got, tt.want)
			}
		})
	}
}

func TestExpectedInProgressState(t *testing.T) {
	tests := []struct {
		phase string
		want  State
	}{
		{PhasePlan, StatePlanning},
		{PhaseImplement, StateImplementing},
		{PhaseSimplify, StateSimplifying},
		{PhaseOptimize, StateOptimizing},
		{PhaseReview, ""}, // review has no in-progress mapping here
		{"bogus", ""},
	}
	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			if got := expectedInProgressState(tt.phase); got != tt.want {
				t.Errorf("expectedInProgressState(%q) = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}

func TestToInt64(t *testing.T) {
	tests := []struct {
		name   string
		in     any
		want   int64
		wantOk bool
	}{
		{"int64", int64(42), 42, true},
		{"float64", float64(3.9), 3, true},
		{"int", 7, 7, true},
		{"string", "nope", 0, false},
		{"nil", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toInt64(tt.in)
			if got != tt.want || ok != tt.wantOk {
				t.Errorf("toInt64(%v) = (%d,%v), want (%d,%v)", tt.in, got, ok, tt.want, tt.wantOk)
			}
		})
	}
}

func TestEstimateCost(t *testing.T) {
	// ollama is free
	if c := estimateCost("ollama", 1_000_000, 1_000_000); c != 0 {
		t.Errorf("ollama cost = %f, want 0", c)
	}
	// anthropic: 1M input @ $3, 1M output @ $15 → $18
	if c := estimateCost("anthropic", 1_000_000, 1_000_000); c != 18.0 {
		t.Errorf("anthropic cost = %f, want 18", c)
	}
	// openai: 1M input @ $2.5, 1M output @ $10 → $12.5
	if c := estimateCost("openai", 1_000_000, 1_000_000); c != 12.5 {
		t.Errorf("openai cost = %f, want 12.5", c)
	}
	// default falls back to anthropic-like pricing
	if c := estimateCost("mystery", 1_000_000, 0); c != 3.0 {
		t.Errorf("default input cost = %f, want 3", c)
	}
	if c := estimateCost("anthropic", 0, 0); c != 0 {
		t.Errorf("zero tokens cost = %f, want 0", c)
	}
}

func TestInterpolatePRTitle(t *testing.T) {
	t.Run("empty pattern falls back to kvelmo prefix", func(t *testing.T) {
		s := settings.DefaultSettings()
		s.Git.PRTitlePattern = ""
		c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		c.ForceWorkUnit(&WorkUnit{ID: "t1", Title: "Add feature"})
		got := c.interpolatePRTitle("Add feature")
		if got != "[kvelmo] Add feature" {
			t.Errorf("got %q, want '[kvelmo] Add feature'", got)
		}
	})

	t.Run("custom pattern with key", func(t *testing.T) {
		s := settings.DefaultSettings()
		s.Git.PRTitlePattern = "[{key}] {title} ({type})"
		c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		c.ForceWorkUnit(&WorkUnit{
			ID:         "t1",
			Title:      "Fix bug",
			ExternalID: "PROJ-99",
			Source:     &Source{Provider: "github"},
		})
		got := c.interpolatePRTitle("Fix bug")
		if got != "[PROJ-99] Fix bug (github)" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("custom pattern empty key strips bracket", func(t *testing.T) {
		s := settings.DefaultSettings()
		s.Git.PRTitlePattern = "[{key}] {title}"
		c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		c.ForceWorkUnit(&WorkUnit{ID: "t1", Title: "No key"})
		got := c.interpolatePRTitle("No key")
		if got != "No key" {
			t.Errorf("got %q, want 'No key'", got)
		}
	})
}

func TestBuildGitConventionInstructions(t *testing.T) {
	t.Run("no conventions returns empty", func(t *testing.T) {
		c := newTestConductor(t)
		c.ForceWorkUnit(&WorkUnit{ID: "t1"})
		if got := c.buildGitConventionInstructions(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("with prefix and pattern", func(t *testing.T) {
		s := settings.DefaultSettings()
		s.Git.CommitPrefix = "[ACME-{key}]"
		s.Git.CommitPattern = "^(feat|fix):"
		c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		c.ForceWorkUnit(&WorkUnit{ID: "t1", ExternalID: "123"})
		got := c.buildGitConventionInstructions()
		if got == "" {
			t.Fatal("expected non-empty instructions")
		}
		if !strings.Contains(got, "Git Conventions") || !strings.Contains(got, "ACME-123") || !strings.Contains(got, "^(feat|fix):") {
			t.Errorf("instructions missing expected content: %q", got)
		}
	})
}

func TestRecordCheckpointMeta(t *testing.T) {
	c := newTestConductor(t)

	// No work unit → no-op, no panic.
	c.recordCheckpointMeta("sha", "msg", "planned")

	c.ForceWorkUnit(&WorkUnit{ID: "t1"})
	c.recordCheckpointMeta("sha-1", "first checkpoint", "implemented")

	wu := c.WorkUnit()
	meta, ok := wu.CheckpointMeta["sha-1"]
	if !ok {
		t.Fatal("checkpoint meta not recorded")
	}
	if meta.Message != "first checkpoint" || meta.State != "implemented" {
		t.Errorf("meta = %+v", meta)
	}
	if meta.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestPrepareCommitValidation(t *testing.T) {
	t.Run("no last checkpoint returns nil", func(t *testing.T) {
		c := newTestConductor(t)
		c.ForceWorkUnit(&WorkUnit{ID: "t1"})
		if got := c.prepareCommitValidation(""); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("no commit pattern returns nil", func(t *testing.T) {
		c := newTestConductor(t)
		c.ForceWorkUnit(&WorkUnit{ID: "t1"})
		if got := c.prepareCommitValidation("abc123"); got != nil {
			t.Errorf("expected nil (no pattern configured), got %+v", got)
		}
	})

	t.Run("configured returns params", func(t *testing.T) {
		s := settings.DefaultSettings()
		s.Git.CommitPattern = "^(feat|fix):"
		c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		c.ForceWorkUnit(&WorkUnit{ID: "t1", Checkpoints: []string{"cp1", "cp2"}})
		params := c.prepareCommitValidation("base-sha")
		if params == nil {
			t.Fatal("expected non-nil params")
		}
		if params.pattern != "^(feat|fix):" {
			t.Errorf("pattern = %q", params.pattern)
		}
		if params.lastCheckpoint != "base-sha" {
			t.Errorf("lastCheckpoint = %q", params.lastCheckpoint)
		}
		if _, ok := params.checkpointSHAs["cp1"]; !ok {
			t.Error("checkpoint cp1 missing from SHA set")
		}
	})
}

func TestBuildJobOptionsForPhase(t *testing.T) {
	s := settings.DefaultSettings()
	s.Agent.PhaseAgent = map[string]string{"implement": "codex"}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.loadStrategiesFromSettings()
	c.ForceWorkUnit(&WorkUnit{ID: "t1", Title: "T"})

	opts := c.buildJobOptionsForPhase("implement")
	if opts == nil {
		t.Fatal("expected non-nil opts")
	}
	if opts.Agent != "codex" {
		t.Errorf("agent override not applied: got %q", opts.Agent)
	}

	// A phase without an override leaves Agent empty.
	plain := c.buildJobOptionsForPhase("plan")
	if plain.Agent != "" {
		t.Errorf("expected no agent override for plan, got %q", plain.Agent)
	}
}

func TestResetPhaseState(t *testing.T) {
	c := newTestConductor(t)
	failed := false
	c.ForceWorkUnit(&WorkUnit{ID: "t1", QualityGatePassed: &failed, QualityGateError: "boom"})

	c.mu.Lock()
	c.iterationCount["implement"] = 3
	c.retryCount["implement"] = 2
	c.varPool.SetScoped(varpool.ScopeImplement, "k", "v", "test")
	c.resetPhaseState("implement")
	c.mu.Unlock()

	if c.iterationCount["implement"] != 0 {
		t.Error("iteration count not reset")
	}
	if c.retryCount["implement"] != 0 {
		t.Error("retry count not reset")
	}
	wu := c.WorkUnit()
	if wu.QualityGatePassed != nil {
		t.Error("quality gate passed not cleared")
	}
	if wu.QualityGateError != "" {
		t.Error("quality gate error not cleared")
	}
}
