package conductor

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/valksor/kvelmo/internal/git"
)

// checkReviewChecklist verifies all review checklist items are checked.
// Caller must hold c.mu.
func (c *Conductor) checkReviewChecklist(checklist []string) error {
	for _, item := range checklist {
		if !slices.Contains(c.workUnit.ChecklistChecked, item) {
			return fmt.Errorf("cannot submit: review checklist item %q not checked. Run: kvelmo review --check %q", item, item)
		}
	}

	return nil
}

// checkRequiredPhases verifies that all required phases were completed since the last submit.
// Caller must hold c.mu.
func (c *Conductor) checkRequiredPhases(requiredPhases []string) error {
	history := c.machine.History()
	// Find the most recent EventSubmit to scope the check to the current cycle
	sinceIdx := 0
	for i, v := range slices.Backward(history) {
		if v.Event == EventSubmit {
			sinceIdx = i + 1

			break
		}
	}
	recentHistory := history[sinceIdx:]
	phaseEventMap := map[string]Event{
		PhasePlan:      EventPlanDone,
		PhaseImplement: EventImplementDone,
		PhaseReview:    EventReviewDone,
		PhaseSimplify:  EventSimplifyDone,
		PhaseOptimize:  EventOptimizeDone,
	}
	for _, phase := range requiredPhases {
		doneEvent, ok := phaseEventMap[phase]
		if !ok {
			continue
		}
		found := false
		for _, h := range recentHistory {
			if h.Event == doneEvent {
				found = true

				break
			}
		}
		if !found {
			return fmt.Errorf("cannot submit: required phase %q was not completed", phase)
		}
	}

	return nil
}

// checkSensitivePaths checks whether changes to sensitive files require review.
// Returns an error if sensitive paths were changed without review.
// Caller must hold c.mu.
func (c *Conductor) checkSensitivePaths(sensitivePaths []string, changedFiles []git.FileStatus) error {
	if len(sensitivePaths) == 0 || changedFiles == nil {
		return nil
	}

	// Scope review check to current cycle (since last EventSubmit),
	// matching the required-phases check above.
	history := c.machine.History()
	sinceIdx := 0
	for i, v := range slices.Backward(history) {
		if v.Event == EventSubmit {
			sinceIdx = i + 1

			break
		}
	}
	reviewDone := false
	for _, h := range history[sinceIdx:] {
		if h.Event == EventReviewDone {
			reviewDone = true

			break
		}
	}
	if reviewDone {
		return nil
	}

	var matchedPatterns []string
	for _, pattern := range sensitivePaths {
		for _, fs := range changedFiles {
			// Use filepath.Match for simple patterns and fall back to
			// prefix matching for directory patterns (filepath.Match
			// does not match across path separators with *).
			matched, _ := filepath.Match(pattern, fs.Path)
			if !matched {
				// Check if pattern is a directory prefix (e.g., "config/" matches "config/prod/secrets.yaml")
				dir := strings.TrimRight(pattern, "*")
				if strings.HasSuffix(dir, "/") || strings.HasSuffix(dir, string(filepath.Separator)) {
					matched = strings.HasPrefix(fs.Path, dir)
				}
			}
			if matched {
				matchedPatterns = append(matchedPatterns, pattern)

				break
			}
		}
	}
	if len(matchedPatterns) > 0 {
		return fmt.Errorf("changes to sensitive files require review: %s", strings.Join(matchedPatterns, ", "))
	}

	return nil
}
