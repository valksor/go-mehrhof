package help

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadContext_NoWorkspace(t *testing.T) {
	// Create an isolated directory structure where parent traversal won't find .mehrhof
	// We need to avoid the parent directory search by using /tmp
	tmpDir := t.TempDir()
	isolatedDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(isolatedDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	t.Chdir(isolatedDir)

	ctx := LoadContext()

	// In an isolated temp directory, there should be no .mehrhof
	// Note: storage.OpenWorkspace walks up directories, so we need a truly isolated path
	// If it finds a .mehrhof somewhere, HasWorkspace will be true
	// The important check is that HasActiveTask should be false
	if ctx.HasActiveTask {
		t.Error("expected HasActiveTask to be false")
	}
	if ctx.HasSpecifications {
		t.Error("expected HasSpecifications to be false")
	}
}

func TestLoadContext_WithWorkspaceNoTask(t *testing.T) {
	// Create a temp directory with .mehrhof but no active task
	tmpDir := t.TempDir()
	mehrhofDir := filepath.Join(tmpDir, ".mehrhof")
	if err := os.Mkdir(mehrhofDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	t.Chdir(tmpDir)

	ctx := LoadContext()

	if !ctx.HasWorkspace {
		t.Error("expected HasWorkspace to be true")
	}
	if ctx.HasActiveTask {
		t.Error("expected HasActiveTask to be false")
	}
}

func TestLoadContext_WithActiveTask(t *testing.T) {
	// Create a temp directory with .mehrhof and an active task
	tmpDir := t.TempDir()
	mehrhofDir := filepath.Join(tmpDir, ".mehrhof")
	if err := os.MkdirAll(mehrhofDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create work directory
	workDir := filepath.Join(mehrhofDir, "work")
	taskDir := filepath.Join(workDir, "test-task-id")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create .active_task file in the PROJECT ROOT (tmpDir), not in .mehrhof
	// This is where storage expects it
	activeTaskYaml := `id: test-task-id
ref: file:task.md
work_dir: .mehrhof/work/test-task-id
state: implementing
use_git: true
started: 2024-01-01T00:00:00Z
`
	activeTaskFile := filepath.Join(tmpDir, ".active_task")
	if err := os.WriteFile(activeTaskFile, []byte(activeTaskYaml), 0o644); err != nil {
		t.Fatalf("write .active_task: %v", err)
	}

	// Create work.yaml with task metadata
	workYaml := `version: "1"
metadata:
  id: test-task-id
  title: Test Task
  created_at: 2024-01-01T00:00:00Z
  updated_at: 2024-01-01T00:00:00Z
source:
  type: file
  ref: task.md
  read_at: 2024-01-01T00:00:00Z
`
	if err := os.WriteFile(filepath.Join(taskDir, "work.yaml"), []byte(workYaml), 0o644); err != nil {
		t.Fatalf("write work.yaml: %v", err)
	}

	t.Chdir(tmpDir)

	ctx := LoadContext()

	if !ctx.HasWorkspace {
		t.Error("expected HasWorkspace to be true")
	}
	if !ctx.HasActiveTask {
		t.Error("expected HasActiveTask to be true")
	}
	if ctx.TaskID != "test-task-id" {
		t.Errorf("TaskID = %q, want 'test-task-id'", ctx.TaskID)
	}
	if ctx.TaskState != "implementing" {
		t.Errorf("TaskState = %q, want 'implementing'", ctx.TaskState)
	}
	if !ctx.UseGit {
		t.Error("expected UseGit to be true")
	}
}

func TestLoadContext_WithSpecifications(t *testing.T) {
	// Create a temp directory with .mehrhof, active task, and specifications
	tmpDir := t.TempDir()
	mehrhofDir := filepath.Join(tmpDir, ".mehrhof")
	if err := os.MkdirAll(mehrhofDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create work directory
	workDir := filepath.Join(mehrhofDir, "work")
	taskDir := filepath.Join(workDir, "test-task-id")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create specifications directory
	specsDir := filepath.Join(taskDir, "specifications")
	if err := os.Mkdir(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a specification file
	specFile := filepath.Join(specsDir, "specification-1.md")
	if err := os.WriteFile(specFile, []byte("# Specification"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Create .active_task file in the PROJECT ROOT
	activeTaskYaml := `id: test-task-id
ref: file:task.md
work_dir: .mehrhof/work/test-task-id
state: planning
use_git: false
started: 2024-01-01T00:00:00Z
`
	activeTaskFile := filepath.Join(tmpDir, ".active_task")
	if err := os.WriteFile(activeTaskFile, []byte(activeTaskYaml), 0o644); err != nil {
		t.Fatalf("write .active_task: %v", err)
	}

	// Create work.yaml
	workYaml := `version: "1"
metadata:
  id: test-task-id
  title: Test Task
  created_at: 2024-01-01T00:00:00Z
  updated_at: 2024-01-01T00:00:00Z
source:
  type: file
  ref: task.md
  read_at: 2024-01-01T00:00:00Z
`
	if err := os.WriteFile(filepath.Join(taskDir, "work.yaml"), []byte(workYaml), 0o644); err != nil {
		t.Fatalf("write work.yaml: %v", err)
	}

	t.Chdir(tmpDir)

	ctx := LoadContext()

	if !ctx.HasWorkspace {
		t.Error("expected HasWorkspace to be true")
	}
	if !ctx.HasActiveTask {
		t.Error("expected HasActiveTask to be true")
	}
	if !ctx.HasSpecifications {
		t.Error("expected HasSpecifications to be true")
	}
}

func TestLoadContext_BadWorkYaml(t *testing.T) {
	// Create a temp directory with .mehrhof and active task but bad work.yaml
	tmpDir := t.TempDir()
	mehrhofDir := filepath.Join(tmpDir, ".mehrhof")
	if err := os.MkdirAll(mehrhofDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create work directory
	workDir := filepath.Join(mehrhofDir, "work")
	taskDir := filepath.Join(workDir, "test-task-id")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create .active_task file in the PROJECT ROOT
	activeTaskYaml := `id: test-task-id
ref: file:task.md
work_dir: .mehrhof/work/test-task-id
state: planning
use_git: false
started: 2024-01-01T00:00:00Z
`
	activeTaskFile := filepath.Join(tmpDir, ".active_task")
	if err := os.WriteFile(activeTaskFile, []byte(activeTaskYaml), 0o644); err != nil {
		t.Fatalf("write .active_task: %v", err)
	}

	// Create invalid work.yaml (not valid YAML)
	if err := os.WriteFile(filepath.Join(taskDir, "work.yaml"), []byte("not valid yaml: [["), 0o644); err != nil {
		t.Fatalf("write work.yaml: %v", err)
	}

	t.Chdir(tmpDir)

	ctx := LoadContext()

	// LoadContext only reads .active_task file, not work.yaml
	// So even with bad work.yaml, HasActiveTask should be true
	// The LoadActiveTask function successfully parses the .active_task file
	if !ctx.HasActiveTask {
		t.Error("expected HasActiveTask to be true (LoadContext doesn't read work.yaml)")
	}
}

func TestHelpContext_StructFields(t *testing.T) {
	// Test that HelpContext struct can be created and accessed
	ctx := &HelpContext{
		HasWorkspace:      true,
		HasActiveTask:     true,
		TaskID:            "task-123",
		TaskState:         "implementing",
		HasSpecifications: true,
		UseGit:            true,
	}

	if !ctx.HasWorkspace {
		t.Error("HasWorkspace should be true")
	}
	if !ctx.HasActiveTask {
		t.Error("HasActiveTask should be true")
	}
	if ctx.TaskID != "task-123" {
		t.Errorf("TaskID = %q, want 'task-123'", ctx.TaskID)
	}
	if ctx.TaskState != "implementing" {
		t.Errorf("TaskState = %q, want 'implementing'", ctx.TaskState)
	}
	if !ctx.HasSpecifications {
		t.Error("HasSpecifications should be true")
	}
	if !ctx.UseGit {
		t.Error("UseGit should be true")
	}
}
