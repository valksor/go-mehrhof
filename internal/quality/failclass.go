package quality

import "github.com/valksor/kvelmo/internal/findings"

// FailClassifier annotates findings with failure pattern classifications.
type FailClassifier struct {
	patterns []FailPattern
	history  *FailHistory
}

// NewFailClassifier creates a FailClassifier with default patterns and optional history.
func NewFailClassifier(history *FailHistory) *FailClassifier {
	return &FailClassifier{
		patterns: DefaultFailPatterns(),
		history:  history,
	}
}

// Classify annotates each finding's Classification field based on message pattern
// matching and historical frequency. Returns the same slice with Classification set.
func (c *FailClassifier) Classify(ff []findings.Finding) []findings.Finding {
	for i := range ff {
		ff[i].Classification = string(c.classify(&ff[i]))
	}

	return ff
}

// classify determines the classification for a single finding.
func (c *FailClassifier) classify(f *findings.Finding) findings.Classification {
	// Check history first: if a rule is historically flaky, classify as flaky
	// regardless of message content.
	if c.history != nil && f.Rule != "" && c.history.IsFlaky(f.Rule) {
		return findings.ClassificationFlaky
	}

	// Match against known patterns.
	for _, p := range c.patterns {
		if p.Regex.MatchString(f.Message) {
			return p.Class
		}
	}

	// Default: genuine failure.
	return findings.ClassificationGenuine
}

// FailClassifierStats returns classification statistics.
type FailClassifierStats struct {
	Total        int `json:"total"`
	Flaky        int `json:"flaky"`
	Genuine      int `json:"genuine"`
	Intermittent int `json:"intermittent"`
	Unclassified int `json:"unclassified"`
}

// Stats returns classification statistics for the given findings.
func (c *FailClassifier) Stats(ff []findings.Finding) FailClassifierStats {
	s := FailClassifierStats{Total: len(ff)}

	for _, f := range ff {
		switch findings.Classification(f.Classification) {
		case findings.ClassificationFlaky:
			s.Flaky++
		case findings.ClassificationGenuine:
			s.Genuine++
		case findings.ClassificationIntermittent:
			s.Intermittent++
		default:
			s.Unclassified++
		}
	}

	return s
}
