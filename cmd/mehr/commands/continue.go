package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
)

var continueAuto bool // Auto-execute the next logical step

var continueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Resume workflow with optional auto-execution",
	Long: `Continue to the next workflow step.

This command is designed for resuming work on a task after a break:
- Without --auto: Shows status and suggested next actions
- With --auto: Automatically executes the next logical workflow step

DIFFERENCES FROM OTHER COMMANDS:
- 'mehr status' - Detailed state inspection (no execution capability)
- 'mehr guide' - Quick suggestions only (no auto-execution)
- 'mehr continue' - Status display + optional auto-execution (this command)

Examples:
  mehr continue       # Show status and suggested next actions
  mehr continue --auto # Auto-execute the next logical step`,
	RunE: runContinue,
}

func init() {
	rootCmd.AddCommand(continueCmd)
	continueCmd.Flags().BoolVarP(&continueAuto, "auto", "a", false, "Auto-execute the next logical step")
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

	// Auto-execute next step if --auto flag is set
	if continueAuto {
		return executeNextStep(ctx, cond, status)
	}

	// Otherwise, show suggested next actions
	fmt.Println("Suggested next actions:")
	switch status.State {
	case "idle":
		if status.Specifications == 0 {
			fmt.Println("  mehr plan       # Create specifications")
			fmt.Println("  mehr note       # Add requirements")
		} else {
			fmt.Println("  mehr implement  # Implement the specifications")
			fmt.Println("  mehr plan       # Create more specifications")
			fmt.Println("  mehr note       # Add notes")
		}
	case "planning":
		fmt.Println("  mehr implement  # Start implementation")
		fmt.Println("  mehr note       # Add notes")
	case "implementing":
		fmt.Println("  mehr implement  # Continue implementation")
		fmt.Println("  mehr note       # Add notes")
		fmt.Println("  mehr undo       # Revert last change")
		fmt.Println("  mehr finish     # Complete and merge")
	case "reviewing":
		fmt.Println("  mehr finish     # Complete and merge")
		fmt.Println("  mehr implement  # Make more changes")
	case "done":
		fmt.Println("  Task is complete!")
		fmt.Println("  mehr start <ref>  # Start a new task")
	default:
		fmt.Println("  mehr note       # Add notes")
		fmt.Println("  mehr status     # View detailed status")
	}

	// Show undo/redo availability
	if status.Checkpoints > 1 {
		fmt.Println()
		fmt.Printf("  mehr undo       # Revert to previous checkpoint (%d available)\n", status.Checkpoints-1)
	}

	// Always show finish and abandon options
	if status.State != "done" {
		fmt.Println()
		fmt.Println("Other options:")
		fmt.Println("  mehr finish     # Complete and merge changes")
		fmt.Println("  mehr abandon    # Abandon task without merging")
	}

	return nil
}

// executeNextStep determines and executes the next logical workflow step
func executeNextStep(ctx context.Context, cond *conductor.Conductor, status *conductor.TaskStatus) error {
	switch status.State {
	case "idle":
		if status.Specifications == 0 {
			fmt.Println("Running: mehr plan")
			return cond.Plan(ctx)
		}
		fmt.Println("Running: mehr implement")
		return cond.Implement(ctx)
	case "planning":
		fmt.Println("Running: mehr implement")
		return cond.Implement(ctx)
	case "implementing", "reviewing":
		fmt.Println("Already in progress - use 'mehr finish' when complete")
		return nil
	case "done":
		fmt.Println("Task is complete!")
		return nil
	default:
		fmt.Printf("State '%s' doesn't have an auto-continue action\n", status.State)
		fmt.Println("Use 'mehr guide' to see available options")
		return nil
	}
}
