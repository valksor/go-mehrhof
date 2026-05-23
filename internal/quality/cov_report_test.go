package quality

import (
	"testing"

	"github.com/valksor/kvelmo/internal/findings"
)

func TestReport_HasErrorsAndErrorCount(t *testing.T) {
	tests := []struct {
		name       string
		issues     []Issue
		wantErrors bool
		wantCount  int
	}{
		{
			name:       "no issues",
			issues:     nil,
			wantErrors: false,
			wantCount:  0,
		},
		{
			name: "only warnings and info",
			issues: []Issue{
				{Severity: SeverityWarning},
				{Severity: SeverityInfo},
			},
			wantErrors: false,
			wantCount:  0,
		},
		{
			name: "mixed with errors",
			issues: []Issue{
				{Severity: SeverityError},
				{Severity: SeverityWarning},
				{Severity: SeverityError},
			},
			wantErrors: true,
			wantCount:  2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Report{Issues: tt.issues}
			if got := r.HasErrors(); got != tt.wantErrors {
				t.Errorf("HasErrors() = %v, want %v", got, tt.wantErrors)
			}
			if got := r.ErrorCount(); got != tt.wantCount {
				t.Errorf("ErrorCount() = %d, want %d", got, tt.wantCount)
			}
		})
	}
}

func TestScore(t *testing.T) {
	tests := []struct {
		name    string
		reports []*Report
		want    float64
	}{
		{
			name:    "no reports is perfect",
			reports: nil,
			want:    100,
		},
		{
			name:    "clean report is perfect",
			reports: []*Report{{Issues: nil}},
			want:    100,
		},
		{
			name: "errors deduct 10 each",
			reports: []*Report{{Issues: []Issue{
				{Severity: SeverityError},
				{Severity: SeverityError},
			}}},
			want: 80,
		},
		{
			name: "mixed severities",
			reports: []*Report{{Issues: []Issue{
				{Severity: SeverityError},   // -10
				{Severity: SeverityWarning}, // -3
				{Severity: SeverityInfo},    // -1
			}}},
			want: 86,
		},
		{
			name: "score floored at zero",
			reports: []*Report{{Issues: func() []Issue {
				is := make([]Issue, 0, 20)
				for range 20 {
					is = append(is, Issue{Severity: SeverityError})
				}

				return is
			}()}},
			want: 0,
		},
		{
			name: "averaged across linters",
			reports: []*Report{
				{Issues: []Issue{{Severity: SeverityError}}}, // 90
				{Issues: nil}, // 100
			},
			want: 95,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Score(tt.reports); got != tt.want {
				t.Errorf("Score() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasBlockers(t *testing.T) {
	if HasBlockers([]*Report{{Issues: []Issue{{Severity: SeverityWarning}}}}) {
		t.Error("HasBlockers() = true with only warnings, want false")
	}
	if !HasBlockers([]*Report{
		{Issues: []Issue{{Severity: SeverityWarning}}},
		{Issues: []Issue{{Severity: SeverityError}}},
	}) {
		t.Error("HasBlockers() = false with an error report, want true")
	}
}

func TestGateNames(t *testing.T) {
	cases := []struct {
		gate Gate
		want string
	}{
		{NoErrorsGate{}, "no-errors"},
		{NoSecurityIssuesGate{}, "no-security-issues"},
		{MaxWarningsGate{Max: 5}, "max-warnings"},
		{CompositeGate{}, "composite"},
	}
	for _, c := range cases {
		if got := c.gate.Name(); got != c.want {
			t.Errorf("%T.Name() = %q, want %q", c.gate, got, c.want)
		}
	}
}

func TestMaxWarningsGate_Evaluate(t *testing.T) {
	ff := []findings.Finding{
		{Severity: findings.SeverityMedium, ID: "1"},
		{Severity: findings.SeverityMedium, ID: "2"},
		{Severity: findings.SeverityHigh, ID: "3"},
	}

	// Under the threshold => no blockers.
	if got := (MaxWarningsGate{Max: 5}).Evaluate(ff); got != nil {
		t.Errorf("Evaluate() = %v, want nil under threshold", got)
	}

	// Over the threshold => all medium findings returned.
	got := (MaxWarningsGate{Max: 1}).Evaluate(ff)
	if len(got) != 2 {
		t.Errorf("Evaluate() returned %d, want 2 medium findings over threshold", len(got))
	}
}

func TestNewFailHistory_DefaultsWindow(t *testing.T) {
	// A non-positive window is clamped to the default of 10.
	h := NewFailHistory(0)
	for range 15 {
		h.Record("rule-a", false)
	}
	// After 15 failures with window 10, the rule should be flaky.
	if !h.IsFlaky("rule-a") {
		t.Error("IsFlaky(rule-a) = false, want true after repeated failures")
	}

	// A custom window is honored.
	h2 := NewFailHistory(4)
	h2.Record("rule-b", true)
	h2.Record("rule-b", true)
	if h2.IsFlaky("rule-b") {
		t.Error("IsFlaky(rule-b) = true with all passes, want false")
	}
	if h2.IsFlaky("unknown") {
		t.Error("IsFlaky(unknown) = true, want false for unseen rule")
	}
}
