package memory

import (
	"fmt"
	"slices"

	"github.com/valksor/kvelmo/internal/storage"
)

const (
	phasePlan      = "plan"
	phaseImplement = "implement"
	phaseSimplify  = "simplify"
	phaseOptimize  = "optimize"
	phaseReview    = "review"
	phaseSubmit    = "submit"
)

// allPhases is the canonical list of phases in the workflow.
var allPhases = []string{phasePlan, phaseImplement, phaseSimplify, phaseOptimize, phaseReview, phaseSubmit}

// SkipSuggestion recommends skipping a phase based on historical patterns.
type SkipSuggestion struct {
	Phase      string  `json:"phase"`
	Confidence float64 `json:"confidence"` // 0.0-1.0
	Reason     string  `json:"reason"`
	SampleSize int     `json:"sample_size"`
}

// AgentSuggestion recommends an agent for a phase based on historical patterns.
type AgentSuggestion struct {
	Phase      string  `json:"phase"`
	Agent      string  `json:"agent"`
	Confidence float64 `json:"confidence"` // 0.0-1.0
	Reason     string  `json:"reason"`
	SampleSize int     `json:"sample_size"`
}

// MinSampleSize is the minimum number of completed tasks before making suggestions.
const MinSampleSize = 5

// DetectSkipPatterns analyzes archived tasks to find phases consistently skipped.
// A phase is suggested for skipping if it was absent from >80% of finished tasks.
func DetectSkipPatterns(tasks []storage.ArchivedTask) []SkipSuggestion {
	finished := filterFinished(tasks)
	if len(finished) < MinSampleSize {
		return nil
	}

	// Count tasks with PhasesRun data (excludes old archive format entries).
	withData := 0
	for _, t := range finished {
		if len(t.PhasesRun) > 0 {
			withData++
		}
	}
	if withData < MinSampleSize {
		return nil
	}

	var suggestions []SkipSuggestion

	for _, phase := range allPhases {
		// Count how many tasks ran this phase.
		ran := 0
		for _, t := range finished {
			if len(t.PhasesRun) == 0 {
				continue // No phase data (old archive format), skip
			}
			if slices.Contains(t.PhasesRun, phase) {
				ran++
			}
		}

		skipRate := 1.0 - float64(ran)/float64(withData)
		if skipRate >= 0.8 {
			suggestions = append(suggestions, SkipSuggestion{
				Phase:      phase,
				Confidence: skipRate,
				Reason:     fmt.Sprintf("%d of %d tasks skipped %s", withData-ran, withData, phase),
				SampleSize: withData,
			})
		}
	}

	return suggestions
}

// DetectAgentPatterns analyzes archived tasks to find which agent is most commonly used.
func DetectAgentPatterns(tasks []storage.ArchivedTask) []AgentSuggestion {
	finished := filterFinished(tasks)
	if len(finished) < MinSampleSize {
		return nil
	}

	// Count agent usage across finished tasks.
	agentCounts := make(map[string]int)
	total := 0
	for _, t := range finished {
		if t.AgentUsed != "" {
			agentCounts[t.AgentUsed]++
			total++
		}
	}

	if total < MinSampleSize {
		return nil
	}

	// Find the dominant agent.
	var suggestions []AgentSuggestion
	for agent, count := range agentCounts {
		rate := float64(count) / float64(total)
		if rate >= 0.7 {
			suggestions = append(suggestions, AgentSuggestion{
				Phase:      "default",
				Agent:      agent,
				Confidence: rate,
				Reason:     fmt.Sprintf("%d of %d tasks used %s", count, total, agent),
				SampleSize: total,
			})
		}
	}

	return suggestions
}

func filterFinished(tasks []storage.ArchivedTask) []storage.ArchivedTask {
	var finished []storage.ArchivedTask
	for _, t := range tasks {
		if t.FinalState == "finished" || t.FinalState == "submitted" {
			finished = append(finished, t)
		}
	}

	return finished
}
