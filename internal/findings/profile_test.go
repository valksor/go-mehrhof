package findings

import "testing"

func TestQuickProfileOnlyCriticalBlocks(t *testing.T) {
	p := Quick()

	// High should not block in quick profile.
	highOnly := []Finding{{Severity: SeverityHigh}}
	if p.HasBlockers(highOnly) {
		t.Error("quick profile should not block on high findings")
	}

	// Critical should block.
	withCritical := []Finding{{Severity: SeverityCritical}}
	if !p.HasBlockers(withCritical) {
		t.Error("quick profile should block on critical findings")
	}
}

func TestStandardProfileHighBlocks(t *testing.T) {
	p := Standard()

	highOnly := []Finding{{Severity: SeverityHigh}}
	if !p.HasBlockers(highOnly) {
		t.Error("standard profile should block on high findings")
	}

	mediumOnly := []Finding{{Severity: SeverityMedium}}
	if p.HasBlockers(mediumOnly) {
		t.Error("standard profile should not block on medium findings")
	}
}

func TestStrictProfileMediumCap(t *testing.T) {
	p := Strict()

	// 3 medium findings: within cap.
	threeMedium := []Finding{
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
	}
	if p.HasBlockers(threeMedium) {
		t.Error("strict profile should allow up to 3 medium findings")
	}

	// 4 medium findings: exceeds cap.
	fourMedium := []Finding{
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
	}
	if !p.HasBlockers(fourMedium) {
		t.Error("strict profile should block on >3 medium findings")
	}
}

func TestProfileCheck(t *testing.T) {
	p := Standard()
	findings := []Finding{
		{Severity: SeverityCritical, Message: "c1"},
		{Severity: SeverityHigh, Message: "h1"},
		{Severity: SeverityMedium, Message: "m1"},
	}
	blockers := p.Check(findings)
	if len(blockers) != 2 { // 1 critical + 1 high
		t.Errorf("expected 2 blockers, got %d", len(blockers))
	}
}

func TestForPhase(t *testing.T) {
	tests := []struct {
		phase string
		name  string
	}{
		{"implement", "quick"},
		{"simplify", "quick"},
		{"optimize", "quick"},
		{"review", "standard"},
		{"submit", "strict"},
		{"unknown", "quick"},
	}
	for _, tt := range tests {
		p := ForPhase(tt.phase)
		if p.Name != tt.name {
			t.Errorf("ForPhase(%q).Name = %q, want %q", tt.phase, p.Name, tt.name)
		}
	}
}
