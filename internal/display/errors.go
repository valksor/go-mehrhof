package display

import (
	"fmt"
	"strings"
)

// Suggestion represents a suggested action for error recovery.
type Suggestion struct {
	Command     string
	Description string
}

// ErrorWithSuggestions formats an error message with actionable suggestions.
func ErrorWithSuggestions(message string, suggestions []Suggestion) string {
	var sb strings.Builder

	// Error header
	sb.WriteString(ErrorMsg("%s", message))
	sb.WriteString("\n")

	// Add suggestions if any
	if len(suggestions) > 0 {
		sb.WriteString("\n")
		sb.WriteString(Muted("Suggested actions:"))
		sb.WriteString("\n")
		for _, s := range suggestions {
			sb.WriteString(fmt.Sprintf("  %s %s - %s\n",
				Muted("•"),
				Cyan(s.Command),
				s.Description,
			))
		}
	}

	return sb.String()
}

// Common error messages with suggestions

// NoActiveTaskError returns a formatted "no active task" error.
func NoActiveTaskError() string {
	return ErrorWithSuggestions(
		"No active task",
		[]Suggestion{
			{Command: "mehr start <reference>", Description: "Start a new task"},
			{Command: "mehr list", Description: "View all tasks in workspace"},
		},
	)
}
