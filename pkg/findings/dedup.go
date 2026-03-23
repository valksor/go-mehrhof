package findings

import (
	"fmt"
	"slices"
	"strings"
)

// dedupKey generates a grouping key for a finding based on file and rule.
// Line proximity is handled separately during merge since bucket-based
// approaches have boundary issues.
func dedupKey(f Finding) string {
	return fmt.Sprintf("%s:%s", f.File, strings.ToLower(f.Rule))
}

// DeduplicateFindings merges findings from multiple sources by file+line+rule.
// Findings with the same file, line (within tolerance), and rule are merged,
// combining their DetectedBy lists.
// The groups parameter maps agent name to that agent's findings.
func DeduplicateFindings(groups map[string][]Finding) []Finding {
	const lineTolerance = 3

	type merged struct {
		finding    Finding
		detectedBy map[string]bool
	}

	// Group by file+rule, then merge entries within line tolerance.
	byKey := make(map[string][]*merged)
	var order []string

	// Sort agent names for deterministic processing order.
	agentNames := make([]string, 0, len(groups))
	for name := range groups {
		agentNames = append(agentNames, name)
	}
	slices.Sort(agentNames)

	for _, agentName := range agentNames {
		agentFindings := groups[agentName]
		for _, f := range agentFindings {
			key := dedupKey(f)

			// Check if any existing entry for this key is within line tolerance.
			found := false

			for _, m := range byKey[key] {
				diff := m.finding.Line - f.Line
				if diff < 0 {
					diff = -diff
				}

				if diff <= lineTolerance {
					m.detectedBy[agentName] = true
					found = true

					break
				}
			}

			if !found {
				f.DetectedBy = nil // will be rebuilt below
				m := &merged{
					finding:    f,
					detectedBy: map[string]bool{agentName: true},
				}

				if len(byKey[key]) == 0 {
					order = append(order, key)
				}

				byKey[key] = append(byKey[key], m)
			}
		}
	}

	var result []Finding

	for _, key := range order {
		for _, m := range byKey[key] {
			agents := make([]string, 0, len(m.detectedBy))

			for a := range m.detectedBy {
				agents = append(agents, a)
			}

			slices.Sort(agents)

			m.finding.DetectedBy = agents
			result = append(result, m.finding)
		}
	}

	return result
}

// FilterByMinAgreement returns only findings detected by at least minAgents agents.
func FilterByMinAgreement(all []Finding, minAgents int) []Finding {
	if minAgents <= 1 {
		return all
	}

	result := make([]Finding, 0, len(all))

	for _, f := range all {
		if len(f.DetectedBy) >= minAgents {
			result = append(result, f)
		}
	}

	return result
}
