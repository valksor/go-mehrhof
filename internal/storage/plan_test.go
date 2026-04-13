package storage

import (
	"strings"
	"testing"
)

func newTestPlanStore(t *testing.T) *PlanStore {
	t.Helper()

	return NewPlanStore(newTestStore(t))
}

func TestCreatePlan(t *testing.T) {
	ps := newTestPlanStore(t)

	plan, err := ps.CreatePlan("task-1", "plan-001", "initial seed")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	if plan.ID != "plan-001" {
		t.Errorf("ID = %q, want %q", plan.ID, "plan-001")
	}
	if plan.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", plan.TaskID, "task-1")
	}
	if plan.Seed != "initial seed" {
		t.Errorf("Seed = %q, want %q", plan.Seed, "initial seed")
	}
	if plan.Version != "1" {
		t.Errorf("Version = %q, want %q", plan.Version, "1")
	}
	if plan.Created.IsZero() {
		t.Error("Created is zero, want non-zero")
	}
}

func TestCreatePlan_InvalidTaskID(t *testing.T) {
	ps := newTestPlanStore(t)

	_, err := ps.CreatePlan("../traversal", "plan-001", "")
	if err == nil {
		t.Error("CreatePlan() expected error for invalid task ID, got nil")
	}
}

func TestSaveLoadPlan(t *testing.T) {
	ps := newTestPlanStore(t)

	plan, err := ps.CreatePlan("task-1", "plan-001", "seed")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	plan.Title = "My Plan Title"
	if err := ps.SavePlan("task-1", plan); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}

	got, err := ps.LoadPlan("task-1", "plan-001")
	if err != nil {
		t.Fatalf("LoadPlan() error = %v", err)
	}

	if got.Title != "My Plan Title" {
		t.Errorf("Title = %q, want %q", got.Title, "My Plan Title")
	}
	if got.ID != plan.ID {
		t.Errorf("ID = %q, want %q", got.ID, plan.ID)
	}
}

func TestAppendPlanHistory(t *testing.T) {
	ps := newTestPlanStore(t)

	if _, err := ps.CreatePlan("task-1", "plan-001", ""); err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	if err := ps.AppendPlanHistory("task-1", "plan-001", "user", "Hello from user"); err != nil {
		t.Fatalf("AppendPlanHistory(user) error = %v", err)
	}
	if err := ps.AppendPlanHistory("task-1", "plan-001", "assistant", "Hello from assistant"); err != nil {
		t.Fatalf("AppendPlanHistory(assistant) error = %v", err)
	}

	plan, err := ps.LoadPlan("task-1", "plan-001")
	if err != nil {
		t.Fatalf("LoadPlan() error = %v", err)
	}

	if len(plan.History) != 2 {
		t.Fatalf("History len = %d, want 2", len(plan.History))
	}
	if plan.History[0].Role != "user" || plan.History[0].Content != "Hello from user" {
		t.Errorf("History[0] = %+v, want role=user content=Hello from user", plan.History[0])
	}
	if plan.History[1].Role != "assistant" {
		t.Errorf("History[1].Role = %q, want assistant", plan.History[1].Role)
	}

	// Check markdown history file
	histContent, err := ps.LoadPlanHistory("task-1", "plan-001")
	if err != nil {
		t.Fatalf("LoadPlanHistory() error = %v", err)
	}
	if !strings.Contains(histContent, "Hello from user") {
		t.Error("LoadPlanHistory() missing user message")
	}
	if !strings.Contains(histContent, "Hello from assistant") {
		t.Error("LoadPlanHistory() missing assistant message")
	}
}

func TestListPlans_Empty(t *testing.T) {
	ps := newTestPlanStore(t)

	plans, err := ps.ListPlans("task-1")
	if err != nil {
		t.Fatalf("ListPlans() error = %v", err)
	}
	if len(plans) != 0 {
		t.Errorf("ListPlans() empty = %v, want []", plans)
	}
}

func TestListPlans_Multiple(t *testing.T) {
	ps := newTestPlanStore(t)

	for _, id := range []string{"plan-2023-001", "plan-2023-002", "plan-2023-003"} {
		if _, err := ps.CreatePlan("task-1", id, ""); err != nil {
			t.Fatalf("CreatePlan(%q) error = %v", id, err)
		}
	}

	plans, err := ps.ListPlans("task-1")
	if err != nil {
		t.Fatalf("ListPlans() error = %v", err)
	}
	if len(plans) != 3 {
		t.Errorf("ListPlans() len = %d, want 3", len(plans))
	}
}

func TestGetLatestPlan_Empty(t *testing.T) {
	ps := newTestPlanStore(t)

	plan, err := ps.GetLatestPlan("task-1")
	if err != nil {
		t.Fatalf("GetLatestPlan() error = %v", err)
	}
	if plan != nil {
		t.Errorf("GetLatestPlan() empty = %v, want nil", plan)
	}
}

func TestGetLatestPlan_ReturnsLast(t *testing.T) {
	ps := newTestPlanStore(t)

	for _, id := range []string{"plan-a", "plan-b", "plan-c"} {
		if _, err := ps.CreatePlan("task-1", id, id); err != nil {
			t.Fatalf("CreatePlan(%q) error = %v", id, err)
		}
	}

	plan, err := ps.GetLatestPlan("task-1")
	if err != nil {
		t.Fatalf("GetLatestPlan() error = %v", err)
	}
	if plan == nil {
		t.Fatal("GetLatestPlan() = nil, want a plan")
	}
	if plan.Seed != "plan-c" {
		t.Errorf("GetLatestPlan().Seed = %q, want plan-c", plan.Seed)
	}
}

func TestDeletePlan(t *testing.T) {
	ps := newTestPlanStore(t)

	// Delete non-existent is not an error
	if err := ps.DeletePlan("task-1", "nonexistent"); err != nil {
		t.Errorf("DeletePlan() non-existent error = %v, want nil", err)
	}

	if _, err := ps.CreatePlan("task-1", "plan-to-delete", ""); err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	if err := ps.DeletePlan("task-1", "plan-to-delete"); err != nil {
		t.Fatalf("DeletePlan() error = %v", err)
	}

	plans, err := ps.ListPlans("task-1")
	if err != nil {
		t.Fatalf("ListPlans() error = %v", err)
	}
	if len(plans) != 0 {
		t.Errorf("ListPlans() after delete = %v, want empty", plans)
	}
}

func TestGeneratePlanID(t *testing.T) {
	id1 := GeneratePlanID()
	if id1 == "" {
		t.Error("GeneratePlanID() returned empty string")
	}
	// IDs should be unique (different timestamps)
	// Just verify format looks like a date-time string
	if len(id1) < 10 {
		t.Errorf("GeneratePlanID() = %q, seems too short", id1)
	}
}

func TestPlanPaths(t *testing.T) {
	ps := newTestPlanStore(t)

	planPath := ps.PlanPath("task-1", "plan-001")
	filePath := ps.PlanFilePath("task-1", "plan-001")
	histPath := ps.PlanHistoryPath("task-1", "plan-001")

	if !strings.HasSuffix(filePath, "plan.yaml") {
		t.Errorf("PlanFilePath() = %q, want suffix plan.yaml", filePath)
	}
	if !strings.HasSuffix(histPath, "plan-history.md") {
		t.Errorf("PlanHistoryPath() = %q, want suffix plan-history.md", histPath)
	}
	if !strings.HasPrefix(filePath, planPath) {
		t.Errorf("PlanFilePath() = %q, want prefix %q", filePath, planPath)
	}
}
