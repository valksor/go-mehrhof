package workflow

import (
	"context"
	"fmt"
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
}

// HistoryEntry records a state transition
type HistoryEntry struct {
	From  State
	To    State
	Event Event
}

// NewMachine creates a new state machine
func NewMachine(eventBus *events.Bus) *Machine {
	return &Machine{
		state:     StateIdle,
		eventBus:  eventBus,
		listeners: make([]StateListener, 0),
		history:   make([]HistoryEntry, 0),
		undoStack: make([]string, 0),
		redoStack: make([]string, 0),
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
	m.mu.Lock()
	defer m.mu.Unlock()

	from := m.state

	// Check global transitions first (error, abort)
	if to, ok := GetGlobalTransition(event); ok {
		return m.transitionTo(ctx, from, to, event)
	}

	// Get possible transitions
	transitions := GetTransitions(from, event)
	if len(transitions) == 0 {
		return fmt.Errorf("no transition from %s on event %s", from, event)
	}

	// Find first transition where all guards pass
	for _, t := range transitions {
		if EvaluateGuards(ctx, m.workUnit, t.Guards) {
			return m.transitionTo(ctx, from, t.To, event)
		}
	}

	return fmt.Errorf("no valid transition from %s on event %s (guards failed)", from, event)
}

// CanDispatch checks if a transition is possible
func (m *Machine) CanDispatch(ctx context.Context, event Event) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	from := m.state

	// Check global transitions
	if _, ok := GetGlobalTransition(event); ok {
		return true, ""
	}

	// Get possible transitions
	transitions := GetTransitions(from, event)
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
	m.history = make([]HistoryEntry, 0)
	m.undoStack = make([]string, 0)
	m.redoStack = make([]string, 0)
}

// PushUndo adds a checkpoint to the undo stack
func (m *Machine) PushUndo(checkpoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.undoStack = append(m.undoStack, checkpoint)
	// Clear redo stack on new action
	m.redoStack = make([]string, 0)
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

	info, ok := StateRegistry[m.state]
	if !ok {
		return false
	}
	return info.Terminal
}
