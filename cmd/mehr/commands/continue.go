package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
)

var continueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Show status and suggested next actions for current task",
	Long: `Show the current mehr status and suggest next actions.

If you're on a task branch (e.g., task/abc123), this will:
- Show the current mehr status
- Suggest the most appropriate next action based on state

This is useful when returning to work on a task after a break.

Examples:
  mehr continue    # Show status and suggestions`,
	RunE: runContinue,
}

func init() {
	rootCmd.AddCommand(continueCmd)
}

func runContinue(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, conductor.WithVerbose(verbose))
	if err != nil {
		return err
	}

	// Check for active task
	activeTask := cond.GetActiveTask()
	if activeTask == nil {
		// Try to detect from branch
		git := cond.GetGit()
		if git != nil {
			branch, err := git.CurrentBranch()
			if err == nil && strings.HasPrefix(branch, "task/") {
				taskID := strings.TrimPrefix(branch, "task/")
				fmt.Printf("On task branch: %s\n", branch)
				fmt.Printf("But no active task found with ID: %s\n\n", taskID)
				fmt.Println("The task may have been completed or deleted.")
				fmt.Println("To start a new task, run: mehr start <reference>")
				return nil
			}
		}

		fmt.Println("No active task found.")
		fmt.Println()
		fmt.Println("To start a new task:")
		fmt.Println("  mehr start <file.md>       # From markdown file")
		fmt.Println("  mehr start <directory/>    # From directory with README.md")
		return nil
	}

	// Get full status
	status, err := cond.Status()
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	// Display status
	fmt.Printf("Task: %s\n", status.TaskID)
	if status.Title != "" {
		fmt.Printf("Title: %s\n", status.Title)
	}
	fmt.Printf("State: %s\n", status.State)
	if status.Branch != "" {
		fmt.Printf("Branch: %s\n", status.Branch)
	}
	fmt.Printf("Specifications: %d\n", status.Specifications)
	fmt.Printf("Checkpoints: %d\n", status.Checkpoints)
	fmt.Println()

	// Suggest next actions based on state
	fmt.Println("Suggested next actions:")
	switch status.State {
	case "idle":
		if status.Specifications == 0 {
			fmt.Println("  mehr plan       # Create specifications")
			fmt.Println("  mehr chat       # Discuss requirements")
		} else {
			fmt.Println("  mehr implement  # Implement the specifications")
			fmt.Println("  mehr plan       # Create more specifications")
			fmt.Println("  mehr chat       # Discuss changes")
		}
	case "planning":
		fmt.Println("  mehr implement  # Start implementation")
		fmt.Println("  mehr chat       # Discuss the plan")
	case "implementing":
		fmt.Println("  mehr implement  # Continue implementation")
		fmt.Println("  mehr chat       # Discuss issues")
		fmt.Println("  mehr undo       # Revert last change")
		fmt.Println("  mehr finish     # Complete and merge")
	case "reviewing":
		fmt.Println("  mehr finish     # Complete and merge")
		fmt.Println("  mehr implement  # Make more changes")
	case "done":
		fmt.Println("  Task is complete!")
		fmt.Println("  mehr start <ref>  # Start a new task")
	default:
		fmt.Println("  mehr chat       # Discuss the task")
		fmt.Println("  mehr status     # View detailed status")
	}

	// Show undo/redo availability
	if status.Checkpoints > 1 {
		fmt.Println()
		fmt.Printf("  mehr undo       # Revert to previous checkpoint (%d available)\n", status.Checkpoints-1)
	}

	// Always show finish and delete options
	if status.State != "done" {
		fmt.Println()
		fmt.Println("Other options:")
		fmt.Println("  mehr finish     # Complete and merge changes")
		fmt.Println("  mehr delete     # Abandon task without merging")
	}

	return nil
}
