package conductor

import (
	"context"
	"testing"
)

// newTestMachine creates a Machine in the given state with the provided work unit.
func newTestMachine(state State, wu *WorkUnit) *Machine {
	if wu == nil {
		wu = &WorkUnit{
			ID:          "test-task",
			Description: "test description",
			Source:      &Source{Provider: "github", Reference: "owner/repo#1"},
		}
	}
	m := NewMachine()
	m.SetWorkUnit(wu)
	m.ForceState(state)

	return m
}

func TestReEntryTransitions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		from    State
		event   Event
		wantTo  State
		wantErr bool
		setupWU func(*WorkUnit)
	}{
		// Re-plan from stable states
		{
			name: "plan from planned",
			from: StatePlanned, event: EventPlan, wantTo: StatePlanning,
		},
		{
			name: "plan from implemented",
			from: StateImplemented, event: EventPlan, wantTo: StatePlanning,
		},
		{
			name: "plan from submitted",
			from: StateSubmitted, event: EventPlan, wantTo: StatePlanning,
		},

		// Re-implement from stable states
		{
			name: "implement from implemented",
			from: StateImplemented, event: EventImplement, wantTo: StateImplementing,
			setupWU: func(wu *WorkUnit) { wu.Specifications = []string{"spec.md"} },
		},
		{
			name: "implement from submitted",
			from: StateSubmitted, event: EventImplement, wantTo: StateImplementing,
			setupWU: func(wu *WorkUnit) { wu.Specifications = []string{"spec.md"} },
		},
		{
			name: "implement from implemented without specs fails",
			from: StateImplemented, event: EventImplement, wantErr: true,
		},

		// Simplify/optimize from stable states (needs HasImplemented)
		{
			name: "simplify from planned with implementation",
			from: StatePlanned, event: EventSimplify, wantTo: StateSimplifying,
			setupWU: func(wu *WorkUnit) { wu.HasImplemented = true },
		},
		{
			name: "simplify from planned without implementation fails",
			from: StatePlanned, event: EventSimplify, wantErr: true,
		},
		{
			name: "optimize from planned with implementation",
			from: StatePlanned, event: EventOptimize, wantTo: StateOptimizing,
			setupWU: func(wu *WorkUnit) { wu.HasImplemented = true },
		},
		{
			name: "simplify from submitted with implementation",
			from: StateSubmitted, event: EventSimplify, wantTo: StateSimplifying,
			setupWU: func(wu *WorkUnit) { wu.HasImplemented = true },
		},
		{
			name: "optimize from submitted with implementation",
			from: StateSubmitted, event: EventOptimize, wantTo: StateOptimizing,
			setupWU: func(wu *WorkUnit) { wu.HasImplemented = true },
		},
		{
			name: "simplify from submitted without implementation fails",
			from: StateSubmitted, event: EventSimplify, wantErr: true,
		},

		// Review from stable states
		{
			name: "review from planned with implementation",
			from: StatePlanned, event: EventReview, wantTo: StateReviewing,
			setupWU: func(wu *WorkUnit) { wu.HasImplemented = true },
		},
		{
			name: "review from planned without implementation fails",
			from: StatePlanned, event: EventReview, wantErr: true,
		},
		{
			name: "review from submitted",
			from: StateSubmitted, event: EventReview, wantTo: StateReviewing,
		},

		// Stop from reviewing (e.g. during re-entry review)
		{
			name: "stop from reviewing",
			from: StateReviewing, event: EventStop, wantTo: StateImplemented,
		},

		// Undo/Redo from Submitted
		{
			name: "undo from submitted",
			from: StateSubmitted, event: EventUndo, wantTo: StateSubmitted,
			setupWU: func(wu *WorkUnit) { wu.Checkpoints = []string{"abc123"} },
		},
		{
			name: "redo from submitted",
			from: StateSubmitted, event: EventRedo, wantTo: StateSubmitted,
			setupWU: func(wu *WorkUnit) { wu.RedoStack = []string{"abc123"} },
		},
		{
			name: "undo from submitted without checkpoints fails",
			from: StateSubmitted, event: EventUndo, wantErr: true,
		},

		// Abort from Submitted
		{
			name: "abort from submitted",
			from: StateSubmitted, event: EventAbort, wantTo: StateFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wu := &WorkUnit{
				ID:          "test-task",
				Description: "test description",
				Source:      &Source{Provider: "github", Reference: "owner/repo#1"},
			}
			if tt.setupWU != nil {
				tt.setupWU(wu)
			}

			m := newTestMachine(tt.from, wu)
			err := m.Dispatch(ctx, tt.event)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil (state=%s)", m.State())
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.State() != tt.wantTo {
				t.Errorf("got state %s, want %s", m.State(), tt.wantTo)
			}
		})
	}
}

func TestGuardHasImplementation(t *testing.T) {
	ctx := context.Background()

	t.Run("nil work unit", func(t *testing.T) {
		if guardHasImplementation(ctx, nil) {
			t.Error("should return false for nil work unit")
		}
	})

	t.Run("HasImplemented false", func(t *testing.T) {
		wu := &WorkUnit{HasImplemented: false}
		if guardHasImplementation(ctx, wu) {
			t.Error("should return false when HasImplemented is false")
		}
	})

	t.Run("HasImplemented true", func(t *testing.T) {
		wu := &WorkUnit{HasImplemented: true}
		if !guardHasImplementation(ctx, wu) {
			t.Error("should return true when HasImplemented is true")
		}
	})
}

func TestPriorStableStateRollback(t *testing.T) {
	ctx := context.Background()

	t.Run("error rolls back to PriorStableState", func(t *testing.T) {
		wu := &WorkUnit{
			ID:             "test",
			Description:    "test",
			Source:         &Source{Provider: "github", Reference: "ref"},
			Specifications: []string{"spec.md"},
		}
		m := newTestMachine(StateSubmitted, wu)

		// Simulate re-entry: set PriorStableState before dispatching implement
		m.SetPriorStableState(StateSubmitted)

		// Dispatch implement to move to Implementing
		if err := m.Dispatch(ctx, EventImplement); err != nil {
			t.Fatalf("dispatch implement: %v", err)
		}
		if m.State() != StateImplementing {
			t.Fatalf("expected implementing, got %s", m.State())
		}

		// Now simulate error — should roll back to Submitted (not Planned)
		if err := m.Dispatch(ctx, EventError); err != nil {
			t.Fatalf("dispatch error: %v", err)
		}
		if m.State() != StateSubmitted {
			t.Errorf("expected rollback to submitted, got %s", m.State())
		}
		// PriorStableState should be consumed
		if m.PriorStableState() != "" {
			t.Errorf("PriorStableState should be cleared, got %s", m.PriorStableState())
		}
	})

	t.Run("stop rolls back to PriorStableState", func(t *testing.T) {
		wu := &WorkUnit{
			ID:             "test",
			Description:    "test",
			Source:         &Source{Provider: "github", Reference: "ref"},
			Specifications: []string{"spec.md"},
		}
		m := newTestMachine(StateSubmitted, wu)

		// Simulate re-entry
		m.SetPriorStableState(StateSubmitted)

		if err := m.Dispatch(ctx, EventImplement); err != nil {
			t.Fatalf("dispatch implement: %v", err)
		}

		// Stop should also roll back to Submitted
		if err := m.Dispatch(ctx, EventStop); err != nil {
			t.Fatalf("dispatch stop: %v", err)
		}
		if m.State() != StateSubmitted {
			t.Errorf("expected rollback to submitted, got %s", m.State())
		}
	})

	t.Run("error without PriorStableState uses default", func(t *testing.T) {
		wu := &WorkUnit{
			ID:             "test",
			Description:    "test",
			Source:         &Source{Provider: "github", Reference: "ref"},
			Specifications: []string{"spec.md"},
		}
		m := newTestMachine(StatePlanned, wu)

		// Normal implement (not re-entry) — no PriorStableState set
		if err := m.Dispatch(ctx, EventImplement); err != nil {
			t.Fatalf("dispatch implement: %v", err)
		}

		// Error should roll back to Planned (default from transition table)
		if err := m.Dispatch(ctx, EventError); err != nil {
			t.Fatalf("dispatch error: %v", err)
		}
		if m.State() != StatePlanned {
			t.Errorf("expected default rollback to planned, got %s", m.State())
		}
	})
}

func TestFullReEntryLoop(t *testing.T) {
	ctx := context.Background()

	wu := &WorkUnit{
		ID:          "test-loop",
		Description: "test description",
		Source:      &Source{Provider: "github", Reference: "owner/repo#1"},
	}
	m := newTestMachine(StateLoaded, wu)

	// Plan
	if err := m.Dispatch(ctx, EventPlan); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if m.State() != StatePlanning {
		t.Fatalf("expected planning, got %s", m.State())
	}
	if err := m.Dispatch(ctx, EventPlanDone); err != nil {
		t.Fatalf("plan done: %v", err)
	}

	// Add specs for implement
	wu.Specifications = []string{"spec.md"}

	// Implement
	if err := m.Dispatch(ctx, EventImplement); err != nil {
		t.Fatalf("implement: %v", err)
	}
	if err := m.Dispatch(ctx, EventImplementDone); err != nil {
		t.Fatalf("implement done: %v", err)
	}
	wu.HasImplemented = true

	// Re-plan from Implemented
	if err := m.Dispatch(ctx, EventPlan); err != nil {
		t.Fatalf("re-plan: %v", err)
	}
	if err := m.Dispatch(ctx, EventPlanDone); err != nil {
		t.Fatalf("re-plan done: %v", err)
	}

	// Re-implement from Planned
	if err := m.Dispatch(ctx, EventImplement); err != nil {
		t.Fatalf("re-implement: %v", err)
	}
	if err := m.Dispatch(ctx, EventImplementDone); err != nil {
		t.Fatalf("re-implement done: %v", err)
	}

	// Optimize
	if err := m.Dispatch(ctx, EventOptimize); err != nil {
		t.Fatalf("optimize: %v", err)
	}
	if err := m.Dispatch(ctx, EventOptimizeDone); err != nil {
		t.Fatalf("optimize done: %v", err)
	}

	// Simplify
	if err := m.Dispatch(ctx, EventSimplify); err != nil {
		t.Fatalf("simplify: %v", err)
	}
	if err := m.Dispatch(ctx, EventSimplifyDone); err != nil {
		t.Fatalf("simplify done: %v", err)
	}

	// Review → Submit (review is a human checkpoint, submit from reviewing state)
	if err := m.Dispatch(ctx, EventReview); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := m.Dispatch(ctx, EventSubmit); err != nil {
		t.Fatalf("submit from reviewing: %v", err)
	}
	if m.State() != StateSubmitted {
		t.Fatalf("expected submitted, got %s", m.State())
	}

	// Post-submit re-entry: implement from submitted
	if err := m.Dispatch(ctx, EventImplement); err != nil {
		t.Fatalf("post-submit implement: %v", err)
	}
	if err := m.Dispatch(ctx, EventImplementDone); err != nil {
		t.Fatalf("post-submit implement done: %v", err)
	}

	// Review again → Submit
	if err := m.Dispatch(ctx, EventReview); err != nil {
		t.Fatalf("post-submit review: %v", err)
	}
	if err := m.Dispatch(ctx, EventSubmit); err != nil {
		t.Fatalf("re-submit from reviewing: %v", err)
	}
	if m.State() != StateSubmitted {
		t.Fatalf("expected submitted after re-submit, got %s", m.State())
	}

	// Finish
	if err := m.Dispatch(ctx, EventFinish); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if m.State() != StateNone {
		t.Fatalf("expected none after finish, got %s", m.State())
	}
}
