package strategy

import (
	"strings"
	"testing"
)

func TestDirectBuildPrompt(t *testing.T) {
	d := &Direct{}

	t.Run("task only", func(t *testing.T) {
		result := d.BuildPrompt(Input{Task: "do something"})
		if result != "do something" {
			t.Fatalf("expected 'do something', got %q", result)
		}
	})

	t.Run("with context", func(t *testing.T) {
		result := d.BuildPrompt(Input{Task: "implement", Context: "existing code"})
		if !strings.Contains(result, "existing code") || !strings.Contains(result, "implement") {
			t.Fatalf("expected context and task, got %q", result)
		}
	})

	t.Run("with constraints", func(t *testing.T) {
		result := d.BuildPrompt(Input{
			Task:        "build it",
			Constraints: []string{"keep it simple", "no deps"},
		})
		if !strings.Contains(result, "keep it simple") || !strings.Contains(result, "no deps") {
			t.Fatalf("expected constraints, got %q", result)
		}
	})
}

func TestDirectEvaluateOutput(t *testing.T) {
	d := &Direct{}
	out := d.EvaluateOutput("done")

	if out.Status != "complete" {
		t.Fatalf("expected 'complete', got %q", out.Status)
	}

	if out.Content != "done" {
		t.Fatalf("expected 'done', got %q", out.Content)
	}
}

func TestIterativeBuildPrompt(t *testing.T) {
	it := &Iterative{}

	t.Run("first iteration", func(t *testing.T) {
		result := it.BuildPrompt(Input{Task: "implement feature"})
		if !strings.Contains(result, "implement feature") {
			t.Fatal("expected task in prompt")
		}
		if !strings.Contains(result, "Self-Review") {
			t.Fatal("expected self-review instructions")
		}
		if strings.Contains(result, "Previous Attempt") {
			t.Fatal("first iteration should not mention previous attempt")
		}
	})

	t.Run("subsequent iteration", func(t *testing.T) {
		result := it.BuildPrompt(Input{
			Task:    "implement feature",
			Context: "prior output with issues",
		})
		if !strings.Contains(result, "Previous Attempt") {
			t.Fatal("subsequent iteration should reference previous attempt")
		}
		if !strings.Contains(result, "prior output with issues") {
			t.Fatal("should include prior context")
		}
	})
}

func TestIterativeEvaluateOutput(t *testing.T) {
	it := &Iterative{}

	tests := []struct {
		name       string
		output     string
		wantStatus string
	}{
		{"clean output", "all done, everything works", "complete"},
		{"has TODO", "implemented but TODO: handle edge case", "needs_iteration"},
		{"has FIXME", "code works FIXME: cleanup later", "needs_iteration"},
		{"has HACK", "HACK: temporary workaround", "needs_iteration"},
		{"case insensitive", "todo: something", "needs_iteration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := it.EvaluateOutput(tt.output)
			if out.Status != tt.wantStatus {
				t.Fatalf("expected status %q, got %q", tt.wantStatus, out.Status)
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	// Built-in strategies registered in init().
	names := List()
	if len(names) < 2 {
		t.Fatalf("expected at least 2 strategies, got %d", len(names))
	}

	// Direct should be available.
	d, ok := Get("direct")
	if !ok || d.Name() != "direct" {
		t.Fatal("direct strategy not found")
	}

	// Iterative should be available.
	it, ok := Get("iterative")
	if !ok || it.Name() != "iterative" {
		t.Fatal("iterative strategy not found")
	}

	// Default returns direct.
	def := Default()
	if def.Name() != "direct" {
		t.Fatalf("expected default 'direct', got %q", def.Name())
	}

	// Unknown returns false.
	_, ok = Get("nonexistent")
	if ok {
		t.Fatal("expected false for unknown strategy")
	}
}
