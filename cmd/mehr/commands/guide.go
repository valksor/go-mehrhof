package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

var guideCmd = &cobra.Command{
	Use:   "guide",
	Short: "Show context-aware next actions",
	Long: `Show suggested next actions based on the current task state.

This command analyzes your current context (active task, state, specifications)
and suggests the most appropriate next action.`,
	RunE: runGuide,
}

func init() {
	rootCmd.AddCommand(guideCmd)
}

func runGuide(cmd *cobra.Command, args []string) error {
	// Resolve workspace root and git context
	res, err := ResolveWorkspaceRoot()
	if err != nil {
		return err
	}

	ws, err := storage.OpenWorkspace(res.Root)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}

	// Check if in a worktree
	var active *storage.ActiveTask
	var work *storage.TaskWork
	if res.IsWorktree {
		// Auto-detect task from current worktree
		active, err = ws.FindTaskByWorktreePath(res.Git.Root())
		if err != nil {
			return fmt.Errorf("find task by worktree: %w", err)
		}
		if active == nil {
			fmt.Println("No task associated with this worktree.")
			fmt.Println()
			fmt.Println("Suggested actions:")
			fmt.Println("  mehr start <reference>   # Start a new task")
			return nil
		}
		work, _ = ws.LoadWork(active.ID)
	} else {
		// Check for active task in main repo
		if !ws.HasActiveTask() {
			fmt.Println("No active task.")
			fmt.Println()
			fmt.Println("Suggested actions:")
			fmt.Println("  mehr start <reference>   # Start a new task")
			fmt.Println("  mehr status --all          # View all tasks in workspace")
			return nil
		}
		active, err = ws.LoadActiveTask()
		if err != nil {
			return fmt.Errorf("load active task: %w", err)
		}
		if active != nil {
			work, _ = ws.LoadWork(active.ID)
		}
	}

	if work == nil {
		fmt.Println("No task found.")
		fmt.Println()
		fmt.Println("Suggested actions:")
		fmt.Println("  mehr start <reference>   # Start a new task")
		return nil
	}

	// Show task context and suggestions
	fmt.Printf("Task: %s\n", work.Metadata.ID)
	fmt.Printf("Title: %s\n", work.Metadata.Title)
	fmt.Printf("State: %s\n", active.State)
	fmt.Println()

	// Get specifications for context
	specs, _ := ws.ListSpecificationsWithStatus(work.Metadata.ID)
	fmt.Printf("Specifications: %d\n", len(specs))

	// Show pending question if any
	if ws.HasPendingQuestion(work.Metadata.ID) {
		q, _ := ws.LoadPendingQuestion(work.Metadata.ID)
		fmt.Println()
		fmt.Println("⚠️  The AI has a question for you:")
		fmt.Printf("  %s\n", q.Question)
		if len(q.Options) > 0 {
			fmt.Println("  Options:")
			for i, opt := range q.Options {
				fmt.Printf("    %d. %s\n", i+1, opt.Label)
			}
		}
		fmt.Println()
		fmt.Println("Suggested action:")
		fmt.Println("  mehr chat \"your answer\"    # Respond to the question")
		fmt.Println("  mehr chat                   # Enter interactive mode")
		return nil
	}

	// Show state-specific suggestions
	fmt.Println()
	fmt.Println("Suggested next actions:")

	switch workflow.State(active.State) {
	case workflow.StateIdle:
		if len(specs) == 0 {
			fmt.Println("  mehr plan                  # Create specifications")
			fmt.Println("  mehr chat                  # Discuss requirements")
		} else {
			// Check if any specs are not done
			hasIncomplete := false
			for _, spec := range specs {
				if spec.Status != storage.SpecificationStatusDone {
					hasIncomplete = true
					break
				}
			}
			if hasIncomplete {
				fmt.Println("  mehr implement              # Implement the specifications")
				fmt.Println("  mehr plan                  # Create more specifications")
			} else {
				fmt.Println("  mehr finish                # Complete and merge")
				fmt.Println("  mehr chat                  # Add notes or discuss")
			}
		}

	case workflow.StatePlanning:
		fmt.Println("  mehr status                # View planning progress")
		fmt.Println("  mehr chat                  # Discuss the plan")

	case workflow.StateImplementing:
		fmt.Println("  mehr status                # View implementation progress")
		fmt.Println("  mehr chat                  # Discuss issues")
		fmt.Println("  mehr undo                  # Revert last change")
		fmt.Println("  mehr finish                # Complete and merge")

	case workflow.StateReviewing:
		fmt.Println("  mehr status                # View review results")
		fmt.Println("  mehr finish                # Complete and merge")
		fmt.Println("  mehr implement              # Make more changes")

	case workflow.StateDone:
		fmt.Println("  Task is complete!")
		fmt.Println("  mehr start <reference>    # Start a new task")

	case workflow.StateWaiting:
		fmt.Println("  mehr chat                  # Respond to agent question")

	case workflow.StateDialogue:
		fmt.Println("  mehr chat                  # Continue conversation")

	case workflow.StateCheckpointing:
		fmt.Println("  mehr status                # View checkpoint progress")

	case workflow.StateReverting, workflow.StateRestoring:
		fmt.Println("  mehr status                # View undo/redo progress")

	case workflow.StateFailed:
		fmt.Println("  mehr status                # View error details")
		fmt.Println("  mehr chat                  # Discuss the error")
		fmt.Println("  mehr start <reference>    # Start a new task")

	default:
		fmt.Println("  mehr chat                  # Discuss the task")
		fmt.Println("  mehr status                # View detailed status")
	}

	return nil
}
