//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

func TestListCommand_Properties(t *testing.T) {
	if listCmd.Use != "list" {
		t.Errorf("Use = %q, want %q", listCmd.Use, "list")
	}

	if listCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if listCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if listCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestListCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{
			name:         "worktrees flag",
			flagName:     "worktrees",
			shorthand:    "w",
			defaultValue: "false",
		},
		{
			name:         "json flag",
			flagName:     "json",
			shorthand:    "",
			defaultValue: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := listCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)

				return
			}

			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}

			if tt.shorthand != "" {
				shorthand := listCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestListCommand_ShortDescription(t *testing.T) {
	expected := "List all tasks in workspace"
	if listCmd.Short != expected {
		t.Errorf("Short = %q, want %q", listCmd.Short, expected)
	}
}

func TestListCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"List all tasks",
		"worktree paths",
		"states",
		"parallel tasks",
	}

	for _, substr := range contains {
		if !containsString(listCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestListCommand_Examples(t *testing.T) {
	examples := []string{
		"mehr list",
		"--worktrees",
		"--json",
	}

	for _, example := range examples {
		if !containsString(listCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestListCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "list" {
			found = true

			break
		}
	}
	if !found {
		t.Error("list command not registered in root command")
	}
}

func TestListCommand_WorktreesFlagShorthand(t *testing.T) {
	flag := listCmd.Flags().Lookup("worktrees")
	if flag == nil {
		t.Fatal("worktrees flag not found")
	}
	if flag.Shorthand != "w" {
		t.Errorf("worktrees flag shorthand = %q, want 'w'", flag.Shorthand)
	}
}

func TestListCommand_NoAliases(t *testing.T) {
	// List command doesn't have aliases currently
	if len(listCmd.Aliases) > 0 {
		// If aliases are added in the future, document them here
		t.Logf("Note: list command has aliases: %v", listCmd.Aliases)
	}
}

func TestListCommand_DocumentsWorktrees(t *testing.T) {
	// Should explain worktree functionality
	if !containsString(listCmd.Long, "worktree") {
		t.Error("Long description does not mention worktrees")
	}

	if !containsString(listCmd.Long, "separate terminals") || !containsString(listCmd.Long, "independent") {
		t.Error("Long description does not explain worktree usage")
	}
}
