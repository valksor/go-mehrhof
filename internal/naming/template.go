package naming

import (
	"regexp"
	"strings"
)

// TemplateVars holds variables for template expansion.
type TemplateVars struct {
	Key    string // External key (e.g., "FEATURE-123")
	TaskID string // Internal task ID (e.g., "a1b2c3d4")
	Type   string // Task type (e.g., "feature", "fix", "task")
	Slug   string // Slugified title (e.g., "add-user-auth")
	Title  string // Original title
}

var templatePattern = regexp.MustCompile(`\{([a-z_]+)\}`)

// ExpandTemplate expands a pattern with template variables.
// Supported variables: {key}, {task_id}, {type}, {slug}, {title}
//
// Example:
//
//	pattern: "{type}/{key}--{slug}"
//	result:  "feature/FEATURE-123--add-user-auth"
func ExpandTemplate(pattern string, vars TemplateVars) string {
	return templatePattern.ReplaceAllStringFunc(pattern, func(match string) string {
		// Extract variable name without braces
		varName := match[1 : len(match)-1]

		switch varName {
		case "key":
			return vars.Key
		case "task_id":
			return vars.TaskID
		case "type":
			return vars.Type
		case "slug":
			return vars.Slug
		case "title":
			return vars.Title
		default:
			// Unknown variable, leave as-is
			return match
		}
	})
}

// ValidatePattern checks if a pattern contains valid template variables.
// Returns a list of unknown variables found in the pattern.
func ValidatePattern(pattern string) []string {
	validVars := map[string]bool{
		"key":     true,
		"task_id": true,
		"type":    true,
		"slug":    true,
		"title":   true,
	}

	var unknown []string
	matches := templatePattern.FindAllStringSubmatch(pattern, -1)
	for _, match := range matches {
		if len(match) > 1 {
			varName := match[1]
			if !validVars[varName] {
				unknown = append(unknown, varName)
			}
		}
	}

	return unknown
}

// CleanBranchName ensures a branch name is valid for git.
// Removes or replaces invalid characters.
// Note: Double hyphens (--) are preserved as they're used as separators.
func CleanBranchName(name string) string {
	// Replace consecutive slashes
	name = regexp.MustCompile(`/{2,}`).ReplaceAllString(name, "/")

	// Replace 3+ consecutive hyphens with double hyphen (preserve -- as separator)
	name = regexp.MustCompile(`-{3,}`).ReplaceAllString(name, "--")

	// Remove trailing slash or hyphen
	name = strings.TrimRight(name, "/-")

	// Remove leading slash
	name = strings.TrimLeft(name, "/")

	return name
}
