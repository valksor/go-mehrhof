package conductor

import (
	"context"
	"testing"

	"github.com/valksor/kvelmo/settings"
)

func TestRunPreGuardrails_NilSettings(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := c.runPreGuardrails(context.Background(), "plan"); err != nil {
		t.Errorf("runPreGuardrails() = %v, want nil", err)
	}
}

func TestRunPreGuardrails_NoGuardrailsForPhase(t *testing.T) {
	s := &settings.Settings{
		Workflow: settings.WorkflowSettings{
			PhaseGuardrails: map[string]settings.GuardrailConfig{
				"implement": {Pre: []string{"some-guardrail"}},
			},
		},
	}

	c, err := New(WithSettings(s))
	if err != nil {
		t.Fatalf("New(WithSettings) error = %v", err)
	}

	// "plan" has no guardrails configured, only "implement" does.
	if err := c.runPreGuardrails(context.Background(), "plan"); err != nil {
		t.Errorf("runPreGuardrails(plan) = %v, want nil", err)
	}
}

func TestRunPreGuardrails_EmptyPreList(t *testing.T) {
	s := &settings.Settings{
		Workflow: settings.WorkflowSettings{
			PhaseGuardrails: map[string]settings.GuardrailConfig{
				"plan": {Pre: []string{}},
			},
		},
	}

	c, err := New(WithSettings(s))
	if err != nil {
		t.Fatalf("New(WithSettings) error = %v", err)
	}

	if err := c.runPreGuardrails(context.Background(), "plan"); err != nil {
		t.Errorf("runPreGuardrails() = %v, want nil", err)
	}
}

func TestExecuteGuardrails_EmptyNames(t *testing.T) {
	result := executeGuardrails(context.Background(), nil, "plan", "/tmp", nil)
	if result != nil {
		t.Errorf("executeGuardrails(nil names) = %v, want nil", result)
	}

	result = executeGuardrails(context.Background(), []string{}, "plan", "/tmp", nil)
	if result != nil {
		t.Errorf("executeGuardrails(empty names) = %v, want nil", result)
	}
}

func TestExecuteGuardrails_UnknownName(t *testing.T) {
	result := executeGuardrails(context.Background(), []string{"nonexistent-guardrail"}, "plan", "/tmp", nil)
	if len(result) != 0 {
		t.Errorf("executeGuardrails(unknown name) returned %d findings, want 0", len(result))
	}
}
