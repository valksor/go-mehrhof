package commands

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/display"
)

var noteCmd = &cobra.Command{
	Use:     "note [message]",
	Aliases: []string{"answer"},
	Short:   "Add notes to the task or answer agent questions",
	Long: `Add notes, context, or requirements to the current task.

This command saves your input directly to notes.md in the work directory.
Notes are included when the agent runs during plan/implement/review phases.

Use this to add requirements, clarify specifications, or provide context
before running plan/implement. The agent will see your notes when processing.

If an agent question is pending (waiting for your response), this command
will submit your answer and clear the pending question state.

ALIASES:
  note                        General note-taking
  answer                      Submit answer to pending agent question

Examples:
  mehr note                                # Enter interactive mode
  mehr note "The API should use REST"      # Add a note
  mehr answer "Use PostgreSQL"              # Answer agent question
  mehr note "Add error handling"            # Add context before planning`,
	RunE: runNote,
}

func init() {
	rootCmd.AddCommand(noteCmd)
}

func runNote(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Build conductor options
	opts := []conductor.Option{
		conductor.WithVerbose(verbose),
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

	taskID := cond.GetActiveTask().ID
	ws := cond.GetWorkspace()

	// Helper function to save a note
	saveNote := func(message string) error {
		if ws.HasPendingQuestion(taskID) {
			q, _ := ws.LoadPendingQuestion(taskID)
			note := fmt.Sprintf("**Q:** %s\n\n**A:** %s", q.Question, message)
			if err := ws.AppendNote(taskID, note, "answer"); err != nil {
				return fmt.Errorf("save answer: %w", err)
			}
			_ = ws.ClearPendingQuestion(taskID)

			return nil
		}

		return ws.AppendNote(taskID, message, cond.GetActiveTask().State)
	}

	// If message provided as argument, save it and exit
	if len(args) > 0 {
		message := strings.Join(args, " ")

		if err := saveNote(message); err != nil {
			return fmt.Errorf("save note: %w", err)
		}

		fmt.Println("Note saved.")
		if ws.HasPendingQuestion(taskID) {
			fmt.Println("\nRun 'mehr plan' to continue planning with your answer.")
		}

		return nil
	}

	// Interactive mode
	scanner := bufio.NewScanner(os.Stdin)

	status, _ := cond.Status()
	fmt.Printf("Task: %s (state: %s)\n", status.TaskID, status.State)

	// Show pending question if any
	if ws.HasPendingQuestion(taskID) {
		q, _ := ws.LoadPendingQuestion(taskID)
		fmt.Printf("\n⚠ Pending question from agent:\n  %s\n", q.Question)
		if len(q.Options) > 0 {
			fmt.Println("  Options:")
			for i, opt := range q.Options {
				fmt.Printf("    %d. %s\n", i+1, opt.Label)
			}
		}
		fmt.Println("\nType your answer, then run 'mehr plan' to continue.")
	}

	fmt.Println("\nEntering interactive mode. Type 'exit' or 'quit' to leave.")
	fmt.Println()

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		message := strings.TrimSpace(scanner.Text())
		if message == "" {
			continue
		}

		if message == "exit" || message == "quit" {
			break
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := saveNote(message); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)

			continue
		}

		fmt.Println("Note saved.")
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	return nil
}
