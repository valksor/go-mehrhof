package cli

import "github.com/spf13/cobra"

var (
	// Quiet suppresses non-essential output.
	Quiet bool
	// Verbose shows additional details (socket paths, timing).
	Verbose bool
	// Debug shows debug information (JSON-RPC traffic, slog debug).
	Debug bool
	// NoColor disables colored output.
	NoColor bool
	// LogFormat controls the log output format (text or json).
	LogFormat string
)

// RegisterPersistentFlags adds global flags to the root command.
// Call this from main.go init().
func RegisterPersistentFlags(root *cobra.Command) {
	pf := root.PersistentFlags()
	pf.BoolVarP(&Quiet, "quiet", "q", false, "Suppress non-essential output")
	pf.BoolVar(&Verbose, "verbose", false, "Show additional details (socket paths, timing)")
	pf.BoolVar(&Debug, "debug", false, "Show debug information (JSON-RPC traffic, slog debug)")
	pf.BoolVar(&NoColor, "no-color", false, "Disable colored output")
	pf.StringVar(&LogFormat, "log-format", "text", "Log output format (text, json)")
}
