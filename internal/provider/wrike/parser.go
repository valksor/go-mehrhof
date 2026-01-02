package wrike

import (
	"fmt"
	"regexp"
	"strings"
)

// Ref represents a parsed Wrike task reference.
type Ref struct {
	TaskID    string // The task ID (numeric or API ID)
	Permalink string // The full permalink if provided
}

// String returns the canonical string representation.
func (r *Ref) String() string {
	if r.Permalink != "" {
		return r.Permalink
	}

	return r.TaskID
}

var (
	// Matches: https://www.wrike.com/open.htm?id=1234567890
	// Also handles additional query params.
	permalinkPattern = regexp.MustCompile(`^https://www\.wrike\.com/open\.htm\?id=(\d+)`)
	// Matches Wrike API IDs (IEAAJXXXXXXXX format) - requires at least 5 chars.
	apiIDPattern = regexp.MustCompile(`^IE[A-Z0-9]{3,}$`)
	// Matches numeric IDs (10 digits).
	numericIDPattern = regexp.MustCompile(`^\d{10,}$`)
)

// ParseReference parses various Wrike task reference formats
// Supported formats:
//   - "wrike:1234567890"      -> numeric ID with scheme
//   - "wrike:IEAAJXXXXXXXX"   -> API ID with scheme
//   - "wk:1234567890"         -> short scheme
//   - "https://www.wrike.com/open.htm?id=1234567890" -> permalink
//   - "1234567890"            -> bare numeric ID (if default provider)
//   - "IEAAJXXXXXXXX"         -> bare API ID (if default provider)
func ParseReference(input string) (*Ref, error) {
	input = strings.TrimSpace(input)

	if input == "" {
		return nil, fmt.Errorf("%w: empty reference", ErrInvalidReference)
	}

	// Strip scheme prefix if present
	schemeStripped := strings.TrimPrefix(input, "wrike:")
	schemeStripped = strings.TrimPrefix(schemeStripped, "wk:")

	// Check for permalink
	if matches := permalinkPattern.FindStringSubmatch(input); matches != nil {
		return &Ref{
			TaskID:    matches[1],
			Permalink: input,
		}, nil
	}

	// Use scheme-stripped version for remaining checks
	taskID := schemeStripped

	// Check for API ID format
	if apiIDPattern.MatchString(taskID) {
		return &Ref{TaskID: taskID}, nil
	}

	// Check for numeric ID format
	if numericIDPattern.MatchString(taskID) {
		return &Ref{TaskID: taskID}, nil
	}

	return nil, fmt.Errorf("%w: unrecognized format: %s (expected wrike:ID, wk:ID, numeric ID, API ID, or permalink)", ErrInvalidReference, input)
}

// ExtractNumericID extracts the numeric ID from a permalink.
// Returns empty string if not a permalink.
func ExtractNumericID(permalink string) string {
	if matches := permalinkPattern.FindStringSubmatch(permalink); matches != nil {
		return matches[1]
	}

	return ""
}
