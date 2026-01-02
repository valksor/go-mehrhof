package naming

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Known type prefixes for task classification.
// These are matched case-insensitively at the start of filenames.
var TypePrefixes = []string{
	"feature", "feat",
	"fix", "bug", "bugfix", "hotfix",
	"chore",
	"docs", "doc",
	"refactor", "refact",
	"perf", "performance",
	"test", "tests",
	"style",
	"ci",
	"build",
	"task",
}

// typeAliases maps alternative prefixes to canonical types.
var typeAliases = map[string]string{
	"feat":        "feature",
	"bug":         "fix",
	"bugfix":      "fix",
	"hotfix":      "fix",
	"doc":         "docs",
	"refact":      "refactor",
	"performance": "perf",
	"tests":       "test",
}

// ticketPattern matches common ticket ID patterns like FEATURE-123, ABC-1, PROJ-9999.
var ticketPattern = regexp.MustCompile(`^([A-Z]+-\d+)`)

// typeHyphenPattern matches type-prefixed filenames like "feature-auth.md", "fix-login-bug.md".
var typeHyphenPattern = regexp.MustCompile(`^([a-z]+)-(.+)$`)

// TaskTypeFromFilename extracts the task type from a filename.
// It recognizes patterns like:
//   - "FEATURE-123.md" -> "feature" (from ticket prefix)
//   - "feature-auth.md" -> "feature" (from type prefix)
//   - "fix-login-bug.md" -> "fix" (from type prefix)
//   - "my-task.md" -> "task" (default)
func TaskTypeFromFilename(filename string) string {
	// Get base name without extension
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	if base == "" {
		return "task"
	}

	// Check for ticket pattern (e.g., FEATURE-123)
	if ticketPattern.MatchString(base) {
		// Extract the prefix part (e.g., "FEATURE" from "FEATURE-123")
		parts := strings.SplitN(base, "-", 2)
		if len(parts) > 0 {
			prefix := strings.ToLower(parts[0])
			// Check if it's a known type
			if isKnownType(prefix) {
				return normalizeType(prefix)
			}
		}
	}

	// Check for type-hyphen pattern (e.g., "feature-auth")
	baseLower := strings.ToLower(base)
	if match := typeHyphenPattern.FindStringSubmatch(baseLower); match != nil {
		prefix := match[1]
		if isKnownType(prefix) {
			return normalizeType(prefix)
		}
	}

	return "task"
}

// KeyFromFilename extracts the external key from a filename.
// It returns the filename without extension, which serves as the default key.
//
// Examples:
//   - "FEATURE-123.md" -> "FEATURE-123"
//   - "feature-auth.md" -> "feature-auth"
//   - "my task.md" -> "my task"
func KeyFromFilename(filename string) string {
	base := filepath.Base(filename)

	return strings.TrimSuffix(base, filepath.Ext(base))
}

// KeyFromDirectory extracts the external key from a directory path.
// It returns the base directory name.
//
// Examples:
//   - "/path/to/FEATURE-123" -> "FEATURE-123"
//   - "./tasks/my-feature" -> "my-feature"
func KeyFromDirectory(dirPath string) string {
	return filepath.Base(dirPath)
}

// isKnownType checks if a prefix is a known task type.
func isKnownType(prefix string) bool {
	for _, known := range TypePrefixes {
		if prefix == known {
			return true
		}
	}

	return false
}

// normalizeType returns the canonical form of a type prefix.
func normalizeType(prefix string) string {
	if canonical, ok := typeAliases[prefix]; ok {
		return canonical
	}

	return prefix
}

// ParseTicketID attempts to extract a ticket ID from a string.
// Returns the ticket ID and true if found, empty string and false otherwise.
//
// Examples:
//   - "FEATURE-123" -> "FEATURE-123", true
//   - "ABC-1" -> "ABC-1", true
//   - "my-task" -> "", false
func ParseTicketID(s string) (string, bool) {
	match := ticketPattern.FindString(s)
	if match != "" {
		return match, true
	}

	return "", false
}
