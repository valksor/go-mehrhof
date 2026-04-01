package quality

import "sync"

// FailHistory tracks failure frequency per rule to detect flaky patterns over time.
type FailHistory struct {
	mu      sync.Mutex
	records map[string]*FailRuleRecord // key: rule name
	window  int                        // max entries per rule
}

// FailRuleRecord tracks recent outcomes for a single rule.
type FailRuleRecord struct {
	Outcomes []bool // true=passed, false=failed
}

// NewFailHistory creates a FailHistory with the given window size.
func NewFailHistory(window int) *FailHistory {
	if window < 1 {
		window = 10
	}

	return &FailHistory{
		records: make(map[string]*FailRuleRecord),
		window:  window,
	}
}

// Record adds an outcome for a rule.
func (h *FailHistory) Record(rule string, passed bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	rec, ok := h.records[rule]
	if !ok {
		rec = &FailRuleRecord{}
		h.records[rule] = rec
	}

	rec.Outcomes = append(rec.Outcomes, passed)
	if len(rec.Outcomes) > h.window {
		rec.Outcomes = rec.Outcomes[len(rec.Outcomes)-h.window:]
	}
}

// IsFlaky returns true if a rule fails more than 60% of the time in its window.
func (h *FailHistory) IsFlaky(rule string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	rec, ok := h.records[rule]
	if !ok || len(rec.Outcomes) == 0 {
		return false
	}

	var failures int
	for _, passed := range rec.Outcomes {
		if !passed {
			failures++
		}
	}

	// Flaky if more than 60% of recent runs failed.
	return float64(failures)/float64(len(rec.Outcomes)) > 0.6
}
