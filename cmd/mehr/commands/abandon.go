package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/display"
)

var (
	abandonYes        bool
	abandonKeepBranch bool
	abandonKeepWork   bool
)

var abandonCmd = &cobra.Command{
	Use:   "abandon",
	Short: "Abandon the current task without merging",
	Long: `Abandon the current task, its branch, and work directory without merging changes.

This is useful when you want to abandon a task completely, such as when:
- The approach didn't work out
- The task is no longer needed
- You want to start fresh

By default, this command:
- Deletes the task branch (if one was created)
- Removes the work directory (.task/work/<task-id>)
- Clears the active task reference

Examples:
  mehr abandon                 # Abandon with confirmation
  mehr abandon --yes           # Abandon without confirmation
  mehr abandon -y              # Same as --yes
  mehr abandon --keep-branch   # Abandon task but keep the git branch
  mehr abandon --keep-work     # Abandon branch but keep the work directory`,
	RunE: runAbandon,
}

func init() {
	rootCmd.AddCommand(abandonCmd)

	abandonCmd.Flags().BoolVarP(&abandonYes, "yes", "y", false, "Skip confirmation prompt")
	abandonCmd.Flags().BoolVar(&abandonKeepBranch, "keep-branch", false, "Keep the git branch")
	abandonCmd.Flags().BoolVar(&abandonKeepWork, "keep-work", false, "Keep the work directory")
}

func runAbandon(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, conductor.WithVerbose(verbose))
	if err != nil {
		return err
	}

	// Check for active task
	activeTask := cond.GetActiveTask()
	if activeTask == nil {
		return fmt.Errorf("no active task: use 'mehr start <reference>' to create a task or 'mehr list' to view existing tasks")
	}

	// Get status for display
	status, err := cond.Status()
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	// Build confirmation prompt
	promptLines := fmt.Sprintf("About to abandon task: %s", status.TaskID)
	if status.Title != "" {
		promptLines += fmt.Sprintf("\n  Title: %s", status.Title)
	}
	if status.Branch != "" {
		promptLines += fmt.Sprintf("\n  Branch: %s", status.Branch)
	}
	promptLines += fmt.Sprintf("\n  State: %s", status.State)
	promptLines += fmt.Sprintf("\n  Specifications: %d", status.Specifications)

	if !abandonKeepBranch && status.Branch != "" {
		promptLines += "\n\nWARNING: This will delete the git branch and all uncommitted changes!"
	}

	// Confirmation prompt (unless --yes or --force)
	confirmed, err := confirmAction(promptLines, abandonYes)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Cancelled")
		return nil
	}

	// Build delete options
	opts := conductor.DeleteOptions{
		Force:       abandonYes,
		KeepBranch:  abandonKeepBranch,
		KeepWorkDir: abandonKeepWork,
	}

	// Perform delete
	if err := cond.Delete(ctx, opts); err != nil {
		return fmt.Errorf("abandon: %w", err)
	}

	fmt.Println(display.SuccessMsg("Task abandoned successfully"))
	return nil
}
