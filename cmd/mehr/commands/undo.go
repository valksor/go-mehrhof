package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/display"
)

var undoYes bool

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Revert to the previous checkpoint",
	Long: `Revert the current task to its previous checkpoint.

This undoes the last set of changes by resetting to the previous git checkpoint.
Use 'mehr redo' to restore undone changes.

Examples:
  mehr undo                    # Undo last changes (with confirmation)
  mehr undo --yes              # Undo without confirmation`,
	RunE: runUndo,
}

func init() {
	rootCmd.AddCommand(undoCmd)
	undoCmd.Flags().BoolVarP(&undoYes, "yes", "y", false, "Skip confirmation prompt")
}

func runUndo(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, conductor.WithVerbose(verbose))
	if err != nil {
		return err
	}

	// Check for active task
	activeTask := cond.GetActiveTask()
	if activeTask == nil {
		fmt.Print(display.NoActiveTaskError())

		return errors.New("no active task")
	}

	// Get status for confirmation
	status, err := cond.Status()
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	// Build confirmation prompt
	promptLines := "About to undo last changes and revert to previous checkpoint"
	if status.Title != "" {
		promptLines += "\n  Task: " + status.Title
	}
	promptLines += "\n  State: " + status.State

	// Confirmation prompt (unless --yes)
	confirmed, err := confirmAction(promptLines, undoYes)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Cancelled")

		return nil
	}

	// Perform undo
	if err := cond.Undo(ctx); err != nil {
		return fmt.Errorf("undo: %w", err)
	}

	fmt.Println(display.SuccessMsg("Reverted to previous checkpoint"))

	return nil
}
