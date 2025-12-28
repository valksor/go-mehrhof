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

// TaskAlreadyActiveError returns a formatted "task already active" error.
func TaskAlreadyActiveError(taskID string) string {
	return ErrorWithSuggestions(
		fmt.Sprintf("A task is already active: %s", taskID),
		[]Suggestion{
			{Command: "mehr status", Description: "View current task status"},
			{Command: "mehr finish", Description: "Complete the current task"},
			{Command: "mehr delete", Description: "Delete the current task"},
			{Command: "mehr start --worktree", Description: "Start in parallel worktree"},
		},
	)
}

// NoSpecificationsError returns a formatted "no specifications" error.
func NoSpecificationsError() string {
	return ErrorWithSuggestions(
		"No specifications to implement",
		[]Suggestion{
			{Command: "mehr plan", Description: "Create implementation specifications"},
			{Command: "mehr talk", Description: "Discuss the task with the agent"},
		},
	)
}

// NotInWorkspaceError returns a formatted "not in workspace" error.
func NotInWorkspaceError() string {
	return ErrorWithSuggestions(
		"Not in a mehrhof workspace",
		[]Suggestion{
			{Command: "mehr init", Description: "Initialize a new workspace"},
		},
	)
}

// InvalidStateError returns a formatted invalid state transition error.
func InvalidStateError(currentState, expectedStates string) string {
	return ErrorWithSuggestions(
		fmt.Sprintf("Cannot perform this action in '%s' state", currentState),
		[]Suggestion{
			{Command: "mehr status", Description: "Check current task state"},
			{Command: "mehr continue", Description: "See suggested next actions"},
		},
	)
}

// AgentNotAvailableError returns a formatted agent unavailable error.
func AgentNotAvailableError(agentName string) string {
	return ErrorWithSuggestions(
		fmt.Sprintf("Agent '%s' is not available", agentName),
		[]Suggestion{
			{Command: "mehr agents list", Description: "View available agents"},
			{Command: "mehr agents check", Description: "Check agent availability"},
		},
	)
}

// BranchExistsError returns a formatted branch already exists error.
func BranchExistsError(branchName string) string {
	return ErrorWithSuggestions(
		fmt.Sprintf("Branch '%s' already exists", branchName),
		[]Suggestion{
			{Command: "mehr start --key=<new-key>", Description: "Use a different task key"},
			{Command: "git branch -d " + branchName, Description: "Delete the existing branch"},
		},
	)
}

// ProviderError returns a formatted provider error.
func ProviderError(provider, message string) string {
	suggestions := []Suggestion{
		{Command: "mehr start file:<path>", Description: "Use file provider"},
		{Command: "mehr start dir:<path>", Description: "Use directory provider"},
	}

	if provider == "github" {
		suggestions = append(suggestions,
			Suggestion{Command: "export GITHUB_TOKEN=<token>", Description: "Set GitHub token"},
		)
	}

	return ErrorWithSuggestions(
		fmt.Sprintf("Provider '%s' error: %s", provider, message),
		suggestions,
	)
}

// PrintErrorWithSuggestions prints an error with suggestions to stdout.
func PrintErrorWithSuggestions(message string, suggestions []Suggestion) {
	fmt.Print(ErrorWithSuggestions(message, suggestions))
}
