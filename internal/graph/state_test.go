package graph

import (
	"testing"
)

func newTestGraph() *Graph {
	g := New()
	_ = g.AddNode(&Node{ID: "a"})
	_ = g.AddNode(&Node{ID: "b", DependsOn: []NodeID{"a"}})
	_ = g.AddNode(&Node{ID: "c", DependsOn: []NodeID{"a"}})

	return g
}

func TestStateManagerInitial(t *testing.T) {
	g := newTestGraph()
	sm := NewStateManager(g)

	for _, id := range g.NodeIDs() {
		if sm.Get(id) != StatePending {
			t.Fatalf("expected pending for %s, got %s", id, sm.Get(id))
		}
	}

	if sm.AllDone() {
		t.Fatal("should not be all done initially")
	}
}

func TestStateManagerTransitions(t *testing.T) {
	g := newTestGraph()
	sm := NewStateManager(g)

	// Valid: pending → queued → running → done
	if err := sm.Transition("a", StateQueued); err != nil {
		t.Fatalf("pending → queued: %v", err)
	}
	if err := sm.Transition("a", StateRunning); err != nil {
		t.Fatalf("queued → running: %v", err)
	}
	if err := sm.Transition("a", StateDone); err != nil {
		t.Fatalf("running → done: %v", err)
	}

	// Invalid: done → running
	if err := sm.Transition("a", StateRunning); err == nil {
		t.Fatal("done → running should fail")
	}
}

func TestStateManagerInvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from NodeState
		to   NodeState
	}{
		{"pending → running", StatePending, StateRunning},
		// pending → done is now valid (frozen nodes).
		{"pending → failed", StatePending, StateFailed},
		{"queued → done", StateQueued, StateDone},
		{"queued → failed", StateQueued, StateFailed},
		{"queued → pending", StateQueued, StatePending},
		{"running → pending", StateRunning, StatePending},
		{"running → queued", StateRunning, StateQueued},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New()
			_ = g.AddNode(&Node{ID: "x"})
			sm := NewStateManager(g)

			// Get to the "from" state.
			switch tt.from {
			case StatePending:
				// Already pending.
			case StateQueued:
				_ = sm.Transition("x", StateQueued)
			case StateRunning:
				_ = sm.Transition("x", StateQueued)
				_ = sm.Transition("x", StateRunning)
			case StateDone, StateFailed, StateSkipped:
				// Not used in these tests.
			}

			if err := sm.Transition("x", tt.to); err == nil {
				t.Fatalf("expected error for %s → %s", tt.from, tt.to)
			}
		})
	}
}

func TestStateManagerResults(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a"})
	sm := NewStateManager(g)

	sm.SetResult("a", "output text")

	if r := sm.Result("a"); r != "output text" {
		t.Fatalf("expected 'output text', got %q", r)
	}

	results := sm.Results()
	if results["a"] != "output text" {
		t.Fatal("Results() should return copy with result")
	}

	// Mutating the copy shouldn't affect internal state.
	results["a"] = "modified"

	if sm.Result("a") != "output text" {
		t.Fatal("Results() should return a copy, not a reference")
	}
}

func TestStateManagerErrors(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a"})
	sm := NewStateManager(g)

	sm.SetError("a", "something broke")

	if e := sm.Error("a"); e != "something broke" {
		t.Fatalf("expected error message, got %q", e)
	}
}

func TestStateManagerAllDone(t *testing.T) {
	g := newTestGraph()
	sm := NewStateManager(g)

	// Move all to done/skipped/failed.
	_ = sm.Transition("a", StateQueued)
	_ = sm.Transition("a", StateRunning)
	_ = sm.Transition("a", StateDone)

	_ = sm.Transition("b", StateSkipped)

	_ = sm.Transition("c", StateQueued)
	_ = sm.Transition("c", StateRunning)
	_ = sm.Transition("c", StateFailed)

	if !sm.AllDone() {
		t.Fatal("all nodes are terminal, should be done")
	}

	if !sm.HasFailures() {
		t.Fatal("node c failed, should report failures")
	}
}

func TestStateManagerDependenciesSatisfied(t *testing.T) {
	g := newTestGraph()
	sm := NewStateManager(g)

	nodeB := g.Nodes["b"]

	// b depends on a, which is still pending.
	if sm.DependenciesSatisfied(nodeB) {
		t.Fatal("b's deps should not be satisfied while a is pending")
	}

	// Complete a.
	_ = sm.Transition("a", StateQueued)
	_ = sm.Transition("a", StateRunning)
	_ = sm.Transition("a", StateDone)

	if !sm.DependenciesSatisfied(nodeB) {
		t.Fatal("b's deps should be satisfied after a is done")
	}
}

func TestStateManagerCountByState(t *testing.T) {
	g := newTestGraph()
	sm := NewStateManager(g)

	counts := sm.CountByState()
	if counts[StatePending] != 3 {
		t.Fatalf("expected 3 pending, got %d", counts[StatePending])
	}

	_ = sm.Transition("a", StateQueued)
	counts = sm.CountByState()

	if counts[StatePending] != 2 || counts[StateQueued] != 1 {
		t.Fatalf("unexpected counts: %v", counts)
	}
}

func TestStateManagerUnknownNode(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a"})
	sm := NewStateManager(g)

	if err := sm.Transition("unknown", StateQueued); err == nil {
		t.Fatal("should error on unknown node")
	}
}
