package commands

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/storage"
)

var (
	planStandalone    bool
	planSeed          string
	planFullContext   bool
	planAgentPlanning string // Per-step agent override
)

var planCmd = &cobra.Command{
	Use:     "plan [seed-topic]",
	Aliases: []string{"p"},
	Short:   "Create implementation specifications for the active task",
	Long: `Run the planning phase to analyze the task and create specification files.

The agent will read the source content and any notes, then generate
structured specifications (specification-N.md files) in the work directory.

You can run this multiple times to create additional specifications.

STANDALONE MODE (--standalone):
  Start a planning session without an active task. This is useful for
  exploring requirements before creating a formal task.
  Plans are saved to .mehrhof/planned/ directory.

SEED TOPIC:
  For standalone mode, you can provide a seed topic in two ways:
    mehr plan --standalone --seed "build a CLI"
    mehr plan --standalone "build a CLI"    # positional argument works too

Examples:
  mehr plan                           # Create specifications for active task
  mehr plan --verbose                 # Show agent output
  mehr plan --full-context            # Include full exploration context
  mehr plan --standalone              # Start standalone planning
  mehr plan --standalone "build CLI"  # Start with seed topic (positional)
  mehr plan --standalone --seed "CLI" # Start with seed topic (flag)`,
	RunE: runPlan,
}

func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.Flags().BoolVar(&planStandalone, "standalone", false, "Start standalone planning without a task")
	planCmd.Flags().StringVarP(&planSeed, "seed", "s", "", "Initial topic for standalone planning")
	planCmd.Flags().BoolVar(&planFullContext, "full-context", false, "Include full exploration context from previous session (default: summary only)")
	planCmd.Flags().StringVar(&planAgentPlanning, "agent-plan", "", "Agent for planning step")
}

func runPlan(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Handle seed from positional arg if not provided via flag
	if planSeed == "" && len(args) > 0 {
		planSeed = args[0]
	}

	// Standalone planning mode
	if planStandalone {
		return runStandalonePlan()
	}

	// Build conductor options using helper
	opts := BuildConductorOptions(CommandOptions{
		Verbose:     verbose,
		FullContext: planFullContext,
	})

	// Per-step agent override for planning
	if planAgentPlanning != "" {
		opts = append(opts, conductor.WithStepAgent("planning", planAgentPlanning))
	}

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, opts...)
	if err != nil {
		return err
	}

	// Check for active task
	if !RequireActiveTask(cond) {
		return nil
	}

	// Set up progress callback using helper
	if verbose {
		SetupVerboseEventHandlers(cond)
	}

	// Enter planning phase
	if err := cond.Plan(ctx); err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	// Run planning with spinner in non-verbose mode
	var planErr error
	if verbose {
		fmt.Println(display.InfoMsg("Planning..."))
		planErr = cond.RunPlanning(ctx)
	} else {
		spinner := display.NewSpinner("Creating specifications...")
		spinner.Start()
		planErr = cond.RunPlanning(ctx)
		if planErr != nil && !errors.Is(planErr, conductor.ErrPendingQuestion) {
			spinner.StopWithError("Planning failed")
		} else if errors.Is(planErr, conductor.ErrPendingQuestion) {
			spinner.Stop()
		} else {
			spinner.StopWithSuccess("Planning complete")
		}
	}
	err = planErr

	// Check if agent asked a question
	if errors.Is(err, conductor.ErrPendingQuestion) {
		q, loadErr := cond.GetWorkspace().LoadPendingQuestion(cond.GetActiveTask().ID)
		if loadErr == nil && q != nil {
			fmt.Println()
			fmt.Println(display.WarningMsg("Agent has a question:"))
			fmt.Println()
			fmt.Printf("  %s\n\n", display.Bold(q.Question))
			if len(q.Options) > 0 {
				fmt.Println(display.Muted("  Options:"))
				for i, opt := range q.Options {
					fmt.Printf("    %s %s", display.Info(fmt.Sprintf("%d.", i+1)), opt.Label)
					if opt.Description != "" {
						fmt.Printf(" %s", display.Muted("- "+opt.Description))
					}
					fmt.Println()
				}
				fmt.Println()
			}
			fmt.Println(display.Muted("Answer with:"))
			fmt.Printf("  %s\n", display.Cyan("mehr answer \"your response\""))
			fmt.Printf("  %s\n", display.Cyan("mehr plan")+" "+display.Muted("(to continue after answering)"))
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("run planning: %w", err)
	}

	// Get status
	status, err := cond.Status()
	if err != nil {
		return err
	}

	if verbose {
		fmt.Println()
		fmt.Println(display.SuccessMsg("Planning complete!"))
	}
	fmt.Printf("  Specifications created: %s\n", display.Bold(strconv.Itoa(status.Specifications)))

	PrintNextSteps(
		"mehr status - View task status and specifications",
		"mehr note - Add notes or clarifications",
		"mehr implement - Implement the specifications",
	)

	return nil
}

// runStandalonePlan runs an interactive planning session without a task.
func runStandalonePlan() error {
	// Get current directory as workspace root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Open workspace
	ws, err := storage.OpenWorkspace(cwd, nil)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}

	// Ensure .task directory exists
	if err := ws.EnsureInitialized(); err != nil {
		return fmt.Errorf("initialize workspace: %w", err)
	}

	// Generate plan ID
	planID := storage.GeneratePlanID()

	// Create the plan
	plan, err := ws.CreatePlan(planID, planSeed)
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}

	fmt.Printf("Standalone planning session started: %s\n", planID)
	fmt.Printf("  History: %s/plan-history.md\n", ws.PlannedPath(planID))
	if planSeed != "" {
		fmt.Printf("  Seed topic: %s\n", planSeed)
	}
	fmt.Println()

	// Start interactive conversation
	fmt.Println("Enter your planning thoughts (type 'quit' or 'exit' to end, 'save' to save and exit):")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// If seed is provided, add it as first entry
	if planSeed != "" {
		if err := ws.AppendPlanHistory(planID, "user", planSeed); err != nil {
			fmt.Printf("Warning: failed to save seed to history: %v\n", err)
		}
		fmt.Printf("Seed recorded: %s\n\n", planSeed)
	}

	for {
		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle exit commands
		switch strings.ToLower(input) {
		case "quit", "exit":
			fmt.Printf("\nPlanning session ended.\n")
			fmt.Printf("  Plan saved to: %s\n", ws.PlannedPath(planID))
			fmt.Printf("  History: %s/plan-history.md\n", ws.PlannedPath(planID))
			fmt.Println("\nTo continue later, review the history file.")
			fmt.Println("To create a task from this plan, copy relevant content to a task file.")

			return nil

		case "save":
			// Update title if we have history
			if len(plan.History) > 0 {
				// Use first line of first entry as title hint
				firstContent := plan.History[0].Content
				if idx := strings.Index(firstContent, "\n"); idx > 0 {
					plan.Title = firstContent[:idx]
				} else if len(firstContent) < 80 {
					plan.Title = firstContent
				}
				_ = ws.SavePlan(plan)
			}
			fmt.Printf("\nPlan saved.\n")
			fmt.Printf("  Location: %s\n", ws.PlannedPath(planID))

			return nil

		case "status":
			plan, _ = ws.LoadPlan(planID)
			fmt.Printf("\nPlan: %s\n", plan.ID)
			fmt.Printf("  Entries: %d\n", len(plan.History))
			fmt.Printf("  Created: %s\n", plan.Created.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Updated: %s\n", plan.Updated.Format("2006-01-02 15:04:05"))
			fmt.Println()

			continue

		case "help":
			fmt.Println("\nCommands:")
			fmt.Println("  quit, exit - End session and save")
			fmt.Println("  save       - Save and exit")
			fmt.Println("  status     - Show plan status")
			fmt.Println("  help       - Show this help")
			fmt.Println("\nAnything else is recorded as a planning entry.")
			fmt.Println()

			continue
		}

		// Record the entry
		if err := ws.AppendPlanHistory(planID, "user", input); err != nil {
			fmt.Printf("Warning: failed to save entry: %v\n", err)
		} else {
			fmt.Println("  (recorded)")
		}

		// Reload plan to get updated history
		plan, _ = ws.LoadPlan(planID)
	}

	//nolint:nilerr // EOF from ReadString ends REPL gracefully
	return nil
}
