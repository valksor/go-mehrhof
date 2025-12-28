package wrike

import (
	"context"
	"fmt"
)

// SubtaskInfo holds summary information about a subtask
type SubtaskInfo struct {
	ID     string
	Title  string
	Status string
}

// fetchSubtasks recursively fetches all subtasks for a task
// Returns a list of subtask info and combined comments from all subtasks
func (p *Provider) fetchSubtasks(ctx context.Context, subtaskIDs []string, depth int) ([]SubtaskInfo, []string, error) {
	const maxDepth = 5 // Prevent infinite recursion
	if depth > maxDepth {
		return nil, nil, nil
	}

	if len(subtaskIDs) == 0 {
		return nil, nil, nil
	}

	// Fetch all subtasks in one batch
	tasks, err := p.client.GetTasks(ctx, subtaskIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch subtasks: %w", err)
	}

	var infos []SubtaskInfo
	var allSubtaskIDs []string

	for _, task := range tasks {
		info := SubtaskInfo{
			ID:     task.ID,
			Title:  task.Title,
			Status: task.Status,
		}
		infos = append(infos, info)

		// Collect nested subtask IDs for recursive fetching
		if len(task.SubTaskIDs) > 0 {
			allSubtaskIDs = append(allSubtaskIDs, task.SubTaskIDs...)
		}
	}

	// Recursively fetch nested subtasks
	if len(allSubtaskIDs) > 0 {
		nestedInfos, _, err := p.fetchSubtasks(ctx, allSubtaskIDs, depth+1)
		if err != nil {
			return infos, nil, err // Return what we have so far
		}
		infos = append(infos, nestedInfos...)
	}

	return infos, nil, nil
}
