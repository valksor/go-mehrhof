package commands

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/events"
)

var (
	autoAgent         string
	autoBranch        bool
	autoWorktree      bool
	autoMaxRetries    int
	autoNoPush        bool
	autoNoDelete      bool
	autoNoSquash      bool
	autoTargetBranch  string
	autoQualityTarget string
	autoSkipQuality   bool
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
  mehr auto --skip-quality task.md     # Skip quality checks entirely`,
	Args: cobra.ExactArgs(1),
	RunE: runAuto,
}

func init() {
	rootCmd.AddCommand(autoCmd)

	autoCmd.Flags().StringVarP(&autoAgent, "agent", "a", "", "Agent to use (default: auto-detect)")
	autoCmd.Flags().BoolVarP(&autoBranch, "branch", "b", true, "Create a git branch for this task (use --branch=false to disable)")
	autoCmd.Flags().BoolVarP(&autoWorktree, "worktree", "w", false, "Create a separate git worktree")
	autoCmd.Flags().IntVar(&autoMaxRetries, "max-retries", 3, "Maximum quality check retry attempts")
	autoCmd.Flags().BoolVar(&autoNoPush, "no-push", false, "Don't push after merge")
	autoCmd.Flags().BoolVar(&autoNoDelete, "no-delete", false, "Don't delete task branch after merge")
	autoCmd.Flags().BoolVar(&autoNoSquash, "no-squash", false, "Use regular merge instead of squash")
	autoCmd.Flags().StringVarP(&autoTargetBranch, "target", "t", "", "Target branch to merge into")
	autoCmd.Flags().StringVar(&autoQualityTarget, "quality-target", "quality", "Make target for quality checks")
	autoCmd.Flags().BoolVar(&autoSkipQuality, "skip-quality", false, "Skip quality checks entirely")
}

func runAuto(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	reference := args[0]

	// Determine branch behavior
	// Worktree implies branch creation
	createBranch := autoBranch
	if autoWorktree {
		createBranch = true
	}

	// Build conductor options with auto mode enabled
	// Always use deduplicating stdout for auto since it displays progress unconditionally
	opts := []conductor.Option{
		conductor.WithVerbose(verbose),
		conductor.WithCreateBranch(createBranch),
		conductor.WithUseWorktree(autoWorktree),
		conductor.WithAutoInit(true),
		conductor.WithYoloMode(true),
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
		return fmt.Errorf("task already active: %s\nUse 'mehr delete --force' to clear it first", cond.GetActiveTask().ID)
	}

	// Subscribe to events for progress display
	w := cond.GetStdout()
	cond.GetEventBus().SubscribeAll(func(e events.Event) {
		switch e.Type {
		case events.TypeProgress:
			if msg, ok := e.Data["message"].(string); ok {
				_, err := fmt.Fprintf(w, "  [AUTO] %s\n", msg)
				if err != nil {
					log.Println(err)
				}
			}
		case events.TypeFileChanged:
			if verbose {
				if path, ok := e.Data["path"].(string); ok {
					op, _ := e.Data["operation"].(string)
					_, err := fmt.Fprintf(w, "  [%s] %s\n", op, path)
					if err != nil {
						log.Println(err)
					}
				}
			}
		case events.TypeCheckpoint:
			if verbose {
				if num, ok := e.Data["checkpoint"].(int); ok {
					_, err := fmt.Fprintf(w, "  Checkpoint #%d created\n", num)
					if err != nil {
						log.Println(err)
					}
				}
			}
		}
	})

	fmt.Printf("Starting auto mode for: %s\n", reference)
	fmt.Println("Full automation: start -> plan -> implement -> quality -> finish")
	fmt.Println()

	// Build auto options
	autoOpts := conductor.YoloOptions{
		QualityTarget: autoQualityTarget,
		MaxRetries:    autoMaxRetries,
		SquashMerge:   !autoNoSquash,
		DeleteBranch:  !autoNoDelete,
		TargetBranch:  autoTargetBranch,
		Push:          !autoNoPush,
	}

	// Skip quality if requested
	if autoSkipQuality {
		autoOpts.MaxRetries = 0
	}

	// Run the full auto cycle
	result, err := cond.RunYolo(ctx, reference, autoOpts)
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
	fmt.Println("Auto complete!")
	fmt.Printf("  Quality attempts: %d\n", result.QualityAttempts)
	if !autoNoPush {
		fmt.Println("  Changes merged and pushed")
	} else {
		fmt.Println("  Changes merged (not pushed)")
	}

	return nil
}

// boolToStatus converts a boolean to a status string
func boolToStatus(done bool) string {
	if done {
		return "done"
	}
	return "pending"
}
