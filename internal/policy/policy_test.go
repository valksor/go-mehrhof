package policy

import (
	"testing"
)

func TestEvaluate_RequiredPhases(t *testing.T) {
	cfg := Settings{
		RequiredPhases: []string{"reviewing"},
	}

	violations := Evaluate(cfg, "submit", "implemented", nil, nil)

	if len(violations) == 0 {
		t.Fatal("expected violations for skipping required phase, got none")
	}

	found := false
	for _, v := range violations {
		if v.Rule == "required_phase" && v.Severity == SeverityError {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected error violation for required_phase, got: %+v", violations)
	}
}

func TestEvaluate_RequiredPhasesNotSkipped(t *testing.T) {
	cfg := Settings{
		RequiredPhases: []string{"reviewing"},
	}

	violations := Evaluate(cfg, "review", "optimizing", nil, nil)

	for _, v := range violations {
		if v.Rule == "required_phase" {
			t.Errorf("expected no required_phase violation, got: %+v", v)
		}
	}
}

func TestEvaluate_MinSpecSections(t *testing.T) {
	cfg := Settings{
		MinSpecSections: 2,
	}

	violations := Evaluate(cfg, "implement", "planned", []string{"spec1.md"}, nil)

	if len(violations) == 0 {
		t.Fatal("expected violations for insufficient specs, got none")
	}

	found := false
	for _, v := range violations {
		if v.Rule == "min_spec_sections" && v.Severity == SeverityError {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected error violation for min_spec_sections, got: %+v", violations)
	}
}

func TestEvaluate_SensitivePaths(t *testing.T) {
	cfg := Settings{
		SensitivePaths: []string{"pkg/auth/*"},
	}

	violations := Evaluate(cfg, "submit", "reviewing", nil, []string{"pkg/auth/login.go"})

	if len(violations) == 0 {
		t.Fatal("expected violations for sensitive path match, got none")
	}

	found := false
	for _, v := range violations {
		if v.Rule == "sensitive_paths" && v.Severity == SeverityWarning {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected warning violation for sensitive_paths, got: %+v", violations)
	}
}

func TestEvaluate_NoViolations(t *testing.T) {
	cfg := Settings{}

	violations := Evaluate(cfg, "plan", "loaded", nil, nil)

	if len(violations) != 0 {
		t.Errorf("expected no violations with empty settings, got: %+v", violations)
	}
}

func TestEvaluate_DocRequirements_Violated(t *testing.T) {
	cfg := Settings{
		DocRequirements: []DocRequirement{
			{Trigger: "pkg/api/*", Requires: "docs/*"},
		},
	}

	violations := Evaluate(cfg, "submit", "reviewing", nil, []string{"pkg/api/handler.go"})

	if len(violations) == 0 {
		t.Fatal("expected violations for missing documentation, got none")
	}

	found := false
	for _, v := range violations {
		if v.Rule == "doc_requirement" && v.Severity == SeverityError {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected error violation for doc_requirement, got: %+v", violations)
	}
}

func TestEvaluate_DocRequirements_Satisfied(t *testing.T) {
	cfg := Settings{
		DocRequirements: []DocRequirement{
			{Trigger: "pkg/api/*", Requires: "docs/*"},
		},
	}

	violations := Evaluate(cfg, "submit", "reviewing", nil, []string{"pkg/api/handler.go", "docs/api.md"})

	for _, v := range violations {
		if v.Rule == "doc_requirement" {
			t.Errorf("expected no doc_requirement violation, got: %+v", v)
		}
	}
}

func TestEvaluate_DocRequirements_NoTrigger(t *testing.T) {
	cfg := Settings{
		DocRequirements: []DocRequirement{
			{Trigger: "pkg/api/*", Requires: "docs/*"},
		},
	}

	// Changed files don't match the trigger pattern, so no violation
	violations := Evaluate(cfg, "submit", "reviewing", nil, []string{"pkg/web/server.go"})

	for _, v := range violations {
		if v.Rule == "doc_requirement" {
			t.Errorf("expected no doc_requirement violation when trigger not matched, got: %+v", v)
		}
	}
}

func TestHasBlockingViolation(t *testing.T) {
	tests := []struct {
		name       string
		violations []Violation
		want       bool
	}{
		{
			name:       "no violations",
			violations: nil,
			want:       false,
		},
		{
			name: "warning only",
			violations: []Violation{
				{Severity: SeverityWarning, Rule: "test", Message: "just a warning"},
			},
			want: false,
		},
		{
			name: "error present",
			violations: []Violation{
				{Severity: SeverityWarning, Rule: "test", Message: "warning"},
				{Severity: SeverityError, Rule: "test", Message: "error"},
			},
			want: true,
		},
		{
			name: "error only",
			violations: []Violation{
				{Severity: SeverityError, Rule: "test", Message: "blocking"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasBlockingViolation(tt.violations)
			if got != tt.want {
				t.Errorf("HasBlockingViolation() = %v, want %v", got, tt.want)
			}
		})
	}
}
