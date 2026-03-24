package conductor

import (
	"testing"

	"github.com/valksor/kvelmo/pkg/settings"
)

func TestFormatCheckpointMessage_Default(t *testing.T) {
	c := newTestConductor(t)
	c.workUnit = &WorkUnit{ID: "test-1"}

	msg := c.formatCheckpointMessage("plan_done complete")
	if msg != "kvelmo: plan_done complete" {
		t.Errorf("expected default prefix, got %q", msg)
	}
}

func TestFormatCheckpointMessage_CustomPrefix(t *testing.T) {
	s := settings.DefaultSettings()
	s.Git.CheckpointPrefix = "chore({key}):"

	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.workUnit = &WorkUnit{
		ID:         "test-1",
		ExternalID: "PROJ-123",
	}

	msg := c.formatCheckpointMessage("plan_done complete")
	if msg != "chore(PROJ-123): plan_done complete" {
		t.Errorf("expected custom prefix with key, got %q", msg)
	}
}

func TestFormatCheckpointMessage_CustomPrefixNoExternalID(t *testing.T) {
	s := settings.DefaultSettings()
	s.Git.CheckpointPrefix = "[{key}]"

	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.workUnit = &WorkUnit{ID: "test-1"}

	msg := c.formatCheckpointMessage("implement_done complete")
	// No external ID → {key} becomes "kvelmo"
	if msg != "[kvelmo] implement_done complete" {
		t.Errorf("expected fallback key, got %q", msg)
	}
}

func TestFormatCheckpointMessage_EmptyPrefix(t *testing.T) {
	s := settings.DefaultSettings()
	s.Git.CheckpointPrefix = ""

	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.workUnit = &WorkUnit{ID: "test-1"}

	msg := c.formatCheckpointMessage("some checkpoint")
	if msg != "kvelmo: some checkpoint" {
		t.Errorf("expected default kvelmo prefix, got %q", msg)
	}
}

func TestFormatCheckpointMessage_WithPhase(t *testing.T) {
	c := newTestConductor(t)
	c.workUnit = &WorkUnit{ID: "test-1"}

	// Force state to implementing
	c.machine.ForceState(StateImplementing)

	msg := c.formatCheckpointMessage("safety checkpoint")
	if msg != "kvelmo: [implement] safety checkpoint" {
		t.Errorf("expected phase tag, got %q", msg)
	}
}

func TestStateToPhase(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StatePlanning, "plan"},
		{StatePlanned, "plan"},
		{StateImplementing, "implement"},
		{StateImplemented, "implement"},
		{StateSimplifying, "simplify"},
		{StateOptimizing, "optimize"},
		{StateReviewing, "review"},
		{StateSubmitted, "submit"},
		{StateNone, ""},
		{StateLoaded, ""},
		{StateFailed, ""},
	}

	for _, tt := range tests {
		got := stateToPhase(tt.state)
		if got != tt.want {
			t.Errorf("stateToPhase(%s) = %q, want %q", tt.state, got, tt.want)
		}
	}
}
