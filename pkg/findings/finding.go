// Package findings provides a unified finding model consumed by quality, policy,
// and security packages. It standardizes severity levels, gate rules, and
// profiles for phase-aware quality decisions.
package findings

// Severity represents the unified severity level across all finding sources.
type Severity int

const (
	SeverityCritical Severity = iota
	SeverityHigh
	SeverityMedium
	SeverityLow
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityCritical:
		return "critical"
	case SeverityHigh:
		return "high"
	case SeverityMedium:
		return "medium"
	case SeverityLow:
		return "low"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}

// Origin indicates whether a finding was introduced by the current change or
// already existed before the change (pre-existing tech debt).
type Origin string

const (
	OriginIntroduced  Origin = "introduced"
	OriginPreExisting Origin = "pre_existing"
	OriginUnknown     Origin = "unknown"
)

// Category identifies which subsystem produced the finding.
type Category string

const (
	CategoryQuality  Category = "quality"
	CategoryPolicy   Category = "policy"
	CategorySecurity Category = "security"
)

// Finding is the unified result type for quality, policy, and security checks.
type Finding struct {
	ID          string   `json:"id"`
	Severity    Severity `json:"severity"`
	Category    Category `json:"category"`
	Rule        string   `json:"rule"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Message     string   `json:"message"`
	Remediation string   `json:"remediation,omitempty"`
	Evidence    string   `json:"evidence,omitempty"`
	Source      string   `json:"source"`
	Origin      Origin   `json:"origin,omitempty"`
}

// CountBySeverity returns the number of findings at each severity level.
func CountBySeverity(findings []Finding) map[Severity]int {
	counts := make(map[Severity]int, 5)
	for _, f := range findings {
		counts[f.Severity]++
	}

	return counts
}
