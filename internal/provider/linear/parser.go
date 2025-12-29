package linear

import (
	"fmt"
	"regexp"
	"strings"
)

// Ref represents a parsed Linear issue reference
type Ref struct {
	IssueID    string // The issue identifier (e.g., "ENG-123")
	TeamKey    string // The team key (e.g., "ENG")
	Number     int    // The issue number (e.g., 123)
	URL        string // The full URL if provided
	IsExplicit bool   // true if explicitly formatted
}

// String returns the canonical string representation
func (r *Ref) String() string {
	if r.URL != "" {
		return r.URL
	}
	if r.IssueID != "" {
		return r.IssueID
	}
	if r.TeamKey != "" {
		return fmt.Sprintf("%s-%d", r.TeamKey, r.Number)
	}
	return ""
}

var (
	// Matches: https://linear.app/team-name/issue/ENG-123-title
	// Also handles: https://linear.app/issue/ENG-123
	// Team names can include letters, numbers, hyphens, and underscores
	linearURLPattern = regexp.MustCompile(`^https://linear\.app/(?:[a-zA-Z0-9_-]+/)?issue/([A-Z0-9]+-[0-9]+)(?:-[^\s]*)?$`)
	// Matches: TEAM-123 format (team key uppercase + dash + number)
	issueIDPattern = regexp.MustCompile(`^([A-Z0-9]+)-([0-9]+)$`)
)

// ParseReference parses various Linear issue reference formats
// Supported formats:
//   - "linear:ENG-123"        -> issue ID with scheme
//   - "ln:ENG-123"            -> short scheme
//   - "linear:https://linear.app/team/issue/ENG-123-title" -> URL with scheme
//   - "https://linear.app/team/issue/ENG-123-title" -> URL
//   - "ENG-123"               -> bare issue ID (if default provider)
func ParseReference(input string) (*Ref, error) {
	input = strings.TrimSpace(input)

	if input == "" {
		return nil, fmt.Errorf("%w: empty reference", ErrInvalidReference)
	}

	// Strip scheme prefix if present
	schemeStripped := strings.TrimPrefix(input, "linear:")
	schemeStripped = strings.TrimPrefix(schemeStripped, "ln:")

	// Check for Linear URL in both original and scheme-stripped input
	if matches := linearURLPattern.FindStringSubmatch(input); matches != nil {
		issueID := matches[1]
		ref, err := parseIssueID(issueID)
		if err != nil {
			return nil, err
		}
		ref.URL = input
		ref.IsExplicit = true
		return ref, nil
	}

	// Also check scheme-stripped for URLs
	if matches := linearURLPattern.FindStringSubmatch(schemeStripped); matches != nil {
		issueID := matches[1]
		ref, err := parseIssueID(issueID)
		if err != nil {
			return nil, err
		}
		ref.URL = schemeStripped
		ref.IsExplicit = true
		return ref, nil
	}

	// Use scheme-stripped version for remaining checks
	issueID := schemeStripped

	// Parse issue ID format (TEAM-123)
	ref, err := parseIssueID(issueID)
	if err != nil {
		return nil, fmt.Errorf("%w: unrecognized format: %s (expected TEAM-123 or linear URL)", ErrInvalidReference, input)
	}

	return ref, nil
}

// parseIssueID parses an issue ID in TEAM-123 format
func parseIssueID(issueID string) (*Ref, error) {
	if matches := issueIDPattern.FindStringSubmatch(issueID); matches != nil {
		teamKey := matches[1]
		var number int
		if _, err := fmt.Sscanf(matches[2], "%d", &number); err != nil {
			return nil, fmt.Errorf("%w: invalid issue number: %s", ErrInvalidReference, matches[2])
		}
		return &Ref{
			IssueID:    issueID,
			TeamKey:    teamKey,
			Number:     number,
			IsExplicit: false,
		}, nil
	}
	return nil, fmt.Errorf("%w: invalid issue ID format: %s (expected TEAM-123)", ErrInvalidReference, issueID)
}

// ExtractIssueID extracts the issue ID from a Linear URL
// Returns empty string if not a valid URL
func ExtractIssueID(url string) string {
	if matches := linearURLPattern.FindStringSubmatch(url); matches != nil {
		return matches[1]
	}
	return ""
}
