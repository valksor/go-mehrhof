package quality

import (
	"regexp"
	"slices"
)

// FailPattern matches an error message to a classification.
type FailPattern struct {
	Name  string
	Regex *regexp.Regexp
	Class FailClassification
}

// defaultFailPatterns are compiled once at package init.
var defaultFailPatterns = []FailPattern{
	// Flaky: transient infrastructure / timing issues
	{Name: "timeout", Regex: regexp.MustCompile(`(?i)timeout`), Class: ClassFlaky},
	{Name: "deadline_exceeded", Regex: regexp.MustCompile(`(?i)deadline exceeded`), Class: ClassFlaky},
	{Name: "eaddrinuse", Regex: regexp.MustCompile(`(?i)EADDRINUSE|address already in use`), Class: ClassFlaky},
	{Name: "econnrefused", Regex: regexp.MustCompile(`(?i)ECONNREFUSED|connection refused`), Class: ClassFlaky},
	{Name: "connection_reset", Regex: regexp.MustCompile(`(?i)connection reset`), Class: ClassFlaky},
	{Name: "race_detected", Regex: regexp.MustCompile(`(?i)race detected|DATA RACE`), Class: ClassFlaky},
	{Name: "file_locked", Regex: regexp.MustCompile(`(?i)file.*locked|lock.*file`), Class: ClassFlaky},
	{Name: "port_in_use", Regex: regexp.MustCompile(`(?i)port already in use`), Class: ClassFlaky},
	{Name: "dns_failure", Regex: regexp.MustCompile(`(?i)temporary failure in name resolution`), Class: ClassFlaky},
	{Name: "tls_handshake", Regex: regexp.MustCompile(`(?i)TLS handshake timeout`), Class: ClassFlaky},

	// Intermittent: may be intentional or may be a bug
	{Name: "context_canceled", Regex: regexp.MustCompile(`(?i)context canceled`), Class: ClassIntermittent},
	{Name: "signal_killed", Regex: regexp.MustCompile(`(?i)signal:\s*killed`), Class: ClassIntermittent},
}

// DefaultFailPatterns returns built-in flaky detection patterns.
// Returns a clone to prevent mutation of the package-level slice.
func DefaultFailPatterns() []FailPattern {
	return slices.Clone(defaultFailPatterns)
}
