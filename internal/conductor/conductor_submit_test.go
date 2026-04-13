package conductor

import (
	"context"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/settings"
)

// ─── Submit ──────────────────────────────────────────────────────────────────

func TestConductorSubmit_NoTask(t *testing.T) {
	c, _ := New()
	err := c.Submit(context.Background(), false)
	if err == nil {
		t.Error("Submit() with no task should return error")
	}
	if !strings.Contains(err.Error(), "no task loaded") {
		t.Errorf("Submit() error = %q, want 'no task loaded'", err.Error())
	}
}

func TestConductorSubmit_WrongState(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{
		ID:    "s1",
		Title: "T",
		Source: &Source{
			Provider:  "github",
			Reference: "owner/repo#1",
		},
	})
	// Set cached quality gate pass to avoid synchronous quality gate run
	passed := true
	c.workUnit.QualityGatePassed = &passed
	// StateNone does not allow EventSubmit
	err := c.Submit(context.Background(), false)
	if err == nil {
		t.Error("Submit() from wrong state should return error")
	}
	if !strings.Contains(err.Error(), "cannot submit") {
		t.Errorf("Submit() error = %q, want 'cannot submit'", err.Error())
	}
}

func TestConductorSubmit_QualityGateCachedFail(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "s2", Title: "T"})
	c.machine.ForceState(StateReviewing) // Submit is allowed from reviewing
	passed := false
	c.workUnit.QualityGatePassed = &passed
	c.workUnit.QualityGateError = "lint failed"
	err := c.Submit(context.Background(), false)
	if err == nil {
		t.Error("Submit() with failed quality gate should return error")
	}
	if !strings.Contains(err.Error(), "quality gate failed") {
		t.Errorf("Submit() error = %q, want 'quality gate failed'", err.Error())
	}
}

// ─── validatePRSections ─────────────────────────────────────────────────────

func TestValidatePRSections_AllFilled(t *testing.T) {
	body := "## Summary\nThis is a summary.\n\n## Test Plan\n- [ ] Manual testing\n\n## Related\nhttps://example.com\n"
	missing := validatePRSections(body, []string{"summary", "test plan", "related"})
	if len(missing) != 0 {
		t.Errorf("expected no missing sections, got %v", missing)
	}
}

func TestValidatePRSections_MissingSections(t *testing.T) {
	body := "## Summary\nThis is a summary.\n\n## Test Plan\n<!-- TODO -->\n"
	// "test plan" has only HTML comment, "rollback" doesn't exist
	missing := validatePRSections(body, []string{"summary", "test plan", "rollback"})
	if len(missing) != 2 {
		t.Errorf("expected 2 missing sections, got %v", missing)
	}
}

func TestValidatePRSections_EmptyBody(t *testing.T) {
	missing := validatePRSections("", []string{"summary"})
	if len(missing) != 1 {
		t.Errorf("expected 1 missing, got %v", missing)
	}
}

func TestValidatePRSections_NoRequirements(t *testing.T) {
	missing := validatePRSections("## Summary\nContent", nil)
	if len(missing) != 0 {
		t.Errorf("expected 0 missing, got %v", missing)
	}
}

func TestValidatePRSections_CaseInsensitive(t *testing.T) {
	body := "## SUMMARY\nContent here.\n\n## What Changed\nDetails.\n"
	missing := validatePRSections(body, []string{"summary", "what changed"})
	if len(missing) != 0 {
		t.Errorf("expected 0 missing, got %v", missing)
	}
}

func TestValidatePRSections_EmptySection(t *testing.T) {
	body := "## Summary\n\n## Test Plan\nTesting steps.\n"
	missing := validatePRSections(body, []string{"summary", "test plan"})
	if len(missing) != 1 || missing[0] != "summary" {
		t.Errorf("expected [summary] missing, got %v", missing)
	}
}

func TestValidatePRSections_SectionAtEndNoContent(t *testing.T) {
	body := "## Summary\nContent.\n\n## Notes\n"
	missing := validatePRSections(body, []string{"notes"})
	if len(missing) != 1 {
		t.Errorf("expected 1 missing (empty section at end), got %v", missing)
	}
}

// ─── Submit approval checks ────────────────────────────────────────────────

func TestSubmit_RequiresApproval(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Policy.ApprovalRequired = map[string]bool{"submit": true}

	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.ForceWorkUnit(&WorkUnit{ID: "a1", Title: "T"})
	c.machine.ForceState(StateReviewing)
	passed := true
	c.workUnit.QualityGatePassed = &passed

	err = c.Submit(context.Background(), false)
	if err == nil {
		t.Fatal("expected approval required error")
	}
	if !strings.Contains(err.Error(), "approval required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubmit_ChecklistBlocks(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Policy.ReviewChecklist = []string{"security", "tests"}

	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.ForceWorkUnit(&WorkUnit{
		ID:               "c1",
		Title:            "T",
		ChecklistChecked: []string{"security"}, // "tests" not checked
	})
	c.machine.ForceState(StateReviewing)

	err = c.Submit(context.Background(), false)
	if err == nil {
		t.Fatal("expected checklist error")
	}
	if !strings.Contains(err.Error(), "tests") {
		t.Errorf("expected error about 'tests', got: %v", err)
	}
}
