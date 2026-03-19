package graph

import (
	"fmt"
	"sync"
)

// NodeState represents the execution state of a graph node.
type NodeState string

const (
	StatePending NodeState = "pending" // Not yet eligible to run
	StateQueued  NodeState = "queued"  // Dependencies satisfied, waiting for worker
	StateRunning NodeState = "running" // Executing on a worker
	StateDone    NodeState = "done"    // Completed successfully
	StateFailed  NodeState = "failed"  // Execution failed
	StateSkipped NodeState = "skipped" // Skipped by condition
)

// StateManager tracks execution state for all nodes and edges in a graph.
// Thread-safe for concurrent scheduler access.
type StateManager struct {
	mu         sync.RWMutex
	states     map[NodeID]NodeState
	results    map[NodeID]string
	errors     map[NodeID]string
	edgeStates map[string]EdgeState // key: "from->to"
	failEdges  map[NodeID]NodeID    // source → fail-branch target (copied from graph)
}

// NewStateManager creates a state manager with all nodes in pending state.
func NewStateManager(g *Graph) *StateManager {
	sm := &StateManager{
		states:     make(map[NodeID]NodeState, len(g.Nodes)),
		results:    make(map[NodeID]string),
		errors:     make(map[NodeID]string),
		edgeStates: make(map[string]EdgeState),
		failEdges:  make(map[NodeID]NodeID),
	}

	for id := range g.Nodes {
		sm.states[id] = StatePending
	}

	// Copy fail-branch mappings (populated after Validate/buildEdges).
	for src, target := range g.failEdges {
		sm.failEdges[src] = target
	}

	return sm
}

// Get returns the current state of a node.
func (sm *StateManager) Get(id NodeID) NodeState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.states[id]
}

// Transition moves a node to a new state. Returns an error for invalid transitions.
func (sm *StateManager) Transition(id NodeID, to NodeState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	from, exists := sm.states[id]
	if !exists {
		return fmt.Errorf("graph: unknown node %q", id)
	}

	if !validTransition(from, to) {
		return fmt.Errorf("graph: invalid transition %s → %s for node %q", from, to, id)
	}

	sm.states[id] = to

	return nil
}

// SetResult stores the output of a completed node.
func (sm *StateManager) SetResult(id NodeID, result string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.results[id] = result
}

// Result returns the stored result for a node.
func (sm *StateManager) Result(id NodeID) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.results[id]
}

// SetError stores the error message for a failed node.
func (sm *StateManager) SetError(id NodeID, errMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.errors[id] = errMsg
}

// Error returns the stored error for a node.
func (sm *StateManager) Error(id NodeID) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.errors[id]
}

// Results returns a copy of all node results.
func (sm *StateManager) Results() map[NodeID]string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	out := make(map[NodeID]string, len(sm.results))
	for k, v := range sm.results {
		out[k] = v
	}

	return out
}

// AllDone returns true if every node is in a terminal state (done, failed, or skipped).
func (sm *StateManager) AllDone() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, s := range sm.states {
		if s != StateDone && s != StateFailed && s != StateSkipped {
			return false
		}
	}

	return true
}

// HasFailures returns true if any node is in failed state.
func (sm *StateManager) HasFailures() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, s := range sm.states {
		if s == StateFailed {
			return true
		}
	}

	return false
}

// CountByState returns the count of nodes in each state.
func (sm *StateManager) CountByState() map[NodeState]int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	counts := make(map[NodeState]int)
	for _, s := range sm.states {
		counts[s]++
	}

	return counts
}

// DependenciesSatisfied checks if all dependencies of a node are resolved.
// A dependency is satisfied when it is done or skipped, or when it failed and
// this node is its fail-branch target.
func (sm *StateManager) DependenciesSatisfied(node *Node) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, dep := range node.DependsOn {
		state := sm.states[dep]

		switch state {
		case StateDone, StateSkipped:
			continue
		case StateFailed:
			// A failed dependency satisfies this node only if there is
			// a fail-branch edge from the dependency to this node.
			if sm.failEdges[dep] == node.ID {
				continue
			}

			return false
		case StatePending, StateQueued, StateRunning:
			return false
		}
	}

	return true
}

// edgeKey creates a map key for an edge.
func edgeKey(from, to NodeID) string {
	return string(from) + "->" + string(to)
}

// SetEdgeState updates the state of an edge.
func (sm *StateManager) SetEdgeState(from, to NodeID, state EdgeState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.edgeStates[edgeKey(from, to)] = state
}

// GetEdgeState returns the state of an edge (defaults to EdgeUnknown).
func (sm *StateManager) GetEdgeState(from, to NodeID) EdgeState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.edgeStates[edgeKey(from, to)]
}

// AllIncomingEdgesSkipped returns true if every incoming edge to a node is EdgeSkipped.
// Returns false if the node has no incoming edges.
func (sm *StateManager) AllIncomingEdgesSkipped(node *Node) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(node.DependsOn) == 0 {
		return false
	}

	for _, dep := range node.DependsOn {
		if sm.edgeStates[edgeKey(dep, node.ID)] != EdgeSkipped {
			return false
		}
	}

	return true
}

// validTransition defines allowed state transitions.
func validTransition(from, to NodeState) bool {
	switch from {
	case StatePending:
		return to == StateQueued || to == StateSkipped || to == StateDone // StateDone for frozen nodes
	case StateQueued:
		return to == StateRunning || to == StateSkipped
	case StateRunning:
		return to == StateDone || to == StateFailed
	case StateDone, StateFailed, StateSkipped:
		return false // Terminal states cannot transition
	default:
		return false
	}
}
