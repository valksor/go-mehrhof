package changelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendEntry_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	err := AppendEntry(path, Entry{
		Date:     time.Now(),
		Title:    "Add user authentication",
		TaskID:   "PROJ-123",
		Category: "Added",
	})
	if err != nil {
		t.Fatalf("AppendEntry() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "# Changelog") {
		t.Error("missing changelog header")
	}
	if !strings.Contains(content, "### Added") {
		t.Error("missing Added section")
	}
	if !strings.Contains(content, "Add user authentication (PROJ-123)") {
		t.Error("missing entry text")
	}
}

func TestAppendEntry_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	// Create initial changelog
	_ = AppendEntry(path, Entry{
		Date:     time.Now(),
		Title:    "First feature",
		Category: "Added",
	})

	// Add second entry with different category
	err := AppendEntry(path, Entry{
		Date:     time.Now(),
		Title:    "Fix bug",
		Category: "Fixed",
	})
	if err != nil {
		t.Fatalf("AppendEntry() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "### Added") {
		t.Error("missing Added section")
	}
	if !strings.Contains(content, "### Fixed") {
		t.Error("missing Fixed section")
	}
	if !strings.Contains(content, "Fix bug") {
		t.Error("missing second entry")
	}
}

func TestAppendEntry_SameCategory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	_ = AppendEntry(path, Entry{
		Date:     time.Now(),
		Title:    "First feature",
		Category: "Added",
	})

	err := AppendEntry(path, Entry{
		Date:     time.Now(),
		Title:    "Second feature",
		Category: "Added",
	})
	if err != nil {
		t.Fatalf("AppendEntry() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "First feature") {
		t.Error("missing first entry")
	}
	if !strings.Contains(content, "Second feature") {
		t.Error("missing second entry")
	}
	// Should only have one Added section
	if strings.Count(content, "### Added") != 1 {
		t.Errorf("expected exactly one ### Added section, got %d", strings.Count(content, "### Added"))
	}
}

func TestAppendEntry_WithPRURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	err := AppendEntry(path, Entry{
		Date:     time.Now(),
		Title:    "Add feature",
		TaskID:   "GH-42",
		PRURL:    "https://github.com/org/repo/pull/42",
		Category: "Added",
	})
	if err != nil {
		t.Fatalf("AppendEntry() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "(GH-42)") {
		t.Error("missing task ID")
	}
	if !strings.Contains(content, "[PR](https://github.com/org/repo/pull/42)") {
		t.Error("missing PR URL")
	}
}

func TestCategorize(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Fix login bug", "Fixed"},
		{"Add new feature", "Added"},
		{"Update dependencies", "Changed"},
		{"Remove deprecated API", "Removed"},
		{"Implement payment system", "Added"},
		{"Delete unused code", "Removed"},
		{"Refactor auth module", "Changed"},
		{"Bug in session handling", "Fixed"},
		{"Change password flow", "Changed"},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := Categorize(tt.title)
			if got != tt.want {
				t.Errorf("Categorize(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestAppendEntry_AutoCategorize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	// Category should be auto-detected from title
	err := AppendEntry(path, Entry{
		Date:  time.Now(),
		Title: "Fix broken login",
	})
	if err != nil {
		t.Fatalf("AppendEntry() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "### Fixed") {
		t.Error("expected auto-categorized as Fixed")
	}
}
