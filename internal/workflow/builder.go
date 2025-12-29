package workflow

import (
	"fmt"

	"github.com/valksor/go-mehrhof/internal/events"
)

// MachineBuilder constructs a Machine with custom phases, guards, and effects.
// Use the builder pattern to configure the workflow before creating the machine.
type MachineBuilder struct {
	states      map[State]StateInfo
	transitions map[TransitionKey][]Transition
	globals     map[Event]State
	phaseOrder  []State // Ordered list of main phases for insertion
}

// NewMachineBuilder creates a builder initialized with the base workflow configuration.
func NewMachineBuilder() *MachineBuilder {
	b := &MachineBuilder{
		states:      make(map[State]StateInfo),
		transitions: make(map[TransitionKey][]Transition),
		globals:     make(map[Event]State),
		phaseOrder:  make([]State, 0),
	}

	// Copy base state registry
	for k, v := range StateRegistry {
		b.states[k] = v
	}

	// Copy base transition table (deep copy to avoid mutation)
	for k, transitions := range TransitionTable {
		copied := make([]Transition, len(transitions))
		for i, t := range transitions {
			copied[i] = Transition{
				From:    t.From,
				Event:   t.Event,
				To:      t.To,
				Guards:  append([]GuardFunc{}, t.Guards...),
				Effects: append([]EffectFunc{}, t.Effects...),
			}
		}
		b.transitions[k] = copied
	}

	// Copy global transitions
	for k, v := range GlobalTransitions {
		b.globals[k] = v
	}

	// Copy phase order
	b.phaseOrder = append(b.phaseOrder, PhaseStates...)

	return b
}

// PhaseDefinition describes a custom phase to be registered with the workflow.
type PhaseDefinition struct {
	State       State            // Unique state identifier for this phase
	Description string           // Human-readable description
	After       State            // Insert after this phase (mutually exclusive with Before)
	Before      State            // Insert before this phase (mutually exclusive with After)
	EntryEvent  Event            // Event to enter this phase
	ExitEvent   Event            // Event to exit this phase (returns to normal flow)
	Guards      []GuardFunc      // Guards for entering this phase
	Effects     []CriticalEffect // Effects to execute on phase transitions
	TalkAllowed bool             // Whether talk mode is available in this phase
}

// RegisterPhase adds a custom phase to the workflow.
// The phase will be inserted based on its After or Before field.
func (b *MachineBuilder) RegisterPhase(phase PhaseDefinition) error {
	// Validate phase definition
	if phase.State == "" {
		return fmt.Errorf("phase state is required")
	}
	if phase.EntryEvent == "" {
		return fmt.Errorf("phase entry event is required")
	}
	if phase.ExitEvent == "" {
		return fmt.Errorf("phase exit event is required")
	}
	if phase.After == "" && phase.Before == "" {
		return fmt.Errorf("phase must specify either After or Before insertion point")
	}
	if phase.After != "" && phase.Before != "" {
		return fmt.Errorf("phase cannot specify both After and Before")
	}

	// Check if state already exists
	if _, exists := b.states[phase.State]; exists {
		return fmt.Errorf("phase state %s already exists", phase.State)
	}

	// Determine insertion point
	insertIdx := -1
	var anchorState State
	if phase.After != "" {
		anchorState = phase.After
		for i, s := range b.phaseOrder {
			if s == anchorState {
				insertIdx = i + 1 // Insert after anchor
				break
			}
		}
	} else {
		anchorState = phase.Before
		for i, s := range b.phaseOrder {
			if s == anchorState {
				insertIdx = i // Insert at anchor position (before it)
				break
			}
		}
	}

	if insertIdx == -1 {
		return fmt.Errorf("anchor state %s not found in phase order", anchorState)
	}

	// Add state info
	b.states[phase.State] = StateInfo{
		Name:        phase.State,
		Description: phase.Description,
		Terminal:    false,
		Phase:       true, // Plugin phases are main phases
		TalkAllowed: phase.TalkAllowed,
	}

	// Insert into phase order
	newPhaseOrder := make([]State, 0, len(b.phaseOrder)+1)
	newPhaseOrder = append(newPhaseOrder, b.phaseOrder[:insertIdx]...)
	newPhaseOrder = append(newPhaseOrder, phase.State)
	newPhaseOrder = append(newPhaseOrder, b.phaseOrder[insertIdx:]...)
	b.phaseOrder = newPhaseOrder

	// Rewire transitions
	if err := b.rewireTransitions(phase, anchorState); err != nil {
		return fmt.Errorf("rewire transitions: %w", err)
	}

	return nil
}

// rewireTransitions modifies transitions to route through the new phase.
func (b *MachineBuilder) rewireTransitions(phase PhaseDefinition, anchorState State) error {
	// Add entry transition: anchorState --entryEvent--> newPhase
	entryKey := TransitionKey{From: anchorState, Event: phase.EntryEvent}
	entryGuards := append([]GuardFunc{}, phase.Guards...)
	b.transitions[entryKey] = []Transition{{
		From:   anchorState,
		Event:  phase.EntryEvent,
		To:     phase.State,
		Guards: entryGuards,
	}}

	// Add exit transition: newPhase --exitEvent--> idle (standard return point)
	exitKey := TransitionKey{From: phase.State, Event: phase.ExitEvent}
	b.transitions[exitKey] = []Transition{{
		From:  phase.State,
		Event: phase.ExitEvent,
		To:    StateIdle,
	}}

	// Add error transition: newPhase --error--> idle
	errorKey := TransitionKey{From: phase.State, Event: EventError}
	b.transitions[errorKey] = []Transition{{
		From:  phase.State,
		Event: EventError,
		To:    StateIdle,
	}}

	return nil
}

// AddGuardToTransition adds a guard to an existing transition.
func (b *MachineBuilder) AddGuardToTransition(from State, event Event, guard GuardFunc) error {
	key := TransitionKey{From: from, Event: event}
	transitions, ok := b.transitions[key]
	if !ok || len(transitions) == 0 {
		return fmt.Errorf("no transition from %s on event %s", from, event)
	}

	// Add guard to all transitions for this key
	for i := range transitions {
		transitions[i].Guards = append(transitions[i].Guards, guard)
	}
	b.transitions[key] = transitions

	return nil
}

// AddEffectToTransition adds an effect to an existing transition.
// Note: This adds the underlying EffectFunc. For critical effects,
// use RegisterTransitionEffects which handles the CriticalEffect wrapper.
func (b *MachineBuilder) AddEffectToTransition(from State, event Event, effect EffectFunc) error {
	key := TransitionKey{From: from, Event: event}
	transitions, ok := b.transitions[key]
	if !ok || len(transitions) == 0 {
		return fmt.Errorf("no transition from %s on event %s", from, event)
	}

	// Add effect to all transitions for this key
	for i := range transitions {
		transitions[i].Effects = append(transitions[i].Effects, effect)
	}
	b.transitions[key] = transitions

	return nil
}

// RegisterState adds a custom state to the registry without wiring transitions.
// Use this for auxiliary states that don't need phase insertion logic.
func (b *MachineBuilder) RegisterState(info StateInfo) error {
	if info.Name == "" {
		return fmt.Errorf("state name is required")
	}
	if _, exists := b.states[info.Name]; exists {
		return fmt.Errorf("state %s already exists", info.Name)
	}
	b.states[info.Name] = info
	return nil
}

// RegisterTransition adds a custom transition.
func (b *MachineBuilder) RegisterTransition(t Transition) {
	key := TransitionKey{From: t.From, Event: t.Event}
	b.transitions[key] = append(b.transitions[key], t)
}

// Build creates a Machine with the configured workflow.
func (b *MachineBuilder) Build(eventBus *events.Bus) *Machine {
	return &Machine{
		state:             StateIdle,
		eventBus:          eventBus,
		listeners:         nil,
		history:           nil,
		undoStack:         nil,
		redoStack:         nil,
		listenerSem:       make(chan struct{}, 10), // Max 10 concurrent listener calls
		stateRegistry:     b.states,
		transitionTable:   b.transitions,
		globalTransitions: b.globals,
		phaseOrder:        b.phaseOrder,
	}
}

// PhaseOrder returns the current phase order (useful for debugging).
func (b *MachineBuilder) PhaseOrder() []State {
	result := make([]State, len(b.phaseOrder))
	copy(result, b.phaseOrder)
	return result
}

// HasState checks if a state is registered.
func (b *MachineBuilder) HasState(s State) bool {
	_, ok := b.states[s]
	return ok
}

// HasTransition checks if a transition exists.
func (b *MachineBuilder) HasTransition(from State, event Event) bool {
	key := TransitionKey{From: from, Event: event}
	transitions, ok := b.transitions[key]
	return ok && len(transitions) > 0
}
