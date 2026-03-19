package security

import (
	"testing"

	"github.com/valksor/kvelmo/pkg/findings"
)

func TestReportToFindings(t *testing.T) {
	r := &Report{
		Scanner: "secrets",
		Findings: []Finding{
			{Severity: SeverityCritical, Type: "secret", File: ".env", Line: 3, Message: "AWS key", Suggestion: "rotate it"},
			{Severity: SeverityHigh, Type: "vulnerability", File: "go.mod", Line: 10, Message: "CVE-2024-1234"},
			{Severity: SeverityMedium, Type: "secret", File: "config.yaml", Line: 1, Message: "generic API key"},
			{Severity: SeverityLow, Type: "info", Message: "best practice"},
			{Severity: SeverityInfo, Type: "tool-missing", Message: "govulncheck not installed"},
		},
	}

	result := r.ToFindings()
	if len(result) != 5 {
		t.Fatalf("expected 5 findings, got %d", len(result))
	}

	// 1:1 severity mapping.
	wantSeverities := []findings.Severity{
		findings.SeverityCritical,
		findings.SeverityHigh,
		findings.SeverityMedium,
		findings.SeverityLow,
		findings.SeverityInfo,
	}
	for i, want := range wantSeverities {
		if result[i].Severity != want {
			t.Errorf("finding[%d] severity = %v, want %v", i, result[i].Severity, want)
		}
	}

	// Check remediation mapping.
	if result[0].Remediation != "rotate it" {
		t.Errorf("remediation = %q, want %q", result[0].Remediation, "rotate it")
	}
	if result[0].Category != findings.CategorySecurity {
		t.Errorf("category = %v, want security", result[0].Category)
	}
}

func TestReportsToFindings(t *testing.T) {
	reports := []*Report{
		{Scanner: "a", Findings: []Finding{{Severity: SeverityCritical, Message: "a1"}}},
		{Scanner: "b", Findings: []Finding{{Severity: SeverityLow, Message: "b1"}, {Severity: SeverityInfo, Message: "b2"}}},
	}
	result := ReportsToFindings(reports)
	if len(result) != 3 {
		t.Errorf("expected 3 findings, got %d", len(result))
	}
}
