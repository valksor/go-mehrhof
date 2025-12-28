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
		fmt.Printf("mehr %s\n", Version)
		fmt.Printf("  Commit: %s\n", Commit)
		fmt.Printf("  Built:  %s\n", BuildTime)
		fmt.Printf("  Go:     %s\n", runtime.Version())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
