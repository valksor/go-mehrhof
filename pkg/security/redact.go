package security

import (
	"fmt"
	"regexp"
)

// Redactor strips secrets from content before sending to LLM APIs.
type Redactor struct {
	patterns []redactPattern
}

type redactPattern struct {
	name  string
	regex *regexp.Regexp
}

// NewRedactor creates a Redactor using the same patterns as SecretScanner
// plus any additional user-configured patterns.
func NewRedactor(extraPatterns []string) *Redactor {
	scanner := NewSecretScanner()

	patterns := make([]redactPattern, 0, len(scanner.patterns)+len(extraPatterns))
	for _, sp := range scanner.patterns {
		patterns = append(patterns, redactPattern{
			name:  sp.name,
			regex: sp.pattern,
		})
	}

	for i, extra := range extraPatterns {
		compiled, err := regexp.Compile(extra)
		if err != nil {
			continue
		}

		patterns = append(patterns, redactPattern{
			name:  fmt.Sprintf("Custom Pattern %d", i+1),
			regex: compiled,
		})
	}

	return &Redactor{patterns: patterns}
}

// Redact replaces detected secrets in content with [REDACTED:TYPE] placeholders.
func (r *Redactor) Redact(content string) string {
	for _, p := range r.patterns {
		content = p.regex.ReplaceAllString(content, fmt.Sprintf("[REDACTED:%s]", p.name))
	}

	return content
}
