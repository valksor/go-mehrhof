package naming

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	// Match non-alphanumeric characters (except hyphens)
	nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]+`)
	// Match multiple consecutive hyphens
	multipleHyphens = regexp.MustCompile(`-{2,}`)
)

// Slugify converts a title to a URL-safe slug suitable for branch names.
// It lowercases, removes diacritics, replaces spaces/special chars with hyphens,
// and truncates to maxLen characters (at word boundary if possible).
func Slugify(title string, maxLen int) string {
	if title == "" {
		return ""
	}

	// Normalize unicode and remove diacritics
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, title)

	// Lowercase
	result = strings.ToLower(result)

	// Replace spaces and underscores with hyphens
	result = strings.ReplaceAll(result, " ", "-")
	result = strings.ReplaceAll(result, "_", "-")

	// Remove non-alphanumeric characters (except hyphens)
	result = nonAlphanumeric.ReplaceAllString(result, "")

	// Collapse multiple hyphens
	result = multipleHyphens.ReplaceAllString(result, "-")

	// Trim leading/trailing hyphens
	result = strings.Trim(result, "-")

	// Truncate if needed
	if maxLen > 0 && len(result) > maxLen {
		result = truncateAtWordBoundary(result, maxLen)
	}

	return result
}

// truncateAtWordBoundary truncates a slug at a hyphen boundary if possible.
func truncateAtWordBoundary(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	// Find the last hyphen before maxLen
	truncated := s[:maxLen]
	lastHyphen := strings.LastIndex(truncated, "-")

	if lastHyphen > maxLen/2 {
		// Use word boundary if it's not too early
		return truncated[:lastHyphen]
	}

	// Otherwise just truncate and trim trailing hyphen
	return strings.TrimRight(truncated, "-")
}
