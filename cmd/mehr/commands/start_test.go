//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

// Note: TestStartCommand_AgentFlagShorthand is in common_test.go

func TestStartCommand_Properties(t *testing.T) {
	if startCmd.Use != "start <reference>" {
		t.Errorf("Use = %q, want %q", startCmd.Use, "start <reference>")
	}

	if startCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if startCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if startCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestStartCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{
			name:         "agent flag",
			flagName:     "agent",
			shorthand:    "A",
			defaultValue: "",
		},
		{
			name:         "no-branch flag",
			flagName:     "no-branch",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "worktree flag",
			flagName:     "worktree",
			shorthand:    "w",
			defaultValue: "false",
		},
		{
			name:         "key flag",
			flagName:     "key",
			shorthand:    "k",
			defaultValue: "",
		},
		{
			name:         "title flag",
			flagName:     "title",
			shorthand:    "",
			defaultValue: "",
		},
		{
			name:         "slug flag",
			flagName:     "slug",
			shorthand:    "",
			defaultValue: "",
		},
		{
			name:         "commit-prefix flag",
			flagName:     "commit-prefix",
			shorthand:    "",
			defaultValue: "",
		},
		{
			name:         "branch-pattern flag",
			flagName:     "branch-pattern",
			shorthand:    "",
			defaultValue: "",
		},
		{
			name:         "template flag",
			flagName:     "template",
			shorthand:    "",
			defaultValue: "",
		},
		{
			name:         "agent-plan flag",
			flagName:     "agent-plan",
			shorthand:    "",
			defaultValue: "",
		},
		{
			name:         "agent-implement flag",
			flagName:     "agent-implement",
			shorthand:    "",
			defaultValue: "",
		},
		{
			name:         "agent-review flag",
			flagName:     "agent-review",
			shorthand:    "",
			defaultValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := startCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)

				return
			}

			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}

			if tt.shorthand != "" {
				shorthand := startCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestStartCommand_ShortDescription(t *testing.T) {
	expected := "Start a new task from a file, directory, or provider"
	if startCmd.Short != expected {
		t.Errorf("Short = %q, want %q", startCmd.Short, expected)
	}
}

func TestStartCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"Start a new task",
		"git branch",
		"mehr plan",
	}

	for _, substr := range contains {
		if !containsString(startCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestStartCommand_DocumentsProviders(t *testing.T) {
	providers := []string{
		"file:",
		"dir:",
		"github:",
		"notion:",
		"jira:",
		"linear:",
	}

	for _, provider := range providers {
		if !containsString(startCmd.Long, provider) {
			t.Errorf("Long description does not document provider %q", provider)
		}
	}
}

func TestStartCommand_DocumentsAgentSelection(t *testing.T) {
	if !containsString(startCmd.Long, "AGENT SELECTION") {
		t.Error("Long description does not document AGENT SELECTION section")
	}
}

func TestStartCommand_DocumentsTemplates(t *testing.T) {
	if !containsString(startCmd.Long, "TEMPLATES") {
		t.Error("Long description does not document TEMPLATES section")
	}

	templates := []string{"bug-fix", "feature", "refactor"}
	for _, tpl := range templates {
		if !containsString(startCmd.Long, tpl) {
			t.Errorf("Long description does not mention template %q", tpl)
		}
	}
}

func TestStartCommand_RequiresExactlyOneArg(t *testing.T) {
	// Should require exactly one argument
	if startCmd.Args == nil {
		t.Error("Args validator not set")
	}
}

func TestStartCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "start <reference>" {
			found = true

			break
		}
	}
	if !found {
		t.Error("start command not registered in root command")
	}
}

func TestStartCommand_Examples(t *testing.T) {
	examples := []string{
		"mehr start file:task.md",
		"mehr start dir:./tasks/",
		"--no-branch",
		"--worktree",
		"--template",
	}

	for _, example := range examples {
		if !containsString(startCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}
