package youtrack

import (
	"fmt"
	"regexp"
	"strings"
)

// Ref represents a parsed YouTrack issue reference
type Ref struct {
	ID        string // The readable ID (e.g., "ABC-123")
	Permalink string // The full URL if provided
	Host      string // Extracted host from URL
}

// String returns the canonical representation
func (r *Ref) String() string {
	return r.ID
}

var (
	// Matches: https://company.myjetbrains.com/youtrack/issue/ABC-123
	// Matches: https://youtrack.cloud/issue/ABC-123 (no /youtrack in path)
	// Matches: http://instance.youtrack.cloud/issue/ABC-123
	urlPattern = regexp.MustCompile(`^https?://[^/]+/(?:youtrack/)?issue/([A-Za-z0-9]+-[0-9]+)`)

	// Matches: ABC-123 format (project prefix + dash + number)
	// Project prefix: alphanumeric characters (case-insensitive), at least 1 char
	// Number: at least 1 digit
	readableIDPattern = regexp.MustCompile(`^([A-Za-z0-9]+)-([0-9]+)$`)
)

// ParseReference parses various YouTrack reference formats
// Supported formats:
//   - "yt:ABC-123"                      -> scheme prefix (short)
//   - "youtrack:ABC-123"                -> scheme prefix (full)
//   - "ABC-123"                         -> bare readable ID
//   - "https://.../youtrack/issue/ABC-123" -> URL
//   - "https://.../issue/ABC-123"       -> URL (youtrack.cloud format)
func ParseReference(input string) (*Ref, error) {
	input = strings.TrimSpace(input)

	if input == "" {
		return nil, fmt.Errorf("%w: empty reference", ErrInvalidReference)
	}

	// Strip scheme prefix
	input = strings.TrimPrefix(input, "youtrack:")
	input = strings.TrimPrefix(input, "yt:")

	// Check for URL
	if matches := urlPattern.FindStringSubmatch(input); matches != nil {
		return &Ref{
			ID:        strings.ToUpper(matches[1]), // Normalize to uppercase
			Permalink: input,
			Host:      extractHost(input),
		}, nil
	}

	// Check for readable ID pattern (ABC-123)
	if readableIDPattern.MatchString(input) {
		return &Ref{ID: strings.ToUpper(input)}, nil // Normalize to uppercase
	}

	return nil, fmt.Errorf("%w: unrecognized format: %s (expected ABC-123 or YouTrack URL)",
		ErrInvalidReference, input)
}

// extractHost extracts the host from a YouTrack URL
func extractHost(rawURL string) string {
	parts := strings.Split(rawURL, "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// IsValidID checks if a string is a valid YouTrack readable ID
func IsValidID(id string) bool {
	return readableIDPattern.MatchString(id)
}
