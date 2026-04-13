package provider

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	taskListRegex  = regexp.MustCompile(`(?m)^[-*]\s+\[([ xX])\]\s+(.+)$`)
	dependsOnRegex = regexp.MustCompile(`(?mi)^depends on:\s*(.+)$`)
	issueRefRegex  = regexp.MustCompile(`(?:[\w-]+/[\w-]+)?#\d+`)
)

// ParseSubtasks extracts markdown task list items from the body text.
// Returns nil if body is empty or contains no task list items.
func ParseSubtasks(taskID, body string) []*Subtask {
	if body == "" {
		return nil
	}

	matches := taskListRegex.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	subtasks := make([]*Subtask, 0, len(matches))
	for i, match := range matches {
		completed := strings.ToLower(match[1]) == "x"
		text := strings.TrimSpace(match[2])
		subtasks = append(subtasks, &Subtask{
			ID:        fmt.Sprintf("%s-task-%d", taskID, i),
			Text:      text,
			Completed: completed,
			Index:     i,
		})
	}

	return subtasks
}

// ParseDependencies extracts issue references from a "Depends on:" line in the body.
// Returns nil if body is empty or contains no dependency references.
func ParseDependencies(body string) []string {
	if body == "" {
		return nil
	}

	match := dependsOnRegex.FindStringSubmatch(body)
	if match == nil {
		return nil
	}

	refs := issueRefRegex.FindAllString(match[1], -1)
	if len(refs) == 0 {
		return nil
	}

	return refs
}
