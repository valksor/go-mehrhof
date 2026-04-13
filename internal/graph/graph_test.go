package graph

import (
	"testing"

	"github.com/valksor/kvelmo/internal/worker"
)

func TestSingleNode(t *testing.T) {
	g := SingleNode("only", "The only node", worker.JobTypePlan, "do the thing")

	if g.NodeCount() != 1 {
		t.Fatalf("expected 1 node, got %d", g.NodeCount())
	}

	if err := g.Validate(); err != nil {
		t.Fatalf("single node graph should be valid: %v", err)
	}

	roots := g.Roots()
	if len(roots) != 1 || roots[0] != "only" {
		t.Fatalf("expected root [only], got %v", roots)
	}
}

func TestGraphRootsAndDependents(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a", JobType: worker.JobTypeImplement, Prompt: "a"})
	_ = g.AddNode(&Node{ID: "b", JobType: worker.JobTypeImplement, Prompt: "b", DependsOn: []NodeID{"a"}})
	_ = g.AddNode(&Node{ID: "c", JobType: worker.JobTypeImplement, Prompt: "c", DependsOn: []NodeID{"a"}})
	_ = g.AddNode(&Node{ID: "d", JobType: worker.JobTypeImplement, Prompt: "d", DependsOn: []NodeID{"b", "c"}})

	if err := g.Validate(); err != nil {
		t.Fatalf("valid graph rejected: %v", err)
	}

	roots := g.Roots()
	if len(roots) != 1 || roots[0] != "a" {
		t.Fatalf("expected root [a], got %v", roots)
	}

	deps := g.Dependents("a")
	if len(deps) != 2 {
		t.Fatalf("expected 2 dependents of a, got %v", deps)
	}

	deps = g.Dependents("d")
	if len(deps) != 0 {
		t.Fatalf("expected 0 dependents of d, got %v", deps)
	}
}

func TestGraphValidation(t *testing.T) {
	tests := []struct {
		name    string
		build   func() *Graph
		wantErr string
	}{
		{
			name: "empty graph",
			build: func() *Graph {
				return New()
			},
			wantErr: "empty graph",
		},
		{
			name: "missing dependency",
			build: func() *Graph {
				g := New()
				_ = g.AddNode(&Node{ID: "a", DependsOn: []NodeID{"missing"}})

				return g
			},
			wantErr: "unknown node",
		},
		{
			name: "self-cycle",
			build: func() *Graph {
				g := New()
				_ = g.AddNode(&Node{ID: "a", DependsOn: []NodeID{"a"}})

				return g
			},
			wantErr: "cycle detected",
		},
		{
			name: "two-node cycle",
			build: func() *Graph {
				g := New()
				_ = g.AddNode(&Node{ID: "a", DependsOn: []NodeID{"b"}})
				_ = g.AddNode(&Node{ID: "b", DependsOn: []NodeID{"a"}})

				return g
			},
			wantErr: "cycle detected",
		},
		{
			name: "three-node cycle",
			build: func() *Graph {
				g := New()
				_ = g.AddNode(&Node{ID: "a", DependsOn: []NodeID{"c"}})
				_ = g.AddNode(&Node{ID: "b", DependsOn: []NodeID{"a"}})
				_ = g.AddNode(&Node{ID: "c", DependsOn: []NodeID{"b"}})

				return g
			},
			wantErr: "cycle detected",
		},
		{
			name: "valid diamond",
			build: func() *Graph {
				g := New()
				_ = g.AddNode(&Node{ID: "a"})
				_ = g.AddNode(&Node{ID: "b", DependsOn: []NodeID{"a"}})
				_ = g.AddNode(&Node{ID: "c", DependsOn: []NodeID{"a"}})
				_ = g.AddNode(&Node{ID: "d", DependsOn: []NodeID{"b", "c"}})

				return g
			},
			wantErr: "",
		},
		{
			name: "multiple roots",
			build: func() *Graph {
				g := New()
				_ = g.AddNode(&Node{ID: "a"})
				_ = g.AddNode(&Node{ID: "b"})
				_ = g.AddNode(&Node{ID: "c", DependsOn: []NodeID{"a", "b"}})

				return g
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := tt.build()
			err := g.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if !contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q should contain %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestAddNodeDuplicate(t *testing.T) {
	g := New()

	if err := g.AddNode(&Node{ID: "a"}); err != nil {
		t.Fatalf("first add should succeed: %v", err)
	}

	if err := g.AddNode(&Node{ID: "a"}); err == nil {
		t.Fatal("duplicate add should fail")
	}
}

func TestNodeIDs(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "c"})
	_ = g.AddNode(&Node{ID: "a"})
	_ = g.AddNode(&Node{ID: "b"})

	ids := g.NodeIDs()
	if len(ids) != 3 || ids[0] != "a" || ids[1] != "b" || ids[2] != "c" {
		t.Fatalf("expected sorted [a b c], got %v", ids)
	}
}

func TestTransitiveDepCount(t *testing.T) {
	// Diamond: A→B, A→C, B→D, C→D
	g := New()
	_ = g.AddNode(&Node{ID: "A", JobType: worker.JobTypeImplement, Prompt: "a"})
	_ = g.AddNode(&Node{ID: "B", JobType: worker.JobTypeImplement, Prompt: "b", DependsOn: []NodeID{"A"}})
	_ = g.AddNode(&Node{ID: "C", JobType: worker.JobTypeImplement, Prompt: "c", DependsOn: []NodeID{"A"}})
	_ = g.AddNode(&Node{ID: "D", JobType: worker.JobTypeImplement, Prompt: "d", DependsOn: []NodeID{"B", "C"}})

	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		id   NodeID
		want int
	}{
		{"A", 3}, // A → B, C, D
		{"B", 1}, // B → D
		{"C", 1}, // C → D
		{"D", 0}, // leaf
	}

	for _, tt := range tests {
		if got := g.TransitiveDepCount(tt.id); got != tt.want {
			t.Errorf("TransitiveDepCount(%q) = %d, want %d", tt.id, got, tt.want)
		}
	}
}

func TestTransitiveDepCountLinear(t *testing.T) {
	// Chain: A→B→C→D
	g := New()
	_ = g.AddNode(&Node{ID: "A", JobType: worker.JobTypeImplement, Prompt: "a"})
	_ = g.AddNode(&Node{ID: "B", JobType: worker.JobTypeImplement, Prompt: "b", DependsOn: []NodeID{"A"}})
	_ = g.AddNode(&Node{ID: "C", JobType: worker.JobTypeImplement, Prompt: "c", DependsOn: []NodeID{"B"}})
	_ = g.AddNode(&Node{ID: "D", JobType: worker.JobTypeImplement, Prompt: "d", DependsOn: []NodeID{"C"}})

	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		id   NodeID
		want int
	}{
		{"A", 3},
		{"B", 2},
		{"C", 1},
		{"D", 0},
	}

	for _, tt := range tests {
		if got := g.TransitiveDepCount(tt.id); got != tt.want {
			t.Errorf("TransitiveDepCount(%q) = %d, want %d", tt.id, got, tt.want)
		}
	}
}

func TestTransitiveDepCountSingleNode(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "solo", JobType: worker.JobTypeImplement, Prompt: "s"})

	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}

	if got := g.TransitiveDepCount("solo"); got != 0 {
		t.Errorf("TransitiveDepCount(solo) = %d, want 0", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}
