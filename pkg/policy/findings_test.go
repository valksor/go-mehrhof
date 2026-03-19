package policy

import (
	"testing"

	"github.com/valksor/kvelmo/pkg/findings"
)

func TestViolationsToFindings(t *testing.T) {
	violations := []Violation{
		{Severity: SeverityError, Rule: "min_spec_sections", Message: "need at least 2 specs"},
		{Severity: SeverityWarning, Rule: "sensitive_path", Message: "touching config.yaml"},
	}

	result := ViolationsToFindings(violations)
	if len(result) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(result))
	}

	// Error maps to High.
	if result[0].Severity != findings.SeverityHigh {
		t.Errorf("error severity mapped to %v, want High", result[0].Severity)
	}
	if result[0].Category != findings.CategoryPolicy {
		t.Errorf("category = %v, want policy", result[0].Category)
	}
	if result[0].Rule != "min_spec_sections" {
		t.Errorf("rule = %q, want %q", result[0].Rule, "min_spec_sections")
	}

	// Warning maps to Medium.
	if result[1].Severity != findings.SeverityMedium {
		t.Errorf("warning severity mapped to %v, want Medium", result[1].Severity)
	}
}

func TestViolationsToFindingsEmpty(t *testing.T) {
	result := ViolationsToFindings(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result))
	}
}
