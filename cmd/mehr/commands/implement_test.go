//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

// Note: TestImplementCommand_Aliases is in common_test.go

func TestImplementCommand_Properties(t *testing.T) {
	if implementCmd.Use != "implement" {
		t.Errorf("Use = %q, want %q", implementCmd.Use, "implement")
	}

	if implementCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if implementCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if implementCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestImplementCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{
			name:         "dry-run flag",
			flagName:     "dry-run",
			shorthand:    "n",
			defaultValue: "false",
		},
		{
			name:         "agent-implement flag",
			flagName:     "agent-implement",
			shorthand:    "",
			defaultValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := implementCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)
				return
			}

			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}

			if tt.shorthand != "" {
				shorthand := implementCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestImplementCommand_ShortDescription(t *testing.T) {
	expected := "Implement the specifications for the active task"
	if implementCmd.Short != expected {
		t.Errorf("Short = %q, want %q", implementCmd.Short, expected)
	}
}

func TestImplementCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"implementation phase",
		"generate code",
		"specifications",
		"mehr plan",
	}

	for _, substr := range contains {
		if !containsString(implementCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestImplementCommand_HasAliases(t *testing.T) {
	if len(implementCmd.Aliases) == 0 {
		t.Error("implement command has no aliases")
	}

	expected := []string{"impl", "i"}
	for _, exp := range expected {
		found := false
		for _, alias := range implementCmd.Aliases {
			if alias == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("implement command missing %q alias", exp)
		}
	}
}

func TestImplementCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "implement" {
			found = true
			break
		}
	}
	if !found {
		t.Error("implement command not registered in root command")
	}
}

func TestImplementCommand_Examples(t *testing.T) {
	examples := []string{
		"mehr implement",
		"--dry-run",
		"--verbose",
	}

	for _, example := range examples {
		if !containsString(implementCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestImplementCommand_DryRunHasShorthand(t *testing.T) {
	flag := implementCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Fatal("dry-run flag not found")
	}
	if flag.Shorthand != "n" {
		t.Errorf("dry-run flag shorthand = %q, want 'n'", flag.Shorthand)
	}
}
