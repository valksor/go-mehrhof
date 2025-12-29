package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
)

var (
	chatContinue      bool
	chatSession       string
	chatAgentDialogue string
)

var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Add notes or discuss the task with the agent",
	Long: `Enter dialogue mode to add notes, discuss requirements, or get clarifications.

This command can be used at ANY state (idle, after planning, after implementation, etc.)
to add context, ask questions, or refine understanding.

Notes from chat sessions are saved to notes.md in the work directory and are
included in subsequent planning and implementation prompts.

If a message is provided, it will be sent as the initial prompt.
Otherwise, enters an interactive loop reading from stdin.

Examples:
  mehr chat                                # Enter interactive mode
  mehr chat "The API should use REST"      # Add a note
  mehr chat "What's the best approach?"    # Ask a question
  mehr chat --continue                     # Continue previous session`,
	RunE: runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)

	chatCmd.Flags().BoolVarP(&chatContinue, "continue", "c", false, "Continue previous session")
	chatCmd.Flags().StringVarP(&chatSession, "session", "s", "", "Specific session file to continue")
	chatCmd.Flags().StringVar(&chatAgentDialogue, "agent-chat", "", "Agent for dialogue/chat step")
}

func runChat(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Build conductor options
	opts := []conductor.Option{
		conductor.WithVerbose(verbose),
	}

	// Per-step agent override
	if chatAgentDialogue != "" {
		opts = append(opts, conductor.WithStepAgent("dialogue", chatAgentDialogue))
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

	chatOpts := conductor.ChatOptions{
		Continue:    chatContinue,
		SessionFile: chatSession,
	}

	taskID := cond.GetActiveTask().ID
	ws := cond.GetWorkspace()

	// If message provided as argument, send it and exit
	if len(args) > 0 {
		message := strings.Join(args, " ")

		// Check if this is answering a pending question
		if ws.HasPendingQuestion(taskID) {
			q, _ := ws.LoadPendingQuestion(taskID)
			fmt.Printf("Answering question...\n")

			// Add the answer as a note with context
			answerNote := fmt.Sprintf("**Q:** %s\n\n**A:** %s", q.Question, message)
			if err := ws.AppendNote(taskID, answerNote, "answer"); err != nil {
				return fmt.Errorf("save answer: %w", err)
			}

			// Clear the pending question
			_ = ws.ClearPendingQuestion(taskID)

			fmt.Println("Answer recorded.")
			fmt.Println("\nRun 'mehr plan' to continue planning with your answer.")
			return nil
		}

		fmt.Printf("Sending message...\n")
		if err := cond.Chat(ctx, message, chatOpts); err != nil {
			return fmt.Errorf("chat: %w", err)
		}
		fmt.Println("Note added to task.")
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

		if err := cond.Chat(ctx, message, chatOpts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	return nil
}
