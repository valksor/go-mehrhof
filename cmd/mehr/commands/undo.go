package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
)

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Revert to the previous checkpoint",
	Long: `Revert the current task to its previous checkpoint.

This undoes the last set of changes by resetting to the previous git checkpoint.
Use 'mehr redo' to restore undone changes.

Examples:
  mehr undo                    # Undo last changes`,
	RunE: runUndo,
}

func init() {
	rootCmd.AddCommand(undoCmd)
}

func runUndo(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, conductor.WithVerbose(cfg.UI.Verbose))
	if err != nil {
		return err
	}

	// Perform undo
	if err := cond.Undo(ctx); err != nil {
		return fmt.Errorf("undo: %w", err)
	}

	fmt.Println("Reverted to previous checkpoint")
	return nil
}
