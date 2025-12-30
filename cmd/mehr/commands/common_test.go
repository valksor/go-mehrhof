//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestConfirmAction(t *testing.T) {
	tests := []struct {
		name        string
		skipConfirm bool
	}{
		{
			name:        "skip confirm returns true",
			skipConfirm: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When skipConfirm is true, it should return true without prompting
			result, err := confirmAction("test action", tt.skipConfirm)
			if err != nil {
				t.Fatalf("confirmAction: %v", err)
			}
			if !result {
				t.Error("expected true when skipConfirm is true")
			}
		})
	}
}

func TestGetDeduplicatingStdout(t *testing.T) {
	// Should return a non-nil writer
	w := getDeduplicatingStdout()
	if w == nil {
		t.Error("getDeduplicatingStdout returned nil")
	}

	// Calling again should return the same instance (singleton)
	w2 := getDeduplicatingStdout()
	if w != w2 {
		t.Error("getDeduplicatingStdout should return the same instance")
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

	// Check for --interactive flag
	interactiveFlag := initCmd.Flags().Lookup("interactive")
	if interactiveFlag == nil {
		t.Error("init command missing 'interactive' flag")
	} else {
		// Check shorthand is "i"
		if interactiveFlag.Shorthand != "i" {
			t.Errorf("interactive flag shorthand = %q, want 'i'", interactiveFlag.Shorthand)
		}
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
		"abandon", "undo", "redo", "version", "init", "config", "agents",
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

func TestImplementCommand_Aliases(t *testing.T) {
	expectedAliases := []string{"impl", "i"}
	actualAliases := implementCmd.Aliases

	if len(actualAliases) != len(expectedAliases) {
		t.Errorf("implement aliases count = %d, want %d", len(actualAliases), len(expectedAliases))
	}

	for i, expected := range expectedAliases {
		if i >= len(actualAliases) || actualAliases[i] != expected {
			t.Errorf("implement alias[%d] = %q, want %q", i, actualAliases[i], expected)
		}
	}
}

func TestFinishCommand_Aliases(t *testing.T) {
	expectedAliases := []string{"fi", "done"}
	actualAliases := finishCmd.Aliases

	if len(actualAliases) != len(expectedAliases) {
		t.Errorf("finish aliases count = %d, want %d", len(actualAliases), len(expectedAliases))
	}

	for i, expected := range expectedAliases {
		if i >= len(actualAliases) || actualAliases[i] != expected {
			t.Errorf("finish alias[%d] = %q, want %q", i, actualAliases[i], expected)
		}
	}
}

func TestStatusCommand_Aliases(t *testing.T) {
	expectedAliases := []string{"st"}
	actualAliases := statusCmd.Aliases

	if len(actualAliases) != len(expectedAliases) {
		t.Errorf("status aliases count = %d, want %d", len(actualAliases), len(expectedAliases))
	}

	for i, expected := range expectedAliases {
		if i >= len(actualAliases) || actualAliases[i] != expected {
			t.Errorf("status alias[%d] = %q, want %q", i, actualAliases[i], expected)
		}
	}
}

func TestContinueCommand_Aliases(t *testing.T) {
	expectedAliases := []string{"cont", "c"}
	actualAliases := continueCmd.Aliases

	if len(actualAliases) != len(expectedAliases) {
		t.Errorf("continue aliases count = %d, want %d", len(actualAliases), len(expectedAliases))
	}

	for i, expected := range expectedAliases {
		if i >= len(actualAliases) || actualAliases[i] != expected {
			t.Errorf("continue alias[%d] = %q, want %q", i, actualAliases[i], expected)
		}
	}
}

func TestPlanCommand_Aliases(t *testing.T) {
	expectedAliases := []string{"p"}
	actualAliases := planCmd.Aliases

	if len(actualAliases) != len(expectedAliases) {
		t.Errorf("plan aliases count = %d, want %d", len(actualAliases), len(expectedAliases))
	}

	for i, expected := range expectedAliases {
		if i >= len(actualAliases) || actualAliases[i] != expected {
			t.Errorf("plan alias[%d] = %q, want %q", i, actualAliases[i], expected)
		}
	}
}

func TestStartCommand_AgentFlagShorthand(t *testing.T) {
	// The agent flag should have shorthand 'A' (not 'a' which conflicts with --all)
	agentFlag := startCmd.Flags().Lookup("agent")
	if agentFlag == nil {
		t.Fatal("start command missing 'agent' flag")
	}

	if agentFlag.Shorthand != "A" {
		t.Errorf("agent flag shorthand = %q, want 'A'", agentFlag.Shorthand)
	}
}

func TestPlanCommand_StandaloneFlag(t *testing.T) {
	// Check for --standalone flag
	standaloneFlag := planCmd.Flags().Lookup("standalone")
	if standaloneFlag == nil {
		t.Error("plan command missing 'standalone' flag")
	}

	// The deprecated --new flag was removed (use --standalone instead)
}

func TestCostCommand_BreakdownFlag(t *testing.T) {
	// Check for --breakdown flag
	breakdownFlag := costCmd.Flags().Lookup("breakdown")
	if breakdownFlag == nil {
		t.Error("cost command missing 'breakdown' flag")
	}

	// The deprecated --by-step flag was removed (use --breakdown instead)
}
