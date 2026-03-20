package quality

import (
	"context"
	"errors"
	"testing"

	"github.com/valksor/kvelmo/pkg/findings"
)

// mockChecker implements Checker for testing.
type mockChecker struct {
	name     string
	findings []findings.Finding
	err      error
}

func (m *mockChecker) Name() string { return m.name }
func (m *mockChecker) Check(_ context.Context, _ string) ([]findings.Finding, error) {
	return m.findings, m.err
}

func TestNoErrorsGatePassesWhenNoErrors(t *testing.T) {
	gate := NoErrorsGate{}
	ff := []findings.Finding{
		{ID: "1", Severity: findings.SeverityMedium, Message: "a warning"},
		{ID: "2", Severity: findings.SeverityLow, Message: "low issue"},
		{ID: "3", Severity: findings.SeverityInfo, Message: "info"},
	}

	blockers := gate.Evaluate(ff)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blockers, got %d", len(blockers))
	}
}

func TestNoErrorsGateBlocksOnErrorSeverity(t *testing.T) {
	gate := NoErrorsGate{}
	ff := []findings.Finding{
		{ID: "1", Severity: findings.SeverityHigh, Message: "error finding"},
		{ID: "2", Severity: findings.SeverityCritical, Message: "critical finding"},
		{ID: "3", Severity: findings.SeverityMedium, Message: "warning"},
	}

	blockers := gate.Evaluate(ff)
	if len(blockers) != 2 {
		t.Fatalf("expected 2 blockers, got %d", len(blockers))
	}
}

func TestNoErrorsGateAllowsWarnings(t *testing.T) {
	gate := NoErrorsGate{}
	ff := []findings.Finding{
		{ID: "1", Severity: findings.SeverityMedium, Message: "just a warning"},
	}

	blockers := gate.Evaluate(ff)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blockers for warnings, got %d", len(blockers))
	}
}

func TestNoSecurityIssuesGateBlocksSecurityFindings(t *testing.T) {
	gate := NoSecurityIssuesGate{}
	ff := []findings.Finding{
		{ID: "1", Category: findings.CategorySecurity, Message: "secret leak"},
		{ID: "2", Category: findings.CategoryQuality, Message: "lint issue"},
	}

	blockers := gate.Evaluate(ff)
	if len(blockers) != 1 {
		t.Fatalf("expected 1 blocker, got %d", len(blockers))
	}
	if blockers[0].ID != "1" {
		t.Errorf("expected blocker ID %q, got %q", "1", blockers[0].ID)
	}
}

func TestNoSecurityIssuesGateAllowsNonSecurityFindings(t *testing.T) {
	gate := NoSecurityIssuesGate{}
	ff := []findings.Finding{
		{ID: "1", Category: findings.CategoryQuality, Message: "lint issue"},
		{ID: "2", Category: findings.CategoryPolicy, Message: "policy issue"},
	}

	blockers := gate.Evaluate(ff)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blockers, got %d", len(blockers))
	}
}

func TestMaxWarningsGatePassesUnderThreshold(t *testing.T) {
	gate := MaxWarningsGate{Max: 3}
	ff := []findings.Finding{
		{ID: "1", Severity: findings.SeverityMedium},
		{ID: "2", Severity: findings.SeverityMedium},
	}

	blockers := gate.Evaluate(ff)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blockers, got %d", len(blockers))
	}
}

func TestMaxWarningsGateBlocksOverThreshold(t *testing.T) {
	gate := MaxWarningsGate{Max: 2}
	ff := []findings.Finding{
		{ID: "1", Severity: findings.SeverityMedium},
		{ID: "2", Severity: findings.SeverityMedium},
		{ID: "3", Severity: findings.SeverityMedium},
		{ID: "4", Severity: findings.SeverityHigh}, // not a warning, should not count
	}

	blockers := gate.Evaluate(ff)
	if len(blockers) != 3 {
		t.Fatalf("expected 3 blockers (the 3 warnings), got %d", len(blockers))
	}
}

func TestCompositeGateCombinesBlockers(t *testing.T) {
	composite := CompositeGate{
		Gates: []Gate{
			NoErrorsGate{},
			NoSecurityIssuesGate{},
		},
	}
	ff := []findings.Finding{
		{ID: "1", Severity: findings.SeverityHigh, Category: findings.CategoryQuality, Message: "error"},
		{ID: "2", Severity: findings.SeverityLow, Category: findings.CategorySecurity, Message: "sec"},
		{ID: "3", Severity: findings.SeverityMedium, Category: findings.CategoryQuality, Message: "warning"},
	}

	blockers := composite.Evaluate(ff)
	if len(blockers) != 2 {
		t.Fatalf("expected 2 blockers, got %d", len(blockers))
	}
}

func TestCompositeGateDeduplicates(t *testing.T) {
	composite := CompositeGate{
		Gates: []Gate{
			NoErrorsGate{},
			NoSecurityIssuesGate{},
		},
	}
	// This finding matches both gates: high severity AND security category.
	ff := []findings.Finding{
		{ID: "1", Severity: findings.SeverityHigh, Category: findings.CategorySecurity, Message: "sec error"},
	}

	blockers := composite.Evaluate(ff)
	if len(blockers) != 1 {
		t.Fatalf("expected 1 deduplicated blocker, got %d", len(blockers))
	}
}

func TestRunGatesIntegration(t *testing.T) {
	checker := &mockChecker{
		name: "test-checker",
		findings: []findings.Finding{
			{ID: "1", Severity: findings.SeverityHigh, Category: findings.CategoryQuality, Message: "error"},
			{ID: "2", Severity: findings.SeverityMedium, Category: findings.CategoryQuality, Message: "warning"},
		},
	}

	result, err := RunGates(context.Background(), "/tmp", []Checker{checker}, []Gate{NoErrorsGate{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("expected gate to fail")
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	if result.Blocked != 1 {
		t.Errorf("blocked = %d, want 1", result.Blocked)
	}
	if result.GateName != "no-errors" {
		t.Errorf("gate_name = %q, want %q", result.GateName, "no-errors")
	}
}

func TestRunGatesWithFailingChecker(t *testing.T) {
	checker := &mockChecker{
		name: "failing-checker",
		err:  errors.New("connection refused"),
	}

	result, err := RunGates(context.Background(), "/tmp", []Checker{checker}, []Gate{NoErrorsGate{}})
	if err == nil {
		t.Fatal("expected error from failing checker")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

func TestRunGatesEmptyCheckersAndGates(t *testing.T) {
	result, err := RunGates(context.Background(), "/tmp", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected passed with no checkers/gates")
	}
	if result.Total != 0 {
		t.Errorf("total = %d, want 0", result.Total)
	}
	if result.Blocked != 0 {
		t.Errorf("blocked = %d, want 0", result.Blocked)
	}
}

func TestRunGatesMultipleCheckers(t *testing.T) {
	checker1 := &mockChecker{
		name: "lint",
		findings: []findings.Finding{
			{ID: "l1", Severity: findings.SeverityMedium, Message: "lint warning"},
		},
	}
	checker2 := &mockChecker{
		name: "security",
		findings: []findings.Finding{
			{ID: "s1", Severity: findings.SeverityHigh, Category: findings.CategorySecurity, Message: "vuln"},
		},
	}

	result, err := RunGates(
		context.Background(), "/tmp",
		[]Checker{checker1, checker2},
		[]Gate{NoErrorsGate{}, NoSecurityIssuesGate{}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("expected gate to fail")
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	// s1 is both high severity (NoErrorsGate) and security (NoSecurityIssuesGate),
	// but should be deduplicated.
	if result.Blocked != 1 {
		t.Errorf("blocked = %d, want 1", result.Blocked)
	}
}

func TestRunGatesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	checker := &mockChecker{
		name:     "slow",
		findings: []findings.Finding{{ID: "1"}},
	}

	_, err := RunGates(ctx, "/tmp", []Checker{checker}, []Gate{NoErrorsGate{}})
	if err == nil {
		t.Error("expected context cancellation error")
	}
}
