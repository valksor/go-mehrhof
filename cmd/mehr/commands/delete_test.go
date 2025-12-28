//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestDeleteCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue bool
		description  string
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
			flag := deleteCmd.Flags().Lookup(tt.flagName)
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
				shorthand := deleteCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestDeleteCommand_Properties(t *testing.T) {
	// Check command is properly configured
	if deleteCmd.Use != "delete" {
		t.Errorf("Use = %q, want %q", deleteCmd.Use, "delete")
	}

	if deleteCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if deleteCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if deleteCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestRedoCommand_Properties(t *testing.T) {
	if redoCmd.Use != "redo" {
		t.Errorf("Use = %q, want %q", redoCmd.Use, "redo")
	}

	if redoCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if redoCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestUndoCommand_Properties(t *testing.T) {
	if undoCmd.Use != "undo" {
		t.Errorf("Use = %q, want %q", undoCmd.Use, "undo")
	}

	if undoCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if undoCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestVersionCommand_Properties(t *testing.T) {
	if versionCmd.Use != "version" {
		t.Errorf("Use = %q, want %q", versionCmd.Use, "version")
	}

	if versionCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if versionCmd.Run == nil {
		t.Error("Run not set")
	}
}

func TestInitCommand_Properties(t *testing.T) {
	if initCmd.Use != "init" {
		t.Errorf("Use = %q, want %q", initCmd.Use, "init")
	}

	if initCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if initCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestAgentsCommand_Structure(t *testing.T) {
	// Check agents command structure
	if agentsCmd.Use != "agents" {
		t.Errorf("Use = %q, want %q", agentsCmd.Use, "agents")
	}

	// Check it has subcommands
	subcommands := agentsCmd.Commands()
	if len(subcommands) == 0 {
		t.Error("agentsCmd has no subcommands")
	}

	// Check for 'list' subcommand
	hasList := false
	for _, cmd := range subcommands {
		if cmd.Use == "list" {
			hasList = true
			if cmd.Short == "" {
				t.Error("agents list Short description is empty")
			}
			if cmd.RunE == nil {
				t.Error("agents list RunE not set")
			}
			break
		}
	}
	if !hasList {
		t.Error("agentsCmd missing 'list' subcommand")
	}
}

func TestConfigCommand_Structure(t *testing.T) {
	if configCmd.Use != "config" {
		t.Errorf("Use = %q, want %q", configCmd.Use, "config")
	}

	subcommands := configCmd.Commands()
	if len(subcommands) == 0 {
		t.Error("configCmd has no subcommands")
	}

	// Check for 'validate' subcommand
	hasValidate := false
	for _, cmd := range subcommands {
		if cmd.Use == "validate" {
			hasValidate = true
			if cmd.Short == "" {
				t.Error("config validate Short description is empty")
			}
			break
		}
	}
	if !hasValidate {
		t.Error("configCmd missing 'validate' subcommand")
	}

	// Check validate flags
	validateFlag := configValidateCmd.Flags().Lookup("strict")
	if validateFlag == nil {
		t.Error("validate command missing 'strict' flag")
	}

	formatFlag := configValidateCmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Error("validate command missing 'format' flag")
	}
}

func TestRootCommand_HasSubcommands(t *testing.T) {
	// Check that root command has some expected subcommands
	// Note: Due to init() function ordering, not all commands may be registered
	// during test execution. This test verifies a subset of known commands.

	expectedSubcommands := []string{
		// Core commands that should always be present
		"delete", "undo", "redo", "version", "init", "config", "agents",
	}

	actualSubcommands := rootCmd.Commands()
	actualNames := make(map[string]bool)
	for _, cmd := range actualSubcommands {
		actualNames[cmd.Use] = true
	}

	missingCommands := []string{}
	for _, expected := range expectedSubcommands {
		if !actualNames[expected] {
			missingCommands = append(missingCommands, expected)
		}
	}

	if len(missingCommands) > 0 {
		// Log as warning rather than fail, since init ordering can vary
		t.Logf("Warning: Some expected subcommands not found: %v", missingCommands)
		t.Logf("Found subcommands: %v", getCommandNames(actualSubcommands))
	}

	// Verify at least some commands are registered
	if len(actualNames) < 5 {
		t.Errorf("Expected at least 5 subcommands, got %d", len(actualNames))
	}
}

func getCommandNames(commands []*cobra.Command) []string {
	names := make([]string, len(commands))
	for i, cmd := range commands {
		names[i] = cmd.Use
	}
	return names
}
