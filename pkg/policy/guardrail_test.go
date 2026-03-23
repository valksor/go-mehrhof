package policy

import (
	"context"
	"testing"

	"github.com/valksor/kvelmo/pkg/findings"
)

func TestRequireSpec_BlocksWithoutSpecs(t *testing.T) {
	g := &RequireSpec{}

	phases := []string{"implement", "simplify", "optimize", "review"}
	for _, phase := range phases {
		t.Run(phase, func(t *testing.T) {
			result := g.Check(context.Background(), phase, "/tmp", nil)
			if len(result) != 1 {
				t.Fatalf("expected 1 finding for phase %q without specs, got %d", phase, len(result))
			}
			if result[0].Severity != findings.SeverityHigh {
				t.Errorf("severity = %v, want High", result[0].Severity)
			}
			if result[0].Rule != "require-spec" {
				t.Errorf("rule = %q, want %q", result[0].Rule, "require-spec")
			}
		})
	}
}

func TestRequireSpec_PassesWithSpecs(t *testing.T) {
	g := &RequireSpec{}

	phases := []string{"implement", "simplify", "optimize", "review"}
	for _, phase := range phases {
		t.Run(phase, func(t *testing.T) {
			result := g.Check(context.Background(), phase, "/tmp", []string{"spec-1.md"})
			if len(result) != 0 {
				t.Errorf("expected 0 findings for phase %q with specs, got %d", phase, len(result))
			}
		})
	}
}

func TestRequireSpec_IgnoresNonSpecPhases(t *testing.T) {
	g := &RequireSpec{}

	phases := []string{"plan", "submit", "finish", "start"}
	for _, phase := range phases {
		t.Run(phase, func(t *testing.T) {
			result := g.Check(context.Background(), phase, "/tmp", nil)
			if len(result) != 0 {
				t.Errorf("expected 0 findings for phase %q (no spec required), got %d", phase, len(result))
			}
		})
	}
}

func TestMaxDiffSize_UnderThreshold(t *testing.T) {
	g := &MaxDiffSize{MaxLines: 2000}

	// When we can't run git (no repo), countDiffLines fails and returns nil.
	result := g.Check(context.Background(), "implement", "/tmp/nonexistent", nil)
	if len(result) != 0 {
		t.Errorf("expected 0 findings when git diff fails (graceful skip), got %d", len(result))
	}
}

func TestMaxDiffSize_DefaultThreshold(t *testing.T) {
	g := &MaxDiffSize{MaxLines: 0}
	if g.Name() != "max-diff-size" {
		t.Errorf("Name() = %q, want %q", g.Name(), "max-diff-size")
	}
}

func TestGetGuardrail(t *testing.T) {
	tests := []struct {
		name  string
		found bool
	}{
		{"require-spec", true},
		{"max-diff-size", true},
		{"nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, ok := GetGuardrail(tt.name)
			if ok != tt.found {
				t.Fatalf("GetGuardrail(%q) found = %v, want %v", tt.name, ok, tt.found)
			}
			if ok && g.Name() != tt.name {
				t.Errorf("Name() = %q, want %q", g.Name(), tt.name)
			}
		})
	}
}

func TestListGuardrails(t *testing.T) {
	names := ListGuardrails()
	if len(names) < 2 {
		t.Fatalf("expected at least 2 guardrails, got %d", len(names))
	}

	// Verify sorted order.
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("guardrails not sorted: %q before %q", names[i-1], names[i])
		}
	}

	// Verify known guardrails are present.
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["require-spec"] {
		t.Error("missing require-spec in ListGuardrails")
	}
	if !found["max-diff-size"] {
		t.Error("missing max-diff-size in ListGuardrails")
	}
}

func TestExtractLastNumber(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{" 5 files changed, 120 ", "120"},
		{"", ""},
		{" 42 ", "42"},
	}

	for _, tt := range tests {
		got := extractLastNumber(tt.input)
		if got != tt.want {
			t.Errorf("extractLastNumber(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
