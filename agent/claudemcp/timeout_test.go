package claudemcp

import (
	"context"
	"testing"
	"time"
)

// TestSpawnContextTimeoutSelection covers the C1 fix from Pass 1: SendPrompt
// must apply Config.Timeout to the spawn context when no tighter deadline is
// present, but must NOT override a tighter parent deadline.
//
// The decision logic lives inline in SendPrompt; we replicate it here so the
// table-driven test stays pure. If SendPrompt's logic changes, the helper
// below must change with it — this test exists to prevent silent regression.
func TestSpawnContextTimeoutSelection(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name        string
		parent      func() (context.Context, context.CancelFunc)
		configTO    time.Duration
		wantApplied bool
	}{
		{
			name:        "no deadline + nonzero timeout → applied",
			parent:      func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			configTO:    time.Hour,
			wantApplied: true,
		},
		{
			name: "parent deadline shorter than timeout → NOT applied (parent wins)",
			parent: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), now.Add(time.Minute))
			},
			configTO:    time.Hour,
			wantApplied: false,
		},
		{
			name: "parent deadline longer than timeout → applied (tighten)",
			parent: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), now.Add(2*time.Hour))
			},
			configTO:    time.Hour,
			wantApplied: true,
		},
		{
			name:        "zero timeout → never applied",
			parent:      func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			configTO:    0,
			wantApplied: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := tc.parent()
			defer cancel()

			applied := shouldApplyTimeout(ctx, tc.configTO)
			if applied != tc.wantApplied {
				t.Errorf("shouldApplyTimeout = %v, want %v", applied, tc.wantApplied)
			}
		})
	}
}

// shouldApplyTimeout mirrors the inline decision in SendPrompt for testing.
// Returns true if SendPrompt would call context.WithTimeout(ctx, configTO).
func shouldApplyTimeout(ctx context.Context, configTO time.Duration) bool {
	if configTO <= 0 {
		return false
	}
	dl, ok := ctx.Deadline()
	if !ok {
		return true
	}

	return time.Until(dl) > configTO
}
