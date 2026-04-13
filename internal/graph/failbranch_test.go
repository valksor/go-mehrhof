package graph

import (
	"testing"

	"github.com/valksor/kvelmo/internal/worker"
)

func ptrNodeID(s string) *NodeID {
	id := NodeID(s)

	return &id
}

func TestFailBranchValidation(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Graph)
		wantErr string
	}{
		{
			name: "valid fail-branch",
			setup: func(g *Graph) {
				_ = g.AddNode(&Node{ID: "a", JobType: worker.JobTypePlan, Prompt: "do a", FailBranch: ptrNodeID("handler")})
				_ = g.AddNode(&Node{ID: "handler", JobType: worker.JobTypePlan, Prompt: "handle", DependsOn: []NodeID{"a"}})
			},
		},
		{
			name: "unknown target",
			setup: func(g *Graph) {
				_ = g.AddNode(&Node{ID: "a", JobType: worker.JobTypePlan, Prompt: "do a", FailBranch: ptrNodeID("missing")})
			},
			wantErr: `fail-branch references unknown node "missing"`,
		},
		{
			name: "self-referencing",
			setup: func(g *Graph) {
				_ = g.AddNode(&Node{ID: "a", JobType: worker.JobTypePlan, Prompt: "do a", FailBranch: ptrNodeID("a")})
			},
			wantErr: `cannot be its own fail-branch target`,
		},
		{
			name: "target missing DependsOn",
			setup: func(g *Graph) {
				_ = g.AddNode(&Node{ID: "a", JobType: worker.JobTypePlan, Prompt: "do a", FailBranch: ptrNodeID("handler")})
				_ = g.AddNode(&Node{ID: "handler", JobType: worker.JobTypePlan, Prompt: "handle"})
			},
			wantErr: `must list "a" in DependsOn`,
		},
		{
			name: "nil fail-branch is fine",
			setup: func(g *Graph) {
				_ = g.AddNode(&Node{ID: "a", JobType: worker.JobTypePlan, Prompt: "do a"})
				_ = g.AddNode(&Node{ID: "b", JobType: worker.JobTypePlan, Prompt: "do b", DependsOn: []NodeID{"a"}})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New()
			tt.setup(g)
			err := g.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				return
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want substring %q", got, tt.wantErr)
			}
		})
	}
}

func TestFailBranchTarget(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a", JobType: worker.JobTypePlan, Prompt: "do a", FailBranch: ptrNodeID("handler")})
	_ = g.AddNode(&Node{ID: "handler", JobType: worker.JobTypePlan, Prompt: "handle", DependsOn: []NodeID{"a"}})
	_ = g.AddNode(&Node{ID: "b", JobType: worker.JobTypePlan, Prompt: "do b", DependsOn: []NodeID{"a"}})

	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	target, ok := g.FailBranchTarget("a")
	if !ok || target != "handler" {
		t.Errorf("FailBranchTarget(a) = %v, %v; want handler, true", target, ok)
	}

	_, ok = g.FailBranchTarget("b")
	if ok {
		t.Error("FailBranchTarget(b) should return false")
	}

	if !g.IsFailBranchEdge("a", "handler") {
		t.Error("IsFailBranchEdge(a, handler) should be true")
	}

	if g.IsFailBranchEdge("a", "b") {
		t.Error("IsFailBranchEdge(a, b) should be false")
	}
}

func TestFailErrorKey(t *testing.T) {
	key := FailErrorKey("my_node")
	if key != "__fail_error:my_node" {
		t.Errorf("FailErrorKey = %q, want %q", key, "__fail_error:my_node")
	}
}

func TestDependenciesSatisfiedWithFailBranch(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a", JobType: worker.JobTypePlan, Prompt: "do a", FailBranch: ptrNodeID("handler")})
	_ = g.AddNode(&Node{ID: "handler", JobType: worker.JobTypePlan, Prompt: "handle", DependsOn: []NodeID{"a"}})
	_ = g.AddNode(&Node{ID: "b", JobType: worker.JobTypePlan, Prompt: "do b", DependsOn: []NodeID{"a"}})

	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	sm := NewStateManager(g)

	// Mark "a" as failed.
	_ = sm.Transition("a", StateQueued)
	_ = sm.Transition("a", StateRunning)
	_ = sm.Transition("a", StateFailed)

	// Handler should be satisfied (it's the fail-branch target of "a").
	if !sm.DependenciesSatisfied(g.Nodes["handler"]) {
		t.Error("handler's dependencies should be satisfied (fail-branch)")
	}

	// Normal dependent "b" should NOT be satisfied.
	if sm.DependenciesSatisfied(g.Nodes["b"]) {
		t.Error("b's dependencies should NOT be satisfied (not a fail-branch)")
	}
}

func TestAllIncomingEdgesSkippedWithFailBranch(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a", JobType: worker.JobTypePlan, Prompt: "do a", FailBranch: ptrNodeID("handler")})
	_ = g.AddNode(&Node{ID: "handler", JobType: worker.JobTypePlan, Prompt: "handle", DependsOn: []NodeID{"a"}})

	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	sm := NewStateManager(g)

	// Mark the fail-branch edge as EdgeTaken (as the scheduler would).
	sm.SetEdgeState("a", "handler", EdgeTaken)

	// Handler should NOT be auto-skipped because it has a taken edge.
	if sm.AllIncomingEdgesSkipped(g.Nodes["handler"]) {
		t.Error("handler should not be auto-skipped when fail-branch edge is taken")
	}
}

// contains and searchSubstring are defined in graph_test.go
