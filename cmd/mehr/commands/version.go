package commands

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Build-time variables set via ldflags
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		out := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(out, "mehr %s\n", Version)
		_, _ = fmt.Fprintf(out, "  by Valksor\n")
		_, _ = fmt.Fprintf(out, "  Commit: %s\n", Commit)
		_, _ = fmt.Fprintf(out, "  Built:  %s\n", BuildTime)
		_, _ = fmt.Fprintf(out, "  Go:     %s\n", runtime.Version())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
