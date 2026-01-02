//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

// Note: TestCostCommand_BreakdownFlag is in common_test.go

func TestCostCommand_Properties(t *testing.T) {
	if costCmd.Use != "cost" {
		t.Errorf("Use = %q, want %q", costCmd.Use, "cost")
	}

	if costCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if costCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if costCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestCostCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{
			name:         "breakdown flag",
			flagName:     "breakdown",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "all flag",
			flagName:     "all",
			shorthand:    "",
			defaultValue: "false",
		},
		{
			name:         "summary flag",
			flagName:     "summary",
			shorthand:    "s",
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
			flag := costCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)

				return
			}

			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}

			if tt.shorthand != "" {
				shorthand := costCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestCostCommand_ShortDescription(t *testing.T) {
	expected := "Show token usage and costs"
	if costCmd.Short != expected {
		t.Errorf("Short = %q, want %q", costCmd.Short, expected)
	}
}

func TestCostCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"token usage",
		"API costs",
		"input/output tokens",
		"cached tokens",
		"estimated costs",
	}

	for _, substr := range contains {
		if !containsString(costCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestCostCommand_Examples(t *testing.T) {
	examples := []string{
		"mehr cost",
		"--breakdown",
		"--all",
		"--summary",
		"--json",
	}

	for _, example := range examples {
		if !containsString(costCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestCostCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "cost" {
			found = true

			break
		}
	}
	if !found {
		t.Error("cost command not registered in root command")
	}
}

func TestCostCommand_SummaryFlagHasShorthand(t *testing.T) {
	flag := costCmd.Flags().Lookup("summary")
	if flag == nil {
		t.Fatal("summary flag not found")
	}
	if flag.Shorthand != "s" {
		t.Errorf("summary flag shorthand = %q, want 's'", flag.Shorthand)
	}
}

func TestCostCommand_NoAliases(t *testing.T) {
	// Cost command should not have aliases to avoid confusion
	if len(costCmd.Aliases) > 0 {
		t.Errorf("cost command has unexpected aliases: %v", costCmd.Aliases)
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{123, "123"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatNumber(tt.input)
			if result != tt.expected {
				t.Errorf("formatNumber(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "$0.00"},
		{0.001, "$0.0010"},
		{0.009, "$0.0090"},
		{0.01, "$0.01"},
		{0.10, "$0.10"},
		{1.00, "$1.00"},
		{1.23, "$1.23"},
		{12.34, "$12.34"},
		{123.45, "$123.45"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatCost(tt.input)
			if result != tt.expected {
				t.Errorf("formatCost(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatStepName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"planning", "Planning"},
		{"implementing", "Implementing"},
		{"reviewing", "Reviewing"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatStepName(tt.input)
			if result != tt.expected {
				t.Errorf("formatStepName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
