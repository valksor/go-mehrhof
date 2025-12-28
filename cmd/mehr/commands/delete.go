package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/display"
)

var (
	deleteYes        bool
	deleteKeepBranch bool
	deleteKeepWork   bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the current task without merging",
	Long: `Delete the current task, its branch, and work directory without merging changes.

This is useful when you want to abandon a task completely, such as when:
- The approach didn't work out
- The task is no longer needed
- You want to start fresh

By default, this command:
- Deletes the task branch (if one was created)
- Removes the work directory (.task/work/<task-id>)
- Clears the active task reference

Examples:
  mehr delete                 # Delete with confirmation
  mehr delete --yes           # Delete without confirmation
  mehr delete -y              # Same as --yes
  mehr delete --keep-branch   # Delete task but keep the git branch
  mehr delete --keep-work     # Delete branch but keep the work directory`,
	RunE: runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "Skip confirmation prompt")
	deleteCmd.Flags().BoolVar(&deleteKeepBranch, "keep-branch", false, "Keep the git branch")
	deleteCmd.Flags().BoolVar(&deleteKeepWork, "keep-work", false, "Keep the work directory")
}

func runDelete(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, conductor.WithVerbose(cfg.UI.Verbose))
	if err != nil {
		return err
	}

	// Check for active task
	activeTask := cond.GetActiveTask()
	if activeTask == nil {
		fmt.Print(display.NoActiveTaskError())
		return nil
	}

	// Get status for display
	status, err := cond.Status()
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	// Build confirmation prompt
	promptLines := fmt.Sprintf("About to delete task: %s", status.TaskID)
	if status.Title != "" {
		promptLines += fmt.Sprintf("\n  Title: %s", status.Title)
	}
	if status.Branch != "" {
		promptLines += fmt.Sprintf("\n  Branch: %s", status.Branch)
	}
	promptLines += fmt.Sprintf("\n  State: %s", status.State)
	promptLines += fmt.Sprintf("\n  Specifications: %d", status.Specifications)

	if !deleteKeepBranch && status.Branch != "" {
		promptLines += "\n\nWARNING: This will delete the git branch and all uncommitted changes!"
	}

	// Confirmation prompt (unless --yes or --force)
	confirmed, err := confirmAction(promptLines, deleteYes)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Cancelled")
		return nil
	}

	// Build delete options
	opts := conductor.DeleteOptions{
		Force:       deleteYes,
		KeepBranch:  deleteKeepBranch,
		KeepWorkDir: deleteKeepWork,
	}

	// Perform delete
	if err := cond.Delete(ctx, opts); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	fmt.Println(display.SuccessMsg("Task deleted successfully"))
	return nil
}
