package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCatalog_ListEmpty(t *testing.T) {
	dir := t.TempDir()
	cat := New(dir)

	templates, err := cat.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(templates) != len(builtinTemplates) {
		t.Fatalf("expected %d built-in templates, got %d", len(builtinTemplates), len(templates))
	}
}

func TestCatalog_ImportAndGet(t *testing.T) {
	catalogDir := t.TempDir()
	cat := New(catalogDir)

	// Create a source template file.
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "bugfix.yaml")

	content := `name: bugfix
description: Fix a bug from an issue
source: github
agent: claude
tags:
  - bug
  - fix
`

	if err := os.WriteFile(srcPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := cat.Import(srcPath); err != nil {
		t.Fatalf("import: %v", err)
	}

	tmpl, err := cat.Get("bugfix")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if tmpl.Name != "bugfix" {
		t.Errorf("expected name 'bugfix', got %q", tmpl.Name)
	}

	if tmpl.Description != "Fix a bug from an issue" {
		t.Errorf("unexpected description: %q", tmpl.Description)
	}

	if tmpl.Source != "github" {
		t.Errorf("expected source 'github', got %q", tmpl.Source)
	}

	if len(tmpl.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tmpl.Tags))
	}
}

func TestCatalog_ListMultiple(t *testing.T) {
	dir := t.TempDir()
	cat := New(dir)

	templates := []struct {
		name    string
		content string
	}{
		{"feature.yaml", "name: feature\ndescription: New feature\nsource: github\n"},
		{"bugfix.yaml", "name: bugfix\ndescription: Bug fix\nsource: linear\n"},
		{"refactor.yaml", "name: refactor\ndescription: Code cleanup\nsource: file\n"},
	}

	for _, tmpl := range templates {
		path := filepath.Join(dir, tmpl.name)
		if err := os.WriteFile(path, []byte(tmpl.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", tmpl.name, err)
		}
	}

	result, err := cat.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	// 3 built-in templates (bug-fix, feature, refactor) + 3 user templates.
	// "feature" and "refactor" override built-ins, "bugfix" is new.
	// Total: 3 built-in + 1 new = 4.
	if len(result) != 4 {
		t.Fatalf("expected 4 templates (3 built-in + 1 new), got %d", len(result))
	}
}
