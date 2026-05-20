package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew_DefaultDir(t *testing.T) {
	// Empty dir should fall back to a non-empty default path.
	c := New("")
	if c.dir == "" {
		t.Error("New(\"\").dir should not be empty")
	}
}

func TestCatalog_Get_BuiltinFallback(t *testing.T) {
	dir := t.TempDir()
	cat := New(dir)

	tmpl, err := cat.Get("feature")
	if err != nil {
		t.Fatalf("Get builtin: %v", err)
	}
	if tmpl == nil || tmpl.Name != "feature" {
		t.Errorf("expected builtin 'feature', got %+v", tmpl)
	}
}

func TestCatalog_Get_UserOverridesBuiltin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "feature.yaml"), []byte("name: feature\ndescription: custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := New(dir)

	tmpl, err := cat.Get("feature")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tmpl.Description != "custom" {
		t.Errorf("expected user override, got %q", tmpl.Description)
	}
}

func TestCatalog_Get_NotFound(t *testing.T) {
	dir := t.TempDir()
	cat := New(dir)

	if _, err := cat.Get("does-not-exist"); err == nil {
		t.Error("expected error for missing template")
	}
}

func TestCatalog_Import_InvalidYAML(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "bad.yaml")
	if err := os.WriteFile(src, []byte("not: valid: yaml: :::"), 0o644); err != nil {
		t.Fatal(err)
	}

	cat := New(t.TempDir())
	if err := cat.Import(src); err == nil {
		t.Error("expected error for invalid yaml")
	}
}

func TestCatalog_Import_MissingName(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "noname.yaml")
	if err := os.WriteFile(src, []byte("description: missing name field\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cat := New(t.TempDir())
	if err := cat.Import(src); err == nil {
		t.Error("expected error when name field missing")
	}
}

func TestCatalog_Import_NonExistentSource(t *testing.T) {
	cat := New(t.TempDir())
	if err := cat.Import("/does/not/exist.yaml"); err == nil {
		t.Error("expected error for missing source file")
	}
}

func TestCatalog_List_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	cat := New(dir)
	templates, err := cat.List()
	if err != nil {
		t.Fatal(err)
	}

	// Only builtin templates should appear (no user yaml files).
	if len(templates) != len(builtinTemplates) {
		t.Errorf("List = %d entries, want %d (builtins only)", len(templates), len(builtinTemplates))
	}
}
