package policy

import (
	"fmt"

	"github.com/valksor/kvelmo/internal/findings"
)

// ViolationsToFindings converts policy violations into unified findings.
func ViolationsToFindings(violations []Violation) []findings.Finding {
	result := make([]findings.Finding, 0, len(violations))
	for i, v := range violations {
		result = append(result, findings.Finding{
			ID:       fmt.Sprintf("policy-%d", i),
			Severity: mapPolicySeverity(v.Severity),
			Category: findings.CategoryPolicy,
			Rule:     v.Rule,
			Message:  v.Message,
			Source:   "policy",
		})
	}

	return result
}

func mapPolicySeverity(s Severity) findings.Severity {
	switch s {
	case SeverityError:
		return findings.SeverityHigh
	case SeverityWarning:
		return findings.SeverityMedium
	default:
		return findings.SeverityInfo
	}
}
