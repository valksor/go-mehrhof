package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

var continueAuto bool // Auto-execute the next logical step

var continueCmd = &cobra.Command{
	Use:     "continue",
	Aliases: []string{"cont", "c"},
	Short:   "Resume workflow, optionally auto-execute (aliases: cont, c)",
	Long: `Resume your task with optional auto-pilot mode.

Perfect for returning after a break - see where you left off and optionally
let the AI continue automatically.

WITHOUT --auto: Shows current state and suggests what to do next
WITH --auto:    Automatically runs the next logical step (plan → implement)

CHOOSING THE RIGHT COMMAND:
  guide     - "What's my next command?" (fastest, minimal output)
  status    - "Show me everything" (full inspection, all details)
  continue  - "Resume and optionally auto-execute" (--auto runs next step)  <-- you are here

AUTO-EXECUTION LOGIC:
  idle (no specs)  → runs 'mehr plan'
  idle (has specs) → runs 'mehr implement'
  planning         → runs 'mehr implement'
  implementing     → suggests 'mehr finish' (won't auto-run)
  done             → nothing to do

Examples:
  mehr continue       # See status + suggestions
  mehr c              # Same (shorthand alias)
  mehr continue --auto # Auto-execute next step`,
	RunE: runContinue,
}

func init() {
	rootCmd.AddCommand(continueCmd)
	continueCmd.Flags().BoolVar(&continueAuto, "auto", false, "Auto-execute the next logical step")
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
			branch, err := git.CurrentBranch(ctx)
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
		fmt.Println(display.Muted("Suggested actions:"))
		fmt.Println("  mehr start <file.md>       # Start from markdown file")
		fmt.Println("  mehr start <directory/>    # Start from directory")

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
	fmt.Println(display.Muted("Suggested actions:"))
	switch workflow.State(status.State) {
	case workflow.StateIdle:
		if status.Specifications == 0 {
			fmt.Println("  mehr plan       # Create specifications")
			fmt.Println("  mehr note       # Add requirements")
		} else {
			fmt.Println("  mehr implement  # Implement the specifications")
			fmt.Println("  mehr plan       # Create more specifications")
			fmt.Println("  mehr note       # Add notes")
		}
	case workflow.StatePlanning:
		fmt.Println("  mehr implement  # Start implementation")
		fmt.Println("  mehr note       # Add notes")
	case workflow.StateImplementing:
		fmt.Println("  mehr implement  # Continue implementation")
		fmt.Println("  mehr note       # Add notes")
		fmt.Println("  mehr undo       # Revert last change")
		fmt.Println("  mehr finish     # Complete and merge")
	case workflow.StateReviewing:
		fmt.Println("  mehr finish     # Complete and merge")
		fmt.Println("  mehr implement  # Make more changes")
	case workflow.StateFailed:
		fmt.Println("  mehr status     # View error details")
		fmt.Println("  mehr note       # Add notes")
		fmt.Println("  mehr implement  # Retry implementation")
	case workflow.StateWaiting:
		fmt.Println("  mehr answer     # Respond to agent question")
	case workflow.StateCheckpointing:
		fmt.Println("  Please wait...  # Creating checkpoint")
	case workflow.StateReverting:
		fmt.Println("  Please wait...  # Reverting to checkpoint")
	case workflow.StateRestoring:
		fmt.Println("  Please wait...  # Restoring checkpoint")
	case workflow.StateDone:
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
	if workflow.State(status.State) != workflow.StateDone {
		fmt.Println()
		fmt.Println("Other options:")
		fmt.Println("  mehr finish     # Complete and merge changes")
		fmt.Println("  mehr abandon    # Abandon task without merging")
	}

	return nil
}

// executeNextStep determines and executes the next logical workflow step.
func executeNextStep(ctx context.Context, cond *conductor.Conductor, status *conductor.TaskStatus) error {
	switch workflow.State(status.State) {
	case workflow.StateIdle:
		if status.Specifications == 0 {
			fmt.Println("Running: mehr plan")

			return cond.Plan(ctx)
		}
		fmt.Println("Running: mehr implement")

		return cond.Implement(ctx)
	case workflow.StatePlanning:
		fmt.Println("Running: mehr implement")

		return cond.Implement(ctx)
	case workflow.StateImplementing, workflow.StateReviewing:
		fmt.Println("Already in progress - use 'mehr finish' when complete")

		return nil
	case workflow.StateFailed:
		fmt.Println("Task failed - cannot auto-continue")

		return errors.New("task is in failed state")
	case workflow.StateWaiting:
		fmt.Println("Agent is waiting for a response - cannot auto-continue")

		return errors.New("agent is waiting for user input")
	case workflow.StateCheckpointing, workflow.StateReverting, workflow.StateRestoring:
		fmt.Println("Operation in progress - please wait")

		return nil
	case workflow.StateDone:
		fmt.Println("Task is complete!")

		return nil
	default:
		fmt.Printf("State '%s' doesn't have an auto-continue action\n", status.State)
		fmt.Println("Use 'mehr guide' to see available options")

		return nil
	}
}
