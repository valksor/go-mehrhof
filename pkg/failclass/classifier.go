package failclass

import "github.com/valksor/kvelmo/pkg/findings"

// Classification is an alias for findings.Classification.
type Classification = findings.Classification

const (
	ClassFlaky        = findings.ClassificationFlaky
	ClassGenuine      = findings.ClassificationGenuine
	ClassIntermittent = findings.ClassificationIntermittent
)

// Classifier annotates findings with failure pattern classifications.
type Classifier struct {
	patterns []Pattern
	history  *History
}

// New creates a Classifier with default patterns and optional history.
func New(history *History) *Classifier {
	return &Classifier{
		patterns: DefaultPatterns(),
		history:  history,
	}
}

// Classify annotates each finding's Classification field based on message pattern
// matching and historical frequency. Returns the same slice with Classification set.
func (c *Classifier) Classify(ff []findings.Finding) []findings.Finding {
	for i := range ff {
		ff[i].Classification = string(c.classify(&ff[i]))
	}

	return ff
}

// classify determines the classification for a single finding.
func (c *Classifier) classify(f *findings.Finding) Classification {
	// Check history first: if a rule is historically flaky, classify as flaky
	// regardless of message content.
	if c.history != nil && f.Rule != "" && c.history.IsFlaky(f.Rule) {
		return ClassFlaky
	}

	// Match against known patterns.
	for _, p := range c.patterns {
		if p.Regex.MatchString(f.Message) {
			return p.Class
		}
	}

	// Default: genuine failure.
	return ClassGenuine
}

// Stats returns classification statistics.
type Stats struct {
	Total        int `json:"total"`
	Flaky        int `json:"flaky"`
	Genuine      int `json:"genuine"`
	Intermittent int `json:"intermittent"`
	Unclassified int `json:"unclassified"`
}

// Stats returns classification statistics for the given findings.
func (c *Classifier) Stats(ff []findings.Finding) Stats {
	s := Stats{Total: len(ff)}

	for _, f := range ff {
		switch findings.Classification(f.Classification) {
		case ClassFlaky:
			s.Flaky++
		case ClassGenuine:
			s.Genuine++
		case ClassIntermittent:
			s.Intermittent++
		default:
			s.Unclassified++
		}
	}

	return s
}
