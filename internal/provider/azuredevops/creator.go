package azuredevops

import (
	"context"
	"fmt"
	"strings"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// CreateWorkUnit implements the provider.WorkUnitCreator interface.
// It creates a new work item in Azure DevOps.
func (p *Provider) CreateWorkUnit(ctx context.Context, opts provider.CreateWorkUnitOptions) (*provider.WorkUnit, error) {
	// Determine work item type - default to Task
	workItemType := "Task"
	if opts.CustomFields != nil {
		if t, ok := opts.CustomFields["work_item_type"].(string); ok {
			workItemType = t
		}
	}

	// Build patch operations
	updates := []PatchOperation{
		{
			Op:    "add",
			Path:  "/fields/System.Title",
			Value: opts.Title,
		},
	}

	if opts.Description != "" {
		updates = append(updates, PatchOperation{
			Op:    "add",
			Path:  "/fields/System.Description",
			Value: opts.Description,
		})
	}

	// Add tags from labels
	if len(opts.Labels) > 0 {
		updates = append(updates, PatchOperation{
			Op:    "add",
			Path:  "/fields/System.Tags",
			Value: strings.Join(opts.Labels, "; "),
		})
	}

	// Set priority
	if opts.Priority != provider.PriorityNormal {
		azPriority := mapProviderPriorityToAzure(opts.Priority)
		updates = append(updates, PatchOperation{
			Op:    "add",
			Path:  "/fields/Microsoft.VSTS.Common.Priority",
			Value: azPriority,
		})
	}

	// Set area path if configured
	if p.config.AreaPath != "" {
		updates = append(updates, PatchOperation{
			Op:    "add",
			Path:  "/fields/System.AreaPath",
			Value: p.config.AreaPath,
		})
	}

	// Set iteration path if configured
	if p.config.IterationPath != "" {
		updates = append(updates, PatchOperation{
			Op:    "add",
			Path:  "/fields/System.IterationPath",
			Value: p.config.IterationPath,
		})
	}

	// Link to parent if specified
	if opts.ParentID != "" {
		parentRef, err := ParseReference(opts.ParentID)
		if err == nil {
			updates = append(updates, PatchOperation{
				Op:   "add",
				Path: "/relations/-",
				Value: map[string]any{
					"rel": "System.LinkTypes.Hierarchy-Reverse",
					"url": fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/wit/workitems/%d",
						p.config.Organization, p.config.Project, parentRef.WorkItemID),
					"attributes": map[string]any{
						"comment": "Created via Mehrhof",
					},
				},
			})
		}
	}

	// Create work item
	workItem, err := p.client.CreateWorkItem(ctx, workItemType, updates)
	if err != nil {
		return nil, fmt.Errorf("create work item: %w", err)
	}

	return p.workItemToWorkUnit(workItem), nil
}

// mapProviderPriorityToAzure converts provider.Priority to Azure DevOps priority (1-4).
func mapProviderPriorityToAzure(priority provider.Priority) int {
	switch priority {
	case provider.PriorityCritical:
		return 1
	case provider.PriorityHigh:
		return 2
	case provider.PriorityNormal:
		return 3
	case provider.PriorityLow:
		return 4
	default:
		return 3
	}
}
