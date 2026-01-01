//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

// Note: TestConfigCommand_Structure is in common_test.go

func TestConfigCommand_LongDescription(t *testing.T) {
	if configCmd.Long == "" {
		t.Error("Long description is empty")
	}
}

func TestConfigCommand_ShortDescription(t *testing.T) {
	expected := "Manage configuration"
	if configCmd.Short != expected {
		t.Errorf("Short = %q, want %q", configCmd.Short, expected)
	}
}

func TestConfigValidateCommand_Properties(t *testing.T) {
	// Check validate subcommand is properly configured
	if configValidateCmd.Use != "validate" {
		t.Errorf("Use = %q, want %q", configValidateCmd.Use, "validate")
	}

	if configValidateCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if configValidateCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if configValidateCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestConfigValidateCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		defaultValue string
	}{
		{
			name:         "strict flag",
			flagName:     "strict",
			defaultValue: "false",
		},
		{
			name:         "format flag",
			flagName:     "format",
			defaultValue: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := configValidateCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)
				return
			}

			// Check default value
			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}
		})
	}
}

func TestConfigValidateCommand_ShortDescription(t *testing.T) {
	expected := "Validate configuration files"
	if configValidateCmd.Short != expected {
		t.Errorf("Short = %q, want %q", configValidateCmd.Short, expected)
	}
}

func TestConfigValidateCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"Validate workspace configuration",
		"config.yaml",
		"YAML syntax",
		"circular dependencies",
	}

	for _, substr := range contains {
		if !containsString(configValidateCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestConfigValidateCommand_ExamplesContains(t *testing.T) {
	examples := []string{
		"mehr config validate",
		"--strict",
		"--format json",
	}

	for _, example := range examples {
		if !containsString(configValidateCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestConfigCommand_RegisteredInRoot(t *testing.T) {
	// Verify configCmd is a subcommand of rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "config" {
			found = true
			break
		}
	}
	if !found {
		t.Error("config command not registered in root command")
	}
}

func TestConfigValidateCommand_RegisteredInConfig(t *testing.T) {
	// Verify configValidateCmd is a subcommand of configCmd
	found := false
	for _, cmd := range configCmd.Commands() {
		if cmd.Use == "validate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("validate subcommand not registered in config command")
	}
}
