package commands

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/events"
)

var (
	implementDryRun            bool
	implementAgentImplementing string
)

var implementCmd = &cobra.Command{
	Use:   "implement",
	Short: "Implement the specifications for the active task",
	Long: `Run the implementation phase to generate code based on specifications.

The agent will read all SPEC files in the work directory along with any
notes, then implement the specifications by creating or modifying files.

Requires at least one SPEC file to exist (run 'mehr plan' first).

Examples:
  mehr implement                # Implement the specs
  mehr implement --dry-run      # Preview without making changes
  mehr implement --verbose      # Show agent output`,
	RunE: runImplement,
}

func init() {
	rootCmd.AddCommand(implementCmd)

	implementCmd.Flags().BoolVarP(&implementDryRun, "dry-run", "n", false, "Don't apply file changes (preview only)")
	implementCmd.Flags().StringVar(&implementAgentImplementing, "agent-implementing", "", "Agent for implementation step")
}

func runImplement(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Build conductor options
	opts := []conductor.Option{
		conductor.WithVerbose(cfg.UI.Verbose),
		conductor.WithDryRun(implementDryRun),
	}

	// Per-step agent override
	if implementAgentImplementing != "" {
		opts = append(opts, conductor.WithStepAgent("implementing", implementAgentImplementing))
	}

	// Use deduplicating stdout in verbose mode to suppress duplicate lines
	if cfg.UI.Verbose {
		opts = append(opts, conductor.WithStdout(getDeduplicatingStdout()))
	}

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, opts...)
	if err != nil {
		return err
	}

	// Check for active task
	if cond.GetActiveTask() == nil {
		return fmt.Errorf("no active task\nUse 'mehr start <reference>' to register a task first")
	}

	// Set up event handlers
	if cfg.UI.Verbose {
		w := cond.GetStdout()
		cond.GetEventBus().SubscribeAll(func(e events.Event) {
			switch e.Type {
			case events.TypeProgress:
				if msg, ok := e.Data["message"].(string); ok {
					_, err := fmt.Fprintf(w, "  %s\n", msg)
					if err != nil {
						log.Println(err)
					}
				}
			case events.TypeFileChanged:
				if path, ok := e.Data["path"].(string); ok {
					op, _ := e.Data["operation"].(string)
					_, err := fmt.Fprintf(w, "  [%s] %s\n", op, path)
					if err != nil {
						log.Println(err)
					}
				}
			case events.TypeCheckpoint:
				if num, ok := e.Data["checkpoint"].(int); ok {
					_, err := fmt.Fprintf(w, "  Checkpoint #%d created\n", num)
					if err != nil {
						log.Println(err)
					}
				}
			}
		})
	}

	// Enter implementation phase
	if err := cond.Implement(ctx); err != nil {
		return fmt.Errorf("implement: %w", err)
	}

	// Run implementation
	if implementDryRun {
		fmt.Println("Implementing (dry-run)...")
	} else {
		fmt.Println("Implementing...")
	}
	if err := cond.RunImplementation(ctx); err != nil {
		return fmt.Errorf("run implementation: %w", err)
	}

	// Get status
	status, err := cond.Status()
	if err != nil {
		return err
	}

	if implementDryRun {
		fmt.Printf("\nImplementation preview finished.\n")
		fmt.Printf("  Checkpoints: %d\n", status.Checkpoints)
		fmt.Println("\n  (Dry-run mode - no files were modified)")
	} else {
		fmt.Printf("\nImplementation complete!\n")
		fmt.Printf("  Checkpoints: %d\n", status.Checkpoints)
	}
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  mehr status    - View mehr status\n")
	fmt.Printf("  mehr review    - Run code review\n")
	fmt.Printf("  mehr undo      - Revert last changes\n")
	fmt.Printf("  mehr finish    - Complete the task\n")

	return nil
}
