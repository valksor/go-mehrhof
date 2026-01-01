//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

// Note: TestRedoCommand_Properties is in common_test.go

func TestRedoCommand_LongDescription(t *testing.T) {
	if redoCmd.Long == "" {
		t.Error("Long description is empty")
	}
}

func TestRedoCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{
			name:         "yes flag",
			flagName:     "yes",
			shorthand:    "y",
			defaultValue: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := redoCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)
				return
			}

			// Check default value
			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}

			// Check shorthand if specified
			if tt.shorthand != "" {
				shorthand := redoCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestRedoCommand_ShortDescription(t *testing.T) {
	expected := "Restore the next checkpoint"
	if redoCmd.Short != expected {
		t.Errorf("Short = %q, want %q", redoCmd.Short, expected)
	}
}

func TestRedoCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"Restore the current task",
		"next checkpoint",
		"previously undone",
	}

	for _, substr := range contains {
		if !containsString(redoCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestRedoCommand_Examples(t *testing.T) {
	// Long description should contain usage examples
	examples := []string{
		"mehr redo",
		"--yes",
	}

	for _, example := range examples {
		if !containsString(redoCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestRedoCommand_RegisteredInRoot(t *testing.T) {
	// Verify redoCmd is a subcommand of rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "redo" {
			found = true
			break
		}
	}
	if !found {
		t.Error("redo command not registered in root command")
	}
}
