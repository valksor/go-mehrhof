//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

func TestContinueCommand_Properties(t *testing.T) {
	if continueCmd.Use != "continue" {
		t.Errorf("Use = %q, want %q", continueCmd.Use, "continue")
	}

	if continueCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if continueCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if continueCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestContinueCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{
			name:         "auto flag",
			flagName:     "auto",
			shorthand:    "",
			defaultValue: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := continueCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)
				return
			}

			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}

			if tt.shorthand != "" {
				shorthand := continueCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestContinueCommand_ShortDescription(t *testing.T) {
	expected := "Resume workflow, optionally auto-execute (aliases: cont, c)"
	if continueCmd.Short != expected {
		t.Errorf("Short = %q, want %q", continueCmd.Short, expected)
	}
}

func TestContinueCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"auto-pilot",
		"CHOOSING THE RIGHT COMMAND",
		"AUTO-EXECUTION LOGIC",
	}

	for _, substr := range contains {
		if !containsString(continueCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestContinueCommand_DocumentsWhenToUse(t *testing.T) {
	// Should document when to use guide, status, continue
	comparisons := []string{
		"guide",
		"status",
		"continue",
	}

	for _, comp := range comparisons {
		if !containsString(continueCmd.Long, comp) {
			t.Errorf("Long description does not document comparison with %q", comp)
		}
	}
}

func TestContinueCommand_HasAliases(t *testing.T) {
	if len(continueCmd.Aliases) == 0 {
		t.Error("continue command has no aliases")
	}

	expected := []string{"cont", "c"}
	for _, exp := range expected {
		found := false
		for _, alias := range continueCmd.Aliases {
			if alias == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("continue command missing %q alias", exp)
		}
	}
}

func TestContinueCommand_Examples(t *testing.T) {
	examples := []string{
		"mehr continue",
		"--auto",
	}

	for _, example := range examples {
		if !containsString(continueCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestContinueCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "continue" {
			found = true
			break
		}
	}
	if !found {
		t.Error("continue command not registered in root command")
	}
}

func TestContinueCommand_DocumentsSeeAlso(t *testing.T) {
	// Should reference related commands in CHOOSING THE RIGHT COMMAND section
	if !containsString(continueCmd.Long, "guide") {
		t.Error("Long description does not reference 'guide'")
	}

	if !containsString(continueCmd.Long, "status") {
		t.Error("Long description does not reference 'status'")
	}
}
