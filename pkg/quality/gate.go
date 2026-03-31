package quality

import (
	"context"
	"fmt"
	"time"

	"github.com/valksor/kvelmo/pkg/findings"
)

// PerCheckerTimeout is the maximum time any single checker gets before being
// cancelled. This prevents a hung external tool from consuming the entire
// quality-gate budget and starving subsequent checkers. Exported so callers
// can scale the outer timeout proportionally (e.g., len(checkers) * PerCheckerTimeout).
const PerCheckerTimeout = 60 * time.Second

// Checker produces findings for a given work directory. Checkers represent
// "what we check" -- they are independent of blocking decisions.
type Checker interface {
	Name() string
	Check(ctx context.Context, workDir string) ([]findings.Finding, error)
}

// Gate decides which findings are blockers. Gates represent "what blocks" --
// they are independent of how findings are produced.
type Gate interface {
	Name() string
	Evaluate(findings []findings.Finding) []findings.Finding // returns blocking findings
}

// GateResult holds the outcome of running checkers through gates.
type GateResult struct {
	Passed   bool               `json:"passed"`
	Blockers []findings.Finding `json:"blockers,omitempty"`
	Total    int                `json:"total"`   // total findings
	Blocked  int                `json:"blocked"` // blocker count
	GateName string             `json:"gate_name"`
}

// RunGates executes all checkers to collect findings, then evaluates all gates
// to determine which findings are blockers. This separates "what we check"
// (checkers) from "what blocks" (gates).
func RunGates(ctx context.Context, workDir string, checkers []Checker, gates []Gate) (*GateResult, error) {
	var allFindings []findings.Finding

	for _, c := range checkers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		checkerCtx, checkerCancel := context.WithTimeout(ctx, PerCheckerTimeout)
		found, err := c.Check(checkerCtx, workDir)
		checkerCancel()
		if err != nil {
			return nil, fmt.Errorf("checker %s: %w", c.Name(), err)
		}

		allFindings = append(allFindings, found...)
	}

	var blockers []findings.Finding

	for _, g := range gates {
		gateBlockers := g.Evaluate(allFindings)
		for _, b := range gateBlockers {
			if !containsFinding(blockers, b) {
				blockers = append(blockers, b)
			}
		}
	}

	gateName := "composite"
	if len(gates) == 1 {
		gateName = gates[0].Name()
	}

	return &GateResult{
		Passed:   len(blockers) == 0,
		Blockers: blockers,
		Total:    len(allFindings),
		Blocked:  len(blockers),
		GateName: gateName,
	}, nil
}

// containsFinding checks if a finding is already in the slice by ID,
// falling back to Rule+Message comparison when both IDs are empty.
func containsFinding(blockers []findings.Finding, f findings.Finding) bool {
	for _, existing := range blockers {
		if existing.ID != "" && existing.ID == f.ID {
			return true
		}
		if existing.ID == "" && f.ID == "" && existing.Rule == f.Rule && existing.Message == f.Message {
			return true
		}
	}

	return false
}

// NoErrorsGate blocks on any finding with severity high or critical
// (mapped from "error" level in quality checks).
type NoErrorsGate struct{}

// Name returns the gate name.
func (NoErrorsGate) Name() string { return "no-errors" }

// Evaluate returns findings that have high or critical severity.
func (NoErrorsGate) Evaluate(ff []findings.Finding) []findings.Finding {
	var blockers []findings.Finding
	for _, f := range ff {
		if f.Severity == findings.SeverityCritical || f.Severity == findings.SeverityHigh {
			blockers = append(blockers, f)
		}
	}

	return blockers
}

// NoSecurityIssuesGate blocks on findings with category "security".
type NoSecurityIssuesGate struct{}

// Name returns the gate name.
func (NoSecurityIssuesGate) Name() string { return "no-security-issues" }

// Evaluate returns findings that have the security category.
func (NoSecurityIssuesGate) Evaluate(ff []findings.Finding) []findings.Finding {
	var blockers []findings.Finding
	for _, f := range ff {
		if f.Category == findings.CategorySecurity {
			blockers = append(blockers, f)
		}
	}

	return blockers
}

// MaxWarningsGate blocks if the number of medium-severity findings exceeds
// the configured threshold.
type MaxWarningsGate struct {
	Max int
}

// Name returns the gate name.
func (g MaxWarningsGate) Name() string { return "max-warnings" }

// Evaluate returns all medium-severity findings if the count exceeds Max.
func (g MaxWarningsGate) Evaluate(ff []findings.Finding) []findings.Finding {
	var warnings []findings.Finding
	for _, f := range ff {
		if f.Severity == findings.SeverityMedium {
			warnings = append(warnings, f)
		}
	}

	if len(warnings) <= g.Max {
		return nil
	}

	return warnings
}

// CompositeGate runs multiple gates and collects all blockers.
type CompositeGate struct {
	Gates []Gate
}

// Name returns the gate name.
func (CompositeGate) Name() string { return "composite" }

// Evaluate runs all sub-gates and returns the union of their blockers.
func (c CompositeGate) Evaluate(ff []findings.Finding) []findings.Finding {
	var blockers []findings.Finding
	for _, g := range c.Gates {
		gateBlockers := g.Evaluate(ff)
		for _, b := range gateBlockers {
			if !containsFinding(blockers, b) {
				blockers = append(blockers, b)
			}
		}
	}

	return blockers
}
