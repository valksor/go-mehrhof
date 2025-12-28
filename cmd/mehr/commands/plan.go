package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/events"
	"github.com/valksor/go-mehrhof/internal/storage"
)

var (
	planStandalone    bool
	planSeed          string
	planFullContext   bool
	planAgentPlanning string // Per-step agent override
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Create implementation specifications for the active task",
	Long: `Run the planning phase to analyze the task and create specification files.

The agent will read the source content and any notes, then generate
structured specifications (specification-N.md files) in the work directory.

You can run this multiple times to create additional specs.

With --new, you can start a standalone planning session without an active
task. This is useful for exploring requirements before creating a formal task.
Plans are saved to .task/planned/ directory.

Examples:
  mehr plan                    # Create specs for active task
  mehr plan --verbose          # Show agent output
  mehr plan --new              # Start standalone planning
  mehr plan --new "build a CLI"  # Start with seed topic`,
	RunE: runPlan,
}

func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.Flags().BoolVarP(&planStandalone, "new", "n", false, "Start standalone planning without a task")
	planCmd.Flags().StringVarP(&planSeed, "seed", "s", "", "Initial topic for standalone planning")
	planCmd.Flags().BoolVar(&planFullContext, "full-context", false, "Include full exploration context from previous session (default: summary only)")
	planCmd.Flags().StringVar(&planAgentPlanning, "agent-planning", "", "Agent for planning step")
}

func runPlan(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Handle seed from positional arg if not provided via flag
	if planSeed == "" && len(args) > 0 {
		planSeed = args[0]
	}

	// Standalone planning mode
	if planStandalone {
		return runStandalonePlan(cmd)
	}

	// Build conductor options
	opts := []conductor.Option{
		conductor.WithVerbose(cfg.UI.Verbose),
		conductor.WithIncludeFullContext(planFullContext),
	}

	// Per-step agent override
	if planAgentPlanning != "" {
		opts = append(opts, conductor.WithStepAgent("planning", planAgentPlanning))
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
		return fmt.Errorf("no active task\nUse 'mehr start <reference>' to register a task first\nOr use 'mehr plan --new' for standalone planning")
	}

	// Set up progress callback
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
			case events.TypeAgentMessage:
				if agentEvent, ok := e.Data["event"].(agent.Event); ok {
					// Extract meaningful content from agent event
					printAgentEventTo(w, agentEvent)
				}
			}
		})
	}

	// Enter planning phase
	if err := cond.Plan(ctx); err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	// Run planning
	fmt.Println("Planning...")
	err = cond.RunPlanning(ctx)

	// Check if agent asked a question
	if errors.Is(err, conductor.ErrPendingQuestion) {
		q, loadErr := cond.GetWorkspace().LoadPendingQuestion(cond.GetActiveTask().ID)
		if loadErr == nil && q != nil {
			fmt.Printf("\n⚠ Agent has a question:\n\n")
			fmt.Printf("  %s\n\n", q.Question)
			if len(q.Options) > 0 {
				fmt.Println("  Options:")
				for i, opt := range q.Options {
					fmt.Printf("    %d. %s", i+1, opt.Label)
					if opt.Description != "" {
						fmt.Printf(" - %s", opt.Description)
					}
					fmt.Println()
				}
				fmt.Println()
			}
			fmt.Println("Answer with:")
			fmt.Println("  mehr talk \"your answer\"")
			fmt.Println("  mehr plan              (to continue after answering)")
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

	fmt.Printf("\nPlanning complete!\n")
	fmt.Printf("  Specifications created: %d\n", status.Specifications)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  mehr status    - View mehr status and specifications\n")
	fmt.Printf("  mehr talk      - Add notes or clarifications\n")
	fmt.Printf("  mehr implement - Implement the specifications\n")

	return nil
}

// runStandalonePlan runs an interactive planning session without a task
func runStandalonePlan(cmd *cobra.Command) error {
	// Get current directory as workspace root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Open workspace
	ws, err := storage.OpenWorkspace(cwd)
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

	return nil
}

// printAgentEventTo prints meaningful content from agent events to the specified writer
func printAgentEventTo(w io.Writer, e agent.Event) {
	// Print text content if available
	if e.Text != "" {
		_, err := fmt.Fprint(w, e.Text)
		if err != nil {
			log.Println(err)
		}
	}

	// Print tool call if available
	if e.ToolCall != nil {
		printToolCallTo(w, e.ToolCall)
	}

	// Also check tool_calls array for multiple tools
	if toolCalls, ok := e.Data["tool_calls"].([]*agent.ToolCall); ok {
		for _, tc := range toolCalls {
			printToolCallTo(w, tc)
		}
	}

	// Fallback: check for result in data
	if e.Text == "" && e.ToolCall == nil {
		if result, ok := e.Data["result"].(string); ok {
			_, err := fmt.Fprint(w, result)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

// printToolCallTo prints a formatted tool call to the specified writer
func printToolCallTo(w io.Writer, tc *agent.ToolCall) {
	if tc == nil {
		return
	}

	if tc.Description != "" {
		_, err := fmt.Fprintf(w, "→ %s: %s\n", tc.Name, tc.Description)
		if err != nil {
			log.Println(err)
		}
	} else {
		_, err := fmt.Fprintf(w, "→ %s\n", tc.Name)
		if err != nil {
			log.Println(err)
		}
	}
}
