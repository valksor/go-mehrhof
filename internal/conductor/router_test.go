package conductor

import (
	"context"
	"testing"
)

func TestDefaultRouterAlwaysAdvances(t *testing.T) {
	t.Parallel()

	router := NewDefaultRouter()

	phases := []string{"plan", "implement", "simplify", "optimize", "review"}
	for _, phase := range phases {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()

			decision := router.Route(context.Background(), phase, "some output", 0)

			if decision.Action != RouteAdvance {
				t.Errorf("expected RouteAdvance, got %s", decision.Action)
			}
			if decision.Reason == "" {
				t.Error("expected non-empty reason")
			}
			if decision.MaxRetries != 2 {
				t.Errorf("expected MaxRetries=2, got %d", decision.MaxRetries)
			}
		})
	}
}

func TestDefaultRouterPreservesAttempt(t *testing.T) {
	t.Parallel()

	router := NewDefaultRouter()

	decision := router.Route(context.Background(), "implement", "output", 3)

	if decision.Attempt != 3 {
		t.Errorf("expected attempt=3, got %d", decision.Attempt)
	}
}

func TestDefaultRouterCustomMaxRetries(t *testing.T) {
	t.Parallel()

	router := &DefaultRouter{MaxRetries: 5}

	decision := router.Route(context.Background(), "plan", "output", 0)

	if decision.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5, got %d", decision.MaxRetries)
	}
}

func TestRouteDecisionActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		action RouteAction
		want   string
	}{
		{RouteAdvance, "advance"},
		{RouteRetry, "retry"},
		{RouteSkip, "skip"},
		{RouteRollback, "rollback"},
	}

	for _, tt := range tests {
		if string(tt.action) != tt.want {
			t.Errorf("RouteAction %v: got %q, want %q", tt.action, string(tt.action), tt.want)
		}
	}
}

func TestPhaseRouterInterface(t *testing.T) {
	t.Parallel()

	// Verify DefaultRouter satisfies PhaseRouter interface.
	var _ PhaseRouter = (*DefaultRouter)(nil)
}
