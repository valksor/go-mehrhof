package notion

import (
	"fmt"
	"regexp"
	"strings"
)

// Ref represents a parsed Notion reference
type Ref struct {
	PageID     string // The 32-char page ID (UUID without dashes)
	URL        string // The full URL if provided
	DatabaseID string // Optional database ID for queries
	IsExplicit bool   // true if explicitly formatted
}

// String returns the canonical string representation
func (r *Ref) String() string {
	if r.URL != "" {
		return r.URL
	}
	if r.PageID != "" {
		return r.PageID
	}
	return ""
}

var (
	// Matches Notion page URLs: https://www.notion.so/username/Page-Title-32charID
	// The regex matches paths ending with -32charID (the ID is the last 32 hex chars)
	// Handles URLs like: https://www.notion.so/Page-Title-abcdef1234567890abcdef12345678
	// Also handles: https://www.notion.so/username/Page-Title-abcdef1234567890abcdef12345678
	notionURLPattern = regexp.MustCompile(`(?i)^https://www\.notion\.so/([a-zA-Z0-9_-]*/)*([a-zA-Z0-9_-]+-)*([a-f0-9]{32})(?:\?[^/]*)?$`)
	// Matches UUID with dashes: a1b2c3d4-e5f6-7890-1234-567890abcd (8-4-4-4-12 format)
	uuidWithDashes = regexp.MustCompile(`(?i)^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
	// Matches 32-char hex ID (UUID without dashes)
	pageIDPattern = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)
)

// ParseReference parses various Notion reference formats
// Supported formats:
//   - "notion:page-id"        -> page ID with scheme
//   - "nt:page-id"            -> short scheme
//   - "notion:https://www.notion.so/...title" -> URL with scheme
//   - "https://www.notion.so/...title" -> URL
//   - "a1b2c3d4e5f6..."       -> bare page ID (32-char hex)
//   - "a1b2-c3d4-e5f6..."     -> bare UUID with dashes
func ParseReference(input string) (*Ref, error) {
	input = strings.TrimSpace(input)

	if input == "" {
		return nil, fmt.Errorf("%w: empty reference", ErrInvalidReference)
	}

	// Strip scheme prefix if present
	schemeStripped := strings.TrimPrefix(input, "notion:")
	schemeStripped = strings.TrimPrefix(schemeStripped, "nt:")

	// Check for Notion URL in both original and scheme-stripped input
	if matches := notionURLPattern.FindStringSubmatch(input); len(matches) > 3 {
		return &Ref{
			PageID:     matches[3],
			URL:        input,
			IsExplicit: true,
		}, nil
	}

	if matches := notionURLPattern.FindStringSubmatch(schemeStripped); len(matches) > 3 {
		return &Ref{
			PageID:     matches[3],
			URL:        schemeStripped,
			IsExplicit: true,
		}, nil
	}

	// Use scheme-stripped version for remaining checks
	pageID := schemeStripped

	// Check for UUID with dashes (convert to 32-char format)
	if uuidWithDashes.MatchString(pageID) {
		normalizedID := strings.ReplaceAll(pageID, "-", "")
		return &Ref{
			PageID:     normalizedID,
			IsExplicit: false,
		}, nil
	}

	// Parse plain 32-char page ID format
	if pageIDPattern.MatchString(pageID) {
		return &Ref{
			PageID:     pageID,
			IsExplicit: false,
		}, nil
	}

	return nil, fmt.Errorf("%w: unrecognized format: %s (expected 32-char page ID or Notion URL)", ErrInvalidReference, input)
}

// ExtractPageID extracts the page ID from a Notion URL
// Returns empty string if not a valid URL
func ExtractPageID(url string) string {
	if matches := notionURLPattern.FindStringSubmatch(url); len(matches) > 3 {
		return matches[3]
	}
	return ""
}

// NormalizePageID converts a UUID with dashes to a 32-char hex string
func NormalizePageID(id string) string {
	// If already 32 chars, return as-is (lowercase for consistency)
	if pageIDPattern.MatchString(id) {
		return strings.ToLower(id)
	}
	// If UUID with dashes, remove them and lowercase
	if uuidWithDashes.MatchString(id) {
		return strings.ToLower(strings.ReplaceAll(id, "-", ""))
	}
	// Try to extract from URL
	if extracted := ExtractPageID(id); extracted != "" {
		return extracted
	}
	return ""
}
