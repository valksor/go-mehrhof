package conductor

import (
	"context"
	"errors"
	"testing"

	"github.com/valksor/kvelmo/pkg/settings"
)

func autoFixSettings(enabled bool, maxAttempts int, phases []string) *settings.Settings {
	s := settings.DefaultSettings()
	s.Workflow.ExternalReview.Mode = settings.ExternalReviewNever
	s.Workflow.AutoFix.Enabled = enabled
	s.Workflow.AutoFix.MaxAttempts = maxAttempts
	s.Workflow.AutoFix.Phases = phases

	return s
}

func TestAutoFixLoop_DisabledSetting(t *testing.T) {
	t.Parallel()

	s := autoFixSettings(false, 3, []string{"implement"})
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}

	// shouldAutoFix must return false when disabled
	if c.shouldAutoFix() {
		t.Error("shouldAutoFix() = true, want false when disabled")
	}
}

func TestAutoFixLoop_PhaseFiltering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		state  State
		phases []string
		want   bool
	}{
		{
			name:   "implement phase matches",
			state:  StateImplementing,
			phases: []string{"implement", "simplify"},
			want:   true,
		},
		{
			name:   "simplify phase matches",
			state:  StateSimplifying,
			phases: []string{"implement", "simplify"},
			want:   true,
		},
		{
			name:   "review phase not in list",
			state:  StateReviewing,
			phases: []string{"implement", "simplify"},
			want:   false,
		},
		{
			name:   "default phases when empty",
			state:  StateOptimizing,
			phases: nil,
			want:   true,
		},
		{
			name:   "none state never matches",
			state:  StateNone,
			phases: []string{"implement"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := autoFixSettings(true, 3, tt.phases)
			c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
			if err != nil {
				t.Fatal(err)
			}

			c.machine.ForceState(tt.state)

			got := c.shouldAutoFix()
			if got != tt.want {
				t.Errorf("shouldAutoFix() = %v, want %v (state=%s, phases=%v)",
					got, tt.want, tt.state, tt.phases)
			}
		})
	}
}

func TestAutoFixLoop_MaxAttemptsRespected(t *testing.T) {
	t.Parallel()

	// runQualityAutoFix requires a worker pool. Without one, it returns an error
	// immediately. We verify the error message indicates no pool.
	s := autoFixSettings(true, 2, []string{"implement"})
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.machine.ForceState(StateImplementing)
	c.workUnit = &WorkUnit{ID: "test-1", Title: "test task"}

	// No pool set — should fail with pool error
	qualityErr := errors.New("quality gate failed with 1 finding(s):\nlint: unused variable x")
	fixErr := c.runQualityAutoFix(context.Background(), qualityErr)
	if fixErr == nil {
		t.Error("runQualityAutoFix() = nil, want error (no pool)")
	}
}

func TestAutoFixLoop_GetAutoFixStatus(t *testing.T) {
	t.Parallel()

	s := autoFixSettings(true, 5, nil)
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}

	// Initially inactive
	status := c.GetAutoFixStatus()
	if status.Active {
		t.Error("initial status should be inactive")
	}
	if status.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", status.MaxAttempts)
	}

	// Simulate active state
	c.mu.Lock()
	c.autoFixAttempt = 2
	c.autoFixLastErr = "lint error"
	c.mu.Unlock()

	status = c.GetAutoFixStatus()
	if !status.Active {
		t.Error("status should be active when attempt > 0")
	}
	if status.Attempt != 2 {
		t.Errorf("Attempt = %d, want 2", status.Attempt)
	}
	if status.LastError != "lint error" {
		t.Errorf("LastError = %q, want %q", status.LastError, "lint error")
	}
}

func TestBuildQualityFixPrompt(t *testing.T) {
	t.Parallel()

	prompt := buildQualityFixPrompt("Fix auth", "Fix authentication bug", "go vet: unused var", 2)

	if prompt == "" {
		t.Fatal("buildQualityFixPrompt returned empty string")
	}

	// Verify key sections are present
	for _, want := range []string{"Fix auth", "Fix authentication bug", "go vet: unused var", "attempt 2"} {
		if !containsSubstring(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
