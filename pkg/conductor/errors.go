package conductor

import (
	"encoding/json"
	"errors"
	"strings"
)

// UserError wraps an error with user-facing context.
type UserError struct {
	Message string `json:"message"` // User-friendly description
	Fix     string `json:"fix"`     // Actionable fix instruction
	Code    string `json:"code"`    // Error category for UI rendering
	Cause   error  `json:"-"`       // Original error (not serialized)
}

func (e *UserError) Error() string {
	return e.Message
}

func (e *UserError) Unwrap() error {
	return e.Cause
}

// MarshalJSON custom serialization including cause string.
func (e *UserError) MarshalJSON() ([]byte, error) {
	type alias UserError

	return json.Marshal(&struct {
		*alias

		Cause string `json:"cause,omitempty"`
	}{
		alias: (*alias)(e),
		Cause: e.causeString(),
	})
}

func (e *UserError) causeString() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}

	return ""
}

// errorPattern maps error substrings to user-friendly messages.
type errorPattern struct {
	Contains string
	Message  string
	Fix      string
	Code     string
}

var errorPatterns = []errorPattern{
	// Agent connectivity
	{
		Contains: "no available agents",
		Message:  "No AI agent is connected",
		Fix:      "Install Claude CLI: https://docs.anthropic.com/en/docs/claude-code/getting-started\nOr run: kvelmo diagnose",
		Code:     "agent_unavailable",
	},
	{
		Contains: "agent connect failed",
		Message:  "Could not connect to AI agent",
		Fix:      "Check that Claude is authenticated: claude auth login",
		Code:     "agent_connect",
	},
	{
		Contains: "connect failed (both WebSocket and CLI)",
		Message:  "Could not connect to AI agent",
		Fix:      "Verify Claude CLI works: claude --version\nThen authenticate: claude auth login",
		Code:     "agent_connect",
	},
	// Pool/worker
	{
		Contains: "no worker pool available",
		Message:  "No worker pool is running",
		Fix:      "Restart kvelmo: kvelmo serve",
		Code:     "pool_unavailable",
	},
	{
		Contains: "pool stopped",
		Message:  "The worker pool has stopped",
		Fix:      "Restart kvelmo: kvelmo serve",
		Code:     "pool_stopped",
	},
	// Git operations
	{
		Contains: "push branch",
		Message:  "Git push failed",
		Fix:      "Check your git credentials and remote access.\nFor GitHub: set token in Settings or run kvelmo config set providers.github.token <token>",
		Code:     "git_push",
	},
	{
		Contains: "create PR",
		Message:  "Could not create pull request",
		Fix:      "Verify the branch has commits and the target branch exists.\nCheck your provider token has sufficient permissions.",
		Code:     "pr_create",
	},
	{
		Contains: "quality gate failed",
		Message:  "Code quality checks did not pass",
		Fix:      "Review the issues listed in the output and fix them.\nOr re-run review to try again: kvelmo review",
		Code:     "quality_gate",
	},
	// Rate limiting
	{
		Contains: "rate_limit",
		Message:  "API rate limit reached",
		Fix:      "Wait a few minutes and retry, or switch to a different model",
		Code:     "rate_limit",
	},
	{
		Contains: "429",
		Message:  "API rate limit reached",
		Fix:      "Wait a few minutes and retry",
		Code:     "rate_limit",
	},
	// Context window
	{
		Contains: "context_length",
		Message:  "Context window exceeded",
		Fix:      "Try simplifying the task or breaking it into smaller steps",
		Code:     "context_overflow",
	},
	{
		Contains: "maximum context",
		Message:  "Context window exceeded",
		Fix:      "Reduce task scope or use kvelmo simplify first",
		Code:     "context_overflow",
	},
	// Process signals
	{
		Contains: "signal: killed",
		Message:  "Agent process was killed (likely OOM)",
		Fix:      "Check system memory. Consider using a smaller model or simpler task",
		Code:     "agent_killed",
	},
	{
		Contains: "signal: terminated",
		Message:  "Agent process was terminated",
		Fix:      "Check if another process killed the agent. Retry with: kvelmo <phase>",
		Code:     "agent_terminated",
	},
	// Timeout
	{
		Contains: "context deadline exceeded",
		Message:  "Operation timed out",
		Fix:      "Increase timeout in settings or break the task into smaller steps",
		Code:     "timeout",
	},
	// Authentication
	{
		Contains: "authentication",
		Message:  "Authentication failed",
		Fix:      "Check your API key or re-authenticate: claude auth login",
		Code:     "auth_failed",
	},
	{
		Contains: "unauthorized",
		Message:  "Authentication failed",
		Fix:      "Your API key may have expired. Re-authenticate: claude auth login",
		Code:     "auth_failed",
	},
	// State machine
	{
		Contains: "no task loaded",
		Message:  "No task is loaded",
		Fix:      "Load a task first: kvelmo start --from <source>",
		Code:     "no_task",
	},
	{
		Contains: "cannot plan",
		Message:  "Cannot start planning",
		Fix:      "Check the task state with: kvelmo status",
		Code:     "state_error",
	},
	{
		Contains: "cannot implement",
		Message:  "Cannot start implementation",
		Fix:      "Run planning first: kvelmo plan",
		Code:     "state_error",
	},
}

// EnrichError wraps a raw error with user-friendly context.
// Returns a UserError if a matching pattern is found, otherwise wraps generically.
func EnrichError(err error, phase string) *UserError {
	if err == nil {
		return nil
	}

	// Check if already a UserError
	var ue *UserError
	if errors.As(err, &ue) {
		return ue
	}

	errStr := err.Error()

	// Try to match against known patterns
	for _, p := range errorPatterns {
		if strings.Contains(errStr, p.Contains) {
			return &UserError{
				Message: p.Message,
				Fix:     p.Fix,
				Code:    p.Code,
				Cause:   err,
			}
		}
	}

	// Generic fallback with phase context
	msg := "An error occurred during " + phase
	if phase == "" {
		msg = "An unexpected error occurred"
	}

	return &UserError{
		Message: msg,
		Fix:     "Check the output for details. Run: kvelmo diagnose",
		Code:    "unknown",
		Cause:   err,
	}
}

// FailureClass categorizes errors for automated handling.
type FailureClass string

const (
	// FailureClassHardStop requires human intervention (git conflict, auth failure, agent crash).
	FailureClassHardStop FailureClass = "hard_stop"
	// FailureClassRecoverable can be auto-retried (rate limit, network timeout, context overflow).
	FailureClassRecoverable FailureClass = "recoverable"
	// FailureClassDegraded allows continuing with a warning (optional check unavailable).
	FailureClassDegraded FailureClass = "degraded"
	// FailureClassSkippable means the phase has nothing to do (empty diff, no changes).
	FailureClassSkippable FailureClass = "skippable"
)

// ClassifyError determines the failure class of an error based on its content and phase.
func ClassifyError(err error, phase string) FailureClass {
	if err == nil {
		return "" // No error, no classification needed
	}

	errStr := err.Error()

	// Recoverable: transient failures that can be retried
	recoverablePatterns := []string{
		"rate_limit", "429", "context_length", "maximum context",
		"context deadline exceeded", "connection refused", "connection reset",
		"temporary failure", "i/o timeout", "EOF",
	}
	for _, p := range recoverablePatterns {
		if strings.Contains(errStr, p) {
			return FailureClassRecoverable
		}
	}

	// Skippable: nothing to do (only for optional phases)
	if phase == "simplify" || phase == "optimize" {
		skippablePatterns := []string{
			"no changes", "nothing to simplify", "nothing to optimize",
			"empty diff", "no modifications",
		}
		for _, p := range skippablePatterns {
			if strings.Contains(errStr, p) {
				return FailureClassSkippable
			}
		}
	}

	// Degraded: optional subsystem failure
	degradedPatterns := []string{
		"memory indexer", "quality check unavailable", "optional",
	}
	for _, p := range degradedPatterns {
		if strings.Contains(errStr, p) {
			return FailureClassDegraded
		}
	}

	// Default: hard stop
	return FailureClassHardStop
}
