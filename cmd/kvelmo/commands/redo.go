package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var RedoCmd = &cobra.Command{
	Use:     "redo",
	Aliases: []string{"r"},
	Short:   "Redo to the next checkpoint",
	Long: `Restores the working directory to the next checkpoint in the redo stack.
Only available after using 'undo'.`,
	RunE: runRedo,
}

func init() {
	RedoCmd.Flags().IntP("steps", "n", 1, "Number of checkpoints to redo")
}

func runRedo(cmd *cobra.Command, args []string) error {
	steps, _ := cmd.Flags().GetInt("steps")

	result, err := callWorktree(context.Background(), "redo", map[string]any{
		"steps": steps,
	})
	if err != nil {
		return fmt.Errorf("redo: %w", err)
	}

	fmt.Printf("Redo: %v\n", result)

	return nil
}
