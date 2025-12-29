package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/valksor/go-mehrhof/internal/events"
)

// StateListener is called when state changes
type StateListener func(from, to State, event Event, wu *WorkUnit)

// Machine manages workflow state transitions
type Machine struct {
	mu sync.RWMutex

	state     State
	workUnit  *WorkUnit
	eventBus  *events.Bus
	listeners []StateListener
	history   []HistoryEntry

	// Undo/redo stacks
	undoStack []string
	redoStack []string

	// Semaphore to limit concurrent listener notifications (prevents unbounded goroutines)
	listenerSem chan struct{}

	// Instance-level configuration (set by builder, or defaults to package-level globals)
	stateRegistry     map[State]StateInfo
	transitionTable   map[TransitionKey][]Transition
	globalTransitions map[Event]State
	phaseOrder        []State
}

// HistoryEntry records a state transition
type HistoryEntry struct {
	From  State
	To    State
	Event Event
}

// NewMachine creates a new state machine with default workflow configuration.
// Use NewMachineBuilder().Build() for custom configurations.
func NewMachine(eventBus *events.Bus) *Machine {
	return &Machine{
		state:       StateIdle,
		eventBus:    eventBus,
		listeners:   nil,
		history:     nil,
		undoStack:   nil,
		redoStack:   nil,
		listenerSem: make(chan struct{}, 10), // Max 10 concurrent listener calls
		// Use package-level defaults
		stateRegistry:     StateRegistry,
		transitionTable:   TransitionTable,
		globalTransitions: GlobalTransitions,
		phaseOrder:        PhaseStates,
	}
}

// State returns the current state
func (m *Machine) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// WorkUnit returns the current work unit
func (m *Machine) WorkUnit() *WorkUnit {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.workUnit
}

// SetWorkUnit sets the work unit
func (m *Machine) SetWorkUnit(wu *WorkUnit) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workUnit = wu
}

// AddListener registers a state change listener
func (m *Machine) AddListener(listener StateListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, listener)
}

// Dispatch attempts to transition based on an event
func (m *Machine) Dispatch(ctx context.Context, event Event) error {
	// First pass: check if transition is possible and evaluate guards
	// We do this without holding the lock to avoid blocking if guards do I/O

	var from State
	var transitions []Transition
	var globalTo State
	hasGlobal := false

	// Snapshot current state and transitions
	m.mu.RLock()
	from = m.state
	if to, ok := m.globalTransitions[event]; ok {
		globalTo = to
		hasGlobal = true
	} else {
		key := TransitionKey{From: from, Event: event}
		transitions = m.transitionTable[key]
	}
	wu := m.workUnit
	m.mu.RUnlock()

	// Handle global transitions (no guards to evaluate)
	if hasGlobal {
		m.mu.Lock()
		defer m.mu.Unlock()
		// Re-check state hasn't changed while we were thinking
		if m.state != from {
			return fmt.Errorf("state changed from %s to %s during dispatch", from, m.state)
		}
		return m.transitionTo(ctx, from, globalTo, event)
	}

	// No transitions available
	if len(transitions) == 0 {
		return fmt.Errorf("no transition from %s on event %s", from, event)
	}

	// Evaluate guards outside lock (guards may do I/O like RPC calls)
	var validTransition *Transition
	for _, t := range transitions {
		if EvaluateGuards(ctx, wu, t.Guards) {
			validTransition = &t
			break
		}
	}

	if validTransition == nil {
		return fmt.Errorf("no valid transition from %s on event %s (guards failed)", from, event)
	}

	// Acquire write lock for the actual transition
	m.mu.Lock()
	defer m.mu.Unlock()

	// Final check: state hasn't changed since we evaluated guards
	if m.state != from {
		return fmt.Errorf("state changed from %s to %s during dispatch", from, m.state)
	}

	return m.transitionTo(ctx, from, validTransition.To, event)
}

// CanDispatch checks if a transition is possible
func (m *Machine) CanDispatch(ctx context.Context, event Event) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	from := m.state

	// Check global transitions
	if _, ok := m.globalTransitions[event]; ok {
		return true, ""
	}

	// Get possible transitions from instance table
	key := TransitionKey{From: from, Event: event}
	transitions := m.transitionTable[key]
	if len(transitions) == 0 {
		return false, fmt.Sprintf("no transition from %s on event %s", from, event)
	}

	// Check if any transition's guards pass
	for _, t := range transitions {
		if EvaluateGuards(ctx, m.workUnit, t.Guards) {
			return true, ""
		}
	}

	return false, fmt.Sprintf("guards failed for transition from %s on event %s", from, event)
}

// transitionTo performs the actual state change (must hold lock)
func (m *Machine) transitionTo(ctx context.Context, from, to State, event Event) error {
	// Record history
	m.history = append(m.history, HistoryEntry{
		From:  from,
		To:    to,
		Event: event,
	})

	// Update state
	m.state = to

	// Capture data for async notifications while holding lock
	listeners := make([]StateListener, len(m.listeners))
	copy(listeners, m.listeners)
	wu := m.workUnit
	taskID := m.getTaskID()

	// Publish event asynchronously (non-blocking, safe to call with lock held)
	if m.eventBus != nil {
		m.eventBus.PublishAsync(events.StateChangedEvent{
			From:   string(from),
			To:     string(to),
			Event:  string(event),
			TaskID: taskID,
		})
	}

	// Notify listeners asynchronously to prevent deadlocks
	// Note: Listeners are called in a separate goroutine to avoid blocking
	// the state machine and to prevent re-entrancy issues if listeners
	// attempt to dispatch events.
	if len(listeners) > 0 {
		go func() {
			// Acquire semaphore
			m.listenerSem <- struct{}{}
			// Ensure semaphore is released even if listener panics
			defer func() {
				if r := recover(); r != nil {
					// Log panic but don't crash the state machine
					slog.Warn("state listener panicked", "panic", r)
				}
				<-m.listenerSem // Release semaphore
			}()

			for _, listener := range listeners {
				listener(from, to, event, wu)
			}
		}()
	}

	return nil
}

// getTaskID returns the current task ID (must hold lock or be called from locked context)
func (m *Machine) getTaskID() string {
	if m.workUnit != nil {
		return m.workUnit.ID
	}
	return ""
}

// History returns the transition history
func (m *Machine) History() []HistoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]HistoryEntry, len(m.history))
	copy(history, m.history)
	return history
}

// Reset resets the machine to idle state
func (m *Machine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state = StateIdle
	m.workUnit = nil
	m.history = nil
	m.undoStack = nil
	m.redoStack = nil
}

// PushUndo adds a checkpoint to the undo stack
func (m *Machine) PushUndo(checkpoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.undoStack = append(m.undoStack, checkpoint)
	// Clear redo stack on new action
	m.redoStack = nil
}

// PopUndo removes and returns the last checkpoint from undo stack
func (m *Machine) PopUndo() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.undoStack) == 0 {
		return "", false
	}

	checkpoint := m.undoStack[len(m.undoStack)-1]
	m.undoStack = m.undoStack[:len(m.undoStack)-1]
	m.redoStack = append(m.redoStack, checkpoint)
	return checkpoint, true
}

// PopRedo removes and returns the last checkpoint from redo stack
func (m *Machine) PopRedo() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.redoStack) == 0 {
		return "", false
	}

	checkpoint := m.redoStack[len(m.redoStack)-1]
	m.redoStack = m.redoStack[:len(m.redoStack)-1]
	m.undoStack = append(m.undoStack, checkpoint)
	return checkpoint, true
}

// CanUndo returns true if undo is possible
func (m *Machine) CanUndo() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.undoStack) > 0
}

// CanRedo returns true if redo is possible
func (m *Machine) CanRedo() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.redoStack) > 0
}

// IsTerminal returns true if current state is terminal
func (m *Machine) IsTerminal() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, ok := m.stateRegistry[m.state]
	if !ok {
		return false
	}
	return info.Terminal
}

// GetStateInfo returns state metadata for the given state
func (m *Machine) GetStateInfo(s State) (StateInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.stateRegistry[s]
	return info, ok
}

// PhaseOrder returns the current phase order
func (m *Machine) PhaseOrder() []State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]State, len(m.phaseOrder))
	copy(result, m.phaseOrder)
	return result
}
