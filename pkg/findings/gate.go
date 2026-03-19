package findings

import "fmt"

// GateRule evaluates a set of findings and returns any blockers that should
// prevent a workflow transition.
type GateRule interface {
	Evaluate(findings []Finding) []Blocker
}

// Blocker represents a gate rule that was triggered.
type Blocker struct {
	Rule    string  `json:"rule"`
	Reason  string  `json:"reason"`
	Finding Finding `json:"finding"`
}

// Evaluate runs all gate rules against the findings and returns any blockers.
func Evaluate(gates []GateRule, findings []Finding) []Blocker {
	var blockers []Blocker
	for _, gate := range gates {
		blockers = append(blockers, gate.Evaluate(findings)...)
	}

	return blockers
}

// AnySeverity blocks on any finding at the given severity level.
type AnySeverity struct {
	Level Severity
}

func (a AnySeverity) Evaluate(findings []Finding) []Blocker {
	var blockers []Blocker
	for _, f := range findings {
		if f.Severity == a.Level {
			blockers = append(blockers, Blocker{
				Rule:    fmt.Sprintf("any_%s", a.Level),
				Reason:  fmt.Sprintf("%s finding: %s", a.Level, f.Message),
				Finding: f,
			})
		}
	}

	return blockers
}

// MaxCount blocks when the count of findings at the given severity exceeds a threshold.
type MaxCount struct {
	Level Severity
	Max   int
}

func (m MaxCount) Evaluate(findings []Finding) []Blocker {
	var matched []Finding
	for _, f := range findings {
		if f.Severity == m.Level {
			matched = append(matched, f)
		}
	}

	if len(matched) <= m.Max {
		return nil
	}

	return []Blocker{{
		Rule:   fmt.Sprintf("max_%s_count", m.Level),
		Reason: fmt.Sprintf("%d %s findings exceed limit of %d", len(matched), m.Level, m.Max),
	}}
}

// MinScore blocks when the computed quality score falls below a threshold.
// Score: starts at 100, deducts per severity (critical: -40, high: -20, medium: -10, low: -5).
type MinScore struct {
	Threshold float64
}

func (m MinScore) Evaluate(findings []Finding) []Blocker {
	score := Score(findings)
	if score >= m.Threshold {
		return nil
	}

	return []Blocker{{
		Rule:   "min_score",
		Reason: fmt.Sprintf("score %.1f below threshold %.1f", score, m.Threshold),
	}}
}

// Score computes a weighted quality score (0-100) from findings.
func Score(findings []Finding) float64 {
	score := 100.0
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			score -= 40
		case SeverityHigh:
			score -= 20
		case SeverityMedium:
			score -= 10
		case SeverityLow:
			score -= 5
		case SeverityInfo:
			// Info findings don't affect the score.
		}
	}

	if score < 0 {
		return 0
	}

	return score
}
