package commands

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/events"
)

var (
	yoloAgent         string
	yoloBranch        bool
	yoloWorktree      bool
	yoloMaxRetries    int
	yoloNoPush        bool
	yoloNoDelete      bool
	yoloNoSquash      bool
	yoloTargetBranch  string
	yoloQualityTarget string
	yoloSkipQuality   bool
)

var yoloCmd = &cobra.Command{
	Use:     "yolo <reference>",
	Aliases: []string{"auto"},
	Short:   "Full automation: start -> plan -> implement -> quality -> finish",
	Long: `Run a complete automation cycle without any user interaction (alias: auto).

This command orchestrates the entire workflow:
1. Register the task from the reference
2. Run planning to create specifications
3. Implement the specifications
4. Run quality checks (with retry loop if failed)
5. Merge and complete the task

Agent questions are automatically skipped (agent proceeds with best guess).
Quality failures trigger re-implementation with feedback, up to max retries.

Examples:
  mehr yolo task.md                    # Full cycle from file
  mehr yolo ./tasks/                   # Full cycle from directory
  mehr yolo --max-retries 5 task.md    # Allow more quality retries
  mehr yolo --no-push task.md          # Don't push after merge
  mehr yolo --skip-quality task.md     # Skip quality checks entirely`,
	Args: cobra.ExactArgs(1),
	RunE: runYolo,
}

func init() {
	rootCmd.AddCommand(yoloCmd)

	yoloCmd.Flags().StringVarP(&yoloAgent, "agent", "a", "", "Agent to use (default: auto-detect)")
	yoloCmd.Flags().BoolVarP(&yoloBranch, "branch", "b", true, "Create a git branch for this task (use --branch=false to disable)")
	yoloCmd.Flags().BoolVarP(&yoloWorktree, "worktree", "w", false, "Create a separate git worktree")
	yoloCmd.Flags().IntVar(&yoloMaxRetries, "max-retries", 3, "Maximum quality check retry attempts")
	yoloCmd.Flags().BoolVar(&yoloNoPush, "no-push", false, "Don't push after merge")
	yoloCmd.Flags().BoolVar(&yoloNoDelete, "no-delete", false, "Don't delete task branch after merge")
	yoloCmd.Flags().BoolVar(&yoloNoSquash, "no-squash", false, "Use regular merge instead of squash")
	yoloCmd.Flags().StringVarP(&yoloTargetBranch, "target", "t", "", "Target branch to merge into")
	yoloCmd.Flags().StringVar(&yoloQualityTarget, "quality-target", "quality", "Make target for quality checks")
	yoloCmd.Flags().BoolVar(&yoloSkipQuality, "skip-quality", false, "Skip quality checks entirely")
}

func runYolo(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	reference := args[0]

	// Determine branch behavior
	// Worktree implies branch creation
	createBranch := yoloBranch
	if yoloWorktree {
		createBranch = true
	}

	// Build conductor options with yolo mode enabled
	// Always use deduplicating stdout for yolo since it displays progress unconditionally
	opts := []conductor.Option{
		conductor.WithVerbose(cfg.UI.Verbose),
		conductor.WithCreateBranch(createBranch),
		conductor.WithUseWorktree(yoloWorktree),
		conductor.WithAutoInit(true),
		conductor.WithYoloMode(true),
		conductor.WithSkipAgentQuestions(true),
		conductor.WithMaxQualityRetries(yoloMaxRetries),
		conductor.WithStdout(getDeduplicatingStdout()),
	}

	if yoloAgent != "" {
		opts = append(opts, conductor.WithAgent(yoloAgent))
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
				_, err := fmt.Fprintf(w, "  [YOLO] %s\n", msg)
				if err != nil {
					log.Println(err)
				}
			}
		case events.TypeFileChanged:
			if cfg.UI.Verbose {
				if path, ok := e.Data["path"].(string); ok {
					op, _ := e.Data["operation"].(string)
					_, err := fmt.Fprintf(w, "  [%s] %s\n", op, path)
					if err != nil {
						log.Println(err)
					}
				}
			}
		case events.TypeCheckpoint:
			if cfg.UI.Verbose {
				if num, ok := e.Data["checkpoint"].(int); ok {
					_, err := fmt.Fprintf(w, "  Checkpoint #%d created\n", num)
					if err != nil {
						log.Println(err)
					}
				}
			}
		}
	})

	fmt.Printf("Starting YOLO mode for: %s\n", reference)
	fmt.Println("Full automation: start -> plan -> implement -> quality -> finish")
	fmt.Println()

	// Build yolo options
	yoloOpts := conductor.YoloOptions{
		QualityTarget: yoloQualityTarget,
		MaxRetries:    yoloMaxRetries,
		SquashMerge:   !yoloNoSquash,
		DeleteBranch:  !yoloNoDelete,
		TargetBranch:  yoloTargetBranch,
		Push:          !yoloNoPush,
	}

	// Skip quality if requested
	if yoloSkipQuality {
		yoloOpts.MaxRetries = 0
	}

	// Run the full yolo cycle
	result, err := cond.RunYolo(ctx, reference, yoloOpts)
	if err != nil {
		fmt.Println()
		fmt.Printf("YOLO failed at: %s\n", result.FailedAt)
		fmt.Printf("  Planning:       %s\n", boolToStatus(result.PlanningDone))
		fmt.Printf("  Implementation: %s\n", boolToStatus(result.ImplementDone))
		fmt.Printf("  Quality:        %d attempt(s), passed=%v\n", result.QualityAttempts, result.QualityPassed)
		fmt.Printf("  Finish:         %s\n", boolToStatus(result.FinishDone))
		return err
	}

	fmt.Println()
	fmt.Println("YOLO complete!")
	fmt.Printf("  Quality attempts: %d\n", result.QualityAttempts)
	if !yoloNoPush {
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
