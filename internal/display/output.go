// Package display provides user-friendly formatting for CLI output.
package display

import (
	"fmt"
	"strings"

	"github.com/valksor/go-mehrhof/internal/workflow"
)

// TaskInfo holds task information for consistent display.
type TaskInfo struct {
	TaskID      string
	Title       string
	ExternalKey string
	State       string
	Source      string
	Branch      string
	Worktree    string
	Started     string
}

// TaskInfoOptions controls what fields to show in task info output.
type TaskInfoOptions struct {
	ShowTitle    bool
	ShowKey      bool
	ShowState    bool
	ShowSource   bool
	ShowBranch   bool
	ShowWorktree bool
	ShowStarted  bool
	Compact      bool // If true, uses shorter format without state description
}

// DefaultTaskInfoOptions returns options that show all non-empty fields.
func DefaultTaskInfoOptions() TaskInfoOptions {
	return TaskInfoOptions{
		ShowTitle:    true,
		ShowKey:      true,
		ShowState:    true,
		ShowSource:   true,
		ShowBranch:   true,
		ShowWorktree: true,
		ShowStarted:  true,
		Compact:      false,
	}
}

// FormatTaskInfo formats task information consistently across all commands.
// Returns a formatted string ready to print.
func FormatTaskInfo(header string, info TaskInfo, opts TaskInfoOptions) string {
	var sb strings.Builder

	// Header line: "Task: abc123" or "Task started: abc123"
	sb.WriteString(fmt.Sprintf("%s: %s\n", header, Bold(info.TaskID)))

	// Key-value pairs with consistent 10-char alignment
	if opts.ShowTitle && info.Title != "" {
		sb.WriteString(fmt.Sprintf("  %-10s%s\n", "Title:", info.Title))
	}

	if opts.ShowKey && info.ExternalKey != "" {
		sb.WriteString(fmt.Sprintf("  %-10s%s\n", "Key:", info.ExternalKey))
	}

	if opts.ShowState && info.State != "" {
		stateStr := FormatStateStringColored(info.State)
		if !opts.Compact {
			desc := StateDescription[workflow.State(info.State)]
			if desc != "" {
				stateStr += " - " + Muted(desc)
			}
		}
		sb.WriteString(fmt.Sprintf("  %-10s%s\n", "State:", stateStr))
	}

	if opts.ShowSource && info.Source != "" {
		sb.WriteString(fmt.Sprintf("  %-10s%s\n", "Source:", info.Source))
	}

	if opts.ShowBranch && info.Branch != "" {
		sb.WriteString(fmt.Sprintf("  %-10s%s\n", "Branch:", info.Branch))
	}

	if opts.ShowWorktree && info.Worktree != "" {
		sb.WriteString(fmt.Sprintf("  %-10s%s\n", "Worktree:", info.Worktree))
	}

	if opts.ShowStarted && info.Started != "" {
		sb.WriteString(fmt.Sprintf("  %-10s%s\n", "Started:", info.Started))
	}

	return sb.String()
}

// NextStep represents a single next step suggestion.
type NextStep struct {
	Command     string
	Description string
}

// FormatNextSteps formats the "Next steps:" section consistently.
func FormatNextSteps(steps []NextStep) string {
	if len(steps) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(Muted("Next steps:"))
	sb.WriteString("\n")

	// Find the longest command for alignment
	maxLen := 0
	for _, s := range steps {
		if len(s.Command) > maxLen {
			maxLen = len(s.Command)
		}
	}

	// Format each step: "  command     - description"
	for _, s := range steps {
		sb.WriteString(fmt.Sprintf("  %-*s  %s\n",
			maxLen,
			Cyan(s.Command),
			Muted("- "+s.Description),
		))
	}

	return sb.String()
}

// PrintNextSteps prints next steps with consistent formatting.
// Convenience function that prints directly.
func PrintNextSteps(steps ...string) {
	if len(steps) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(Muted("Next steps:"))

	for _, step := range steps {
		// Parse "command - description" format
		parts := strings.SplitN(step, " - ", 2)
		if len(parts) == 2 {
			fmt.Printf("  %s  %s\n", Cyan(parts[0]), Muted("- "+parts[1]))
		} else {
			fmt.Printf("  %s\n", Cyan(step))
		}
	}
}

// FormatConfirmation formats a confirmation prompt consistently.
// summary: Main action being confirmed (e.g., "Finish task: abc123")
// details: Optional list of details to show (e.g., title, branch)
// warning: Optional warning message to show (highlighted in yellow)
func FormatConfirmation(summary string, details []string, warning string) string {
	var sb strings.Builder

	sb.WriteString(Bold(summary))
	sb.WriteString("\n")

	for _, d := range details {
		sb.WriteString(fmt.Sprintf("  %s\n", d))
	}

	if warning != "" {
		sb.WriteString("\n")
		sb.WriteString(WarningMsg("%s", warning))
		sb.WriteString("\n")
	}

	return sb.String()
}
