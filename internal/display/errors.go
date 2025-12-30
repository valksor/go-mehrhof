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

// ErrorWithContext formats an error with contextual information and suggestions.
func ErrorWithContext(err error, context string, suggestions []Suggestion) string {
	var sb strings.Builder

	// Context header
	sb.WriteString(ErrorMsg("Error: %s", context))
	sb.WriteString("\n")

	// Add the underlying error if present
	if err != nil {
		sb.WriteString(Muted(fmt.Sprintf("  Cause: %v", err)))
		sb.WriteString("\n")
	}

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

// ValidationError formats a validation error with field-specific suggestions.
func ValidationError(field string, message string, suggestions []Suggestion) string {
	var sb strings.Builder

	sb.WriteString(ErrorMsg("Validation Error: %s", field))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s\n", message))

	if len(suggestions) > 0 {
		sb.WriteString("\n")
		sb.WriteString(Muted("Fix:"))
		sb.WriteString("\n")
		for _, s := range suggestions {
			sb.WriteString(fmt.Sprintf("  %s %s\n",
				Muted("•"),
				s.Description,
			))
		}
	}

	return sb.String()
}

// ProviderError formats a provider-specific error with contextual help.
func ProviderError(provider string, err error, suggestions []Suggestion) string {
	return ErrorWithContext(
		err,
		fmt.Sprintf("Failed to load from %s provider", provider),
		suggestions,
	)
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

// TaskFailedError returns a formatted task failure error with next steps.
func TaskFailedError(step string, err error) string {
	return ErrorWithContext(
		err,
		fmt.Sprintf("Task failed during %s", step),
		[]Suggestion{
			{Command: "mehr status", Description: "View detailed error information"},
			{Command: "mehr note", Description: "Add notes about the error"},
			{Command: "mehr undo", Description: "Revert last changes if applicable"},
		},
	)
}

// ConfigError returns a formatted configuration error.
func ConfigError(err error, configPath string) string {
	return ErrorWithContext(
		err,
		fmt.Sprintf("Configuration error in %s", configPath),
		[]Suggestion{
			{Command: "mehr config validate", Description: "Validate workspace configuration"},
			{Command: "cat .mehrhof/config.yaml", Description: "View configuration file"},
		},
	)
}

// AgentError returns a formatted agent-related error.
func AgentError(agent string, err error) string {
	return ErrorWithContext(
		err,
		fmt.Sprintf("Agent error: %s", agent),
		[]Suggestion{
			{Command: "mehr agents list", Description: "List available agents"},
			{Command: "mehr --agent=<name> <ref>", Description: "Try with a different agent"},
		},
	)
}

// GitError returns a formatted git-related error.
func GitError(operation string, err error) string {
	return ErrorWithContext(
		err,
		fmt.Sprintf("Git %s failed", operation),
		[]Suggestion{
			{Command: "git status", Description: "Check git repository status"},
			{Command: "mehr start --no-branch <ref>", Description: "Skip branch creation"},
		},
	)
}
