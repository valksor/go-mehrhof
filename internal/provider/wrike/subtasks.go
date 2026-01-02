package wrike

import (
	"context"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// SubtaskInfo holds summary information about a subtask.
type SubtaskInfo struct {
	ID     string
	Title  string
	Status string
}

// FetchSubtasks implements the provider.SubtaskFetcher interface.
// It retrieves all subtasks for a given work unit as full WorkUnit objects.
func (p *Provider) FetchSubtasks(ctx context.Context, workUnitID string) ([]*provider.WorkUnit, error) {
	// First fetch the parent task to get subtask IDs
	task, err := p.client.GetTask(ctx, workUnitID)
	if err != nil {
		// Try by permalink
		task, err = p.client.GetTaskByPermalink(ctx, workUnitID)
		if err != nil {
			return nil, fmt.Errorf("fetch parent task: %w", err)
		}
	}

	if len(task.SubTaskIDs) == 0 {
		return nil, nil
	}

	// Fetch subtasks as full Task objects
	tasks, err := p.client.GetTasks(ctx, task.SubTaskIDs)
	if err != nil {
		return nil, fmt.Errorf("fetch subtasks: %w", err)
	}

	// Convert to WorkUnits
	workUnits := make([]*provider.WorkUnit, 0, len(tasks))
	for _, st := range tasks {
		wu := p.subtaskToWorkUnit(&st, workUnitID)
		workUnits = append(workUnits, wu)
	}

	return workUnits, nil
}

// subtaskToWorkUnit converts a subtask Task to a WorkUnit with parent reference.
func (p *Provider) subtaskToWorkUnit(task *Task, parentID string) *provider.WorkUnit {
	numericID := ExtractNumericID(task.Permalink)
	if numericID == "" {
		numericID = task.ID
	}

	return &provider.WorkUnit{
		ID:          numericID,
		ExternalID:  task.ID,
		Provider:    ProviderName,
		Title:       task.Title,
		Description: task.Description,
		Status:      mapStatus(task.Status),
		Priority:    mapPriority(task.Priority),
		Labels:      []string{},
		Assignees:   []provider.Person{},
		Subtasks:    task.SubTaskIDs,
		Metadata: map[string]any{
			"permalink":      task.Permalink,
			"api_id":         task.ID,
			"wrike_status":   task.Status,
			"wrike_priority": task.Priority,
			"parent_id":      parentID,
			"is_subtask":     true,
			"subtask_count":  len(task.SubTaskIDs),
		},
		CreatedAt: task.CreatedDate,
		UpdatedAt: task.UpdatedDate,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: task.Permalink,
			SyncedAt:  time.Now(),
		},
		ExternalKey: numericID,
		TaskType:    "subtask",
		Slug:        naming.Slugify(task.Title, 50),
	}
}

// fetchSubtasks recursively fetches all subtasks for a task (internal helper)
// Returns a list of subtask info and combined comments from all subtasks.
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
