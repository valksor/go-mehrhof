//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

// Note: TestUndoCommand_Properties is in common_test.go

func TestUndoCommand_LongDescription(t *testing.T) {
	if undoCmd.Long == "" {
		t.Error("Long description is empty")
	}
}

func TestUndoCommand_Flags(t *testing.T) {
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
			flag := undoCmd.Flags().Lookup(tt.flagName)
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
				shorthand := undoCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestUndoCommand_ShortDescription(t *testing.T) {
	expected := "Revert to the previous checkpoint"
	if undoCmd.Short != expected {
		t.Errorf("Short = %q, want %q", undoCmd.Short, expected)
	}
}

func TestUndoCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"Revert the current task",
		"previous checkpoint",
		"mehr redo",
	}

	for _, substr := range contains {
		if !containsString(undoCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestUndoCommand_Examples(t *testing.T) {
	// Long description should contain usage examples
	examples := []string{
		"mehr undo",
		"--yes",
	}

	for _, example := range examples {
		if !containsString(undoCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestUndoCommand_RegisteredInRoot(t *testing.T) {
	// Verify undoCmd is a subcommand of rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "undo" {
			found = true

			break
		}
	}
	if !found {
		t.Error("undo command not registered in root command")
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && contains(s, substr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
