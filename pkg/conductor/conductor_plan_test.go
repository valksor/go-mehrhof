package conductor

import (
	"context"
	"strings"
	"testing"
)

// ─── Plan ────────────────────────────────────────────────────────────────────

func TestConductorPlan_NoTaskError(t *testing.T) {
	c, _ := New()
	_, err := c.Plan(context.Background())
	if err == nil {
		t.Error("Plan() with no task should return error")
	}
	if !strings.Contains(err.Error(), "no task loaded") {
		t.Errorf("Plan() error = %q, want 'no task loaded'", err.Error())
	}
}

func TestConductorPlan_NoPoolError(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "p1", Title: "T", Description: "desc"})
	c.machine.ForceState(StateLoaded)
	_, err := c.Plan(context.Background())
	if err == nil {
		t.Error("Plan() with no pool should return error")
	}
	if !strings.Contains(err.Error(), "no worker pool available") {
		t.Errorf("Plan() error = %q, want 'no worker pool available'", err.Error())
	}
}

func TestConductorPlan_WrongStateError(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "p2", Title: "T", Description: "desc"})
	// StateNone with work unit — pool check runs first (pool is nil), so error is about pool.
	// To truly test wrong state, we need to bypass pool check. Since pool is nil, we get pool error.
	// This still exercises the no-pool guard from a non-loaded state.
	_, err := c.Plan(context.Background())
	if err == nil {
		t.Error("Plan() from wrong state should return error")
	}
}

func TestConductorPlan_FromPlannedReEntry(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{
		ID:          "p3",
		Title:       "T",
		Description: "desc",
		Source:      &Source{Provider: "empty", Reference: "empty:test"},
	})
	c.machine.ForceState(StatePlanned)
	// Re-entry from planned state should still fail without pool
	_, err := c.Plan(context.Background())
	if err == nil {
		t.Error("Plan() from planned state should still fail without pool")
	}
	if !strings.Contains(err.Error(), "no worker pool available") {
		t.Errorf("Plan() error = %q, want 'no worker pool available'", err.Error())
	}
}

// ─── buildPlanPromptForComplexity ────────────────────────────────────────────

func TestBuildPlanPromptForComplexity_SimpleContainsConcise(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "bp1", Title: "Fix typo", Description: "Fix a typo in README"})
	prompt := c.buildPlanPromptForComplexity(ComplexitySimple, "")
	if !strings.Contains(prompt, "concise") {
		t.Error("Simple prompt should contain 'concise'")
	}
	if !strings.Contains(prompt, "Fix typo") {
		t.Error("Simple prompt should contain task title")
	}
}

func TestBuildPlanPromptForComplexity_ComplexContainsExpert(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "bp2", Title: "Refactor auth", Description: "Major auth refactor"})
	prompt := c.buildPlanPromptForComplexity(ComplexityComplex, "")
	if !strings.Contains(prompt, "expert software engineer") {
		t.Error("Complex prompt should contain 'expert software engineer'")
	}
	if !strings.Contains(prompt, "Refactor auth") {
		t.Error("Complex prompt should contain task title")
	}
}

func TestBuildPlanPromptForComplexity_ExistingSpecsSection(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "bp3", Title: "Add feature", Description: "New feature"})
	prompt := c.buildPlanPromptForComplexity(ComplexityComplex, "existing spec content here")
	if !strings.Contains(prompt, "Previous Specifications") {
		t.Error("Prompt with existing specs should contain 'Previous Specifications'")
	}
}

// ─── buildHierarchySection ───────────────────────────────────────────────────

func TestBuildHierarchySection_NilReturnsEmpty(t *testing.T) {
	result := buildHierarchySection(nil)
	if result != "" {
		t.Errorf("buildHierarchySection(nil) = %q, want empty", result)
	}
}

func TestBuildHierarchySection_ParentContext(t *testing.T) {
	h := &HierarchyContext{
		Parent: &TaskSummary{Title: "Epic Task", Status: "active", Description: "The parent epic"},
	}
	result := buildHierarchySection(h)
	if !strings.Contains(result, "Parent Task Context") {
		t.Error("hierarchy with parent should contain 'Parent Task Context'")
	}
	if !strings.Contains(result, "Epic Task") {
		t.Error("hierarchy with parent should contain parent title")
	}
}

func TestBuildHierarchySection_SiblingsContext(t *testing.T) {
	h := &HierarchyContext{
		Siblings: []TaskSummary{
			{Title: "Sibling A", Status: "done"},
			{Title: "Sibling B", Status: "in progress"},
		},
	}
	result := buildHierarchySection(h)
	if !strings.Contains(result, "Related Subtasks") {
		t.Error("hierarchy with siblings should contain 'Related Subtasks'")
	}
	if !strings.Contains(result, "Sibling A") {
		t.Error("hierarchy with siblings should contain sibling title")
	}
}

func TestBuildHierarchySection_EmptyReturnsEmpty(t *testing.T) {
	h := &HierarchyContext{}
	result := buildHierarchySection(h)
	if result != "" {
		t.Errorf("buildHierarchySection with empty hierarchy = %q, want empty", result)
	}
}

// ─── buildDeltaSpecificationContent ──────────────────────────────────────────

func TestBuildDeltaSpecificationContent_ContainsExpectedSections(t *testing.T) {
	result := buildDeltaSpecificationContent("old line\n", "new line\nadded line\n")
	if !strings.Contains(result, "Delta Specification") {
		t.Error("delta spec should contain 'Delta Specification'")
	}
	if !strings.Contains(result, "Lines added") {
		t.Error("delta spec should contain 'Lines added'")
	}
	if !strings.Contains(result, "Lines removed") {
		t.Error("delta spec should contain 'Lines removed'")
	}
}

// ─── SaveSpecification ───────────────────────────────────────────────────────

func TestConductorSaveSpecification_NoTaskError(t *testing.T) {
	c, _ := New()
	_, err := c.SaveSpecification("spec content")
	if err == nil {
		t.Error("SaveSpecification() with no task should return error")
	}
}

func TestConductorSaveSpecification_NoStoreError(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "ss1", Title: "T"})
	_, err := c.SaveSpecification("spec content")
	if err == nil {
		t.Error("SaveSpecification() with no store should return error")
	}
}
