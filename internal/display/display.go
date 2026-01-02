// Package display provides user-friendly formatting for internal state values.
// This separates display concerns from internal state representation, allowing
// user-facing text to evolve without breaking stored YAML data.
package display

import (
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

// StateDisplay maps internal workflow state values to user-friendly names.
var StateDisplay = map[workflow.State]string{
	workflow.StateIdle:          "Ready",
	workflow.StatePlanning:      "Planning",
	workflow.StateImplementing:  "Implementing",
	workflow.StateReviewing:     "Reviewing",
	workflow.StateDone:          "Completed",
	workflow.StateFailed:        "Failed",
	workflow.StateWaiting:       "Waiting",
	workflow.StateCheckpointing: "Checkpointing",
	workflow.StateReverting:     "Reverting",
	workflow.StateRestoring:     "Restoring",
}

// StateDescription provides additional context for each state.
var StateDescription = map[workflow.State]string{
	workflow.StateIdle:          "Ready to start",
	workflow.StatePlanning:      "AI is creating specifications",
	workflow.StateImplementing:  "AI is generating code",
	workflow.StateReviewing:     "Code review in progress",
	workflow.StateDone:          "Task completed successfully",
	workflow.StateFailed:        "Task failed with error",
	workflow.StateWaiting:       "Action required: Awaiting your response",
	workflow.StateCheckpointing: "Creating checkpoint",
	workflow.StateReverting:     "Reverting to previous state",
	workflow.StateRestoring:     "Restoring from checkpoint",
}

// StateAccessiblePrefix provides short text prefixes for accessibility.
// These help color-blind users distinguish states without relying on color alone.
var StateAccessiblePrefix = map[workflow.State]string{
	workflow.StateIdle:          "[R]", // Ready
	workflow.StatePlanning:      "[P]", // Planning
	workflow.StateImplementing:  "[I]", // Implementing
	workflow.StateReviewing:     "[V]", // reViewing
	workflow.StateDone:          "[D]", // Done
	workflow.StateFailed:        "[F]", // Failed
	workflow.StateWaiting:       "[W]", // Waiting
	workflow.StateCheckpointing: "[C]", // Checkpointing
	workflow.StateReverting:     "[U]", // Undoing
	workflow.StateRestoring:     "[S]", // reStoring
}

// SpecificationStatusDisplay maps internal specification status values to user-friendly names.
var SpecificationStatusDisplay = map[string]string{
	storage.SpecificationStatusDraft:        "Draft",
	storage.SpecificationStatusReady:        "Ready",
	storage.SpecificationStatusImplementing: "Implementing",
	storage.SpecificationStatusDone:         "Completed",
}

// SpecificationStatusIcon returns the visual icon for a specification status.
var SpecificationStatusIcon = map[string]string{
	storage.SpecificationStatusDraft:        "○", // empty circle
	storage.SpecificationStatusReady:        "◐", // half-filled
	storage.SpecificationStatusImplementing: "◑", // half-filled alternate
	storage.SpecificationStatusDone:         "●", // filled circle
}

// FormatState returns the user-friendly display name for a workflow state.
// Falls back to the raw state string if not found in the mapping.
func FormatState(state workflow.State) string {
	if name, ok := StateDisplay[state]; ok {
		return name
	}
	return string(state)
}

// FormatStateString returns the user-friendly display name for a state string.
func FormatStateString(state string) string {
	return FormatState(workflow.State(state))
}

// GetStateDescription returns a brief description of what the state means.
func GetStateDescription(state workflow.State) string {
	if desc, ok := StateDescription[state]; ok {
		return desc
	}
	return ""
}

// FormatSpecificationStatus returns the user-friendly display name for a specification status.
// Falls back to the raw status string if not found in the mapping.
func FormatSpecificationStatus(status string) string {
	if name, ok := SpecificationStatusDisplay[status]; ok {
		return name
	}
	return status
}

// GetSpecificationStatusIcon returns the icon for a specification status.
// Returns "?" for unknown statuses.
func GetSpecificationStatusIcon(status string) string {
	if icon, ok := SpecificationStatusIcon[status]; ok {
		return icon
	}
	return "?"
}

// FormatSpecificationStatusWithIcon returns "icon status" format for display.
func FormatSpecificationStatusWithIcon(status string) string {
	return GetSpecificationStatusIcon(status) + " " + FormatSpecificationStatus(status)
}

// Color-aware formatting functions

// GetStateAccessiblePrefix returns the accessibility prefix for a state.
func GetStateAccessiblePrefix(state workflow.State) string {
	if prefix, ok := StateAccessiblePrefix[state]; ok {
		return prefix
	}
	return "[?]"
}

// FormatStateColored returns a colored state display name with accessibility prefix.
// Format: "[P] Planning" where the prefix is muted and the name is colored.
func FormatStateColored(state workflow.State) string {
	prefix := Muted(GetStateAccessiblePrefix(state))
	displayName := FormatState(state)
	coloredName := ColorState(string(state), displayName)
	return prefix + " " + coloredName
}

// FormatStateStringColored returns a colored state display name from a string.
func FormatStateStringColored(state string) string {
	return FormatStateColored(workflow.State(state))
}

// FormatSpecificationStatusColored returns a colored specification status.
func FormatSpecificationStatusColored(status string) string {
	displayName := FormatSpecificationStatus(status)
	return ColorSpecStatus(status, displayName)
}

// FormatSpecificationStatusWithIconColored returns colored "icon status" format.
func FormatSpecificationStatusWithIconColored(status string) string {
	icon := GetSpecificationStatusIcon(status)
	name := FormatSpecificationStatusColored(status)
	return icon + " " + name
}
