//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

func TestAbandonCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		description  string
		defaultValue bool
	}{
		{
			name:         "yes flag",
			flagName:     "yes",
			shorthand:    "y",
			defaultValue: false,
			description:  "Skip confirmation prompt",
		},
		{
			name:         "keep-branch flag",
			flagName:     "keep-branch",
			defaultValue: false,
			description:  "Keep the git branch",
		},
		{
			name:         "keep-work flag",
			flagName:     "keep-work",
			defaultValue: false,
			description:  "Keep the work directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := abandonCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)

				return
			}

			// Check default value
			if flag.DefValue != "false" {
				t.Errorf("flag %q default value = %q, want false", tt.flagName, flag.DefValue)
			}

			// Check shorthand if specified
			if tt.shorthand != "" {
				shorthand := abandonCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestAbandonCommand_Properties(t *testing.T) {
	// Check command is properly configured
	if abandonCmd.Use != "abandon" {
		t.Errorf("Use = %q, want %q", abandonCmd.Use, "abandon")
	}

	if abandonCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if abandonCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if abandonCmd.RunE == nil {
		t.Error("RunE not set")
	}
}
