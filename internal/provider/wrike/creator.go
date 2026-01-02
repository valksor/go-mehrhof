package wrike

import (
	"context"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// CreateWorkUnit implements the provider.WorkUnitCreator interface.
// It creates a new task in Wrike.
// Note: Wrike requires a folder ID to create tasks.
// Configure via "folder_id" or use ParentID in options.
func (p *Provider) CreateWorkUnit(ctx context.Context, opts provider.CreateWorkUnitOptions) (*provider.WorkUnit, error) {
	// Determine target folder
	folderID := opts.ParentID
	if folderID == "" {
		folderID = p.client.folderID
	}
	if folderID == "" {
		return nil, fmt.Errorf("wrike: CreateWorkUnit requires folder_id or ParentID")
	}

	// Build create options
	createOpts := CreateTaskOptions{
		Title:       opts.Title,
		Description: opts.Description,
		Priority:    mapProviderPriorityToWrike(opts.Priority),
	}

	// Create the task
	task, err := p.client.CreateTask(ctx, folderID, createOpts)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	// Extract numeric ID from permalink
	numericID := ExtractNumericID(task.Permalink)
	if numericID == "" {
		numericID = task.ID
	}

	// Build WorkUnit response
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
		Metadata: map[string]any{
			"permalink":      task.Permalink,
			"api_id":         task.ID,
			"wrike_status":   task.Status,
			"wrike_priority": task.Priority,
			"folder_id":      folderID,
		},
		CreatedAt: task.CreatedDate,
		UpdatedAt: task.UpdatedDate,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: task.Permalink,
			SyncedAt:  time.Now(),
		},
		ExternalKey: numericID,
		TaskType:    "task",
		Slug:        naming.Slugify(task.Title, 50),
	}, nil
}

// mapProviderPriorityToWrike converts a provider.Priority to a Wrike priority string.
func mapProviderPriorityToWrike(priority provider.Priority) string {
	switch priority {
	case provider.PriorityCritical:
		return "High"
	case provider.PriorityHigh:
		return "High"
	case provider.PriorityNormal:
		return "Normal"
	case provider.PriorityLow:
		return "Low"
	}
	return "Normal"
}
