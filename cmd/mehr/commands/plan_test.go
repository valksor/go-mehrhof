//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

// Note: TestPlanCommand_Aliases and TestPlanCommand_StandaloneFlag are in common_test.go

func TestPlanCommand_Properties(t *testing.T) {
	if planCmd.Use != "plan [seed-topic]" {
		t.Errorf("Use = %q, want %q", planCmd.Use, "plan [seed-topic]")
	}

	if planCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if planCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if planCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestPlanCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{
			name:         "standalone flag",
			flagName:     "standalone",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "seed flag",
			flagName:     "seed",
			shorthand:    "s",
			defaultValue: "",
		},
		{
			name:         "full-context flag",
			flagName:     "full-context",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "agent-plan flag",
			flagName:     "agent-plan",
			shorthand:    "",
			defaultValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := planCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)
				return
			}

			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}

			if tt.shorthand != "" {
				shorthand := planCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestPlanCommand_ShortDescription(t *testing.T) {
	expected := "Create implementation specifications for the active task"
	if planCmd.Short != expected {
		t.Errorf("Short = %q, want %q", planCmd.Short, expected)
	}
}

func TestPlanCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"planning phase",
		"specification files",
		"work directory",
	}

	for _, substr := range contains {
		if !containsString(planCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestPlanCommand_DocumentsStandaloneMode(t *testing.T) {
	if !containsString(planCmd.Long, "STANDALONE MODE") {
		t.Error("Long description does not document STANDALONE MODE section")
	}

	if !containsString(planCmd.Long, "--standalone") {
		t.Error("Long description does not mention --standalone flag")
	}
}

func TestPlanCommand_DocumentsSeedTopic(t *testing.T) {
	if !containsString(planCmd.Long, "SEED TOPIC") {
		t.Error("Long description does not document SEED TOPIC section")
	}

	if !containsString(planCmd.Long, "--seed") {
		t.Error("Long description does not mention --seed flag")
	}
}

func TestPlanCommand_HasAliases(t *testing.T) {
	if len(planCmd.Aliases) == 0 {
		t.Error("plan command has no aliases")
	}

	found := false
	for _, alias := range planCmd.Aliases {
		if alias == "p" {
			found = true
			break
		}
	}
	if !found {
		t.Error("plan command missing 'p' alias")
	}
}

func TestPlanCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "plan [seed-topic]" {
			found = true
			break
		}
	}
	if !found {
		t.Error("plan command not registered in root command")
	}
}

func TestPlanCommand_Examples(t *testing.T) {
	examples := []string{
		"mehr plan",
		"--verbose",
		"--standalone",
		"--full-context",
	}

	for _, example := range examples {
		if !containsString(planCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}
