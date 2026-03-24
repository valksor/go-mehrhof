package conductor

import (
	"testing"

	"github.com/valksor/kvelmo/pkg/settings"
)

func TestFormatCheckpointMessage_Default(t *testing.T) {
	c := newTestConductor(t)
	c.workUnit = &WorkUnit{ID: "test-1"}

	msg := c.formatCheckpointMessage("plan_done complete")
	if msg != "[kvelmo] plan_done complete" {
		t.Errorf("expected default commit prefix, got %q", msg)
	}
}

func TestFormatCheckpointMessage_CustomPrefix(t *testing.T) {
	s := settings.DefaultSettings()
	s.Git.CommitPrefix = "chore({key}):"

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
	s.Git.CommitPrefix = "[{key}]"

	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.workUnit = &WorkUnit{ID: "test-1"}

	msg := c.formatCheckpointMessage("implement_done complete")
	// No external ID → {key} becomes empty, [] becomes [kvelmo]
	if msg != "[kvelmo] implement_done complete" {
		t.Errorf("expected fallback key, got %q", msg)
	}
}

func TestFormatCheckpointMessage_EmptyPrefix(t *testing.T) {
	s := settings.DefaultSettings()
	s.Git.CommitPrefix = ""

	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.workUnit = &WorkUnit{ID: "test-1"}

	msg := c.formatCheckpointMessage("some checkpoint")
	// Empty CommitPrefix → falls back to [kvelmo]
	if msg != "[kvelmo] some checkpoint" {
		t.Errorf("expected default kvelmo prefix, got %q", msg)
	}
}
