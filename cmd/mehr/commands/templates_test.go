//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

func TestTemplatesCommand_Properties(t *testing.T) {
	if templatesCmd.Use != "templates" {
		t.Errorf("Use = %q, want %q", templatesCmd.Use, "templates")
	}

	if templatesCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if templatesCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if templatesCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestTemplateShowCommand_Properties(t *testing.T) {
	if templateShowCmd.Use != "show <name>" {
		t.Errorf("Use = %q, want %q", templateShowCmd.Use, "show <name>")
	}

	if templateShowCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if templateShowCmd.RunE == nil {
		t.Error("RunE not set")
	}

	if templateShowCmd.Args == nil {
		t.Error("Args validator not set")
	}
}

func TestTemplateApplyCommand_Properties(t *testing.T) {
	if templateApplyCmd.Use != "apply <name> <file>" {
		t.Errorf("Use = %q, want %q", templateApplyCmd.Use, "apply <name> <file>")
	}

	if templateApplyCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if templateApplyCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if templateApplyCmd.RunE == nil {
		t.Error("RunE not set")
	}

	if templateApplyCmd.Args == nil {
		t.Error("Args validator not set")
	}
}

func TestTemplatesCommand_ShortDescription(t *testing.T) {
	expected := "Manage task templates"
	if templatesCmd.Short != expected {
		t.Errorf("Short = %q, want %q", templatesCmd.Short, expected)
	}
}

func TestTemplatesCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"templates",
		"frontmatter",
		"workflow settings",
	}

	for _, substr := range contains {
		if !containsString(templatesCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestTemplatesCommand_DocumentsBuiltInTemplates(t *testing.T) {
	templates := []string{
		"bug-fix",
		"feature",
		"refactor",
		"docs",
		"test",
		"chore",
	}

	for _, tpl := range templates {
		if !containsString(templatesCmd.Long, tpl) {
			t.Errorf("Long description does not document template %q", tpl)
		}
	}
}

func TestTemplatesCommand_HasSubcommands(t *testing.T) {
	subcommands := templatesCmd.Commands()
	if len(subcommands) < 2 {
		t.Errorf("templates command has %d subcommands, want at least 2", len(subcommands))
	}

	expectedSubcommands := []string{"show <name>", "apply <name> <file>"}
	for _, exp := range expectedSubcommands {
		found := false
		for _, cmd := range subcommands {
			if cmd.Use == exp {
				found = true

				break
			}
		}
		if !found {
			t.Errorf("templates command missing subcommand %q", exp)
		}
	}
}

func TestTemplatesCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "templates" {
			found = true

			break
		}
	}
	if !found {
		t.Error("templates command not registered in root command")
	}
}

func TestTemplateApplyCommand_Examples(t *testing.T) {
	examples := []string{
		"mehr templates apply bug-fix task.md",
		"mehr templates apply feature",
	}

	for _, example := range examples {
		if !containsString(templateApplyCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestTemplateApplyCommand_DocumentsMerge(t *testing.T) {
	if !containsString(templateApplyCmd.Long, "merge") {
		t.Error("Long description does not mention merging with existing frontmatter")
	}
}
