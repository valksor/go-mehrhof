package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var DeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the current task",
	Long: `Clear the current task. Only allowed when the task is in a terminal state
(submitted, failed, or none).`,
	RunE: runDelete,
}

func init() {
	DeleteCmd.Flags().Bool("delete-branch", false, "Also delete the git branch")
}

func runDelete(cmd *cobra.Command, args []string) error {
	deleteBranch, _ := cmd.Flags().GetBool("delete-branch")

	_, err := callWorktree(context.Background(), "delete", map[string]any{
		"delete_branch": deleteBranch,
	})
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	fmt.Println("Task deleted")

	return nil
}
