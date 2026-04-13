package cli

import (
	"os"

	"github.com/fatih/color"
)

// InitColor configures color output based on flags and environment.
// Call from PersistentPreRun after flags are parsed.
func InitColor() {
	if NoColor || os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
	}
}

// Color helper functions for consistent CLI output.
var (
	Red    = color.New(color.FgRed)
	Green  = color.New(color.FgGreen)
	Yellow = color.New(color.FgYellow)
	Cyan   = color.New(color.FgCyan)
	Bold   = color.New(color.Bold)
)
