package quality

import (
	"testing"

	"github.com/valksor/kvelmo/pkg/findings"
)

func TestReportToFindings(t *testing.T) {
	r := &Report{
		Linter: "golangci-lint",
		Issues: []Issue{
			{Severity: SeverityError, File: "main.go", Line: 10, Message: "err not checked", Rule: "errcheck"},
			{Severity: SeverityWarning, File: "util.go", Line: 5, Message: "shadowed var", Rule: "govet"},
			{Severity: SeverityInfo, File: "doc.go", Line: 1, Message: "missing comment", Rule: "godoc"},
		},
	}

	result := r.ToFindings()
	if len(result) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(result))
	}

	// Error maps to High.
	if result[0].Severity != findings.SeverityHigh {
		t.Errorf("error severity mapped to %v, want High", result[0].Severity)
	}
	if result[0].Category != findings.CategoryQuality {
		t.Errorf("category = %v, want quality", result[0].Category)
	}
	if result[0].Source != "golangci-lint" {
		t.Errorf("source = %q, want %q", result[0].Source, "golangci-lint")
	}

	// Warning maps to Medium.
	if result[1].Severity != findings.SeverityMedium {
		t.Errorf("warning severity mapped to %v, want Medium", result[1].Severity)
	}

	// Info maps to Info.
	if result[2].Severity != findings.SeverityInfo {
		t.Errorf("info severity mapped to %v, want Info", result[2].Severity)
	}
}

func TestReportsToFindings(t *testing.T) {
	reports := []*Report{
		{Linter: "a", Issues: []Issue{{Severity: SeverityError, Message: "a1"}}},
		{Linter: "b", Issues: []Issue{{Severity: SeverityWarning, Message: "b1"}, {Severity: SeverityInfo, Message: "b2"}}},
	}
	result := ReportsToFindings(reports)
	if len(result) != 3 {
		t.Errorf("expected 3 findings, got %d", len(result))
	}
}
