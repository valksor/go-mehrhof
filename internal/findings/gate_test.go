package findings

import "testing"

func TestAnySeverityCriticalBlocks(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityHigh, Message: "high issue"},
		{Severity: SeverityCritical, Message: "critical issue"},
	}
	blockers := AnySeverity{SeverityCritical}.Evaluate(findings)
	if len(blockers) != 1 {
		t.Fatalf("expected 1 blocker, got %d", len(blockers))
	}
	if blockers[0].Rule != "any_critical" {
		t.Errorf("rule = %q, want %q", blockers[0].Rule, "any_critical")
	}
}

func TestAnySeverityCriticalNone(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityHigh},
		{Severity: SeverityMedium},
	}
	blockers := AnySeverity{SeverityCritical}.Evaluate(findings)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blockers, got %d", len(blockers))
	}
}

func TestAnySeverityHighBlocks(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityHigh, Message: "h1"},
		{Severity: SeverityHigh, Message: "h2"},
	}
	blockers := AnySeverity{SeverityHigh}.Evaluate(findings)
	if len(blockers) != 2 {
		t.Fatalf("expected 2 blockers, got %d", len(blockers))
	}
}

func TestMaxCountHighBelowThreshold(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityHigh},
		{Severity: SeverityHigh},
	}
	blockers := MaxCount{Level: SeverityHigh, Max: 3}.Evaluate(findings)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blockers, got %d", len(blockers))
	}
}

func TestMaxCountHighExceeded(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityHigh},
		{Severity: SeverityHigh},
		{Severity: SeverityHigh},
		{Severity: SeverityHigh},
	}
	blockers := MaxCount{Level: SeverityHigh, Max: 3}.Evaluate(findings)
	if len(blockers) != 1 {
		t.Fatalf("expected 1 blocker, got %d", len(blockers))
	}
	if blockers[0].Rule != "max_high_count" {
		t.Errorf("rule = %q, want %q", blockers[0].Rule, "max_high_count")
	}
}

func TestMaxCountMediumExceeded(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
	}
	blockers := MaxCount{Level: SeverityMedium, Max: 3}.Evaluate(findings)
	if len(blockers) != 1 {
		t.Fatalf("expected 1 blocker, got %d", len(blockers))
	}
}

func TestMinScoreAboveThreshold(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityLow},
	}
	blockers := MinScore{Threshold: 90}.Evaluate(findings)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blockers, got %d", len(blockers))
	}
}

func TestMinScoreBelowThreshold(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityCritical},
		{Severity: SeverityCritical},
		{Severity: SeverityHigh},
	}
	// 100 - 40 - 40 - 20 = 0
	blockers := MinScore{Threshold: 50}.Evaluate(findings)
	if len(blockers) != 1 {
		t.Fatalf("expected 1 blocker, got %d", len(blockers))
	}
}

func TestScore(t *testing.T) {
	tests := []struct {
		name     string
		findings []Finding
		want     float64
	}{
		{"empty", nil, 100},
		{"one critical", []Finding{{Severity: SeverityCritical}}, 60},
		{"floor at zero", []Finding{
			{Severity: SeverityCritical},
			{Severity: SeverityCritical},
			{Severity: SeverityCritical},
		}, 0},
		{"mixed", []Finding{
			{Severity: SeverityHigh},
			{Severity: SeverityMedium},
			{Severity: SeverityLow},
		}, 65}, // 100 - 20 - 10 - 5
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Score(tt.findings)
			if got != tt.want {
				t.Errorf("Score() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateMultipleGates(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityCritical, Message: "crit"},
		{Severity: SeverityHigh, Message: "high"},
	}
	gates := []GateRule{AnySeverity{SeverityCritical}, AnySeverity{SeverityHigh}}
	blockers := Evaluate(gates, findings)
	if len(blockers) != 2 {
		t.Errorf("expected 2 blockers, got %d", len(blockers))
	}
}

func TestHoldTheLineFiltersPreExisting(t *testing.T) {
	allFindings := []Finding{
		{Severity: SeverityCritical, Message: "introduced", Origin: OriginIntroduced},
		{Severity: SeverityCritical, Message: "legacy", Origin: OriginPreExisting},
		{Severity: SeverityCritical, Message: "unknown loc", Origin: OriginUnknown},
	}
	gate := HoldTheLine{Inner: AnySeverity{SeverityCritical}}
	blockers := gate.Evaluate(allFindings)
	// Should only block on introduced + unknown, not pre-existing
	if len(blockers) != 2 {
		t.Fatalf("expected 2 blockers, got %d", len(blockers))
	}
	if blockers[0].Finding.Message != "introduced" {
		t.Errorf("blockers[0] = %q, want %q", blockers[0].Finding.Message, "introduced")
	}
	if blockers[1].Finding.Message != "unknown loc" {
		t.Errorf("blockers[1] = %q, want %q", blockers[1].Finding.Message, "unknown loc")
	}
}

func TestHoldTheLineNoBlockers(t *testing.T) {
	allFindings := []Finding{
		{Severity: SeverityCritical, Message: "old", Origin: OriginPreExisting},
	}
	gate := HoldTheLine{Inner: AnySeverity{SeverityCritical}}
	blockers := gate.Evaluate(allFindings)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blockers, got %d", len(blockers))
	}
}
