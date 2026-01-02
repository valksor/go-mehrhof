//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

func TestFinishCommand_Properties(t *testing.T) {
	if finishCmd.Use != "finish" {
		t.Errorf("Use = %q, want %q", finishCmd.Use, "finish")
	}

	if finishCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if finishCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if finishCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestFinishCommand_Flags(t *testing.T) {
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
		{
			name:         "merge flag",
			flagName:     "merge",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "delete flag",
			flagName:     "delete",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "push flag",
			flagName:     "push",
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
			name:         "skip-quality flag",
			flagName:     "skip-quality",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "quality-target flag",
			flagName:     "quality-target",
			shorthand:    "",
			defaultValue: "quality",
		},
		{
			name:         "delete-work flag",
			flagName:     "delete-work",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "draft flag",
			flagName:     "draft",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "pr-title flag",
			flagName:     "pr-title",
			shorthand:    "",
			defaultValue: "",
		},
		{
			name:         "pr-body flag",
			flagName:     "pr-body",
			shorthand:    "",
			defaultValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := finishCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)

				return
			}

			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}

			if tt.shorthand != "" {
				shorthand := finishCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestFinishCommand_ShortDescription(t *testing.T) {
	expected := "Complete the task (creates PR by default for supported providers)"
	if finishCmd.Short != expected {
		t.Errorf("Short = %q, want %q", finishCmd.Short, expected)
	}
}

func TestFinishCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"Complete the current task",
		"pull request",
		"merge",
		"PROVIDER BEHAVIOR",
	}

	for _, substr := range contains {
		if !containsString(finishCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestFinishCommand_DocumentsProviderBehaviors(t *testing.T) {
	providers := []string{
		"github:",
		"gitlab:",
		"file:, dir:",
		"jira:",
	}

	for _, provider := range providers {
		if !containsString(finishCmd.Long, provider) {
			t.Errorf("Long description does not document provider %q", provider)
		}
	}
}

func TestFinishCommand_DocumentsFlagCombinations(t *testing.T) {
	if !containsString(finishCmd.Long, "FLAG COMBINATIONS") {
		t.Error("Long description does not document FLAG COMBINATIONS section")
	}

	if !containsString(finishCmd.Long, "PR mode") {
		t.Error("Long description does not mention PR mode")
	}

	if !containsString(finishCmd.Long, "Merge mode") {
		t.Error("Long description does not mention Merge mode")
	}
}

func TestFinishCommand_HasAliases(t *testing.T) {
	if len(finishCmd.Aliases) == 0 {
		t.Error("finish command has no aliases")
	}

	expected := []string{"fi", "done"}
	for _, exp := range expected {
		found := false
		for _, alias := range finishCmd.Aliases {
			if alias == exp {
				found = true

				break
			}
		}
		if !found {
			t.Errorf("finish command missing %q alias", exp)
		}
	}
}

func TestFinishCommand_Examples(t *testing.T) {
	examples := []string{
		"mehr finish",
		"--yes",
		"--merge",
		"--delete",
		"--push",
		"--no-squash",
		"--target",
		"--skip-quality",
		"--draft",
		"--pr-title",
		"--delete-work",
	}

	for _, example := range examples {
		if !containsString(finishCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestFinishCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "finish" {
			found = true

			break
		}
	}
	if !found {
		t.Error("finish command not registered in root command")
	}
}

func TestFinishCommand_YesFlagHasShorthand(t *testing.T) {
	flag := finishCmd.Flags().Lookup("yes")
	if flag == nil {
		t.Fatal("yes flag not found")
	}
	if flag.Shorthand != "y" {
		t.Errorf("yes flag shorthand = %q, want 'y'", flag.Shorthand)
	}
}

func TestFinishCommand_TargetFlagHasShorthand(t *testing.T) {
	flag := finishCmd.Flags().Lookup("target")
	if flag == nil {
		t.Fatal("target flag not found")
	}
	if flag.Shorthand != "t" {
		t.Errorf("target flag shorthand = %q, want 't'", flag.Shorthand)
	}
}

func TestFinishCommand_QualityTargetDefault(t *testing.T) {
	flag := finishCmd.Flags().Lookup("quality-target")
	if flag == nil {
		t.Fatal("quality-target flag not found")
	}
	if flag.DefValue != "quality" {
		t.Errorf("quality-target default = %q, want 'quality'", flag.DefValue)
	}
}
