package storage

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestSaveLoadTaskState(t *testing.T) {
	store := newTestStore(t)

	ts := &TaskState{
		State:       "loaded",
		ID:          "task-abc",
		Title:       "Test task",
		Description: "A test task description",
		Branch:      "kvelmo/task-abc",
		Metadata:    map[string]string{"key": "value"},
		Source: &TaskSource{
			Provider:  "file",
			Reference: "test.md",
		},
		CreatedAt: time.Now().Round(time.Second),
		UpdatedAt: time.Now().Round(time.Second),
	}

	if err := store.SaveTaskState(ts); err != nil {
		t.Fatalf("SaveTaskState() error = %v", err)
	}

	got, err := store.LoadTaskState("task-abc")
	if err != nil {
		t.Fatalf("LoadTaskState() error = %v", err)
	}

	if got.State != ts.State {
		t.Errorf("State = %q, want %q", got.State, ts.State)
	}
	if got.Title != ts.Title {
		t.Errorf("Title = %q, want %q", got.Title, ts.Title)
	}
	if got.Description != ts.Description {
		t.Errorf("Description = %q, want %q", got.Description, ts.Description)
	}
	if got.Branch != ts.Branch {
		t.Errorf("Branch = %q, want %q", got.Branch, ts.Branch)
	}
	if got.Metadata["key"] != "value" {
		t.Errorf("Metadata[key] = %q, want %q", got.Metadata["key"], "value")
	}
	if got.Source == nil || got.Source.Provider != "file" || got.Source.Reference != "test.md" {
		t.Errorf("Source = %+v, want provider=file reference=test.md", got.Source)
	}
}

func TestSaveTaskState_Overwrite(t *testing.T) {
	store := newTestStore(t)

	ts := &TaskState{ID: "task-1", Title: "Original"}
	if err := store.SaveTaskState(ts); err != nil {
		t.Fatalf("first SaveTaskState() error = %v", err)
	}

	ts.Title = "Updated"
	if err := store.SaveTaskState(ts); err != nil {
		t.Fatalf("second SaveTaskState() error = %v", err)
	}

	got, err := store.LoadTaskState("task-1")
	if err != nil {
		t.Fatalf("LoadTaskState() error = %v", err)
	}
	if got.Title != "Updated" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated")
	}
}

func TestTaskStateExists(t *testing.T) {
	store := newTestStore(t)

	if store.TaskStateExists("nonexistent") {
		t.Error("TaskStateExists() = true for nonexistent task, want false")
	}

	ts := &TaskState{ID: "my-task", Title: "Title"}
	if err := store.SaveTaskState(ts); err != nil {
		t.Fatalf("SaveTaskState() error = %v", err)
	}

	if !store.TaskStateExists("my-task") {
		t.Error("TaskStateExists() = false after save, want true")
	}
}

func TestDeleteTaskState(t *testing.T) {
	store := newTestStore(t)

	// Deleting non-existent is not an error
	if err := store.DeleteTaskState("nonexistent"); err != nil {
		t.Errorf("DeleteTaskState() non-existent error = %v, want nil", err)
	}

	ts := &TaskState{ID: "to-delete", Title: "Delete me"}
	if err := store.SaveTaskState(ts); err != nil {
		t.Fatalf("SaveTaskState() error = %v", err)
	}

	if err := store.DeleteTaskState("to-delete"); err != nil {
		t.Fatalf("DeleteTaskState() error = %v", err)
	}

	if store.TaskStateExists("to-delete") {
		t.Error("TaskStateExists() = true after delete, want false")
	}
}

func TestFindActiveTask_Empty(t *testing.T) {
	store := newTestStore(t)

	id, err := store.FindActiveTask()
	if err != nil {
		t.Fatalf("FindActiveTask() empty error = %v", err)
	}
	if id != "" {
		t.Errorf("FindActiveTask() empty = %q, want empty string", id)
	}
}

func TestFindActiveTask_MostRecent(t *testing.T) {
	store := newTestStore(t)

	if err := store.SaveTaskState(&TaskState{ID: "task-a", Title: "A"}); err != nil {
		t.Fatalf("SaveTaskState(task-a) error = %v", err)
	}
	time.Sleep(20 * time.Millisecond) // ensure different mtime
	if err := store.SaveTaskState(&TaskState{ID: "task-b", Title: "B"}); err != nil {
		t.Fatalf("SaveTaskState(task-b) error = %v", err)
	}

	got, err := store.FindActiveTask()
	if err != nil {
		t.Fatalf("FindActiveTask() error = %v", err)
	}
	if got != "task-b" {
		t.Errorf("FindActiveTask() = %q, want %q", got, "task-b")
	}
}

func TestLoadTaskState_NotExist(t *testing.T) {
	store := newTestStore(t)

	_, err := store.LoadTaskState("nonexistent")
	if err == nil {
		t.Error("LoadTaskState() expected error for non-existent task, got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadTaskState() error = %v, want to wrap os.ErrNotExist", err)
	}
}

func TestLoadTaskState_InvalidYAML(t *testing.T) {
	store := newTestStore(t)

	if err := EnsureDir(store.WorkDir("bad-task")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.TaskStateFile("bad-task"), []byte("{invalid: yaml: [}"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := store.LoadTaskState("bad-task")
	if err == nil {
		t.Error("LoadTaskState() expected error for invalid YAML, got nil")
	}
}

func TestTaskState_WithHierarchy(t *testing.T) {
	store := newTestStore(t)

	ts := &TaskState{
		ID:    "task-child",
		Title: "Child task",
		Hierarchy: &TaskHierarchy{
			Parent: &TaskHierarchySummary{
				ID:    "parent-1",
				Title: "Parent task",
			},
			Siblings: []TaskHierarchySummary{
				{ID: "sibling-1", Title: "Sibling"},
			},
		},
	}

	if err := store.SaveTaskState(ts); err != nil {
		t.Fatalf("SaveTaskState() error = %v", err)
	}

	got, err := store.LoadTaskState("task-child")
	if err != nil {
		t.Fatalf("LoadTaskState() error = %v", err)
	}

	if got.Hierarchy == nil {
		t.Fatal("Hierarchy is nil after round-trip")
	}
	if got.Hierarchy.Parent == nil || got.Hierarchy.Parent.ID != "parent-1" {
		t.Errorf("Parent.ID = %v, want parent-1", got.Hierarchy.Parent)
	}
	if len(got.Hierarchy.Siblings) != 1 || got.Hierarchy.Siblings[0].ID != "sibling-1" {
		t.Errorf("Siblings = %v, want [{sibling-1}]", got.Hierarchy.Siblings)
	}
}

func TestTaskState_WithApprovalRecords(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().Round(time.Second)
	ts := &TaskState{
		ID:    "task-approved",
		Title: "Approved task",
		Approvals: map[string]TaskApprovalRecord{
			"submit": {
				ApprovedBy: "testuser",
				ApprovedAt: now,
			},
			"plan": {
				ApprovedBy: "admin",
				ApprovedAt: now.Add(-time.Hour),
			},
		},
		ChecklistChecked: []string{"security", "tests"},
	}

	if err := store.SaveTaskState(ts); err != nil {
		t.Fatalf("SaveTaskState() error = %v", err)
	}

	got, err := store.LoadTaskState("task-approved")
	if err != nil {
		t.Fatalf("LoadTaskState() error = %v", err)
	}

	if len(got.Approvals) != 2 {
		t.Fatalf("Approvals count = %d, want 2", len(got.Approvals))
	}

	submit, ok := got.Approvals["submit"]
	if !ok {
		t.Fatal("submit approval not found")
	}
	if submit.ApprovedBy != "testuser" {
		t.Errorf("submit.ApprovedBy = %q, want testuser", submit.ApprovedBy)
	}
	if !submit.ApprovedAt.Equal(now) {
		t.Errorf("submit.ApprovedAt = %v, want %v", submit.ApprovedAt, now)
	}

	plan, ok := got.Approvals["plan"]
	if !ok {
		t.Fatal("plan approval not found")
	}
	if plan.ApprovedBy != "admin" {
		t.Errorf("plan.ApprovedBy = %q, want admin", plan.ApprovedBy)
	}

	if len(got.ChecklistChecked) != 2 {
		t.Errorf("ChecklistChecked = %v, want [security, tests]", got.ChecklistChecked)
	}
}
