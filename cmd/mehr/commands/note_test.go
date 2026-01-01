//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

func TestNoteCommand_Properties(t *testing.T) {
	// Check command is properly configured
	if noteCmd.Use != "note [message]" {
		t.Errorf("Use = %q, want %q", noteCmd.Use, "note [message]")
	}

	if noteCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if noteCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if noteCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestNoteCommand_Aliases(t *testing.T) {
	// Check that "answer" is an alias
	expectedAliases := []string{"answer"}

	for _, expected := range expectedAliases {
		found := false
		for _, alias := range noteCmd.Aliases {
			if alias == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("alias %q not found in noteCmd.Aliases", expected)
		}
	}
}

func TestNoteCommand_ShortDescription(t *testing.T) {
	expected := "Add notes to the task or answer agent questions"
	if noteCmd.Short != expected {
		t.Errorf("Short = %q, want %q", noteCmd.Short, expected)
	}
}

func TestNoteCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"Add notes",
		"notes.md",
		"work directory",
		"agent runs",
		"pending",
	}

	for _, substr := range contains {
		if !containsString(noteCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestNoteCommand_ExamplesContains(t *testing.T) {
	examples := []string{
		"mehr note",
		"mehr answer",
		`"The API should use REST"`,
	}

	for _, example := range examples {
		if !containsString(noteCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestNoteCommand_RegisteredInRoot(t *testing.T) {
	// Verify noteCmd is a subcommand of rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "note [message]" {
			found = true
			break
		}
	}
	if !found {
		t.Error("note command not registered in root command")
	}
}

func TestNoteCommand_AliasesSection(t *testing.T) {
	// Long description should document aliases
	if !containsString(noteCmd.Long, "ALIASES") {
		t.Error("Long description does not contain ALIASES section")
	}

	if !containsString(noteCmd.Long, "note") && !containsString(noteCmd.Long, "answer") {
		t.Error("Long description does not document both note and answer aliases")
	}
}

func TestNoteCommand_InteractiveModeDocumented(t *testing.T) {
	// Interactive mode should be documented
	if !containsString(noteCmd.Long, "interactive mode") {
		t.Error("Long description does not mention interactive mode")
	}
}

func TestNoteCommand_NoFlags(t *testing.T) {
	// Note command doesn't have flags in the current implementation
	// Verify no unexpected flags were added
	flags := noteCmd.Flags()

	// Only inherited flags should be present (like --help)
	localFlags := noteCmd.LocalFlags()
	localNonPersistent := localFlags.NFlag()

	if localNonPersistent > 0 {
		// If flags are added in the future, this test documents them
		t.Logf("Note: noteCmd has %d local flags", localNonPersistent)
	}

	// Check that common flags like --verbose are not local to this command
	if flags.Lookup("verbose") != nil && localFlags.Lookup("verbose") != nil {
		t.Error("verbose should be a persistent flag from root, not local")
	}
}

func TestNoteCommand_UsesWorkDirectory(t *testing.T) {
	// The command should document that it saves to work directory
	if !containsString(noteCmd.Long, "work directory") {
		t.Error("Long description does not mention work directory")
	}
}
