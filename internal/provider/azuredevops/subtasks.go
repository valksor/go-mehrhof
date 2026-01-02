package azuredevops

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// FetchSubtasks implements the provider.SubtaskFetcher interface.
// It retrieves child work items for a given work item.
func (p *Provider) FetchSubtasks(ctx context.Context, workUnitID string) ([]*provider.WorkUnit, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	// Override org/project if specified in reference
	if ref.Organization != "" && ref.Project != "" {
		p.client.SetOrganization(ref.Organization)
		p.client.SetProject(ref.Project)
	}

	// Query for child work items using WIQL
	wiql := fmt.Sprintf(`
		SELECT [System.Id], [System.Title], [System.State]
		FROM WorkItemLinks
		WHERE (
			[Source].[System.Id] = %d
			AND [System.Links.LinkType] = 'System.LinkTypes.Hierarchy-Forward'
		)
		MODE (MustContain)
	`, ref.WorkItemID)

	// Execute tree query
	childIDs, err := p.client.QueryWorkItemLinks(ctx, wiql)
	if err != nil {
		// If tree query fails, try to get from relations
		return p.fetchSubtasksFromRelations(ctx, ref.WorkItemID)
	}

	if len(childIDs) == 0 {
		return nil, nil
	}

	// Fetch full work items
	workItems, err := p.client.GetWorkItems(ctx, childIDs)
	if err != nil {
		return nil, fmt.Errorf("get child work items: %w", err)
	}

	// Convert to WorkUnits
	result := make([]*provider.WorkUnit, 0, len(workItems))
	for _, wi := range workItems {
		wu := p.workItemToWorkUnit(&wi)
		// Add parent reference
		wu.Metadata["parent_id"] = workUnitID
		wu.Metadata["is_subtask"] = true
		wu.TaskType = "subtask"
		result = append(result, wu)
	}

	return result, nil
}

// fetchSubtasksFromRelations fetches child work items from the relations array.
func (p *Provider) fetchSubtasksFromRelations(ctx context.Context, workItemID int) ([]*provider.WorkUnit, error) {
	// Get work item with relations
	workItem, err := p.client.GetWorkItem(ctx, workItemID)
	if err != nil {
		return nil, fmt.Errorf("get work item: %w", err)
	}

	// Extract child IDs from relations
	var childIDs []int
	for _, rel := range workItem.Relations {
		if rel.Rel == "System.LinkTypes.Hierarchy-Forward" {
			// Extract ID from URL
			// URL format: https://dev.azure.com/{org}/{project}/_apis/wit/workItems/{id}
			parts := strings.Split(rel.URL, "/")
			if len(parts) > 0 {
				idStr := parts[len(parts)-1]
				if id, err := strconv.Atoi(idStr); err == nil {
					childIDs = append(childIDs, id)
				}
			}
		}
	}

	if len(childIDs) == 0 {
		return nil, nil
	}

	// Fetch full work items
	workItems, err := p.client.GetWorkItems(ctx, childIDs)
	if err != nil {
		return nil, fmt.Errorf("get child work items: %w", err)
	}

	// Convert to WorkUnits
	parentID := strconv.Itoa(workItemID)
	result := make([]*provider.WorkUnit, 0, len(workItems))
	for _, wi := range workItems {
		wu := p.workItemToWorkUnit(&wi)
		wu.Metadata["parent_id"] = parentID
		wu.Metadata["is_subtask"] = true
		wu.TaskType = "subtask"
		result = append(result, wu)
	}

	return result, nil
}
