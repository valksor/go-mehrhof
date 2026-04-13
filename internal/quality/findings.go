package quality

import (
	"fmt"

	"github.com/valksor/kvelmo/internal/findings"
)

// ToFindings converts a quality Report's issues into unified findings.
func (r *Report) ToFindings() []findings.Finding {
	result := make([]findings.Finding, 0, len(r.Issues))
	for i, issue := range r.Issues {
		result = append(result, findings.Finding{
			ID:          fmt.Sprintf("quality-%s-%d", r.Linter, i),
			Severity:    mapQualitySeverity(issue.Severity),
			Category:    findings.CategoryQuality,
			Rule:        issue.Rule,
			File:        issue.File,
			Line:        issue.Line,
			Message:     issue.Message,
			Remediation: issue.Remediation,
			Source:      r.Linter,
		})
	}

	return result
}

// ReportsToFindings converts multiple quality reports into unified findings.
func ReportsToFindings(reports []*Report) []findings.Finding {
	var all []findings.Finding
	for _, r := range reports {
		all = append(all, r.ToFindings()...)
	}

	return all
}

func mapQualitySeverity(s Severity) findings.Severity {
	switch s {
	case SeverityError:
		return findings.SeverityHigh
	case SeverityWarning:
		return findings.SeverityMedium
	case SeverityInfo:
		return findings.SeverityInfo
	default:
		return findings.SeverityInfo
	}
}
