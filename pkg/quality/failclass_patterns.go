package quality

import (
	"regexp"
	"slices"

	"github.com/valksor/kvelmo/pkg/findings"
)

// FailPattern matches an error message to a classification.
type FailPattern struct {
	Name  string
	Regex *regexp.Regexp
	Class findings.Classification
}

// defaultFailPatterns are compiled once at package init.
var defaultFailPatterns = []FailPattern{
	// Flaky: transient infrastructure / timing issues
	{Name: "timeout", Regex: regexp.MustCompile(`(?i)timeout`), Class: findings.ClassificationFlaky},
	{Name: "deadline_exceeded", Regex: regexp.MustCompile(`(?i)deadline exceeded`), Class: findings.ClassificationFlaky},
	{Name: "eaddrinuse", Regex: regexp.MustCompile(`(?i)EADDRINUSE|address already in use`), Class: findings.ClassificationFlaky},
	{Name: "econnrefused", Regex: regexp.MustCompile(`(?i)ECONNREFUSED|connection refused`), Class: findings.ClassificationFlaky},
	{Name: "connection_reset", Regex: regexp.MustCompile(`(?i)connection reset`), Class: findings.ClassificationFlaky},
	{Name: "race_detected", Regex: regexp.MustCompile(`(?i)race detected|DATA RACE`), Class: findings.ClassificationFlaky},
	{Name: "file_locked", Regex: regexp.MustCompile(`(?i)file.*locked|lock.*file`), Class: findings.ClassificationFlaky},
	{Name: "port_in_use", Regex: regexp.MustCompile(`(?i)port already in use`), Class: findings.ClassificationFlaky},
	{Name: "dns_failure", Regex: regexp.MustCompile(`(?i)temporary failure in name resolution`), Class: findings.ClassificationFlaky},
	{Name: "tls_handshake", Regex: regexp.MustCompile(`(?i)TLS handshake timeout`), Class: findings.ClassificationFlaky},

	// Intermittent: may be intentional or may be a bug
	{Name: "context_canceled", Regex: regexp.MustCompile(`(?i)context canceled`), Class: findings.ClassificationIntermittent},
	{Name: "signal_killed", Regex: regexp.MustCompile(`(?i)signal:\s*killed`), Class: findings.ClassificationIntermittent},
}

// DefaultFailPatterns returns built-in flaky detection patterns.
// Returns a clone to prevent mutation of the package-level slice.
func DefaultFailPatterns() []FailPattern {
	return slices.Clone(defaultFailPatterns)
}
