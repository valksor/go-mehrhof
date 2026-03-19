package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var UndoCmd = &cobra.Command{
	Use:     "undo",
	Aliases: []string{"u"},
	Short:   "Undo to the previous checkpoint",
	Long: `Reverts the working directory to the previous checkpoint.
Checkpoints are created after each agent operation (plan, implement).
Use 'redo' to restore undone checkpoints.`,
	RunE: runUndo,
}

func init() {
	UndoCmd.Flags().IntP("steps", "n", 1, "Number of checkpoints to undo")
}

func runUndo(cmd *cobra.Command, args []string) error {
	steps, _ := cmd.Flags().GetInt("steps")

	result, err := callWorktree(context.Background(), "undo", map[string]any{
		"steps": steps,
	})
	if err != nil {
		return fmt.Errorf("undo: %w", err)
	}

	fmt.Printf("Undo: %v\n", result)

	return nil
}
