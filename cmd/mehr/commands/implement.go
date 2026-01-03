package commands

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/events"
)

var (
	implementDryRun            bool
	implementAgentImplementing string
)

var implementCmd = &cobra.Command{
	Use:     "implement",
	Aliases: []string{"impl", "i"},
	Short:   "Implement the specifications for the active task",
	Long: `Run the implementation phase to generate code based on specifications.

The agent will read all specification files in the work directory along with any
notes, then implement them by creating or modifying files.

Requires at least one specification file to exist (run 'mehr plan' first).

Examples:
  mehr implement                # Implement the specifications
  mehr implement --dry-run      # Preview without making changes
  mehr implement --verbose      # Show agent output`,
	RunE: runImplement,
}

func init() {
	rootCmd.AddCommand(implementCmd)

	implementCmd.Flags().BoolVarP(&implementDryRun, "dry-run", "n", false, "Don't apply file changes (preview only)")
	implementCmd.Flags().StringVar(&implementAgentImplementing, "agent-implement", "", "Agent for implementation step")
}

func runImplement(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Build conductor options
	opts := []conductor.Option{
		conductor.WithVerbose(verbose),
		conductor.WithDryRun(implementDryRun),
	}

	// Per-step agent override
	if implementAgentImplementing != "" {
		opts = append(opts, conductor.WithStepAgent("implementing", implementAgentImplementing))
	}

	// Use deduplicating stdout in verbose mode to suppress duplicate lines
	if verbose {
		opts = append(opts, conductor.WithStdout(getDeduplicatingStdout()))
	}

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, opts...)
	if err != nil {
		return err
	}

	// Check for active task
	if cond.GetActiveTask() == nil {
		fmt.Print(display.NoActiveTaskError())

		return errors.New("no active task")
	}

	// Set up event handlers
	if verbose {
		w := cond.GetStdout()
		cond.GetEventBus().SubscribeAll(func(e events.Event) {
			switch e.Type {
			case events.TypeProgress:
				if msg, ok := e.Data["message"].(string); ok {
					_, err := fmt.Fprintf(w, "  %s\n", msg)
					if err != nil {
						slog.Debug("write progress", "error", err)
					}
				}
			case events.TypeFileChanged:
				if path, ok := e.Data["path"].(string); ok {
					op, _ := e.Data["operation"].(string)
					_, err := fmt.Fprintf(w, "  [%s] %s\n", op, path)
					if err != nil {
						slog.Debug("write file change", "error", err)
					}
				}
			case events.TypeCheckpoint:
				if num, ok := e.Data["checkpoint"].(int); ok {
					_, err := fmt.Fprintf(w, "  Checkpoint #%d created\n", num)
					if err != nil {
						slog.Debug("write checkpoint", "error", err)
					}
				}
			case events.TypeStateChanged, events.TypeError, events.TypeAgentMessage, events.TypeBlueprintReady, events.TypeBranchCreated, events.TypePlanCompleted, events.TypeImplementDone, events.TypePRCreated:
				// Ignore other event types
			}
		})
	}

	// Enter implementation phase
	if err := cond.Implement(ctx); err != nil {
		return fmt.Errorf("implement: %w", err)
	}

	// Run implementation with spinner in non-verbose mode
	var implErr error
	spinnerMsg := "Implementing code..."
	if implementDryRun {
		spinnerMsg = "Implementing code (dry-run)..."
	}

	if verbose {
		if implementDryRun {
			fmt.Println(display.InfoMsg("Implementing (dry-run)..."))
		} else {
			fmt.Println(display.InfoMsg("Implementing..."))
		}
		implErr = cond.RunImplementation(ctx)
	} else {
		spinner := display.NewSpinner(spinnerMsg)
		spinner.Start()
		implErr = cond.RunImplementation(ctx)
		if implErr != nil {
			spinner.StopWithError("Implementation failed")
		} else {
			if implementDryRun {
				spinner.StopWithSuccess("Implementation preview complete")
			} else {
				spinner.StopWithSuccess("Implementation complete")
			}
		}
	}
	if implErr != nil {
		return fmt.Errorf("run implementation: %w", implErr)
	}

	// Get status
	status, err := cond.Status()
	if err != nil {
		return err
	}

	if verbose {
		fmt.Println()
		if implementDryRun {
			fmt.Println(display.SuccessMsg("Implementation preview finished"))
		} else {
			fmt.Println(display.SuccessMsg("Implementation complete!"))
		}
	}
	fmt.Printf("  Checkpoints: %s\n", display.Bold(strconv.Itoa(status.Checkpoints)))
	if implementDryRun {
		fmt.Println()
		fmt.Println(display.Muted("  (Dry-run mode - no files were modified)"))
	}
	fmt.Println()
	fmt.Println(display.Muted("Next steps:"))
	fmt.Printf("  %s - View task status\n", display.Cyan("mehr status"))
	fmt.Printf("  %s - Run code review\n", display.Cyan("mehr review"))
	fmt.Printf("  %s - Revert last changes\n", display.Cyan("mehr undo"))
	fmt.Printf("  %s - Complete the task\n", display.Cyan("mehr finish"))

	return nil
}
