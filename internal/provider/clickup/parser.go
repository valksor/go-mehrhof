package clickup

import (
	"regexp"
	"strings"
)

var (
	// taskIDPattern matches ClickUp task IDs (alphanumeric, 7-9 chars like "abc1234").
	taskIDPattern = regexp.MustCompile(`^[a-zA-Z0-9]{7,9}$`)

	// customTaskIDPattern matches custom task IDs like "PROJ-123".
	customTaskIDPattern = regexp.MustCompile(`^[A-Z]+-\d+$`)

	// appURLPattern matches ClickUp app URLs
	// Format: https://app.clickup.com/t/TEAM_ID/TASK_ID or https://app.clickup.com/t/TASK_ID
	appURLPattern = regexp.MustCompile(`(?:https?://)?app\.clickup\.com/t/(?:(\d+)/)?([a-zA-Z0-9]+)`)

	// shareURLPattern matches ClickUp share URLs
	// Format: https://sharing.clickup.com/TEAM_ID/t/h/TASK_ID/HASH
	shareURLPattern = regexp.MustCompile(`(?:https?://)?sharing\.clickup\.com/\d+/t/h/([a-zA-Z0-9]+)/`)

	// extractTaskIDsPattern finds task IDs in text.
	extractTaskIDsPattern = regexp.MustCompile(`app\.clickup\.com/t/(?:\d+/)?([a-zA-Z0-9]+)`)
)

// Reference represents a parsed ClickUp reference.
type Reference struct {
	TaskID     string // ClickUp task ID (e.g., "abc1234")
	CustomID   string // Custom task ID if using custom task IDs (e.g., "PROJ-123")
	TeamID     string // Team ID if present in URL
	IsExplicit bool   // True if parsed from explicit URL/reference format
}

// String returns the string representation of the reference.
func (r Reference) String() string {
	if r.CustomID != "" {
		return r.CustomID
	}
	return r.TaskID
}

// ParseReference parses a ClickUp reference from various formats:
// - Task ID: "abc1234"
// - Custom task ID: "PROJ-123"
// - App URL: "https://app.clickup.com/t/TEAM_ID/abc1234"
// - Share URL: "https://sharing.clickup.com/TEAM_ID/t/h/abc1234/HASH"
func ParseReference(input string) (*Reference, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, ErrInvalidReference
	}

	// Strip clickup: or cu: prefix if present
	input = strings.TrimPrefix(input, "clickup:")
	input = strings.TrimPrefix(input, "cu:")
	input = strings.TrimSpace(input)

	// Try app URL pattern
	if matches := appURLPattern.FindStringSubmatch(input); matches != nil {
		ref := &Reference{
			TaskID:     matches[2],
			IsExplicit: true,
		}
		if matches[1] != "" {
			ref.TeamID = matches[1]
		}
		return ref, nil
	}

	// Try share URL pattern
	if matches := shareURLPattern.FindStringSubmatch(input); matches != nil {
		return &Reference{
			TaskID:     matches[1],
			IsExplicit: true,
		}, nil
	}

	// Try custom task ID pattern (PROJ-123)
	if customTaskIDPattern.MatchString(input) {
		return &Reference{
			CustomID:   input,
			IsExplicit: false,
		}, nil
	}

	// Try bare task ID pattern
	if taskIDPattern.MatchString(input) {
		return &Reference{
			TaskID:     input,
			IsExplicit: false,
		}, nil
	}

	return nil, ErrInvalidReference
}

// ExtractTaskIDs extracts all task IDs from text (e.g., commit messages, descriptions).
func ExtractTaskIDs(text string) []string {
	matches := extractTaskIDsPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	// Deduplicate
	seen := make(map[string]bool)
	var result []string
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			result = append(result, match[1])
		}
	}

	return result
}
