package failclass

import "sync"

// History tracks failure frequency per rule to detect flaky patterns over time.
type History struct {
	mu      sync.Mutex
	records map[string]*RuleRecord // key: rule name
	window  int                    // max entries per rule
}

// RuleRecord tracks recent outcomes for a single rule.
type RuleRecord struct {
	Outcomes []bool // true=passed, false=failed
}

// NewHistory creates a History with the given window size.
func NewHistory(window int) *History {
	if window < 1 {
		window = 10
	}

	return &History{
		records: make(map[string]*RuleRecord),
		window:  window,
	}
}

// Record adds an outcome for a rule.
func (h *History) Record(rule string, passed bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	rec, ok := h.records[rule]
	if !ok {
		rec = &RuleRecord{}
		h.records[rule] = rec
	}

	rec.Outcomes = append(rec.Outcomes, passed)
	if len(rec.Outcomes) > h.window {
		rec.Outcomes = rec.Outcomes[len(rec.Outcomes)-h.window:]
	}
}

// IsFlaky returns true if a rule fails more than 60% of the time in its window.
func (h *History) IsFlaky(rule string) bool {
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
