package cli

import (
	"testing"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func TestRegisterPersistentFlags(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	RegisterPersistentFlags(root)

	tests := []struct {
		name      string
		flag      string
		shorthand string
	}{
		{"quiet", "quiet", "q"},
		{"verbose", "verbose", ""},
		{"debug", "debug", ""},
		{"no-color", "no-color", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := root.PersistentFlags().Lookup(tt.flag)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.flag)
			}
			if f.Shorthand != tt.shorthand {
				t.Errorf("flag %q shorthand = %q, want %q", tt.flag, f.Shorthand, tt.shorthand)
			}
			if f.DefValue != "false" {
				t.Errorf("flag %q default = %q, want %q", tt.flag, f.DefValue, "false")
			}
		})
	}
}

func TestInitColorRespectsNoColorEnv(t *testing.T) {
	// Save and restore original state.
	origNoColor := color.NoColor
	origFlag := NoColor
	t.Cleanup(func() {
		color.NoColor = origNoColor
		NoColor = origFlag
	})

	// Reset state.
	color.NoColor = false
	NoColor = false

	t.Setenv("NO_COLOR", "1")
	InitColor()

	if !color.NoColor {
		t.Error("InitColor did not set color.NoColor when NO_COLOR env is set")
	}
}

func TestInitColorRespectsFlag(t *testing.T) {
	origNoColor := color.NoColor
	origFlag := NoColor
	t.Cleanup(func() {
		color.NoColor = origNoColor
		NoColor = origFlag
	})

	color.NoColor = false
	NoColor = true

	t.Setenv("NO_COLOR", "")
	InitColor()

	if !color.NoColor {
		t.Error("InitColor did not set color.NoColor when NoColor flag is true")
	}
}
