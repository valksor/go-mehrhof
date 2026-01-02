package workflow

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/events"
)

func TestNewMachine(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	if m == nil {
		t.Fatal("NewMachine returned nil")
	}
	if m.State() != StateIdle {
		t.Errorf("initial state = %v, want %v", m.State(), StateIdle)
	}
	if m.WorkUnit() != nil {
		t.Error("initial work unit should be nil")
	}
}

func TestSetWorkUnit(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	wu := &WorkUnit{
		ID:    "test-123",
		Title: "Test Task",
	}
	m.SetWorkUnit(wu)

	if m.WorkUnit() != wu {
		t.Error("SetWorkUnit did not set work unit")
	}
}

func TestDispatch_ValidTransition(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	// Set up work unit with source (required by GuardHasSource)
	m.SetWorkUnit(&WorkUnit{
		ID:     "test-123",
		Source: &Source{Reference: "file:task.md"},
	})

	// EventStart from Idle stays in Idle (registers task)
	err := m.Dispatch(context.Background(), EventStart)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if m.State() != StateIdle {
		t.Errorf("state = %v, want %v", m.State(), StateIdle)
	}

	// EventPlan transitions to StatePlanning
	err = m.Dispatch(context.Background(), EventPlan)
	if err != nil {
		t.Fatalf("Dispatch EventPlan failed: %v", err)
	}

	if m.State() != StatePlanning {
		t.Errorf("state = %v, want %v", m.State(), StatePlanning)
	}
}

func TestDispatch_InvalidTransition(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	// Try to dispatch PlanDone from idle (no transition defined)
	err := m.Dispatch(context.Background(), EventPlanDone)
	if err == nil {
		t.Error("expected error for invalid transition, got nil")
	}
}

func TestDispatch_GuardFails(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	// No work unit - GuardHasSource should fail
	err := m.Dispatch(context.Background(), EventStart)
	if err == nil {
		t.Error("expected error when guard fails, got nil")
	}

	if m.State() != StateIdle {
		t.Errorf("state should remain %v when guard fails, got %v", StateIdle, m.State())
	}
}

func TestDispatch_GlobalTransition(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	// Set up and move to planning state
	m.SetWorkUnit(&WorkUnit{
		ID:     "test-123",
		Source: &Source{Reference: "file:task.md"},
	})
	_ = m.Dispatch(context.Background(), EventStart)
	_ = m.Dispatch(context.Background(), EventPlan)

	// Global abort event should work from any state
	err := m.Dispatch(context.Background(), EventAbort)
	if err != nil {
		t.Fatalf("global transition failed: %v", err)
	}

	if m.State() != StateFailed {
		t.Errorf("state = %v, want %v", m.State(), StateFailed)
	}
}

func TestCanDispatch(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	m.SetWorkUnit(&WorkUnit{
		ID:     "test-123",
		Source: &Source{Reference: "file:task.md"},
	})

	can, reason := m.CanDispatch(context.Background(), EventStart)
	if !can {
		t.Errorf("CanDispatch returned false: %s", reason)
	}

	// EventPlanDone is invalid from Idle state
	can, _ = m.CanDispatch(context.Background(), EventPlanDone)
	if can {
		t.Error("CanDispatch should return false for invalid transition")
	}
}

func TestHistory(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	m.SetWorkUnit(&WorkUnit{
		ID:     "test-123",
		Source: &Source{Reference: "file:task.md"},
	})
	_ = m.Dispatch(context.Background(), EventStart)
	_ = m.Dispatch(context.Background(), EventPlan)

	history := m.History()
	if len(history) != 2 {
		t.Fatalf("history length = %d, want 2", len(history))
	}

	// Check second entry (EventPlan: Idle -> Planning)
	entry := history[1]
	if entry.From != StateIdle {
		t.Errorf("history from = %v, want %v", entry.From, StateIdle)
	}
	if entry.To != StatePlanning {
		t.Errorf("history to = %v, want %v", entry.To, StatePlanning)
	}
	if entry.Event != EventPlan {
		t.Errorf("history event = %v, want %v", entry.Event, EventPlan)
	}
}

func TestReset(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	m.SetWorkUnit(&WorkUnit{
		ID:     "test-123",
		Source: &Source{Reference: "file:task.md"},
	})
	_ = m.Dispatch(context.Background(), EventStart)

	m.Reset()

	if m.State() != StateIdle {
		t.Errorf("state after reset = %v, want %v", m.State(), StateIdle)
	}
	if m.WorkUnit() != nil {
		t.Error("work unit should be nil after reset")
	}
	if len(m.History()) != 0 {
		t.Error("history should be empty after reset")
	}
}

func TestUndoRedoStacks(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	// Initially empty
	if m.CanUndo() {
		t.Error("CanUndo should be false initially")
	}
	if m.CanRedo() {
		t.Error("CanRedo should be false initially")
	}

	// Push checkpoint
	m.PushUndo("checkpoint-1")
	if !m.CanUndo() {
		t.Error("CanUndo should be true after PushUndo")
	}

	// Pop undo
	checkpoint, ok := m.PopUndo()
	if !ok || checkpoint != "checkpoint-1" {
		t.Errorf("PopUndo = (%v, %v), want (checkpoint-1, true)", checkpoint, ok)
	}
	if m.CanUndo() {
		t.Error("CanUndo should be false after PopUndo")
	}
	if !m.CanRedo() {
		t.Error("CanRedo should be true after PopUndo")
	}

	// Pop redo
	checkpoint, ok = m.PopRedo()
	if !ok || checkpoint != "checkpoint-1" {
		t.Errorf("PopRedo = (%v, %v), want (checkpoint-1, true)", checkpoint, ok)
	}
	if m.CanRedo() {
		t.Error("CanRedo should be false after PopRedo")
	}
	if !m.CanUndo() {
		t.Error("CanUndo should be true after PopRedo")
	}
}

func TestPushUndoClearsRedo(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	m.PushUndo("checkpoint-1")
	m.PopUndo() // Move to redo
	if !m.CanRedo() {
		t.Fatal("expected CanRedo to be true")
	}

	m.PushUndo("checkpoint-2")
	if m.CanRedo() {
		t.Error("PushUndo should clear redo stack")
	}
}

func TestAddListener(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	var mu sync.Mutex
	var called bool
	var receivedFrom, receivedTo State

	m.AddListener(func(from, to State, event Event, wu *WorkUnit) {
		mu.Lock()
		defer mu.Unlock()
		called = true
		receivedFrom = from
		receivedTo = to
	})

	m.SetWorkUnit(&WorkUnit{
		ID:     "test-123",
		Source: &Source{Reference: "file:task.md"},
	})
	_ = m.Dispatch(context.Background(), EventPlan)

	// Give async listener time to run
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !called {
		t.Error("listener was not called")
	}
	if receivedFrom != StateIdle {
		t.Errorf("listener from = %v, want %v", receivedFrom, StateIdle)
	}
	if receivedTo != StatePlanning {
		t.Errorf("listener to = %v, want %v", receivedTo, StatePlanning)
	}
}

func TestEventBusIntegration(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	var received atomic.Bool
	bus.Subscribe(events.TypeStateChanged, func(e events.Event) {
		received.Store(true)
	})

	m.SetWorkUnit(&WorkUnit{
		ID:     "test-123",
		Source: &Source{Reference: "file:task.md"},
	})
	_ = m.Dispatch(context.Background(), EventStart)

	// Give async event time to be published
	time.Sleep(20 * time.Millisecond)

	if !received.Load() {
		t.Error("event bus did not receive state change event")
	}
}

func TestIsTerminal(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	if m.IsTerminal() {
		t.Error("idle state should not be terminal")
	}

	// Move to failed state using global abort
	// Note: Failed state is now non-terminal to allow recovery via EventReset
	m.SetWorkUnit(&WorkUnit{
		ID:     "test-123",
		Source: &Source{Reference: "file:task.md"},
	})
	_ = m.Dispatch(context.Background(), EventAbort)

	if m.IsTerminal() {
		t.Error("failed state should not be terminal (allows recovery via EventReset)")
	}

	// Move to done state (truly terminal) to verify terminal check works
	m.Reset()
	m.SetWorkUnit(&WorkUnit{
		ID:             "test-123",
		Source:         &Source{Reference: "file:task.md"},
		Specifications: []string{"specification-1.md"}, // Required for finish guard
	})
	_ = m.Dispatch(context.Background(), EventFinish)

	if !m.IsTerminal() {
		t.Error("done state should be terminal")
	}
}

func TestConcurrentDispatch(t *testing.T) {
	bus := events.NewBus()
	m := NewMachine(bus)

	m.SetWorkUnit(&WorkUnit{
		ID:     "test-123",
		Source: &Source{Reference: "file:task.md"},
	})

	errors := make([]error, 10)

	// Multiple concurrent dispatch attempts - only one should succeed
	// Using EventPlan which transitions from Idle to Planning
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Go(func() {
			errors[i] = m.Dispatch(context.Background(), EventPlan)
		})
	}

	wg.Wait()

	// Count successes
	successCount := 0
	for _, err := range errors {
		if err == nil {
			successCount++
		}
	}

	// Only one should succeed (from idle to planning), rest should fail
	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount)
	}
}

// Table-driven tests for transitions.
func TestTransitions(t *testing.T) {
	tests := []struct {
		setup     func(*Machine)
		name      string
		fromState State
		event     Event
		wantState State
		wantErr   bool
	}{
		{
			name:      "idle start stays idle",
			fromState: StateIdle,
			event:     EventStart,
			wantState: StateIdle,
			setup: func(m *Machine) {
				m.SetWorkUnit(&WorkUnit{ID: "t", Source: &Source{Reference: "f"}})
			},
			wantErr: false,
		},
		{
			name:      "idle start fails without source",
			fromState: StateIdle,
			event:     EventStart,
			wantState: StateIdle,
			setup:     func(m *Machine) {},
			wantErr:   true,
		},
		{
			name:      "idle to planning",
			fromState: StateIdle,
			event:     EventPlan,
			wantState: StatePlanning,
			setup:     func(m *Machine) {},
			wantErr:   false,
		},
		{
			name:      "planning to idle on done",
			fromState: StatePlanning,
			event:     EventPlanDone,
			wantState: StateIdle,
			setup: func(m *Machine) {
				m.mu.Lock()
				m.state = StatePlanning
				m.mu.Unlock()
			},
			wantErr: false,
		},
		{
			name:      "global abort transition",
			fromState: StatePlanning,
			event:     EventAbort,
			wantState: StateFailed,
			setup: func(m *Machine) {
				m.mu.Lock()
				m.state = StatePlanning
				m.mu.Unlock()
			},
			wantErr: false,
		},
		{
			name:      "idle to implementing with specifications",
			fromState: StateIdle,
			event:     EventImplement,
			wantState: StateImplementing,
			setup: func(m *Machine) {
				m.SetWorkUnit(&WorkUnit{ID: "t", Specifications: []string{"specification-1.md"}})
			},
			wantErr: false,
		},
		{
			name:      "idle to implementing fails without specifications",
			fromState: StateIdle,
			event:     EventImplement,
			wantState: StateIdle,
			setup:     func(m *Machine) {},
			wantErr:   true,
		},
		{
			name:      "idle to done on finish",
			fromState: StateIdle,
			event:     EventFinish,
			wantState: StateDone,
			setup: func(m *Machine) {
				// Guard requires specifications to finish
				m.SetWorkUnit(&WorkUnit{
					ID:             "test-123",
					Specifications: []string{"specification-1.md"},
				})
			},
			wantErr: false,
		},
		{
			name:      "idle to done fails without specifications",
			fromState: StateIdle,
			event:     EventFinish,
			wantState: StateIdle, // Stays in idle
			setup:     func(m *Machine) {},
			wantErr:   true, // Guard fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := events.NewBus()
			m := NewMachine(bus)
			tt.setup(m)

			err := m.Dispatch(context.Background(), tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dispatch() error = %v, wantErr %v", err, tt.wantErr)
			}

			if m.State() != tt.wantState {
				t.Errorf("state = %v, want %v", m.State(), tt.wantState)
			}
		})
	}
}

func TestIsPhaseState(t *testing.T) {
	tests := []struct {
		state State
		want  bool
	}{
		{StateIdle, true},
		{StatePlanning, true},
		{StateImplementing, true},
		{StateReviewing, true},
		{StateDone, true},
		{StateFailed, false},
		{StateWaiting, false},
		{StateCheckpointing, false},
		{StateReverting, false},
		{StateRestoring, false},
		{State("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			got := IsPhaseState(tt.state)
			if got != tt.want {
				t.Errorf("IsPhaseState(%v) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestIsTerminalState(t *testing.T) {
	tests := []struct {
		state State
		want  bool
	}{
		{StateIdle, false},
		{StatePlanning, false},
		{StateImplementing, false},
		{StateReviewing, false},
		{StateDone, true},
		{StateFailed, false},
		{StateWaiting, false},
		{StateCheckpointing, false},
		{State("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			got := IsTerminal(tt.state)
			if got != tt.want {
				t.Errorf("IsTerminal(%v) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestWorkUnitStruct(t *testing.T) {
	wu := WorkUnit{
		ID:             "task-123",
		ExternalID:     "ext-456",
		Title:          "Test Task",
		Description:    "A test task",
		Specifications: []string{"spec-1.md", "spec-2.md"},
		Checkpoints:    []string{"cp-1", "cp-2"},
	}

	if wu.ID != "task-123" {
		t.Errorf("ID = %v, want task-123", wu.ID)
	}
	if wu.ExternalID != "ext-456" {
		t.Errorf("ExternalID = %v, want ext-456", wu.ExternalID)
	}
	if len(wu.Specifications) != 2 {
		t.Errorf("Specifications length = %d, want 2", len(wu.Specifications))
	}
	if len(wu.Checkpoints) != 2 {
		t.Errorf("Checkpoints length = %d, want 2", len(wu.Checkpoints))
	}
}

func TestSourceStruct(t *testing.T) {
	src := Source{
		Reference: "file:task.md",
		Provider:  nil,
		Content:   "# Task Content",
	}

	if src.Reference != "file:task.md" {
		t.Errorf("Reference = %v, want file:task.md", src.Reference)
	}
	if src.Content != "# Task Content" {
		t.Errorf("Content = %v, want '# Task Content'", src.Content)
	}
}

func TestStateRegistryContainsAllStates(t *testing.T) {
	expectedStates := []State{
		StateIdle,
		StatePlanning,
		StateImplementing,
		StateReviewing,
		StateDone,
		StateFailed,
		StateWaiting,
		StateCheckpointing,
		StateReverting,
		StateRestoring,
	}

	for _, state := range expectedStates {
		if _, ok := StateRegistry[state]; !ok {
			t.Errorf("StateRegistry missing state: %v", state)
		}
	}
}

func TestPhaseStatesSlice(t *testing.T) {
	if len(PhaseStates) != 5 {
		t.Errorf("PhaseStates length = %d, want 5", len(PhaseStates))
	}

	expectedPhases := map[State]bool{
		StateIdle:         true,
		StatePlanning:     true,
		StateImplementing: true,
		StateReviewing:    true,
		StateDone:         true,
	}

	for _, state := range PhaseStates {
		if !expectedPhases[state] {
			t.Errorf("unexpected state in PhaseStates: %v", state)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests for uncovered functions
// ──────────────────────────────────────────────────────────────────────────────

func TestGetStateInfo(t *testing.T) {
	m := NewMachine(nil)

	tests := []struct {
		name     string
		state    State
		wantOK   bool
		wantTerm bool
	}{
		{"idle state", StateIdle, true, false},
		{"planning state", StatePlanning, true, false},
		{"implementing state", StateImplementing, true, false},
		{"reviewing state", StateReviewing, true, false},
		{"done state", StateDone, true, true},
		{"failed state", StateFailed, true, false}, // Can be recovered via EventReset
		{"waiting state", StateWaiting, true, false},
		{"checkpointing state", StateCheckpointing, true, false},
		{"reverting state", StateReverting, true, false},
		{"restoring state", StateRestoring, true, false},
		{"invalid state", State("invalid-state"), false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := m.GetStateInfo(tt.state)
			if ok != tt.wantOK {
				t.Errorf("GetStateInfo() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && info.Terminal != tt.wantTerm {
				t.Errorf("GetStateInfo() Terminal = %v, want %v", info.Terminal, tt.wantTerm)
			}
		})
	}
}

func TestPhaseOrder(t *testing.T) {
	m := NewMachine(nil)

	order := m.PhaseOrder()

	if len(order) != 5 {
		t.Errorf("PhaseOrder() length = %d, want 5", len(order))
	}

	// Verify the expected order
	expected := []State{StateIdle, StatePlanning, StateImplementing, StateReviewing, StateDone}
	for i, state := range order {
		if state != expected[i] {
			t.Errorf("PhaseOrder()[%d] = %v, want %v", i, state, expected[i])
		}
	}

	// Verify it's a copy (modifying returned slice doesn't affect internal)
	order[0] = StateFailed
	newOrder := m.PhaseOrder()
	if newOrder[0] == StateFailed {
		t.Error("PhaseOrder() should return a copy, not reference internal slice")
	}
}

func TestAllSteps(t *testing.T) {
	steps := AllSteps()

	expectedSteps := []Step{
		StepPlanning,
		StepImplementing,
		StepReviewing,
		StepCheckpointing,
	}

	if len(steps) != len(expectedSteps) {
		t.Errorf("AllSteps() length = %d, want %d", len(steps), len(expectedSteps))
	}

	for i, step := range steps {
		if step != expectedSteps[i] {
			t.Errorf("AllSteps()[%d] = %v, want %v", i, step, expectedSteps[i])
		}
	}
}

func TestIsValidStep(t *testing.T) {
	tests := []struct {
		name string
		step string
		want bool
	}{
		{"planning", "planning", true},
		{"implementing", "implementing", true},
		{"reviewing", "reviewing", true},
		{"checkpointing", "checkpointing", true},
		{"invalid step", "invalid", false},
		{"empty string", "", false},
		{"mixed case", "Planning", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidStep(tt.step)
			if got != tt.want {
				t.Errorf("IsValidStep(%q) = %v, want %v", tt.step, got, tt.want)
			}
		})
	}
}

func TestStep_String(t *testing.T) {
	tests := []struct {
		step Step
		want string
	}{
		{StepPlanning, "planning"},
		{StepImplementing, "implementing"},
		{StepReviewing, "reviewing"},
		{StepCheckpointing, "checkpointing"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.step.String()
			if got != tt.want {
				t.Errorf("Step.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetTransitions(t *testing.T) {
	tests := []struct {
		name    string
		from    State
		event   Event
		wantMin int
		wantMax int
	}{
		{
			name:    "idle to planning",
			from:    StateIdle,
			event:   EventPlan,
			wantMin: 1,
			wantMax: 10,
		},
		{
			name:    "planning done",
			from:    StatePlanning,
			event:   EventPlanDone,
			wantMin: 1,
			wantMax: 10,
		},
		{
			name:    "idle to implementing",
			from:    StateIdle,
			event:   EventImplement,
			wantMin: 1,
			wantMax: 10,
		},
		{
			name:    "implementing done",
			from:    StateImplementing,
			event:   EventImplementDone,
			wantMin: 1,
			wantMax: 10,
		},
		{
			name:    "invalid transition",
			from:    StateIdle,
			event:   EventImplementDone,
			wantMin: 0,
			wantMax: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transitions := GetTransitions(tt.from, tt.event)
			if len(transitions) < tt.wantMin || len(transitions) > tt.wantMax {
				t.Errorf("GetTransitions() length = %d, want between %d and %d",
					len(transitions), tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestGetGlobalTransition(t *testing.T) {
	tests := []struct {
		name   string
		event  Event
		want   State
		wantOK bool
	}{
		{
			name:   "abort event",
			event:  EventAbort,
			want:   StateFailed,
			wantOK: true,
		},
		{
			name:   "non-global event",
			event:  EventPlan,
			want:   "",
			wantOK: false,
		},
		{
			name:   "unknown event",
			event:  Event("unknown"),
			want:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := GetGlobalTransition(tt.event)
			if ok != tt.wantOK {
				t.Errorf("GetGlobalTransition() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("GetGlobalTransition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name  string
		from  State
		event Event
		want  bool
	}{
		{"idle to planning", StateIdle, EventPlan, true},
		{"idle to implementing", StateIdle, EventImplement, true},
		{"idle to reviewing", StateIdle, EventReview, true},
		{"idle to finish", StateIdle, EventFinish, true},
		{"planning to plan done", StatePlanning, EventPlanDone, true},
		{"planning error", StatePlanning, EventError, true},
		{"implementing done", StateImplementing, EventImplementDone, true},
		{"implementing error", StateImplementing, EventError, true},
		{"reviewing done", StateReviewing, EventReviewDone, true},
		{"reviewing error", StateReviewing, EventError, true},
		{"global abort from any state", StateIdle, EventAbort, true},
		{"global abort from planning", StatePlanning, EventAbort, true},
		{"invalid transition", StatePlanning, EventImplement, false},
		{"invalid event", StateIdle, Event("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransition(tt.from, tt.event)
			if got != tt.want {
				t.Errorf("CanTransition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEffectRegistry_Has(t *testing.T) {
	registry := NewEffectRegistry()

	// Initially empty
	if registry.Has(EffectInitWorkUnit) {
		t.Error("registry should not have InitWorkUnit effect initially")
	}

	// Register an effect
	registry.Register(EffectInitWorkUnit, EffectFunc(func(ctx context.Context, wu *WorkUnit) error {
		return nil
	}))

	// Now it should exist
	if !registry.Has(EffectInitWorkUnit) {
		t.Error("registry should have InitWorkUnit effect after registration")
	}

	// Unknown effect should not exist
	if registry.Has(EffectType("unknown")) {
		t.Error("registry should not have unknown effect type")
	}
}

func TestStepConstants(t *testing.T) {
	if StepPlanning != "planning" {
		t.Errorf("StepPlanning = %q, want %q", StepPlanning, "planning")
	}
	if StepImplementing != "implementing" {
		t.Errorf("StepImplementing = %q, want %q", StepImplementing, "implementing")
	}
	if StepReviewing != "reviewing" {
		t.Errorf("StepReviewing = %q, want %q", StepReviewing, "reviewing")
	}
	if StepCheckpointing != "checkpointing" {
		t.Errorf("StepCheckpointing = %q, want %q", StepCheckpointing, "checkpointing")
	}
}
