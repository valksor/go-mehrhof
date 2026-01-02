package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/valksor/go-mehrhof/internal/events"
)

func TestNewMachineBuilder(t *testing.T) {
	builder := NewMachineBuilder()

	// Should have copied base states
	if !builder.HasState(StateIdle) {
		t.Error("builder should have StateIdle")
	}
	if !builder.HasState(StatePlanning) {
		t.Error("builder should have StatePlanning")
	}

	// Should have copied base transitions
	if !builder.HasTransition(StateIdle, EventPlan) {
		t.Error("builder should have idle->planning transition")
	}

	// Should have correct phase order
	order := builder.PhaseOrder()
	if len(order) != len(PhaseStates) {
		t.Errorf("phase order length = %d, want %d", len(order), len(PhaseStates))
	}
}

func TestMachineBuilder_RegisterPhase(t *testing.T) {
	builder := NewMachineBuilder()

	phase := PhaseDefinition{
		State:       State("plugin_approval_approve"),
		Description: "Manager approval phase",
		After:       StateReviewing,
		EntryEvent:  Event("approve_start"),
		ExitEvent:   Event("approve_done"),
	}

	err := builder.RegisterPhase(phase)
	if err != nil {
		t.Fatalf("RegisterPhase failed: %v", err)
	}

	// State should be registered
	if !builder.HasState(phase.State) {
		t.Error("phase state should be registered")
	}

	// Entry transition should be registered
	if !builder.HasTransition(StateReviewing, phase.EntryEvent) {
		t.Error("entry transition should be registered")
	}

	// Exit transition should be registered
	if !builder.HasTransition(phase.State, phase.ExitEvent) {
		t.Error("exit transition should be registered")
	}

	// Error transition should be registered
	if !builder.HasTransition(phase.State, EventError) {
		t.Error("error transition should be registered")
	}

	// Phase order should include new phase
	order := builder.PhaseOrder()
	found := false
	for _, s := range order {
		if s == phase.State {
			found = true

			break
		}
	}
	if !found {
		t.Error("phase should be in phase order")
	}
}

func TestMachineBuilder_RegisterPhase_Validation(t *testing.T) {
	tests := []struct {
		name    string
		phase   PhaseDefinition
		wantErr bool
	}{
		{
			name:    "missing state",
			phase:   PhaseDefinition{EntryEvent: "start", ExitEvent: "done", After: StateIdle},
			wantErr: true,
		},
		{
			name:    "missing entry event",
			phase:   PhaseDefinition{State: "test", ExitEvent: "done", After: StateIdle},
			wantErr: true,
		},
		{
			name:    "missing exit event",
			phase:   PhaseDefinition{State: "test", EntryEvent: "start", After: StateIdle},
			wantErr: true,
		},
		{
			name:    "missing insertion point",
			phase:   PhaseDefinition{State: "test", EntryEvent: "start", ExitEvent: "done"},
			wantErr: true,
		},
		{
			name:    "both after and before",
			phase:   PhaseDefinition{State: "test", EntryEvent: "start", ExitEvent: "done", After: StateIdle, Before: StateDone},
			wantErr: true,
		},
		{
			name:    "invalid anchor state",
			phase:   PhaseDefinition{State: "test", EntryEvent: "start", ExitEvent: "done", After: State("nonexistent")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewMachineBuilder()
			err := builder.RegisterPhase(tt.phase)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterPhase() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMachineBuilder_Build(t *testing.T) {
	builder := NewMachineBuilder()
	bus := events.NewBus()

	machine := builder.Build(bus)

	if machine == nil {
		t.Fatal("Build() returned nil")
	}

	// Machine should start in idle state
	if machine.State() != StateIdle {
		t.Errorf("initial state = %v, want %v", machine.State(), StateIdle)
	}

	// Machine should be able to dispatch events
	machine.SetWorkUnit(&WorkUnit{
		ID:    "test",
		Title: "Test Task",
		Source: &Source{
			Reference: "test:123",
		},
	})

	err := machine.Dispatch(context.Background(), EventPlan)
	if err != nil {
		t.Errorf("Dispatch(EventPlan) failed: %v", err)
	}

	if machine.State() != StatePlanning {
		t.Errorf("state after plan = %v, want %v", machine.State(), StatePlanning)
	}
}

func TestMachineBuilder_AddGuardToTransition(t *testing.T) {
	builder := NewMachineBuilder()

	guardCalled := false
	guard := func(ctx context.Context, wu *WorkUnit) bool {
		guardCalled = true

		return false // Block transition
	}

	err := builder.AddGuardToTransition(StateIdle, EventPlan, guard)
	if err != nil {
		t.Fatalf("AddGuardToTransition failed: %v", err)
	}

	bus := events.NewBus()
	machine := builder.Build(bus)
	machine.SetWorkUnit(&WorkUnit{ID: "test"})

	// Dispatch should fail because guard returns false
	err = machine.Dispatch(context.Background(), EventPlan)
	if err == nil {
		t.Error("Dispatch should fail when guard returns false")
	}

	if !guardCalled {
		t.Error("guard should have been called")
	}
}

func TestMachineBuilder_WithPluginPhase(t *testing.T) {
	builder := NewMachineBuilder()

	// Register a plugin phase after reviewing
	phase := PhaseDefinition{
		State:       State("plugin_test_approval"),
		Description: "Test approval phase",
		After:       StateReviewing,
		EntryEvent:  Event("test_approval_start"),
		ExitEvent:   Event("test_approval_done"),
	}

	err := builder.RegisterPhase(phase)
	if err != nil {
		t.Fatalf("RegisterPhase failed: %v", err)
	}

	bus := events.NewBus()
	machine := builder.Build(bus)
	machine.SetWorkUnit(&WorkUnit{
		ID:             "test",
		Specifications: []string{"spec.md"},
		Source:         &Source{Reference: "test:123"},
	})

	// Navigate to reviewing state
	_ = machine.Dispatch(context.Background(), EventReview)
	if machine.State() != StateReviewing {
		t.Skipf("couldn't get to reviewing state: %v", machine.State())
	}

	// Dispatch to plugin phase
	err = machine.Dispatch(context.Background(), phase.EntryEvent)
	if err != nil {
		t.Errorf("Dispatch to plugin phase failed: %v", err)
	}

	if machine.State() != phase.State {
		t.Errorf("state = %v, want %v", machine.State(), phase.State)
	}

	// Exit plugin phase
	err = machine.Dispatch(context.Background(), phase.ExitEvent)
	if err != nil {
		t.Errorf("Dispatch exit event failed: %v", err)
	}

	if machine.State() != StateIdle {
		t.Errorf("state after exit = %v, want %v", machine.State(), StateIdle)
	}
}

func TestExecuteEffects_Critical(t *testing.T) {
	wu := &WorkUnit{ID: "test"}

	t.Run("critical effect failure blocks workflow", func(t *testing.T) {
		effects := []CriticalEffect{
			{Name: "effect1", Fn: func(ctx context.Context, wu *WorkUnit) error { return nil }, Critical: false},
			{Name: "critical", Fn: func(ctx context.Context, wu *WorkUnit) error { return errors.New("failed") }, Critical: true},
		}

		err := ExecuteEffects(context.Background(), wu, effects)
		if err == nil {
			t.Error("expected error from critical effect")
		}
	})

	t.Run("non-critical effect failure continues", func(t *testing.T) {
		effects := []CriticalEffect{
			{Name: "effect1", Fn: func(ctx context.Context, wu *WorkUnit) error { return errors.New("failed") }, Critical: false},
			{Name: "effect2", Fn: func(ctx context.Context, wu *WorkUnit) error { return nil }, Critical: false},
		}

		err := ExecuteEffects(context.Background(), wu, effects)
		if err != nil {
			t.Errorf("non-critical effect should not return error: %v", err)
		}
	})

	t.Run("all effects execute in order", func(t *testing.T) {
		order := []string{}
		effects := []CriticalEffect{
			{Name: "first", Fn: func(ctx context.Context, wu *WorkUnit) error {
				order = append(order, "first")

				return nil
			}},
			{Name: "second", Fn: func(ctx context.Context, wu *WorkUnit) error {
				order = append(order, "second")

				return nil
			}},
		}

		_ = ExecuteEffects(context.Background(), wu, effects)
		if len(order) != 2 || order[0] != "first" || order[1] != "second" {
			t.Errorf("effects not executed in order: %v", order)
		}
	})
}

func TestWrapEffect(t *testing.T) {
	fn := func(ctx context.Context, wu *WorkUnit) error { return nil }

	effect := WrapEffect("test", fn)
	if effect.Name != "test" {
		t.Errorf("Name = %v, want test", effect.Name)
	}
	if effect.Critical {
		t.Error("WrapEffect should create non-critical effect")
	}

	critical := WrapCriticalEffect("critical", fn)
	if !critical.Critical {
		t.Error("WrapCriticalEffect should create critical effect")
	}
}
