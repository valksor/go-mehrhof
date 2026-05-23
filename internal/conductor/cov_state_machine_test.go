package conductor

import (
	"context"
	"slices"
	"testing"
	"time"
)

// fullWU returns a work unit that satisfies every guard in the transition table.
func fullWU() *WorkUnit {
	return &WorkUnit{
		ID:             "sm-task",
		Title:          "SM Task",
		Description:    "a description",
		Source:         &Source{Provider: "github", Reference: "owner/repo#1"},
		Specifications: []string{"spec.md"},
		Checkpoints:    []string{"sha-a"},
		RedoStack:      []string{"sha-b"},
		HasImplemented: true,
	}
}

// TestEveryTransition_GuardsSatisfied drives every (from, event) entry in
// TransitionTable with a work unit that satisfies all guards, asserting the
// machine lands in the declared target state.
func TestEveryTransition_GuardsSatisfied(t *testing.T) {
	ctx := context.Background()

	for key, transitions := range TransitionTable {
		t.Run(string(key.From)+"+"+string(key.Event), func(t *testing.T) {
			m := newTestMachine(key.From, fullWU())
			err := m.Dispatch(ctx, key.Event)
			if err != nil {
				t.Fatalf("Dispatch(%s,%s) error = %v", key.From, key.Event, err)
			}
			// The first transition whose guards pass determines target.
			// With fullWU all guards pass, so the first transition wins.
			want := transitions[0].To
			if m.State() != want {
				t.Errorf("from %s event %s: got %s, want %s", key.From, key.Event, m.State(), want)
			}
		})
	}
}

// TestGuardedTransitions_Rejected drives each guarded transition with a work
// unit that fails its guard, asserting the dispatch is rejected and state is
// unchanged.
func TestGuardedTransitions_Rejected(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		from  State
		event Event
		// mutate breaks exactly one guard precondition.
		mutate func(*WorkUnit)
	}{
		{"start without source", StateNone, EventStart, func(wu *WorkUnit) { wu.Source = nil }},
		{"plan without description", StateLoaded, EventPlan, func(wu *WorkUnit) { wu.Description = "" }},
		{"implement from planned without specs", StatePlanned, EventImplement, func(wu *WorkUnit) { wu.Specifications = nil }},
		{"implement from loaded without description", StateLoaded, EventImplement, func(wu *WorkUnit) { wu.Description = "" }},
		{"undo without checkpoints", StateImplementing, EventUndo, func(wu *WorkUnit) { wu.Checkpoints = nil }},
		{"redo without redo stack", StateLoaded, EventRedo, func(wu *WorkUnit) { wu.RedoStack = nil }},
		{"simplify from planned without implementation", StatePlanned, EventSimplify, func(wu *WorkUnit) { wu.HasImplemented = false }},
		{"optimize from planned without implementation", StatePlanned, EventOptimize, func(wu *WorkUnit) { wu.HasImplemented = false }},
		{"review from planned without implementation", StatePlanned, EventReview, func(wu *WorkUnit) { wu.HasImplemented = false }},
		{"submit without provider", StateReviewing, EventSubmit, func(wu *WorkUnit) { wu.Source.Provider = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wu := fullWU()
			tt.mutate(wu)
			m := newTestMachine(tt.from, wu)

			err := m.Dispatch(ctx, tt.event)
			if err == nil {
				t.Fatalf("expected guard rejection for %s+%s, got nil (state=%s)", tt.from, tt.event, m.State())
			}
			if m.State() != tt.from {
				t.Errorf("state should remain %s after rejection, got %s", tt.from, m.State())
			}
		})
	}
}

func TestSubmitGuard_QualityGateFailed(t *testing.T) {
	ctx := context.Background()
	wu := fullWU()
	failed := false
	wu.QualityGatePassed = &failed
	m := newTestMachine(StateReviewing, wu)

	err := m.Dispatch(ctx, EventSubmit)
	if err == nil {
		t.Fatal("submit should be rejected when quality gate failed")
	}
	if m.State() != StateReviewing {
		t.Errorf("state should remain reviewing, got %s", m.State())
	}
}

func TestSubmitGuard_QualityGatePassed(t *testing.T) {
	ctx := context.Background()
	wu := fullWU()
	passed := true
	wu.QualityGatePassed = &passed
	m := newTestMachine(StateReviewing, wu)

	if err := m.Dispatch(ctx, EventSubmit); err != nil {
		t.Fatalf("submit should pass when quality gate passed: %v", err)
	}
	if m.State() != StateSubmitted {
		t.Errorf("got %s, want submitted", m.State())
	}
}

func TestGuardQualityGatePassed_NilWorkUnit(t *testing.T) {
	if guardQualityGatePassed(context.Background(), nil) {
		t.Error("nil work unit should fail quality gate guard")
	}
}

func TestDispatchWithResume_FromWaiting(t *testing.T) {
	ctx := context.Background()
	m := newTestMachine(StatePlanning, fullWU())

	// Move to waiting (records previousState = planning).
	if err := m.Dispatch(ctx, EventWait); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if m.State() != StateWaiting {
		t.Fatalf("expected waiting, got %s", m.State())
	}

	// Answer should resume to previous state (planning), not the table fallback (loaded).
	if err := m.DispatchWithResume(ctx, EventAnswer); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if m.State() != StatePlanning {
		t.Errorf("resume returned to %s, want planning", m.State())
	}
}

func TestDispatchWithResume_FromPaused(t *testing.T) {
	ctx := context.Background()
	m := newTestMachine(StateImplementing, fullWU())

	if err := m.Dispatch(ctx, EventPause); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if m.State() != StatePaused {
		t.Fatalf("expected paused, got %s", m.State())
	}

	if err := m.DispatchWithResume(ctx, EventResume); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if m.State() != StateImplementing {
		t.Errorf("resume returned to %s, want implementing", m.State())
	}
}

func TestDispatchWithResume_NoPreviousState_FallsThrough(t *testing.T) {
	ctx := context.Background()
	// Answer from waiting without a recorded previousState falls through to
	// the table transition (waiting+answer -> loaded).
	m := newTestMachine(StateWaiting, fullWU())
	if err := m.DispatchWithResume(ctx, EventAnswer); err != nil {
		t.Fatalf("answer fallthrough: %v", err)
	}
	if m.State() != StateLoaded {
		t.Errorf("got %s, want loaded (table fallback)", m.State())
	}
}

func TestCanTransition(t *testing.T) {
	if !CanTransition(StateNone, StateLoaded) {
		t.Error("None->Loaded should be valid")
	}
	if !CanTransition(StateReviewing, StateSubmitted) {
		t.Error("Reviewing->Submitted should be valid")
	}
	if CanTransition(StateNone, StateSubmitted) {
		t.Error("None->Submitted should be invalid")
	}
	if CanTransition(State("nonexistent"), StateLoaded) {
		t.Error("transitions from unknown state should be invalid")
	}
}

func TestNextStates(t *testing.T) {
	next := NextStates(StateReviewing)
	for _, want := range []State{StateSubmitted, StatePlanning, StateImplemented, StateFailed} {
		if !slices.Contains(next, want) {
			t.Errorf("NextStates(reviewing) missing %s; got %v", want, next)
		}
	}

	// No duplicates.
	seen := map[State]bool{}
	for _, s := range next {
		if seen[s] {
			t.Errorf("duplicate state %s in NextStates", s)
		}
		seen[s] = true
	}

	if len(NextStates(State("unknown"))) != 0 {
		t.Error("NextStates of unknown state should be empty")
	}
}

func TestMachineIsTerminalIsPhase(t *testing.T) {
	m := newTestMachine(StatePlanning, fullWU())
	if !m.IsPhase() {
		t.Error("planning should be a phase state")
	}
	if m.IsTerminal() {
		t.Error("planning should not be terminal")
	}

	m.ForceState(StateFailed)
	if m.IsPhase() {
		t.Error("failed is not a phase state")
	}

	m.ForceState(State("unknown"))
	if m.IsTerminal() || m.IsPhase() {
		t.Error("unknown state should be neither terminal nor phase")
	}
}

func TestMachineRestoreHistoryAndWorkUnit(t *testing.T) {
	m := NewMachine()
	entries := []HistoryEntry{
		{From: StateNone, To: StateLoaded, Event: EventStart},
		{From: StateLoaded, To: StatePlanning, Event: EventPlan},
	}
	m.RestoreHistory(entries)
	if got := m.History(); len(got) != 2 {
		t.Errorf("history len = %d, want 2", len(got))
	}

	// History returns a copy: mutating it must not affect the machine.
	h := m.History()
	h[0].To = StateFailed
	if m.History()[0].To != StateLoaded {
		t.Error("History should return a defensive copy")
	}

	wu := fullWU()
	m.SetWorkUnit(wu)
	if m.WorkUnit() != wu {
		t.Error("SetWorkUnit/WorkUnit mismatch")
	}
	m.SetWorkUnit(nil) // should not panic
}

func TestMachinePriorStableStateAccessors(t *testing.T) {
	m := NewMachine()
	if m.PriorStableState() != "" {
		t.Error("prior stable state should start empty")
	}
	m.SetPriorStableState(StateSubmitted)
	if m.PriorStableState() != StateSubmitted {
		t.Errorf("got %s, want submitted", m.PriorStableState())
	}
	m.ClearPriorStableState()
	if m.PriorStableState() != "" {
		t.Error("prior stable state should be cleared")
	}
}

func TestMachineAddListener_FiresOnDispatch(t *testing.T) {
	ctx := context.Background()
	m := newTestMachine(StateNone, fullWU())

	fired := make(chan State, 1)
	m.AddListener(func(_, to State, _ Event, _ *WorkUnit) {
		select {
		case fired <- to:
		default:
		}
	})

	if err := m.Dispatch(ctx, EventStart); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	select {
	case to := <-fired:
		if to != StateLoaded {
			t.Errorf("listener got %s, want loaded", to)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener did not fire within 2s")
	}
}

func TestEvaluateGuards_EmptyAlwaysPasses(t *testing.T) {
	if !EvaluateGuards(context.Background(), nil, nil) {
		t.Error("empty guard set should always pass")
	}
}

func TestWaitPauseRecordsPreviousState(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		from  State
		event Event
		to    State
	}{
		{StatePlanning, EventWait, StateWaiting},
		{StatePlanning, EventPause, StatePaused},
		{StateImplementing, EventWait, StateWaiting},
		{StateImplementing, EventPause, StatePaused},
		{StateSimplifying, EventWait, StateWaiting},
		{StateOptimizing, EventPause, StatePaused},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"+"+string(tt.event), func(t *testing.T) {
			m := newTestMachine(tt.from, fullWU())
			if err := m.Dispatch(ctx, tt.event); err != nil {
				t.Fatalf("dispatch: %v", err)
			}
			if m.State() != tt.to {
				t.Errorf("got %s, want %s", m.State(), tt.to)
			}
		})
	}
}
