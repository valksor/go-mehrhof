package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/settings"
)

func TestRegistry_GetPRStatus_UnknownSource(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())
	_, err := r.GetPRStatus(context.Background(), "invalid-source")
	if err == nil {
		t.Error("GetPRStatus() should return error for invalid source format")
	}
}

func TestRegistry_GetPRStatus_UnsupportedProvider(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())
	// File provider doesn't support PR status
	_, err := r.GetPRStatus(context.Background(), "file:/tmp/task.md")
	if err == nil {
		t.Error("GetPRStatus() should return error for provider without PR status support")
	}
}

func TestRegistry_GetPRStatus_UnknownProvider(t *testing.T) {
	r := &Registry{providers: make(map[string]Provider)}
	_, err := r.GetPRStatus(context.Background(), "github:owner/repo#1")
	if err == nil {
		t.Error("GetPRStatus() should return error when provider not registered")
	}
}

func TestRegistry_FetchWithHierarchy_FileProvider(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())

	dir := t.TempDir()
	taskFile := filepath.Join(dir, "task.md")
	if err := os.WriteFile(taskFile, []byte("# Simple Task\nDo something"), 0o644); err != nil {
		t.Fatal(err)
	}

	task, err := r.FetchWithHierarchy(context.Background(), "file", taskFile, HierarchyOptions{
		IncludeParent:   true,
		IncludeSiblings: true,
	})
	if err != nil {
		t.Fatalf("FetchWithHierarchy() error = %v", err)
	}
	if task == nil {
		t.Fatal("FetchWithHierarchy() returned nil")
	}
	if task.ParentTask != nil {
		t.Error("file provider should not return parent task")
	}
	if task.SiblingTasks != nil {
		t.Error("file provider should not return sibling tasks")
	}
}

func TestRegistry_FetchWithHierarchy_UnknownProvider(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())
	_, err := r.FetchWithHierarchy(context.Background(), "nonexistent", "id", HierarchyOptions{})
	if err == nil {
		t.Error("FetchWithHierarchy() should return error for unknown provider")
	}
}

func TestRegistry_Register_Override(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())

	// Register a custom provider with the same name as an existing one
	custom := &nonHierarchyProvider{
		name: "file",
		task: &Task{ID: "custom-task", Title: "Custom", Source: "file"},
	}
	r.Register(custom)

	p, err := r.Get("file")
	if err != nil {
		t.Fatalf("Get(file) error = %v", err)
	}

	// Should get the overridden provider
	task, err := p.FetchTask(context.Background(), "anything")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}
	if task.ID != "custom-task" {
		t.Errorf("task.ID = %q, want custom-task (overridden provider)", task.ID)
	}
}

func TestRegistry_NilSettings(t *testing.T) {
	// NewRegistry with nil settings should not panic
	r := NewRegistry(nil)
	if r == nil {
		t.Fatal("NewRegistry(nil) returned nil")
	}

	// Providers should still be registered (with empty tokens)
	p, err := r.Get("file")
	if err != nil {
		t.Errorf("Get(file) error = %v", err)
	}
	if p == nil {
		t.Error("file provider should not be nil")
	}
}

func TestRegistry_FetchTask_ValidFile(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())

	dir := t.TempDir()
	taskFile := filepath.Join(dir, "task.md")
	if err := os.WriteFile(taskFile, []byte("# My Task\nDescription here"), 0o644); err != nil {
		t.Fatal(err)
	}

	task, err := r.FetchTask(context.Background(), "file:"+taskFile)
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}
	if task.Title != "My Task" {
		t.Errorf("task.Title = %q, want 'My Task'", task.Title)
	}
}

func TestRegistry_Fetch_ValidFile(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())

	dir := t.TempDir()
	taskFile := filepath.Join(dir, "task.md")
	if err := os.WriteFile(taskFile, []byte("# Direct Fetch\nBody"), 0o644); err != nil {
		t.Fatal(err)
	}

	task, err := r.Fetch(context.Background(), "file", taskFile)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if task.Title != "Direct Fetch" {
		t.Errorf("task.Title = %q, want 'Direct Fetch'", task.Title)
	}
}
