package display

import (
	"testing"

	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

func TestFormatState(t *testing.T) {
	tests := []struct {
		name  string
		state workflow.State
		want  string
	}{
		{"idle", workflow.StateIdle, "Ready"},
		{"planning", workflow.StatePlanning, "Planning"},
		{"implementing", workflow.StateImplementing, "In Progress"},
		{"reviewing", workflow.StateReviewing, "Reviewing"},
		{"done", workflow.StateDone, "Completed"},
		{"failed", workflow.StateFailed, "Failed"},
		{"waiting", workflow.StateWaiting, "Waiting"},
		{"dialogue", workflow.StateDialogue, "Dialogue"},
		{"checkpointing", workflow.StateCheckpointing, "Checkpointing"},
		{"reverting", workflow.StateReverting, "Reverting"},
		{"restoring", workflow.StateRestoring, "Restoring"},
		{"unknown state", workflow.State("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatState(tt.state)
			if got != tt.want {
				t.Errorf("FormatState(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestFormatStateString(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  string
	}{
		{"idle", "idle", "Ready"},
		{"planning", "planning", "Planning"},
		{"implementing", "implementing", "In Progress"},
		{"unknown", "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStateString(tt.state)
			if got != tt.want {
				t.Errorf("FormatStateString(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestGetStateDescription(t *testing.T) {
	tests := []struct {
		name  string
		state workflow.State
		want  string
	}{
		{"idle", workflow.StateIdle, "Ready for next action"},
		{"planning", workflow.StatePlanning, "AI is creating specifications"},
		{"implementing", workflow.StateImplementing, "AI is generating code"},
		{"reviewing", workflow.StateReviewing, "Code review in progress"},
		{"done", workflow.StateDone, "Task completed successfully"},
		{"failed", workflow.StateFailed, "Task failed with error"},
		{"waiting", workflow.StateWaiting, "Waiting for your input"},
		{"dialogue", workflow.StateDialogue, "Interactive conversation mode"},
		{"checkpointing", workflow.StateCheckpointing, "Creating checkpoint"},
		{"reverting", workflow.StateReverting, "Reverting to previous state"},
		{"restoring", workflow.StateRestoring, "Restoring from checkpoint"},
		{"unknown", workflow.State("unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStateDescription(tt.state)
			if got != tt.want {
				t.Errorf("GetStateDescription(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestFormatSpecificationStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"draft", storage.SpecificationStatusDraft, "Draft"},
		{"ready", storage.SpecificationStatusReady, "Pending"},
		{"implementing", storage.SpecificationStatusImplementing, "In Progress"},
		{"done", storage.SpecificationStatusDone, "Completed"},
		{"unknown", "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSpecificationStatus(tt.status)
			if got != tt.want {
				t.Errorf("FormatSpecificationStatus(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestGetSpecificationStatusIcon(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"draft", storage.SpecificationStatusDraft, "○"},
		{"ready", storage.SpecificationStatusReady, "◐"},
		{"implementing", storage.SpecificationStatusImplementing, "◑"},
		{"done", storage.SpecificationStatusDone, "●"},
		{"unknown", "unknown", "?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSpecificationStatusIcon(tt.status)
			if got != tt.want {
				t.Errorf("GetSpecificationStatusIcon(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestFormatSpecificationStatusWithIcon(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"draft", storage.SpecificationStatusDraft, "○ Draft"},
		{"ready", storage.SpecificationStatusReady, "◐ Pending"},
		{"implementing", storage.SpecificationStatusImplementing, "◑ In Progress"},
		{"done", storage.SpecificationStatusDone, "● Completed"},
		{"unknown", "unknown", "? unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSpecificationStatusWithIcon(tt.status)
			if got != tt.want {
				t.Errorf("FormatSpecificationStatusWithIcon(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestStateDisplayMapCompleteness(t *testing.T) {
	// Verify all known workflow states have display mappings
	knownStates := []workflow.State{
		workflow.StateIdle,
		workflow.StatePlanning,
		workflow.StateImplementing,
		workflow.StateReviewing,
		workflow.StateDone,
		workflow.StateFailed,
		workflow.StateWaiting,
		workflow.StateDialogue,
		workflow.StateCheckpointing,
		workflow.StateReverting,
		workflow.StateRestoring,
	}

	for _, state := range knownStates {
		if _, ok := StateDisplay[state]; !ok {
			t.Errorf("StateDisplay missing mapping for %q", state)
		}
		if _, ok := StateDescription[state]; !ok {
			t.Errorf("StateDescription missing mapping for %q", state)
		}
	}
}

func TestSpecificationStatusMapsCompleteness(t *testing.T) {
	// Verify all known specification statuses have display mappings
	knownStatuses := []string{
		storage.SpecificationStatusDraft,
		storage.SpecificationStatusReady,
		storage.SpecificationStatusImplementing,
		storage.SpecificationStatusDone,
	}

	for _, status := range knownStatuses {
		if _, ok := SpecificationStatusDisplay[status]; !ok {
			t.Errorf("SpecificationStatusDisplay missing mapping for %q", status)
		}
		if _, ok := SpecificationStatusIcon[status]; !ok {
			t.Errorf("SpecificationStatusIcon missing mapping for %q", status)
		}
	}
}

func TestWaitingStateConstant(t *testing.T) {
	if WaitingState != "waiting" {
		t.Errorf("WaitingState = %q, want %q", WaitingState, "waiting")
	}
}
