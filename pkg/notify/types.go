package notify

// NotificationType defines a category of notification.
type NotificationType struct {
	Name        string `json:"name"`        // e.g., "task.phase_complete"
	Description string `json:"description"` // Human-readable
	Default     bool   `json:"default"`     // Enabled by default
}

// Built-in notification types.
var (
	TypePhaseComplete = NotificationType{Name: "task.phase_complete", Description: "Task phase completed", Default: true}
	TypeReviewNeeded  = NotificationType{Name: "task.review_needed", Description: "Task needs review", Default: true}
	TypeFailed        = NotificationType{Name: "task.failed", Description: "Task failed", Default: true}
	TypeSubmitted     = NotificationType{Name: "task.submitted", Description: "PR submitted", Default: true}
	TypeAgentError    = NotificationType{Name: "agent.error", Description: "Agent encountered an error", Default: true}
	TypeGateFailed    = NotificationType{Name: "quality.gate_failed", Description: "Quality gate failed", Default: false}
)

// allTypes is the registry of built-in notification types.
var allTypes = []NotificationType{
	TypePhaseComplete,
	TypeReviewNeeded,
	TypeFailed,
	TypeSubmitted,
	TypeAgentError,
	TypeGateFailed,
}

// AllTypes returns all registered notification types.
func AllTypes() []NotificationType {
	out := make([]NotificationType, len(allTypes))
	copy(out, allTypes)

	return out
}

// LookupType returns the notification type with the given name and true,
// or a zero value and false if not found.
func LookupType(name string) (NotificationType, bool) {
	for _, t := range allTypes {
		if t.Name == name {
			return t, true
		}
	}

	return NotificationType{}, false
}
