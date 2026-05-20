package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
)

func TestBuildContextItemsFromFlags(t *testing.T) {
	items := buildContextItemsFromFlags(
		[]string{"main.go", "pkg/x.go:40-50"},
		[]string{"HandleRequest"},
		[]string{"abc123"},
	)

	if len(items) != 4 {
		t.Fatalf("len = %d, want 4", len(items))
	}

	counts := map[conductor.ContextType]int{}
	for _, it := range items {
		counts[it.Type]++
	}
	if counts[conductor.ContextTypeFile] != 2 {
		t.Errorf("file items = %d, want 2", counts[conductor.ContextTypeFile])
	}
	if counts[conductor.ContextTypeSymbol] != 1 {
		t.Errorf("symbol items = %d, want 1", counts[conductor.ContextTypeSymbol])
	}
	if counts[conductor.ContextTypeCommit] != 1 {
		t.Errorf("commit items = %d, want 1", counts[conductor.ContextTypeCommit])
	}
}

func TestBuildContextItemsFromFlags_Empty(t *testing.T) {
	if items := buildContextItemsFromFlags(nil, nil, nil); len(items) != 0 {
		t.Errorf("empty inputs should produce 0 items, got %d", len(items))
	}
}

func TestBuildContextItems_UsesPackageVars(t *testing.T) {
	// buildContextItems delegates to buildContextItemsFromFlags using package vars.
	old := make([]string, 0, len(startContextFiles))
	old = append(old, startContextFiles...)
	t.Cleanup(func() { startContextFiles = old })

	startContextFiles = []string{"main.go"}
	startContextSymbol = []string{"Sym"}
	startContextCommit = []string{"deadbeef"}
	t.Cleanup(func() {
		startContextFiles = nil
		startContextSymbol = nil
		startContextCommit = nil
	})

	if items := buildContextItems(); len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestValidateContextItems_AcceptsExistingFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	items := []conductor.ContextItem{{Type: conductor.ContextTypeFile, Ref: "main.go"}}
	if err := validateContextItems(items); err != nil {
		t.Errorf("validate existing file: unexpected %v", err)
	}
}

func TestValidateContextItems_RejectsAbsolutePath(t *testing.T) {
	items := []conductor.ContextItem{{Type: conductor.ContextTypeFile, Ref: "/etc/passwd"}}
	err := validateContextItems(items)
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestValidateContextItems_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	items := []conductor.ContextItem{{Type: conductor.ContextTypeFile, Ref: "../../../etc/passwd"}}
	err := validateContextItems(items)
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestValidateContextItems_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	items := []conductor.ContextItem{{Type: conductor.ContextTypeFile, Ref: "does-not-exist.go"}}
	err := validateContextItems(items)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidateContextItems_SkipsNonFileTypes(t *testing.T) {
	// Symbol and commit types must not trigger filesystem checks.
	items := []conductor.ContextItem{
		{Type: conductor.ContextTypeSymbol, Ref: "DoesNotExist"},
		{Type: conductor.ContextTypeCommit, Ref: "deadbeef"},
	}
	if err := validateContextItems(items); err != nil {
		t.Errorf("non-file items: unexpected %v", err)
	}
}

func TestValidateContextItems_FileWithLineRange(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("//\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	items := []conductor.ContextItem{{Type: conductor.ContextTypeFile, Ref: "x.go:40-50"}}
	if err := validateContextItems(items); err != nil {
		t.Errorf("file with line range: unexpected %v", err)
	}
}
