package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkDir_SaveInProject(t *testing.T) {
	root := t.TempDir()
	got := WorkDir(root, "task-1", true)
	if !strings.HasPrefix(got, root) {
		t.Errorf("WorkDir(saveInProject=true) = %q, want prefix %q", got, root)
	}
	if !strings.Contains(got, "task-1") {
		t.Errorf("WorkDir() = %q, want to contain task ID", got)
	}
}

func TestWorkDir_HomeStorage(t *testing.T) {
	root := t.TempDir()
	home, _ := os.UserHomeDir()
	got := WorkDir(root, "task-1", false)
	if !strings.HasPrefix(got, home) {
		t.Errorf("WorkDir(saveInProject=false) = %q, want prefix %q", got, home)
	}
}

func TestWorkDir_DifferentTaskIDs(t *testing.T) {
	root := t.TempDir()
	a := WorkDir(root, "task-a", true)
	b := WorkDir(root, "task-b", true)
	if a == b {
		t.Error("WorkDir() returned same path for different task IDs")
	}
}

func TestStorePaths(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root, true)
	taskID := "test-task"
	workDir := store.WorkDir(taskID)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"SpecificationsDir", store.SpecificationsDir(taskID), filepath.Join(workDir, "specifications")},
		{"PlansDir", store.PlansDir(taskID), filepath.Join(workDir, "plans")},
		{"ReviewsDir", store.ReviewsDir(taskID), filepath.Join(workDir, "reviews")},
		{"ChatFile", store.ChatFile(taskID), filepath.Join(workDir, "chat.json")},
		{"TaskStateFile", store.TaskStateFile(taskID), filepath.Join(workDir, "task.yaml")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestStorePaths_PackageFunctions(t *testing.T) {
	root := t.TempDir()
	taskID := "test-task"
	workDir := WorkDir(root, taskID, true)

	if SpecificationsDir(root, taskID, true) != filepath.Join(workDir, "specifications") {
		t.Error("SpecificationsDir() mismatch")
	}
	if PlansDir(root, taskID, true) != filepath.Join(workDir, "plans") {
		t.Error("PlansDir() mismatch")
	}
	if ReviewsDir(root, taskID, true) != filepath.Join(workDir, "reviews") {
		t.Error("ReviewsDir() mismatch")
	}
	if ChatFile(root, taskID, true) != filepath.Join(workDir, "chat.json") {
		t.Error("ChatFile() mismatch")
	}
	if TaskStateFile(root, taskID, true) != filepath.Join(workDir, "task.yaml") {
		t.Error("TaskStateFile() mismatch")
	}
}

func TestWorkRoot(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root, true)
	workRoot := store.WorkRoot()
	taskWorkDir := store.WorkDir("some-task")
	if filepath.Dir(taskWorkDir) != workRoot {
		t.Errorf("WorkRoot() = %q, want parent of WorkDir(); got parent %q", workRoot, filepath.Dir(taskWorkDir))
	}
}

func TestEnsureDir(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "a", "b", "c")

	if err := EnsureDir(nested); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}
	if _, err := os.Stat(nested); err != nil {
		t.Errorf("EnsureDir() did not create directory: %v", err)
	}

	// Idempotent second call
	if err := EnsureDir(nested); err != nil {
		t.Errorf("EnsureDir() second call error = %v", err)
	}
}

func TestStoreAccessors(t *testing.T) {
	root := t.TempDir()

	s1 := NewStore(root, true)
	if s1.ProjectRoot() != root {
		t.Errorf("ProjectRoot() = %q, want %q", s1.ProjectRoot(), root)
	}
	if !s1.SaveInProject() {
		t.Error("SaveInProject() = false, want true")
	}

	s2 := NewStore(root, false)
	if s2.SaveInProject() {
		t.Error("SaveInProject() = true, want false")
	}
}

func TestSessionsFile(t *testing.T) {
	root := t.TempDir()
	got := SessionsFile(root)
	if !strings.HasPrefix(got, root) {
		t.Errorf("SessionsFile() = %q, want prefix %q", got, root)
	}
	if !strings.HasSuffix(got, "sessions.json") {
		t.Errorf("SessionsFile() = %q, want suffix sessions.json", got)
	}
}
