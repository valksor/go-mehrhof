package asana

import (
	"fmt"
	"regexp"
	"strings"
)

// Reference represents a parsed Asana task reference
type Reference struct {
	TaskGID    string // Asana task GID (numeric string)
	ProjectGID string // Optional project GID for context
	IsExplicit bool   // True if project was explicitly provided
}

// String returns a canonical string representation
func (r *Reference) String() string {
	if r.ProjectGID != "" {
		return fmt.Sprintf("%s/%s", r.ProjectGID, r.TaskGID)
	}
	return r.TaskGID
}

// Patterns for parsing Asana references
var (
	// Matches: numeric GID (task only)
	// Asana GIDs are numeric strings, typically 16-19 digits
	taskGIDPattern = regexp.MustCompile(`^(\d{10,25})$`)

	// Matches: project_gid/task_gid format
	projectTaskPattern = regexp.MustCompile(`^(\d{10,25})/(\d{10,25})$`)

	// Matches Asana app URL: https://app.asana.com/0/project_gid/task_gid
	// or https://app.asana.com/0/0/task_gid (no project)
	appURLPattern = regexp.MustCompile(`(?:https?://)?app\.asana\.com/0/(\d+)/(\d+)(?:/f)?$`)
)

// ParseReference parses an Asana task reference
// Supported formats:
//   - asana:1234567890123456 or as:1234567890123456 - task GID only
//   - asana:project_gid/task_gid - with project context
//   - https://app.asana.com/0/project_gid/task_gid - full URL
func ParseReference(input string) (*Reference, error) {
	if input == "" {
		return nil, ErrInvalidReference
	}

	// Strip scheme prefix
	ref := input
	ref = strings.TrimPrefix(ref, "asana:")
	ref = strings.TrimPrefix(ref, "as:")
	ref = strings.TrimSpace(ref)

	if ref == "" {
		return nil, ErrInvalidReference
	}

	// Try URL format first
	if matches := appURLPattern.FindStringSubmatch(ref); matches != nil {
		projectGID := matches[1]
		taskGID := matches[2]

		// Asana uses "0" as a placeholder for "no project" in URLs
		if projectGID == "0" {
			projectGID = ""
		}

		return &Reference{
			TaskGID:    taskGID,
			ProjectGID: projectGID,
			IsExplicit: projectGID != "",
		}, nil
	}

	// Try project/task format
	if matches := projectTaskPattern.FindStringSubmatch(ref); matches != nil {
		return &Reference{
			TaskGID:    matches[2],
			ProjectGID: matches[1],
			IsExplicit: true,
		}, nil
	}

	// Try simple task GID format
	if matches := taskGIDPattern.FindStringSubmatch(ref); matches != nil {
		return &Reference{
			TaskGID:    matches[1],
			IsExplicit: false,
		}, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrInvalidReference, input)
}

// ExtractTaskGIDs extracts task GIDs from text (e.g., mentions in descriptions)
func ExtractTaskGIDs(text string) []string {
	// Look for Asana task URLs
	pattern := regexp.MustCompile(`app\.asana\.com/0/\d+/(\d+)`)
	matches := pattern.FindAllStringSubmatch(text, -1)

	var gids []string
	seen := make(map[string]bool)

	for _, match := range matches {
		gid := match[1]
		if !seen[gid] {
			gids = append(gids, gid)
			seen[gid] = true
		}
	}

	return gids
}
