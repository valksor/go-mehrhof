package conductor

import (
	"testing"
	"time"

	"github.com/valksor/kvelmo/pkg/settings"
)

func TestApprove_SetsTimestamp(t *testing.T) {
	c := newTestConductor(t)
	c.workUnit = &WorkUnit{
		ID:    "test-1",
		Title: "Test Task",
	}

	before := time.Now()
	if err := c.Approve("submit"); err != nil {
		t.Fatalf("Approve() error: %v", err)
	}

	if c.workUnit.Approvals == nil {
		t.Fatal("expected Approvals map to be initialized")
	}
	record, ok := c.workUnit.Approvals["submit"]
	if !ok {
		t.Fatal("expected submit approval to be set")
	}
	if record.ApprovedAt.Before(before) {
		t.Errorf("approval timestamp %v is before test start %v", record.ApprovedAt, before)
	}
	if record.ApprovedBy == "" {
		t.Error("expected ApprovedBy to be set")
	}
}

func TestApprove_NoTask(t *testing.T) {
	c := newTestConductor(t)

	if err := c.Approve("submit"); err == nil {
		t.Fatal("expected error when no task loaded")
	}
}

func TestCheckReviewItem(t *testing.T) {
	c := newTestConductor(t)
	c.workUnit = &WorkUnit{
		ID:    "test-1",
		Title: "Test Task",
	}

	if err := c.CheckReviewItem("security"); err != nil {
		t.Fatalf("CheckReviewItem() error: %v", err)
	}

	if len(c.workUnit.ChecklistChecked) != 1 || c.workUnit.ChecklistChecked[0] != "security" {
		t.Errorf("expected [security], got %v", c.workUnit.ChecklistChecked)
	}

	// Check idempotency - checking same item again should not duplicate
	if err := c.CheckReviewItem("security"); err != nil {
		t.Fatalf("CheckReviewItem() error: %v", err)
	}

	if len(c.workUnit.ChecklistChecked) != 1 {
		t.Errorf("expected 1 item after duplicate check, got %d", len(c.workUnit.ChecklistChecked))
	}
}

func TestUncheckReviewItem(t *testing.T) {
	c := newTestConductor(t)
	c.workUnit = &WorkUnit{
		ID:               "test-1",
		Title:            "Test Task",
		ChecklistChecked: []string{"security", "performance"},
	}

	if err := c.UncheckReviewItem("security"); err != nil {
		t.Fatalf("UncheckReviewItem() error: %v", err)
	}

	if len(c.workUnit.ChecklistChecked) != 1 || c.workUnit.ChecklistChecked[0] != "performance" {
		t.Errorf("expected [performance], got %v", c.workUnit.ChecklistChecked)
	}
}

func TestReviewChecklistStatus(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Policy.ReviewChecklist = []string{"security", "performance", "tests"}

	c, err := New(
		WithWorkDir(t.TempDir()),
		WithSettings(s),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	c.workUnit = &WorkUnit{
		ID:               "test-1",
		Title:            "Test Task",
		ChecklistChecked: []string{"security"},
	}

	required, checked := c.ReviewChecklistStatus()

	if len(required) != 3 {
		t.Errorf("expected 3 required items, got %d", len(required))
	}
	if len(checked) != 1 || checked[0] != "security" {
		t.Errorf("expected [security] checked, got %v", checked)
	}
}
