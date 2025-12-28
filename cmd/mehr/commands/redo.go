package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
)

var redoCmd = &cobra.Command{
	Use:   "redo",
	Short: "Restore the next checkpoint",
	Long: `Restore the current task to the next checkpoint.

This redoes changes that were previously undone.

Examples:
  mehr redo                    # Redo changes`,
	RunE: runRedo,
}

func init() {
	rootCmd.AddCommand(redoCmd)
}

func runRedo(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, conductor.WithVerbose(verbose))
	if err != nil {
		return err
	}

	// Perform redo
	if err := cond.Redo(ctx); err != nil {
		return fmt.Errorf("redo: %w", err)
	}

	fmt.Println("Restored to next checkpoint")
	return nil
}
