package security

import (
	"fmt"

	"github.com/valksor/kvelmo/pkg/findings"
)

// ToFindings converts a security Report's findings into unified findings.
func (r *Report) ToFindings() []findings.Finding {
	result := make([]findings.Finding, 0, len(r.Findings))
	for i, f := range r.Findings {
		result = append(result, findings.Finding{
			ID:          fmt.Sprintf("security-%s-%d", r.Scanner, i),
			Severity:    mapSecuritySeverity(f.Severity),
			Category:    findings.CategorySecurity,
			Rule:        f.Type,
			File:        f.File,
			Line:        f.Line,
			Message:     f.Message,
			Remediation: f.Suggestion,
			Source:      r.Scanner,
		})
	}

	return result
}

// ReportsToFindings converts multiple security reports into unified findings.
func ReportsToFindings(reports []*Report) []findings.Finding {
	var all []findings.Finding
	for _, r := range reports {
		all = append(all, r.ToFindings()...)
	}

	return all
}

func mapSecuritySeverity(s Severity) findings.Severity {
	switch s {
	case SeverityCritical:
		return findings.SeverityCritical
	case SeverityHigh:
		return findings.SeverityHigh
	case SeverityMedium:
		return findings.SeverityMedium
	case SeverityLow:
		return findings.SeverityLow
	case SeverityInfo:
		return findings.SeverityInfo
	default:
		return findings.SeverityInfo
	}
}
