package conductor

import (
	"context"
	"sync"
	"time"
)

// Machine manages workflow state transitions.
type Machine struct {
	mu sync.RWMutex

	state         State
	workUnit      *WorkUnit
	history       []HistoryEntry
	listeners     []StateListener
	previousState State // For resuming after wait/pause
	// priorStableState records the stable state before a re-entry transition.
	// Used by EventError/EventStop to roll back to the correct state instead of
	// the hardcoded default. Set by conductor before re-entry dispatches,
	// cleared after error/stop uses it.
	// Intentionally NOT persisted: only relevant while a job is running, and
	// running jobs don't survive server restarts. On restart, LoadState restores
	// the last persisted state (the in-progress state), which is acceptable.
	// Access via SetPriorStableState/ClearPriorStableState (holds m.mu).
	priorStableState State
}

// HistoryEntry records a state transition.
type HistoryEntry struct {
	From      State     `json:"from"`
	To        State     `json:"to"`
	Event     Event     `json:"event"`
	Timestamp time.Time `json:"timestamp"`
}

// StateListener is called when state changes.
type StateListener func(from, to State, event Event, wu *WorkUnit)

// NewMachine creates a new state machine.
func NewMachine() *Machine {
	return &Machine{
		state:   StateNone,
		history: make([]HistoryEntry, 0),
	}
}

// State returns the current state.
func (m *Machine) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.state
}

// WorkUnit returns the current work unit.
func (m *Machine) WorkUnit() *WorkUnit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.workUnit
}

// SetWorkUnit sets the work unit.
func (m *Machine) SetWorkUnit(wu *WorkUnit) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workUnit = wu
	if wu != nil {
		wu.UpdatedAt = time.Now()
	}
}

// SetPriorStableState records the stable state before a re-entry transition.
// Thread-safe: holds m.mu so callers do not need to coordinate with Dispatch.
func (m *Machine) SetPriorStableState(s State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.priorStableState = s
}

// ClearPriorStableState resets the prior stable state (e.g. on dispatch failure).
func (m *Machine) ClearPriorStableState() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.priorStableState = ""
}

// PriorStableState returns the current prior stable state (for testing).
func (m *Machine) PriorStableState() State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.priorStableState
}

// AddListener registers a state change listener.
func (m *Machine) AddListener(listener StateListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, listener)
}

// Dispatch attempts to transition based on an event.
func (m *Machine) Dispatch(ctx context.Context, event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	from := m.state

	// Get possible transitions
	key := TransitionKey{From: from, Event: event}
	transitions, ok := TransitionTable[key]
	if !ok || len(transitions) == 0 {
		return formatTransitionError(from, event, m.workUnit)
	}

	// Find first transition whose guards pass
	var validTransition *Transition
	for i := range transitions {
		if EvaluateGuards(ctx, m.workUnit, transitions[i].Guards) {
			validTransition = &transitions[i]

			break
		}
	}

	if validTransition == nil {
		return formatGuardError(from, event, m.workUnit, transitions) //nolint:contextcheck // Guard check only
	}

	// Track previous state for wait/pause resume
	if event == EventWait || event == EventPause {
		m.previousState = from
	}

	// For error/stop during re-entry, override the hardcoded rollback target
	// with the actual prior stable state.
	targetState := validTransition.To
	if (event == EventError || event == EventStop) && m.priorStableState != "" {
		targetState = m.priorStableState
		m.priorStableState = "" // Consumed
	}

	// Execute transition
	m.state = targetState
	m.history = append(m.history, HistoryEntry{
		From:      from,
		To:        targetState,
		Event:     event,
		Timestamp: time.Now(),
	})

	// Update work unit timestamp
	if m.workUnit != nil {
		m.workUnit.UpdatedAt = time.Now()
	}

	// Notify listeners (copy to avoid holding lock during callbacks)
	listeners := make([]StateListener, len(m.listeners))
	copy(listeners, m.listeners)
	wu := m.workUnit

	// Call listeners outside lock
	go func() {
		for _, listener := range listeners {
			listener(from, targetState, event, wu)
		}
	}()

	return nil
}

// DispatchWithResume handles Answer/Resume events by returning to previous state.
func (m *Machine) DispatchWithResume(ctx context.Context, event Event) error {
	m.mu.Lock()

	if event == EventAnswer || event == EventResume {
		if m.previousState != "" {
			// Modify transition table temporarily to go back to previous state
			from := m.state
			to := m.previousState
			m.state = to
			m.previousState = ""
			m.history = append(m.history, HistoryEntry{
				From:      from,
				To:        to,
				Event:     event,
				Timestamp: time.Now(),
			})
			m.mu.Unlock()

			return nil
		}
	}

	m.mu.Unlock()

	return m.Dispatch(ctx, event)
}

// CanDispatch checks if a transition is possible.
func (m *Machine) CanDispatch(ctx context.Context, event Event) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := TransitionKey{From: m.state, Event: event}
	transitions, ok := TransitionTable[key]
	if !ok || len(transitions) == 0 {
		return false, formatTransitionError(m.state, event, m.workUnit).Error()
	}

	for _, t := range transitions {
		if EvaluateGuards(ctx, m.workUnit, t.Guards) {
			return true, ""
		}
	}

	return false, formatGuardError(m.state, event, m.workUnit, transitions).Error() //nolint:contextcheck // Guard check only
}

// History returns the transition history.
func (m *Machine) History() []HistoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	history := make([]HistoryEntry, len(m.history))
	copy(history, m.history)

	return history
}

// RestoreHistory replaces the machine's transition history with the provided entries.
// Used when restoring persisted state from disk.
func (m *Machine) RestoreHistory(entries []HistoryEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = entries
}

// Reset resets the machine to None state.
func (m *Machine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = StateNone
	m.workUnit = nil
	m.history = nil
	m.previousState = ""
}

// ForceState forcefully sets the state without checking transitions.
// Used for re-running phases with --force flag.
func (m *Machine) ForceState(state State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
}

// IsTerminal returns true if current state is terminal.
func (m *Machine) IsTerminal() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := StateRegistry[m.state]

	return ok && info.Terminal
}

// IsPhase returns true if current state is a main phase.
func (m *Machine) IsPhase() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := StateRegistry[m.state]

	return ok && info.Phase
}

// AvailableEvents returns events that can be dispatched from current state.
func (m *Machine) AvailableEvents(ctx context.Context) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var events []Event
	for key, transitions := range TransitionTable {
		if key.From != m.state {
			continue
		}
		for _, t := range transitions {
			if EvaluateGuards(ctx, m.workUnit, t.Guards) {
				events = append(events, key.Event)

				break
			}
		}
	}

	return events
}

// CanTransition checks if a direct state transition is valid.
func CanTransition(from, to State) bool {
	for key, transitions := range TransitionTable {
		if key.From != from {
			continue
		}
		for _, t := range transitions {
			if t.To == to {
				return true
			}
		}
	}

	return false
}

// NextStates returns possible next states from a given state.
func NextStates(from State) []State {
	seen := make(map[State]bool)
	var next []State
	for key, transitions := range TransitionTable {
		if key.From != from {
			continue
		}
		for _, t := range transitions {
			if !seen[t.To] {
				seen[t.To] = true
				next = append(next, t.To)
			}
		}
	}

	return next
}
