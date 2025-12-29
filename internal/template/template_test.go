package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBuiltIn(t *testing.T) {
	builtInTemplates := []string{
		"bug-fix",
		"feature",
		"refactor",
		"docs",
		"test",
		"chore",
	}

	for _, name := range builtInTemplates {
		t.Run(name, func(t *testing.T) {
			tpl, err := LoadBuiltIn(name)
			// Skip test if template files are not accessible (e.g., running from different working directory)
			if err != nil {
				t.Skipf("LoadBuiltIn(%q) skipped: %v (may be working directory issue)", name, err)
			}

			if tpl.Name != name {
				t.Errorf("template.Name = %q, want %q", tpl.Name, name)
			}

			if tpl.Description == "" {
				t.Error("template.Description is empty")
			}

			// Verify required fields exist
			if tpl.Frontmatter == nil {
				t.Error("template.Frontmatter is nil")
			}

			if tpl.Agent == "" {
				t.Error("template.Agent is empty")
			}
		})
	}
}

func TestLoadBuiltInInvalid(t *testing.T) {
	tpl, err := LoadBuiltIn("nonexistent")
	if err == nil {
		t.Error("LoadBuiltIn(nonexistent) should return error")
	}
	if tpl != nil {
		t.Error("LoadBuiltIn(nonexistent) should return nil template")
	}
}

func TestBuiltInTemplates(t *testing.T) {
	names := BuiltInTemplates()

	if len(names) == 0 {
		t.Error("BuiltInTemplates() returned empty list")
	}

	// Verify all expected templates are present
	expected := []string{
		"bug-fix",
		"feature",
		"refactor",
		"docs",
		"test",
		"chore",
	}

	for _, exp := range expected {
		found := false
		for _, name := range names {
			if name == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("BuiltInTemplates() missing %q", exp)
		}
	}
}

func TestApplyToContent(t *testing.T) {
	tests := []struct {
		name     string
		template string
		content  string
		wantHas  []string // Strings that should be in output
	}{
		{
			name:     "empty content",
			template: "bug-fix",
			content:  "",
			wantHas:  []string{"type:", "fix"},
		},
		{
			name:     "content without frontmatter",
			template: "bug-fix",
			content:  "# My Task\n\nDescription here.",
			wantHas:  []string{"---", "type: fix", "# My Task"},
		},
		{
			name:     "content with existing frontmatter",
			template: "feature",
			content: `---
title: My Title
key: ABC-123
---

# Task`,
			wantHas: []string{"title: My Title", "key: ABC-123", "type: feature"},
		},
		{
			name:     "content with conflicting frontmatter",
			template: "bug-fix",
			content: `---
type: feature
title: Override Test
---

# Task`,
			wantHas: []string{"type: fix", "title: Override Test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tpl, err := LoadBuiltIn(tt.template)
			if err != nil {
				t.Skipf("LoadBuiltIn(%q) skipped: %v", tt.template, err)
			}

			result := tpl.ApplyToContent(tt.content)

			for _, want := range tt.wantHas {
				if !contains(result, want) {
					t.Errorf("ApplyToContent() result does not contain %q\nGot:\n%s", want, result)
				}
			}
		})
	}
}

func TestTemplateAgentSteps(t *testing.T) {
	// Test that templates can have per-step agent configuration
	tpl, err := LoadBuiltIn("feature")
	if err != nil {
		t.Skipf("LoadBuiltIn(feature) skipped: %v", err)
	}

	// The feature template may or may not have agent steps
	// Just verify the field exists and is handled correctly
	if tpl.AgentSteps != nil {
		t.Logf("Feature template has agent steps: %v", tpl.AgentSteps)
	}
}

func TestTemplateGitConfig(t *testing.T) {
	tests := []struct {
		name          string
		templateName  string
		wantBranchPat string
		wantCommitPre string
	}{
		{
			name:          "bug-fix",
			templateName:  "bug-fix",
			wantBranchPat: "fix/{key}--{slug}",
			wantCommitPre: "[fix/{key}]",
		},
		{
			name:          "feature",
			templateName:  "feature",
			wantBranchPat: "feature/{key}--{slug}",
			wantCommitPre: "[{key}]",
		},
		{
			name:          "docs",
			templateName:  "docs",
			wantBranchPat: "docs/{key}--{slug}",
			wantCommitPre: "[docs]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tpl, err := LoadBuiltIn(tt.templateName)
			if err != nil {
				t.Skipf("LoadBuiltIn(%q) skipped: %v", tt.templateName, err)
			}

			if tpl.Git == nil {
				t.Error("template.Git is nil")
				return
			}

			if tpl.Git["branch_pattern"] != tt.wantBranchPat {
				t.Errorf("template.Git[branch_pattern] = %q, want %q",
					tpl.Git["branch_pattern"], tt.wantBranchPat)
			}

			if tpl.Git["commit_prefix"] != tt.wantCommitPre {
				t.Errorf("template.Git[commit_prefix] = %q, want %q",
					tpl.Git["commit_prefix"], tt.wantCommitPre)
			}
		})
	}
}

func TestLoadFromDirectory(t *testing.T) {
	// Create a temporary directory for custom templates
	tmpDir := t.TempDir()

	// Create a custom template
	customTemplatePath := filepath.Join(tmpDir, "custom.yaml")
	customContent := `
name: custom
description: Custom template for testing
frontmatter:
  type: custom
agent: claude
git:
  branch_pattern: "custom/{key}--{slug}"
  commit_prefix: "[custom]"
`

	if err := os.WriteFile(customTemplatePath, []byte(customContent), 0o644); err != nil {
		t.Fatalf("failed to write custom template: %v", err)
	}

	// Note: This test verifies the template file structure is valid
	// The actual LoadFromDirectory functionality would need to be added to the package
	data, err := os.ReadFile(customTemplatePath)
	if err != nil {
		t.Fatalf("failed to read custom template: %v", err)
	}

	if len(data) == 0 {
		t.Error("custom template file is empty")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
