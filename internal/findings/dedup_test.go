package findings

import (
	"slices"
	"testing"
)

func TestDeduplicateFindings_MergesOverlapping(t *testing.T) {
	groups := map[string][]Finding{
		"claude": {
			{File: "main.go", Line: 10, Rule: "error-check", Message: "unchecked error", Severity: SeverityHigh},
			{File: "main.go", Line: 50, Rule: "unused-var", Message: "unused variable x", Severity: SeverityLow},
		},
		"codex": {
			{File: "main.go", Line: 11, Rule: "error-check", Message: "missing error check", Severity: SeverityHigh},
			{File: "util.go", Line: 5, Rule: "nil-deref", Message: "possible nil dereference", Severity: SeverityCritical},
		},
	}

	result := DeduplicateFindings(groups)

	// main.go:10 error-check and main.go:11 error-check should merge (within tolerance 3)
	// main.go:50 unused-var should stand alone
	// util.go:5 nil-deref should stand alone
	if len(result) != 3 {
		t.Fatalf("expected 3 deduplicated findings, got %d", len(result))
	}

	// Find the merged finding
	var merged *Finding

	for i := range result {
		if result[i].File == "main.go" && result[i].Rule == "error-check" {
			merged = &result[i]

			break
		}
	}

	if merged == nil {
		t.Fatal("expected merged error-check finding")
	}

	if len(merged.DetectedBy) != 2 {
		t.Errorf("expected 2 agents in DetectedBy, got %d: %v", len(merged.DetectedBy), merged.DetectedBy)
	}

	if !slices.Contains(merged.DetectedBy, "claude") || !slices.Contains(merged.DetectedBy, "codex") {
		t.Errorf("expected DetectedBy to contain claude and codex, got %v", merged.DetectedBy)
	}
}

func TestDeduplicateFindings_NoOverlap(t *testing.T) {
	groups := map[string][]Finding{
		"agent-a": {
			{File: "a.go", Line: 1, Rule: "rule-a"},
		},
		"agent-b": {
			{File: "b.go", Line: 100, Rule: "rule-b"},
		},
	}

	result := DeduplicateFindings(groups)

	if len(result) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(result))
	}

	for _, f := range result {
		if len(f.DetectedBy) != 1 {
			t.Errorf("expected 1 agent in DetectedBy for %s, got %d", f.File, len(f.DetectedBy))
		}
	}
}

func TestDeduplicateFindings_EmptyGroups(t *testing.T) {
	result := DeduplicateFindings(map[string][]Finding{})

	if len(result) != 0 {
		t.Fatalf("expected 0 findings for empty groups, got %d", len(result))
	}
}

func TestDeduplicateFindings_SingleAgent(t *testing.T) {
	groups := map[string][]Finding{
		"claude": {
			{File: "x.go", Line: 10, Rule: "lint"},
			{File: "x.go", Line: 20, Rule: "lint"},
		},
	}

	result := DeduplicateFindings(groups)

	if len(result) != 2 {
		t.Fatalf("expected 2 findings from single agent, got %d", len(result))
	}

	for _, f := range result {
		if len(f.DetectedBy) != 1 || f.DetectedBy[0] != "claude" {
			t.Errorf("expected DetectedBy=[claude], got %v", f.DetectedBy)
		}
	}
}

func TestDeduplicateFindings_ThreeAgentsAgree(t *testing.T) {
	groups := map[string][]Finding{
		"agent-1": {{File: "f.go", Line: 10, Rule: "bug"}},
		"agent-2": {{File: "f.go", Line: 11, Rule: "bug"}},
		"agent-3": {{File: "f.go", Line: 12, Rule: "bug"}},
	}

	result := DeduplicateFindings(groups)

	if len(result) != 1 {
		t.Fatalf("expected 1 merged finding, got %d", len(result))
	}

	if len(result[0].DetectedBy) != 3 {
		t.Errorf("expected 3 agents, got %d", len(result[0].DetectedBy))
	}
}

func TestFilterByMinAgreement(t *testing.T) {
	all := []Finding{
		{Rule: "a", DetectedBy: []string{"agent-1", "agent-2"}},
		{Rule: "b", DetectedBy: []string{"agent-1"}},
		{Rule: "c", DetectedBy: []string{"agent-1", "agent-2", "agent-3"}},
	}

	tests := []struct {
		name      string
		min       int
		wantCount int
	}{
		{"min=1 returns all", 1, 3},
		{"min=0 returns all", 0, 3},
		{"min=2 filters singles", 2, 2},
		{"min=3 filters doubles", 3, 1},
		{"min=4 returns none", 4, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterByMinAgreement(all, tt.min)
			if len(got) != tt.wantCount {
				t.Errorf("FilterByMinAgreement(min=%d) returned %d findings, want %d", tt.min, len(got), tt.wantCount)
			}
		})
	}
}

func TestDedupKey_CaseInsensitiveRule(t *testing.T) {
	f1 := Finding{File: "a.go", Line: 5, Rule: "ErrorCheck"}
	f2 := Finding{File: "a.go", Line: 5, Rule: "errorcheck"}

	if dedupKey(f1) != dedupKey(f2) {
		t.Error("expected case-insensitive rule matching in dedup key")
	}
}
