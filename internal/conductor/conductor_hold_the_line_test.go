package conductor

import (
	"testing"

	"github.com/valksor/kvelmo/internal/findings"
)

func TestConvertHunks(t *testing.T) {
	raw := map[string][][2]int{
		"main.go": {{10, 20}, {50, 55}},
		"util.go": {{1, 5}},
	}

	result := convertHunks(raw)

	if len(result) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result))
	}

	mainRanges := result["main.go"]
	if len(mainRanges) != 2 {
		t.Fatalf("main.go: expected 2 ranges, got %d", len(mainRanges))
	}
	if mainRanges[0].Start != 10 || mainRanges[0].End != 20 {
		t.Errorf("main.go range[0] = %v, want {10, 20}", mainRanges[0])
	}
	if mainRanges[1].Start != 50 || mainRanges[1].End != 55 {
		t.Errorf("main.go range[1] = %v, want {50, 55}", mainRanges[1])
	}

	utilRanges := result["util.go"]
	if len(utilRanges) != 1 {
		t.Fatalf("util.go: expected 1 range, got %d", len(utilRanges))
	}
	if utilRanges[0].Start != 1 || utilRanges[0].End != 5 {
		t.Errorf("util.go range[0] = %v, want {1, 5}", utilRanges[0])
	}
}

func TestHoldTheLineEnabled_Default(t *testing.T) {
	c, err := New(WithWorkDir(t.TempDir()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !c.holdTheLineEnabled() {
		t.Error("holdTheLineEnabled() should default to true")
	}
}

func TestHoldTheLineGateIntegration(t *testing.T) {
	// Verify the HoldTheLine gate wrapper filters pre-existing findings
	allFindings := []findings.Finding{
		{Severity: findings.SeverityCritical, Message: "new bug", Origin: findings.OriginIntroduced},
		{Severity: findings.SeverityCritical, Message: "old bug", Origin: findings.OriginPreExisting},
		{Severity: findings.SeverityHigh, Message: "new warning", Origin: findings.OriginIntroduced},
	}

	// Without hold-the-line: 2 critical blockers
	gate := findings.AnySeverity{Level: findings.SeverityCritical}
	blockers := gate.Evaluate(allFindings)
	if len(blockers) != 2 {
		t.Errorf("without hold-the-line: expected 2 blockers, got %d", len(blockers))
	}

	// With hold-the-line: only 1 critical blocker (pre-existing filtered out)
	htlGate := findings.HoldTheLine{Inner: gate}
	blockers = htlGate.Evaluate(allFindings)
	if len(blockers) != 1 {
		t.Errorf("with hold-the-line: expected 1 blocker, got %d", len(blockers))
	}
	if len(blockers) > 0 && blockers[0].Finding.Message != "new bug" {
		t.Errorf("expected blocker for 'new bug', got %q", blockers[0].Finding.Message)
	}
}
