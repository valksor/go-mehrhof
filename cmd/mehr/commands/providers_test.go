//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestProvidersCommand_Structure(t *testing.T) {
	tests := []struct {
		cmd       *cobra.Command
		wantArgs  cobra.PositionalArgs
		name      string
		wantUse   string
		wantShort string
	}{
		{
			name:      "providers command",
			cmd:       providersCmd,
			wantUse:   "providers",
			wantShort: "List and manage task providers",
		},
		{
			name:      "providers list subcommand",
			cmd:       providersListCmd,
			wantUse:   "list",
			wantShort: "List all available providers",
		},
		{
			name:      "providers info subcommand",
			cmd:       providersInfoCmd,
			wantUse:   "info <provider>",
			wantShort: "Show provider information",
			wantArgs:  cobra.ExactArgs(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cmd.Use != tt.wantUse {
				t.Errorf("Use = %q, want %q", tt.cmd.Use, tt.wantUse)
			}
			if tt.cmd.Short != tt.wantShort {
				t.Errorf("Short = %q, want %q", tt.cmd.Short, tt.wantShort)
			}
			if tt.wantArgs != nil && tt.cmd.Args != nil {
				// Verify that args validation is set (ExactArgs(1))
				// by checking it rejects empty args
				err := tt.cmd.Args(tt.cmd, []string{})
				if err == nil {
					t.Errorf("Expected args validation to reject empty args, but it didn't")
				}
			}
		})
	}
}

func TestProvidersCommand_SubcommandsRegistered(t *testing.T) {
	// Verify that providersCmd has the expected subcommands
	subcommands := providersCmd.Commands()
	if len(subcommands) != 2 {
		t.Fatalf("expected 2 subcommands, got %d", len(subcommands))
	}

	subcommandNames := make(map[string]bool)
	for _, cmd := range subcommands {
		subcommandNames[cmd.Name()] = true
	}

	expectedSubcommands := []string{"list", "info"}
	for _, expected := range expectedSubcommands {
		if !subcommandNames[expected] {
			t.Errorf("missing subcommand %q", expected)
		}
	}
}

func TestProvidersCommand_HasParent(t *testing.T) {
	// Verify that providersCmd is added to rootCmd
	if !hasCommand(rootCmd, "providers") {
		t.Error("providers command not registered with rootCmd")
	}
}

// Helper function to check if a command exists in the command tree.
func hasCommand(cmd *cobra.Command, name string) bool {
	for _, subCmd := range cmd.Commands() {
		if subCmd.Name() == name {
			return true
		}
	}

	return false
}
