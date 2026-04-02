package quality

import (
	"testing"

	"github.com/valksor/kvelmo/pkg/findings"
)

func TestClassify_TimeoutIsFlaky(t *testing.T) {
	t.Parallel()

	c := NewFailClassifier(nil)
	ff := []findings.Finding{
		{Message: "context deadline exceeded", Rule: "test_timeout"},
	}
	result := c.Classify(ff)

	if result[0].Classification != string(findings.ClassificationFlaky) {
		t.Errorf("expected flaky, got %s", result[0].Classification)
	}
}

func TestClassify_LintErrorIsGenuine(t *testing.T) {
	t.Parallel()

	c := NewFailClassifier(nil)
	ff := []findings.Finding{
		{Message: "unused variable x", Rule: "lint_unused"},
	}
	result := c.Classify(ff)

	if result[0].Classification != string(findings.ClassificationGenuine) {
		t.Errorf("expected genuine, got %s", result[0].Classification)
	}
}

func TestClassify_EADDRINUSEIsFlaky(t *testing.T) {
	t.Parallel()

	c := NewFailClassifier(nil)
	ff := []findings.Finding{
		{Message: "bind: address already in use", Rule: "test_bind"},
	}
	result := c.Classify(ff)

	if result[0].Classification != string(findings.ClassificationFlaky) {
		t.Errorf("expected flaky, got %s", result[0].Classification)
	}
}

func TestClassify_UnknownIsGenuine(t *testing.T) {
	t.Parallel()

	c := NewFailClassifier(nil)
	ff := []findings.Finding{
		{Message: "some completely unknown error pattern xyz123", Rule: "unknown_rule"},
	}
	result := c.Classify(ff)

	if result[0].Classification != string(findings.ClassificationGenuine) {
		t.Errorf("expected genuine, got %s", result[0].Classification)
	}
}

func TestClassify_HistoryOverridesPattern(t *testing.T) {
	t.Parallel()

	h := NewFailHistory(5)
	// Record mostly failures for this rule (4/5 = 80% > 60%)
	for range 4 {
		h.Record("my_rule", false)
	}
	h.Record("my_rule", true)

	c := NewFailClassifier(h)
	ff := []findings.Finding{
		{Message: "perfectly normal error message", Rule: "my_rule"},
	}
	result := c.Classify(ff)

	if result[0].Classification != string(findings.ClassificationFlaky) {
		t.Errorf("expected flaky from history, got %s", result[0].Classification)
	}
}

func TestFailClassifierStats_CountsCorrectly(t *testing.T) {
	t.Parallel()

	c := NewFailClassifier(nil)
	ff := []findings.Finding{
		{Classification: string(findings.ClassificationFlaky)},
		{Classification: string(findings.ClassificationFlaky)},
		{Classification: string(findings.ClassificationGenuine)},
		{Classification: string(findings.ClassificationIntermittent)},
		{Classification: ""},
	}
	s := c.Stats(ff)

	if s.Total != 5 {
		t.Errorf("expected total 5, got %d", s.Total)
	}
	if s.Flaky != 2 {
		t.Errorf("expected flaky 2, got %d", s.Flaky)
	}
	if s.Genuine != 1 {
		t.Errorf("expected genuine 1, got %d", s.Genuine)
	}
	if s.Intermittent != 1 {
		t.Errorf("expected intermittent 1, got %d", s.Intermittent)
	}
	if s.Unclassified != 1 {
		t.Errorf("expected unclassified 1, got %d", s.Unclassified)
	}
}

func TestFailPatterns_AllMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantCls findings.Classification
	}{
		{"timeout", "operation timeout after 30s", findings.ClassificationFlaky},
		{"deadline_exceeded", "context deadline exceeded", findings.ClassificationFlaky},
		{"eaddrinuse", "listen tcp :8080: bind: address already in use", findings.ClassificationFlaky},
		{"eaddrinuse_literal", "EADDRINUSE", findings.ClassificationFlaky},
		{"econnrefused", "dial tcp 127.0.0.1:3000: connect: connection refused", findings.ClassificationFlaky},
		{"econnrefused_literal", "ECONNREFUSED", findings.ClassificationFlaky},
		{"connection_reset", "read: connection reset by peer", findings.ClassificationFlaky},
		{"race_detected", "WARNING: DATA RACE", findings.ClassificationFlaky},
		{"race_detected_lower", "race detected", findings.ClassificationFlaky},
		{"file_locked", "file is locked by another process", findings.ClassificationFlaky},
		{"port_in_use", "port already in use", findings.ClassificationFlaky},
		{"dns_failure", "temporary failure in name resolution", findings.ClassificationFlaky},
		{"tls_handshake", "TLS handshake timeout", findings.ClassificationFlaky},
		{"context_canceled", "context canceled", findings.ClassificationIntermittent},
		{"signal_killed", "signal: killed", findings.ClassificationIntermittent},
		{"genuine_lint", "unused import fmt", findings.ClassificationGenuine},
	}

	c := NewFailClassifier(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ff := []findings.Finding{{Message: tt.input}}
			result := c.Classify(ff)

			if findings.Classification(result[0].Classification) != tt.wantCls {
				t.Errorf("input %q: expected %s, got %s", tt.input, tt.wantCls, result[0].Classification)
			}
		})
	}
}

func TestFailHistory_IsFlaky(t *testing.T) {
	t.Parallel()

	t.Run("empty history is not flaky", func(t *testing.T) {
		t.Parallel()

		h := NewFailHistory(10)
		if h.IsFlaky("unknown_rule") {
			t.Error("expected empty history to not be flaky")
		}
	})

	t.Run("mostly passing is not flaky", func(t *testing.T) {
		t.Parallel()

		h := NewFailHistory(10)
		for range 8 {
			h.Record("rule", true)
		}
		h.Record("rule", false)
		h.Record("rule", false)

		if h.IsFlaky("rule") {
			t.Error("expected mostly passing to not be flaky")
		}
	})

	t.Run("mostly failing is flaky", func(t *testing.T) {
		t.Parallel()

		h := NewFailHistory(5)
		h.Record("rule", false)
		h.Record("rule", false)
		h.Record("rule", false)
		h.Record("rule", false)
		h.Record("rule", true)

		if !h.IsFlaky("rule") {
			t.Error("expected mostly failing to be flaky")
		}
	})

	t.Run("window truncation", func(t *testing.T) {
		t.Parallel()

		h := NewFailHistory(3)
		// Old failures
		h.Record("rule", false)
		h.Record("rule", false)
		h.Record("rule", false)
		// New passes push old failures out of window
		h.Record("rule", true)
		h.Record("rule", true)
		h.Record("rule", true)

		if h.IsFlaky("rule") {
			t.Error("expected window to drop old failures")
		}
	})
}
