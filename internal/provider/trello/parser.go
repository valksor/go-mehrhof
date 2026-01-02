package trello

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Common errors.
var (
	ErrInvalidReference  = fmt.Errorf("invalid Trello reference")
	ErrNoBoardConfigured = fmt.Errorf("no board configured")
)

// Ref represents a parsed Trello card reference.
type Ref struct {
	CardID string // The card ID (24-character alphanumeric)
	URL    string // Original URL if provided
}

// String returns the reference as a string.
func (r *Ref) String() string {
	if r.URL != "" {
		return "trello:" + r.URL
	}
	return "trello:" + r.CardID
}

// ParseReference parses a Trello card reference from various formats
//
// Supported formats:
//   - trello:cardId
//   - tr:cardId
//   - trello:https://trello.com/c/shortLink/card-name
//   - tr:https://trello.com/c/shortLink/card-name
//   - https://trello.com/c/shortLink/card-name
//   - Just the card ID (24-character alphanumeric)
//   - Just the short link (8-character alphanumeric)
func ParseReference(input string) (*Ref, error) {
	// Strip scheme prefix
	input = strings.TrimPrefix(input, "trello:")
	input = strings.TrimPrefix(input, "tr:")
	input = strings.TrimSpace(input)

	if input == "" {
		return nil, ErrInvalidReference
	}

	// Check if it's a URL
	if strings.Contains(input, "trello.com/c/") {
		return parseURL(input)
	}

	// Check if it's a bare card ID (24 hex chars)
	if isCardID(input) {
		return &Ref{CardID: input}, nil
	}

	// Check if it's a short link (8 alphanumeric chars)
	if isShortLink(input) {
		return &Ref{CardID: input}, nil
	}

	return nil, fmt.Errorf("%w: %q", ErrInvalidReference, input)
}

// parseURL extracts the card short link from a Trello URL.
func parseURL(urlStr string) (*Ref, error) {
	// Pattern: https://trello.com/c/shortLink/card-name
	re := regexp.MustCompile(`trello\.com/c/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(urlStr)
	if len(matches) > 1 {
		return &Ref{
			CardID: matches[1],
			URL:    urlStr,
		}, nil
	}

	return nil, fmt.Errorf("%w: unable to extract card ID from URL", ErrInvalidReference)
}

// isCardID checks if a string is a valid Trello card ID (24 hex chars).
func isCardID(s string) bool {
	if len(s) != 24 {
		return false
	}
	for _, c := range s {
		isDigit := c >= '0' && c <= '9'
		isLowerHex := c >= 'a' && c <= 'f'
		isUpperHex := c >= 'A' && c <= 'F'
		if !isDigit && !isLowerHex && !isUpperHex {
			return false
		}
	}
	return true
}

// isShortLink checks if a string is a valid Trello short link (8 alphanumeric).
func isShortLink(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, c := range s {
		isDigit := c >= '0' && c <= '9'
		isLower := c >= 'a' && c <= 'z'
		isUpper := c >= 'A' && c <= 'Z'
		if !isDigit && !isLower && !isUpper {
			return false
		}
	}
	return true
}

// ──────────────────────────────────────────────────────────────────────────────
// Token Resolution
// ──────────────────────────────────────────────────────────────────────────────

// ResolveAPIKey resolves the Trello API key from environment.
func ResolveAPIKey(provided string) string {
	if provided != "" {
		return provided
	}

	// Try environment variables
	if key := os.Getenv("MEHR_TRELLO_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("TRELLO_API_KEY"); key != "" {
		return key
	}

	return ""
}

// ResolveToken resolves the Trello token from environment.
func ResolveToken(provided string) string {
	if provided != "" {
		return provided
	}

	// Try environment variables
	if token := os.Getenv("MEHR_TRELLO_TOKEN"); token != "" {
		return token
	}
	if token := os.Getenv("TRELLO_TOKEN"); token != "" {
		return token
	}

	return ""
}
