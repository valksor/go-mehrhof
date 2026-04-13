package findings

import "testing"

func TestSeverityString(t *testing.T) {
	tests := []struct {
		sev  Severity
		want string
	}{
		{SeverityCritical, "critical"},
		{SeverityHigh, "high"},
		{SeverityMedium, "medium"},
		{SeverityLow, "low"},
		{SeverityInfo, "info"},
		{Severity(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.sev.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.sev, got, tt.want)
		}
	}
}

func TestCountBySeverity(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityCritical},
		{Severity: SeverityHigh},
		{Severity: SeverityHigh},
		{Severity: SeverityMedium},
		{Severity: SeverityInfo},
	}
	counts := CountBySeverity(findings)
	if counts[SeverityCritical] != 1 {
		t.Errorf("critical count = %d, want 1", counts[SeverityCritical])
	}
	if counts[SeverityHigh] != 2 {
		t.Errorf("high count = %d, want 2", counts[SeverityHigh])
	}
	if counts[SeverityMedium] != 1 {
		t.Errorf("medium count = %d, want 1", counts[SeverityMedium])
	}
	if counts[SeverityLow] != 0 {
		t.Errorf("low count = %d, want 0", counts[SeverityLow])
	}
}

func TestCountBySeverityEmpty(t *testing.T) {
	counts := CountBySeverity(nil)
	if len(counts) != 0 {
		t.Errorf("expected empty map, got %v", counts)
	}
}
