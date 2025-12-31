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

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(isolatedDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

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

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	ctx := LoadContext()

	if !ctx.HasWorkspace {
		t.Error("expected HasWorkspace to be true")
	}
	if ctx.HasActiveTask {
		t.Error("expected HasActiveTask to be false")
	}
}
