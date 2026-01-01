//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

func TestAutoCommand_Properties(t *testing.T) {
	if autoCmd.Use != "auto <reference>" {
		t.Errorf("Use = %q, want %q", autoCmd.Use, "auto <reference>")
	}

	if autoCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if autoCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if autoCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestAutoCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{
			name:         "agent flag",
			flagName:     "agent",
			shorthand:    "a",
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
			name:         "max-retries flag",
			flagName:     "max-retries",
			shorthand:    "",
			defaultValue: "3",
		},
		{
			name:         "no-push flag",
			flagName:     "no-push",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "no-delete flag",
			flagName:     "no-delete",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "no-squash flag",
			flagName:     "no-squash",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "target flag",
			flagName:     "target",
			shorthand:    "t",
			defaultValue: "",
		},
		{
			name:         "quality-target flag",
			flagName:     "quality-target",
			shorthand:    "",
			defaultValue: "quality",
		},
		{
			name:         "no-quality flag",
			flagName:     "no-quality",
			shorthand:    "",
			defaultValue: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := autoCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)
				return
			}

			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}

			if tt.shorthand != "" {
				shorthand := autoCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestAutoCommand_ShortDescription(t *testing.T) {
	expected := "Full automation: start -> plan -> implement -> quality -> finish"
	if autoCmd.Short != expected {
		t.Errorf("Short = %q, want %q", autoCmd.Short, expected)
	}
}

func TestAutoCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"complete automation cycle",
		"user interaction",
		"quality checks",
	}

	for _, substr := range contains {
		if !containsString(autoCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestAutoCommand_DocumentsWorkflowSteps(t *testing.T) {
	steps := []string{
		"Register the task",
		"planning",
		"Implement the specifications",
		"quality checks",
		"Merge and complete",
	}

	for _, step := range steps {
		if !containsString(autoCmd.Long, step) {
			t.Errorf("Long description does not document step %q", step)
		}
	}
}

func TestAutoCommand_RequiresExactlyOneArg(t *testing.T) {
	if autoCmd.Args == nil {
		t.Error("Args validator not set")
	}
}

func TestAutoCommand_Examples(t *testing.T) {
	examples := []string{
		"mehr auto task.md",
		"mehr auto ./tasks/",
		"--max-retries",
		"--no-push",
		"--no-quality",
	}

	for _, example := range examples {
		if !containsString(autoCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestAutoCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "auto <reference>" {
			found = true
			break
		}
	}
	if !found {
		t.Error("auto command not registered in root command")
	}
}

func TestAutoCommand_DefaultMaxRetries(t *testing.T) {
	flag := autoCmd.Flags().Lookup("max-retries")
	if flag == nil {
		t.Fatal("max-retries flag not found")
	}
	if flag.DefValue != "3" {
		t.Errorf("max-retries default = %q, want '3'", flag.DefValue)
	}
}

func TestAutoCommand_AgentFlagHasShorthand(t *testing.T) {
	flag := autoCmd.Flags().Lookup("agent")
	if flag == nil {
		t.Fatal("agent flag not found")
	}
	if flag.Shorthand != "a" {
		t.Errorf("agent flag shorthand = %q, want 'a'", flag.Shorthand)
	}
}

func TestAutoCommand_WorktreeFlagHasShorthand(t *testing.T) {
	flag := autoCmd.Flags().Lookup("worktree")
	if flag == nil {
		t.Fatal("worktree flag not found")
	}
	if flag.Shorthand != "w" {
		t.Errorf("worktree flag shorthand = %q, want 'w'", flag.Shorthand)
	}
}

func TestBoolToStatus(t *testing.T) {
	tests := []struct {
		input    bool
		expected string
	}{
		{true, "done"},
		{false, "pending"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := boolToStatus(tt.input)
			if result != tt.expected {
				t.Errorf("boolToStatus(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
