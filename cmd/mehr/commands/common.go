package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/agent/claude"
	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/events"
	"github.com/valksor/go-mehrhof/internal/output"
	"github.com/valksor/go-mehrhof/internal/provider/directory"
	"github.com/valksor/go-mehrhof/internal/provider/file"
	"github.com/valksor/go-mehrhof/internal/provider/github"
	"github.com/valksor/go-mehrhof/internal/provider/gitlab"
	"github.com/valksor/go-mehrhof/internal/provider/jira"
	"github.com/valksor/go-mehrhof/internal/provider/linear"
	"github.com/valksor/go-mehrhof/internal/provider/notion"
	"github.com/valksor/go-mehrhof/internal/provider/wrike"
	"github.com/valksor/go-mehrhof/internal/provider/youtrack"
)

var (
	dedupStdout sync.Once
	dedupWriter *output.DeduplicatingWriter
)

// getDeduplicatingStdout returns a deduplicating writer that wraps os.Stdout.
// The writer suppresses consecutive identical lines.
// Uses sync.Once to ensure thread-safe initialization.
func getDeduplicatingStdout() io.Writer {
	dedupStdout.Do(func() {
		dedupWriter = output.NewDeduplicatingWriter(os.Stdout)
	})
	return dedupWriter
}

// initializeConductor creates and initializes a conductor with the standard
// providers (file, directory) and agents (claude) registered.
//
// This is the common initialization sequence used by most commands.
// Options should be built by the caller to customize behavior per command.
func initializeConductor(ctx context.Context, opts ...conductor.Option) (*conductor.Conductor, error) {
	// Create conductor with provided options
	cond, err := conductor.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create conductor: %w", err)
	}

	// Register standard providers
	file.Register(cond.GetProviderRegistry())
	directory.Register(cond.GetProviderRegistry())
	github.Register(cond.GetProviderRegistry())
	gitlab.Register(cond.GetProviderRegistry())
	wrike.Register(cond.GetProviderRegistry())
	linear.Register(cond.GetProviderRegistry())
	jira.Register(cond.GetProviderRegistry())
	notion.Register(cond.GetProviderRegistry())
	youtrack.Register(cond.GetProviderRegistry())

	// Register standard agents
	if err := claude.Register(cond.GetAgentRegistry()); err != nil {
		return nil, fmt.Errorf("register claude agent: %w", err)
	}

	// Initialize the conductor (loads workspace, detects agent, etc.)
	if err := cond.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}

	return cond, nil
}

// confirmAction prompts the user for confirmation unless skipConfirm is true.
// Returns true if the action should proceed, false if cancelled.
// The prompt parameter should describe what will happen (e.g., "delete this task").
func confirmAction(prompt string, skipConfirm bool) (bool, error) {
	if skipConfirm {
		return true, nil
	}

	fmt.Printf("%s\nAre you sure? [y/N]: ", prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper functions for reducing command duplication
// ─────────────────────────────────────────────────────────────────────────────

// CommandOptions holds common options for command execution.
type CommandOptions struct {
	Verbose     bool
	DryRun      bool
	StepAgent   string // Per-step agent override (e.g., "planning", "implementing")
	FullContext bool
}

// BuildConductorOptions creates conductor options from command options.
// This centralizes the common pattern of building options.
func BuildConductorOptions(cmdOpts CommandOptions) []conductor.Option {
	opts := []conductor.Option{
		conductor.WithVerbose(cmdOpts.Verbose),
	}

	if cmdOpts.DryRun {
		opts = append(opts, conductor.WithDryRun(true))
	}

	if cmdOpts.FullContext {
		opts = append(opts, conductor.WithIncludeFullContext(true))
	}

	if cmdOpts.StepAgent != "" {
		// Derive step name from the step agent variable name
		// e.g., "planAgentPlanning" -> "planning"
		stepName := deriveStepName(cmdOpts.StepAgent)
		if stepName != "" {
			opts = append(opts, conductor.WithStepAgent(stepName, cmdOpts.StepAgent))
		}
	}

	// Use deduplicating stdout in verbose mode
	if cmdOpts.Verbose {
		opts = append(opts, conductor.WithStdout(getDeduplicatingStdout()))
	}

	return opts
}

// deriveStepName converts a step agent variable suffix to a step name.
// For example: "planning" -> "planning", "implementing" -> "implementing"
// This is used when the agent name is passed directly without a step prefix.
func deriveStepName(agentVar string) string {
	// Map common variable names to step names
	stepMap := map[string]string{
		"planning":      "planning",
		"implementing":  "implementing",
		"implement":     "implementing",
		"reviewing":     "reviewing",
		"review":        "reviewing",
		"chat":          "dialogue",
		"checkpointing": "checkpointing",
	}

	if step, ok := stepMap[strings.ToLower(agentVar)]; ok {
		return step
	}
	return ""
}

// RequireActiveTask checks for an active task and prints an error if none exists.
// Returns true if an active task exists, false otherwise.
func RequireActiveTask(cond *conductor.Conductor) bool {
	if cond.GetActiveTask() == nil {
		fmt.Print(display.ErrorWithSuggestions(
			"No active task",
			[]display.Suggestion{
				{Command: "mehr start <reference>", Description: "Register a task first"},
				{Command: "mehr plan --new", Description: "Start standalone planning"},
			},
		))
		return false
	}
	return true
}

// SetupVerboseEventHandlers subscribes to common events for verbose output.
// This centralizes the verbose event subscription pattern.
func SetupVerboseEventHandlers(cond *conductor.Conductor) {
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
		case events.TypeAgentMessage:
			if agentEvent, ok := e.Data["event"].(agent.Event); ok {
				printAgentEventTo(w, agentEvent)
			}
		}
	})
}

// RunWithSpinner runs the given function with a spinner in non-verbose mode.
// In verbose mode, it runs without a spinner and prints the start message.
func RunWithSpinner(verbose bool, startMsg, spinnerMsg, successMsg string, fn func() error) error {
	if verbose {
		fmt.Println(display.InfoMsg("%s", startMsg))
		err := fn()
		if err == nil {
			fmt.Println(display.SuccessMsg("%s", successMsg))
		}
		return err
	}

	spinner := display.NewSpinner(spinnerMsg)
	spinner.Start()
	err := fn()
	if err != nil {
		spinner.StopWithError("Operation failed")
	} else {
		spinner.StopWithSuccess(successMsg)
	}
	return err
}

// PrintNextSteps prints common next steps after a command completes.
func PrintNextSteps(steps ...string) {
	fmt.Println()
	fmt.Println(display.Muted("Next steps:"))
	for _, step := range steps {
		fmt.Printf("  %s\n", display.Cyan(step))
	}
	fmt.Println()
}

// printAgentEventTo prints meaningful content from agent events to the specified writer.
// This is exported so it can be used by other commands.
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

// printToolCallTo prints a formatted tool call to the specified writer.
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
