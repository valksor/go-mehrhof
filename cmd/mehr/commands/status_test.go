//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

// Note: TestStatusCommand_Aliases is in common_test.go

func TestStatusCommand_Properties(t *testing.T) {
	if statusCmd.Use != "status" {
		t.Errorf("Use = %q, want %q", statusCmd.Use, "status")
	}

	if statusCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if statusCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if statusCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestStatusCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{
			name:         "all flag",
			flagName:     "all",
			shorthand:    "a",
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
			flag := statusCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)

				return
			}

			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}

			if tt.shorthand != "" {
				shorthand := statusCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestStatusCommand_ShortDescription(t *testing.T) {
	expected := "Show full task details"
	if statusCmd.Short != expected {
		t.Errorf("Short = %q, want %q", statusCmd.Short, expected)
	}
}

func TestStatusCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"Comprehensive view",
		"Task metadata",
		"Specifications",
		"Git checkpoints",
		"Session history",
	}

	for _, substr := range contains {
		if !containsString(statusCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestStatusCommand_WhenToUseSection(t *testing.T) {
	// Should document when to use status vs guide vs continue
	expected := []string{
		"CHOOSING THE RIGHT COMMAND",
		"guide",
		"status",
		"continue",
	}

	for _, s := range expected {
		if !containsString(statusCmd.Long, s) {
			t.Errorf("Long description does not contain %q", s)
		}
	}
}

func TestStatusCommand_OutputFormats(t *testing.T) {
	// Should document output formats
	if !containsString(statusCmd.Long, "OUTPUT FORMATS") {
		t.Error("Long description does not document OUTPUT FORMATS section")
	}

	if !containsString(statusCmd.Long, "--json") {
		t.Error("Long description does not mention --json flag")
	}
}

func TestStatusCommand_Examples(t *testing.T) {
	examples := []string{
		"mehr status",
		"--all",
		"--json",
	}

	for _, example := range examples {
		if !containsString(statusCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestStatusCommand_SeeAlsoSection(t *testing.T) {
	// Should reference related commands (in CHOOSING THE RIGHT COMMAND section)
	related := []string{
		"guide",
		"continue",
	}

	for _, cmd := range related {
		if !containsString(statusCmd.Long, cmd) {
			t.Errorf("Long description does not reference %q", cmd)
		}
	}
}

func TestStatusCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "status" {
			found = true

			break
		}
	}
	if !found {
		t.Error("status command not registered in root command")
	}
}

func TestStatusCommand_HasAliases(t *testing.T) {
	if len(statusCmd.Aliases) == 0 {
		t.Error("status command has no aliases")
	}

	// Should have "st" alias
	found := false
	for _, alias := range statusCmd.Aliases {
		if alias == "st" {
			found = true

			break
		}
	}
	if !found {
		t.Error("status command missing 'st' alias")
	}
}

func TestStatusCommand_JSONFlagNoShorthand(t *testing.T) {
	// JSON flag should not have a shorthand to avoid conflicts
	flag := statusCmd.Flags().Lookup("json")
	if flag == nil {
		t.Fatal("json flag not found")
	}
	if flag.Shorthand != "" {
		t.Errorf("json flag has shorthand %q, expected none", flag.Shorthand)
	}
}
