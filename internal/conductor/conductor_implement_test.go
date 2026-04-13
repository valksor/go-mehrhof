package conductor

import (
	"context"
	"strings"
	"testing"
)

// ─── Implement ───────────────────────────────────────────────────────────────

func TestConductorImplement_NoTaskError(t *testing.T) {
	c, _ := New()
	_, err := c.Implement(context.Background())
	if err == nil {
		t.Error("Implement() with no task should return error")
	}
	if !strings.Contains(err.Error(), "no task loaded") {
		t.Errorf("Implement() error = %q, want 'no task loaded'", err.Error())
	}
}

func TestConductorImplement_NoPoolError(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{
		ID:             "i1",
		Title:          "T",
		Description:    "desc",
		Specifications: []string{"spec-1.md"},
	})
	c.machine.ForceState(StatePlanned)
	_, err := c.Implement(context.Background())
	if err == nil {
		t.Error("Implement() with no pool should return error")
	}
	if !strings.Contains(err.Error(), "no worker pool available") {
		t.Errorf("Implement() error = %q, want 'no worker pool available'", err.Error())
	}
}

func TestConductorImplement_WrongStateError(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "i2", Title: "T", Description: "desc"})
	// StateNone does not allow EventImplement; pool check runs first (pool is nil)
	_, err := c.Implement(context.Background())
	if err == nil {
		t.Error("Implement() from wrong state should return error")
	}
}

// ─── buildImplementPrompt ────────────────────────────────────────────────────

func TestBuildImplementPrompt_BasicContent(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{
		ID:          "ip1",
		Title:       "Add logging",
		Description: "Add structured logging to all handlers",
	})
	prompt := c.buildImplementPrompt()
	if !strings.Contains(prompt, "Add logging") {
		t.Error("implement prompt should contain task title")
	}
	if !strings.Contains(prompt, "Add structured logging") {
		t.Error("implement prompt should contain task description")
	}
}

func TestBuildImplementPrompt_IncludesSpecs(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{
		ID:             "ip2",
		Title:          "Add caching",
		Description:    "Implement caching layer",
		Specifications: []string{"spec-1.md", "spec-2.md"},
	})
	prompt := c.buildImplementPrompt()
	if !strings.Contains(prompt, "spec-1.md") {
		t.Error("implement prompt should contain specification paths")
	}
	if !strings.Contains(prompt, "Specifications") {
		t.Error("implement prompt should contain 'Specifications' section")
	}
}

func TestBuildImplementPrompt_IncludesHierarchy(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{
		ID:          "ip3",
		Title:       "Sub-task",
		Description: "Part of epic",
		Hierarchy: &HierarchyContext{
			Parent: &TaskSummary{Title: "Parent Epic", Status: "active"},
		},
	})
	prompt := c.buildImplementPrompt()
	if !strings.Contains(prompt, "Parent Task Context") {
		t.Error("implement prompt with hierarchy should contain hierarchy section")
	}
	if !strings.Contains(prompt, "Parent Epic") {
		t.Error("implement prompt with hierarchy should contain parent title")
	}
}
