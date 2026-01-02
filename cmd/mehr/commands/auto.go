package commands

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/events"
)

var (
	autoAgent         string
	autoNoBranch      bool
	autoWorktree      bool
	autoMaxRetries    int
	autoNoPush        bool
	autoNoDelete      bool
	autoNoSquash      bool
	autoTargetBranch  string
	autoQualityTarget string
	autoNoQuality     bool
)

var autoCmd = &cobra.Command{
	Use:   "auto <reference>",
	Short: "Full automation: start -> plan -> implement -> quality -> finish",
	Long: `Run a complete automation cycle without any user interaction.

This command orchestrates the entire workflow:
1. Register the task from the reference
2. Run planning to create specifications
3. Implement the specifications
4. Run quality checks (with retry loop if failed)
5. Merge and complete the task

Agent questions are automatically skipped (agent proceeds with best guess).
Quality failures trigger re-implementation with feedback, up to max retries.

Examples:
  mehr auto task.md                    # Full cycle from file
  mehr auto ./tasks/                   # Full cycle from directory
  mehr auto --max-retries 5 task.md    # Allow more quality retries
  mehr auto --no-push task.md          # Don't push after merge
  mehr auto --no-quality task.md       # Skip quality checks entirely`,
	Args: cobra.ExactArgs(1),
	RunE: runAuto,
}

func init() {
	rootCmd.AddCommand(autoCmd)

	autoCmd.Flags().StringVarP(&autoAgent, "agent", "a", "", "Agent to use (default: auto-detect)")
	autoCmd.Flags().BoolVar(&autoNoBranch, "no-branch", false, "Do not create a git branch")
	autoCmd.Flags().BoolVarP(&autoWorktree, "worktree", "w", false, "Create a separate git worktree")
	autoCmd.Flags().IntVar(&autoMaxRetries, "max-retries", 3, "Maximum quality check retry attempts")
	autoCmd.Flags().BoolVar(&autoNoPush, "no-push", false, "Don't push after merge")
	autoCmd.Flags().BoolVar(&autoNoDelete, "no-delete", false, "Don't delete task branch after merge")
	autoCmd.Flags().BoolVar(&autoNoSquash, "no-squash", false, "Use regular merge instead of squash")
	autoCmd.Flags().StringVarP(&autoTargetBranch, "target", "t", "", "Target branch to merge into")
	autoCmd.Flags().StringVar(&autoQualityTarget, "quality-target", "quality", "Make target for quality checks")
	autoCmd.Flags().BoolVar(&autoNoQuality, "no-quality", false, "Skip quality checks entirely")
}

func runAuto(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	reference := args[0]

	// Determine branch behavior
	// Branch is created by default; --no-branch disables it; worktree implies branch
	createBranch := !autoNoBranch || autoWorktree

	// Build conductor options with auto mode enabled
	// Always use deduplicating stdout for auto since it displays progress unconditionally
	opts := []conductor.Option{
		conductor.WithVerbose(verbose),
		conductor.WithCreateBranch(createBranch),
		conductor.WithUseWorktree(autoWorktree),
		conductor.WithAutoInit(true),
		conductor.WithAutoMode(true),
		conductor.WithSkipAgentQuestions(true),
		conductor.WithMaxQualityRetries(autoMaxRetries),
		conductor.WithStdout(getDeduplicatingStdout()),
	}

	if autoAgent != "" {
		opts = append(opts, conductor.WithAgent(autoAgent))
	}

	// Initialize conductor
	cond, err := initializeConductor(ctx, opts...)
	if err != nil {
		return err
	}

	// Check for existing task
	if cond.GetActiveTask() != nil {
		return fmt.Errorf("task already active: %s\nUse 'mehr abandon' to clear it first, or 'mehr status' for details", cond.GetActiveTask().ID)
	}

	// Subscribe to events for progress display
	w := cond.GetStdout()
	cond.GetEventBus().SubscribeAll(func(e events.Event) {
		switch e.Type {
		case events.TypeProgress:
			if msg, ok := e.Data["message"].(string); ok {
				// Map progress percentage to phase number
				phase := "[2/5]"
				if pct, ok := e.Data["percentage"].(int); ok {
					switch {
					case pct <= 10:
						phase = "[1/5]" // Start
					case pct <= 30:
						phase = "[2/5]" // Planning
					case pct <= 50:
						phase = "[3/5]" // Implementation
					case pct <= 80:
						phase = "[4/5]" // Quality
					default:
						phase = "[5/5]" // Finish
					}
				}
				_, err := fmt.Fprintf(w, "  %s %s\n", display.Info(phase), msg)
				if err != nil {
					slog.Debug("write progress", "error", err)
				}
			}
		case events.TypeFileChanged:
			if verbose {
				if path, ok := e.Data["path"].(string); ok {
					op, _ := e.Data["operation"].(string)
					_, err := fmt.Fprintf(w, "  %s [%s] %s\n", display.Muted("     "), op, path)
					if err != nil {
						slog.Debug("write file change", "error", err)
					}
				}
			}
		case events.TypeCheckpoint:
			if verbose {
				if num, ok := e.Data["checkpoint"].(int); ok {
					_, err := fmt.Fprintf(w, "  %s Checkpoint #%d created\n", display.Muted("     "), num)
					if err != nil {
						slog.Debug("write checkpoint", "error", err)
					}
				}
			}
		case events.TypeStateChanged, events.TypeError, events.TypeAgentMessage, events.TypeBlueprintReady, events.TypeBranchCreated, events.TypePlanCompleted, events.TypeImplementDone, events.TypePRCreated:
			// Ignore other event types in auto mode
		}
	})

	fmt.Printf("%s Starting auto mode for: %s\n", display.Info("[1/5]"), display.Bold(reference))
	fmt.Printf("%s Workflow: start → plan → implement → quality → finish\n", display.Muted("     "))

	// Build auto options
	autoOpts := conductor.AutoOptions{
		QualityTarget: autoQualityTarget,
		MaxRetries:    autoMaxRetries,
		SquashMerge:   !autoNoSquash,
		DeleteBranch:  !autoNoDelete,
		TargetBranch:  autoTargetBranch,
		Push:          !autoNoPush,
	}

	// Skip quality if requested
	if autoNoQuality {
		autoOpts.MaxRetries = 0
	}

	// Run the full auto cycle
	result, err := cond.RunAuto(ctx, reference, autoOpts)
	if err != nil {
		fmt.Println()
		fmt.Printf("Auto failed at: %s\n", result.FailedAt)
		fmt.Printf("  Planning:       %s\n", boolToStatus(result.PlanningDone))
		fmt.Printf("  Implementation: %s\n", boolToStatus(result.ImplementDone))
		fmt.Printf("  Quality:        %d attempt(s), passed=%v\n", result.QualityAttempts, result.QualityPassed)
		fmt.Printf("  Finish:         %s\n", boolToStatus(result.FinishDone))
		return err
	}

	fmt.Println()
	fmt.Println(display.SuccessMsg("Task completed automatically"))
	fmt.Printf("  %s Quality attempts: %d\n", display.Muted("•"), result.QualityAttempts)
	if !autoNoPush {
		fmt.Printf("  %s Changes merged and pushed\n", display.Muted("•"))
	} else {
		fmt.Printf("  %s Changes merged (not pushed)\n", display.Muted("•"))
	}

	return nil
}

// boolToStatus converts a boolean to a status string.
func boolToStatus(done bool) string {
	if done {
		return "done"
	}
	return "pending"
}
