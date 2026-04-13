package conductor

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestStateMachine(t *testing.T) {
	m := NewMachine()

	if m.State() != StateNone {
		t.Errorf("initial state = %s, want %s", m.State(), StateNone)
	}

	ctx := context.Background()

	// Create a work unit for guards to pass
	wu := &WorkUnit{
		ID:    "test-1",
		Title: "Test Task",
		Source: &Source{
			Provider:  "test",
			Reference: "test-ref",
			Content:   "Test content",
		},
		Description:    "Test description",
		Specifications: []string{"spec1"},
	}
	m.SetWorkUnit(wu)

	// Test valid transition: None -> Loaded
	if err := m.Dispatch(ctx, EventStart); err != nil {
		t.Errorf("Dispatch(EventStart) error = %v", err)
	}
	if m.State() != StateLoaded {
		t.Errorf("state after start = %s, want %s", m.State(), StateLoaded)
	}

	// Test valid transition: Loaded -> Planning
	if err := m.Dispatch(ctx, EventPlan); err != nil {
		t.Errorf("Dispatch(EventPlan) error = %v", err)
	}
	if m.State() != StatePlanning {
		t.Errorf("state after plan = %s, want %s", m.State(), StatePlanning)
	}

	// Test valid transition: Planning -> Planned
	if err := m.Dispatch(ctx, EventPlanDone); err != nil {
		t.Errorf("Dispatch(EventPlanDone) error = %v", err)
	}
	if m.State() != StatePlanned {
		t.Errorf("state after plan done = %s, want %s", m.State(), StatePlanned)
	}

	// Test valid transition: Planned -> Implementing
	if err := m.Dispatch(ctx, EventImplement); err != nil {
		t.Errorf("Dispatch(EventImplement) error = %v", err)
	}
	if m.State() != StateImplementing {
		t.Errorf("state after implement = %s, want %s", m.State(), StateImplementing)
	}

	// Test valid transition: Implementing -> Implemented
	if err := m.Dispatch(ctx, EventImplementDone); err != nil {
		t.Errorf("Dispatch(EventImplementDone) error = %v", err)
	}
	if m.State() != StateImplemented {
		t.Errorf("state after implement done = %s, want %s", m.State(), StateImplemented)
	}
}

func TestStateMachineInvalidTransition(t *testing.T) {
	m := NewMachine()
	ctx := context.Background()

	// Try to implement from None state (invalid)
	err := m.Dispatch(ctx, EventImplement)
	if err == nil {
		t.Error("expected error for invalid transition None -> Implementing")
	}

	// Try to submit from None state (invalid)
	err = m.Dispatch(ctx, EventSubmit)
	if err == nil {
		t.Error("expected error for invalid transition None -> Submitted")
	}
}

func TestStateMachineHistory(t *testing.T) {
	m := NewMachine()
	ctx := context.Background()

	wu := &WorkUnit{
		ID:             "test-1",
		Title:          "Test",
		Source:         &Source{Provider: "test", Reference: "ref", Content: "content"},
		Description:    "desc",
		Specifications: []string{"spec"},
	}
	m.SetWorkUnit(wu)

	_ = m.Dispatch(ctx, EventStart)
	_ = m.Dispatch(ctx, EventPlan)
	_ = m.Dispatch(ctx, EventPlanDone)

	history := m.History()
	if len(history) != 3 {
		t.Errorf("history length = %d, want 3", len(history))
	}

	// Verify history order
	expected := []State{StateLoaded, StatePlanning, StatePlanned}
	for i, entry := range history {
		if entry.To != expected[i] {
			t.Errorf("history[%d].To = %s, want %s", i, entry.To, expected[i])
		}
	}
}

func TestStateMachineCanDispatch(t *testing.T) {
	m := NewMachine()
	ctx := context.Background()

	wu := &WorkUnit{
		ID:          "test-1",
		Title:       "Test",
		Source:      &Source{Provider: "test", Reference: "ref", Content: "content"},
		Description: "desc",
	}
	m.SetWorkUnit(wu)

	// From None, can start
	can, _ := m.CanDispatch(ctx, EventStart)
	if !can {
		t.Error("should be able to dispatch EventStart from None")
	}

	// From None, cannot implement
	can, _ = m.CanDispatch(ctx, EventImplement)
	if can {
		t.Error("should not be able to dispatch EventImplement from None")
	}
}

func TestStateMachineReset(t *testing.T) {
	m := NewMachine()
	ctx := context.Background()

	wu := &WorkUnit{
		ID:          "test-1",
		Title:       "Test",
		Source:      &Source{Provider: "test", Reference: "ref", Content: "content"},
		Description: "desc",
	}
	m.SetWorkUnit(wu)

	_ = m.Dispatch(ctx, EventStart)
	_ = m.Dispatch(ctx, EventPlan)

	m.Reset()

	if m.State() != StateNone {
		t.Errorf("state after reset = %s, want %s", m.State(), StateNone)
	}

	if len(m.History()) != 0 {
		t.Error("history should be empty after reset")
	}
}

func TestStateMachineAvailableEvents(t *testing.T) {
	m := NewMachine()
	ctx := context.Background()

	wu := &WorkUnit{
		ID:          "test-1",
		Title:       "Test",
		Source:      &Source{Provider: "test", Reference: "ref", Content: "content"},
		Description: "desc",
	}
	m.SetWorkUnit(wu)

	events := m.AvailableEvents(ctx)

	// From None, should have Start available
	if !slices.Contains(events, EventStart) {
		t.Error("EventStart should be available from None state")
	}
}

func TestWorkUnit(t *testing.T) {
	wu := &WorkUnit{
		ID:          "test-123",
		ExternalID:  "gh-456",
		Title:       "Test Task",
		Description: "A test task",
		Source: &Source{
			Provider:  "github",
			Reference: "owner/repo#456",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if wu.ID != "test-123" {
		t.Errorf("WorkUnit.ID = %s, want test-123", wu.ID)
	}

	if wu.Source.Provider != "github" {
		t.Errorf("WorkUnit.Source.Provider = %s, want github", wu.Source.Provider)
	}
}

func TestConductorNew(t *testing.T) {
	cfg := ConductorConfig{}
	c := NewConductor(cfg)

	if c == nil {
		t.Fatal("NewConductor returned nil")
	}

	if c.State() != StateNone {
		t.Errorf("initial state = %s, want %s", c.State(), StateNone)
	}
}

func TestConductorWorkUnit(t *testing.T) {
	cfg := ConductorConfig{}
	c := NewConductor(cfg)

	if c.WorkUnit() != nil {
		t.Error("WorkUnit should be nil before task is loaded")
	}
}

func TestTransitionTable(t *testing.T) {
	// Verify key transitions exist
	transitions := []struct {
		from  State
		event Event
		to    State
	}{
		{StateNone, EventStart, StateLoaded},
		{StateLoaded, EventPlan, StatePlanning},
		{StatePlanning, EventPlanDone, StatePlanned},
		{StatePlanned, EventImplement, StateImplementing},
		{StateImplementing, EventImplementDone, StateImplemented},
		{StateImplemented, EventReview, StateReviewing},
		{StateReviewing, EventSubmit, StateSubmitted},
	}

	for _, tc := range transitions {
		key := TransitionKey{From: tc.from, Event: tc.event}
		rules, ok := TransitionTable[key]
		if !ok {
			t.Errorf("missing transition: %s + %s", tc.from, tc.event)

			continue
		}
		if len(rules) == 0 {
			t.Errorf("empty rules for transition: %s + %s", tc.from, tc.event)

			continue
		}
		if rules[0].To != tc.to {
			t.Errorf("transition %s + %s = %s, want %s", tc.from, tc.event, rules[0].To, tc.to)
		}
	}
}

func TestGuardFailureMessage(t *testing.T) {
	m := NewMachine()
	ctx := context.Background()

	// Work unit with source but no description
	wu := &WorkUnit{
		ID:     "test-guard",
		Source: &Source{Provider: "test", Reference: "ref", Content: "content"},
	}
	m.SetWorkUnit(wu)
	_ = m.Dispatch(ctx, EventStart)

	// Try to plan without description - should get description-specific error
	err := m.Dispatch(ctx, EventPlan)
	if err == nil {
		t.Fatal("expected error for missing description")
	}
	if !strings.Contains(err.Error(), "no description") {
		t.Errorf("error = %q, want message about missing description", err.Error())
	}

	// Set description but no specs, advance to planned
	wu.Description = "test desc"
	_ = m.Dispatch(ctx, EventPlan)
	_ = m.Dispatch(ctx, EventPlanDone)

	// Try to implement without specifications
	err = m.Dispatch(ctx, EventImplement)
	if err == nil {
		t.Fatal("expected error for missing specifications")
	}
	if !strings.Contains(err.Error(), "no specification") {
		t.Errorf("error = %q, want message about missing specification", err.Error())
	}
}

func TestFullLifecycle(t *testing.T) {
	m := NewMachine()
	ctx := context.Background()

	wu := &WorkUnit{
		ID:             "lifecycle-test",
		Title:          "Full Lifecycle Test",
		Source:         &Source{Provider: "test", Reference: "ref", Content: "content"},
		Description:    "Testing full lifecycle",
		Specifications: []string{"spec1", "spec2"},
		Checkpoints:    []string{"abc123"},
	}
	m.SetWorkUnit(wu)

	steps := []struct {
		event    Event
		expected State
	}{
		{EventStart, StateLoaded},
		{EventPlan, StatePlanning},
		{EventPlanDone, StatePlanned},
		{EventImplement, StateImplementing},
		{EventImplementDone, StateImplemented},
		{EventReview, StateReviewing},
		{EventSubmit, StateSubmitted},
	}

	for _, step := range steps {
		if err := m.Dispatch(ctx, step.event); err != nil {
			t.Errorf("Dispatch(%s) error = %v", step.event, err)

			continue
		}
		if m.State() != step.expected {
			t.Errorf("after %s: state = %s, want %s", step.event, m.State(), step.expected)
		}
	}
}
