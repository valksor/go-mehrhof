package storage

import (
	"testing"
	"time"
)

func TestArchiveTask(t *testing.T) {
	store := NewStore(t.TempDir(), true)

	task := ArchivedTask{
		ID:           "task-1",
		Title:        "Fix login bug",
		Branch:       "fix/login",
		Source:       "github:owner/repo#1",
		FinalState:   "finished",
		StartedAt:    time.Now().Add(-1 * time.Hour),
		CompletedAt:  time.Now(),
		Duration:     "1h0m",
		FilesTouched: []string{"pkg/auth/login.go", "pkg/auth/login_test.go"},
	}

	if err := store.ArchiveTask(task); err != nil {
		t.Fatalf("ArchiveTask: %v", err)
	}

	tasks, err := store.ListArchivedTasks()
	if err != nil {
		t.Fatalf("ListArchivedTasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
	if tasks[0].ID != "task-1" {
		t.Errorf("ID = %q, want %q", tasks[0].ID, "task-1")
	}
	if tasks[0].Title != "Fix login bug" {
		t.Errorf("Title = %q, want %q", tasks[0].Title, "Fix login bug")
	}
}

func TestArchiveMultiple(t *testing.T) {
	store := NewStore(t.TempDir(), true)

	now := time.Now()

	_ = store.ArchiveTask(ArchivedTask{
		ID:           "task-1",
		Title:        "First",
		FinalState:   "finished",
		CompletedAt:  now.Add(-2 * time.Hour),
		Duration:     "30m",
		FilesTouched: []string{"cmd/main.go"},
	})
	_ = store.ArchiveTask(ArchivedTask{
		ID:           "task-2",
		Title:        "Second",
		FinalState:   "finished",
		CompletedAt:  now.Add(-1 * time.Hour),
		Duration:     "45m",
		FilesTouched: []string{"pkg/api/handler.go", "pkg/api/handler_test.go"},
	})
	_ = store.ArchiveTask(ArchivedTask{
		ID:           "task-3",
		Title:        "Third",
		FinalState:   "abandoned",
		CompletedAt:  now,
		Duration:     "15m",
		FilesTouched: []string{"pkg/web/server.go"},
	})

	tasks, err := store.ListArchivedTasks()
	if err != nil {
		t.Fatalf("ListArchivedTasks: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("got %d tasks, want 3", len(tasks))
	}
	// Newest first
	if tasks[0].Title != "Third" {
		t.Errorf("tasks[0] = %q, want Third (newest first)", tasks[0].Title)
	}
	if tasks[2].Title != "First" {
		t.Errorf("tasks[2] = %q, want First (oldest last)", tasks[2].Title)
	}
}

func TestListArchivedTasksEmpty(t *testing.T) {
	store := NewStore(t.TempDir(), true)

	tasks, err := store.ListArchivedTasks()
	if err != nil {
		t.Fatalf("ListArchivedTasks: %v", err)
	}
	if tasks != nil {
		t.Errorf("expected nil, got %v", tasks)
	}
}
