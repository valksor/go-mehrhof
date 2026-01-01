//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

func TestGuideCommand_Structure(t *testing.T) {
	if guideCmd.Use != "guide" {
		t.Errorf("expected Use to be 'guide', got %q", guideCmd.Use)
	}
	if guideCmd.Short != "Quick next-action suggestion (1-second answer)" {
		t.Errorf("expected Short to be 'Quick next-action suggestion (1-second answer)', got %q", guideCmd.Short)
	}
	// Verify Long description is set
	if guideCmd.Long == "" {
		t.Error("expected Long description to be set")
	}
}

func TestGuideCommand_HasParent(t *testing.T) {
	// Verify that guideCmd is added to rootCmd
	if !hasCommand(rootCmd, "guide") {
		t.Error("guide command not registered with rootCmd")
	}
}

func TestGuideCommand_NoArgsRequired(t *testing.T) {
	// guide command takes no arguments
	if guideCmd.Args != nil {
		// Args should be nil (no validation)
		err := guideCmd.Args(guideCmd, []string{})
		if err != nil {
			t.Errorf("expected no args validation, got: %v", err)
		}
	}
}
