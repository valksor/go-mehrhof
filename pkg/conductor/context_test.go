package conductor

import (
	"context"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/pkg/varpool"
)

func TestBuildPhaseContext_PlanProfile(t *testing.T) {
	wu := &WorkUnit{
		Title:       "Add authentication",
		Description: "Implement OAuth2 login flow with Google provider",
	}
	pool := varpool.New()

	profile := DefaultContextProfiles()["plan"]
	result, metrics := BuildPhaseContext(context.Background(), profile, wu, pool)

	if result == "" {
		t.Fatal("expected non-empty context for plan phase")
	}
	if len(metrics.SectionsIncluded) == 0 {
		t.Fatal("expected at least one section included")
	}
	if metrics.TokenBudget != 8000 {
		t.Errorf("expected budget 8000, got %d", metrics.TokenBudget)
	}
}

func TestBuildPhaseContext_ImplementProfile(t *testing.T) {
	wu := &WorkUnit{
		Title:          "Add authentication",
		Description:    "A very long description that should be truncated in implement phase...",
		Specifications: []string{"spec_1.md"},
	}
	pool := varpool.New()

	profile := DefaultContextProfiles()["implement"]
	result, metrics := BuildPhaseContext(context.Background(), profile, wu, pool)

	if result == "" {
		t.Fatal("expected non-empty context")
	}
	// Should include task summary and specs
	found := false
	for _, s := range metrics.SectionsIncluded {
		if s == "Task Summary" {
			found = true
		}
	}
	if !found {
		t.Error("expected Task Summary in included sections")
	}
}

func TestBuildPhaseContext_NilWorkUnit(t *testing.T) {
	pool := varpool.New()
	profile := DefaultContextProfiles()["plan"]
	result, _ := BuildPhaseContext(context.Background(), profile, nil, pool)
	if result != "" {
		t.Error("expected empty context for nil work unit")
	}
}

func TestBuildPhaseContext_SimplifyWithDiff(t *testing.T) {
	wu := &WorkUnit{Title: "test"}
	pool := varpool.New()
	pool.SetScoped("sys", "last_diff", "diff --git a/file.go...", "test")
	pool.SetScoped("sys", "last_findings", "lint: unused variable", "test")

	profile := DefaultContextProfiles()["simplify"]
	result, metrics := BuildPhaseContext(context.Background(), profile, wu, pool)

	if result == "" {
		t.Fatal("expected context with diff and findings")
	}
	if len(metrics.SectionsIncluded) != 2 {
		t.Errorf("expected 2 sections (diff + findings), got %d", len(metrics.SectionsIncluded))
	}
}

func TestDefaultContextProfiles(t *testing.T) {
	profiles := DefaultContextProfiles()
	phases := []string{"plan", "implement", "simplify", "optimize", "review"}
	for _, phase := range phases {
		if _, ok := profiles[phase]; !ok {
			t.Errorf("missing profile for phase %q", phase)
		}
	}
}

func TestBuildPhaseContext_NilPool(t *testing.T) {
	wu := &WorkUnit{
		Title:       "Test task",
		Description: "Description",
	}
	profile := DefaultContextProfiles()["simplify"]
	// Simplify only wants diff+findings which come from pool; with nil pool should be empty
	result, _ := BuildPhaseContext(context.Background(), profile, wu, nil)
	if result != "" {
		t.Error("expected empty context when pool is nil and profile only uses pool-sourced data")
	}
}

func TestBuildPhaseContext_BudgetExceeded(t *testing.T) {
	// Create a work unit with a large description that will exceed a tiny budget
	wu := &WorkUnit{
		Title:       "Test",
		Description: strings.Repeat("x", 2000), // 2000 chars = ~500 tokens
	}
	pool := varpool.New()
	pool.SetScoped("sys", "last_diff", strings.Repeat("d", 2000), "test")

	profile := PhaseContextProfile{
		IncludeTask:    true,
		IncludeDiff:    true,
		MaxTokenBudget: 300, // Only enough for ~1200 chars
	}

	_, metrics := BuildPhaseContext(context.Background(), profile, wu, pool)

	// Task section should fit, diff should be excluded
	if len(metrics.SectionsExcluded) == 0 {
		t.Error("expected at least one section excluded due to budget")
	}
}
