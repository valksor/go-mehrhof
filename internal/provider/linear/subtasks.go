package linear

import (
	"context"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// FetchSubtasks implements the provider.SubtaskFetcher interface.
// It retrieves child issues for a given Linear issue.
func (p *Provider) FetchSubtasks(ctx context.Context, workUnitID string) ([]*provider.WorkUnit, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	// Get child issues via GraphQL
	children, err := p.client.GetChildIssues(ctx, ref.IssueID)
	if err != nil {
		return nil, fmt.Errorf("get child issues: %w", err)
	}

	if len(children) == 0 {
		return nil, nil
	}

	// Convert to WorkUnits
	result := make([]*provider.WorkUnit, 0, len(children))
	for _, issue := range children {
		wu := &provider.WorkUnit{
			ID:          issue.ID,
			ExternalID:  issue.Identifier,
			ExternalKey: issue.Identifier,
			Provider:    ProviderName,
			Title:       issue.Title,
			Description: issue.Description,
			Status:      mapLinearStatus(issue.State),
			Priority:    mapLinearPriority(issue.Priority),
			Labels:      extractLabelNames(issue.Labels),
			Assignees:   mapAssignees(issue.Assignee),
			CreatedAt:   issue.CreatedAt,
			UpdatedAt:   issue.UpdatedAt,
			TaskType:    "subtask",
			Slug:        naming.Slugify(issue.Title, 50),
			Source: provider.SourceInfo{
				Type:      ProviderName,
				Reference: issue.Identifier,
				SyncedAt:  time.Now(),
			},
			Metadata: map[string]any{
				"parent_id":  workUnitID,
				"is_subtask": true,
				"url":        issue.URL,
			},
		}

		if issue.State != nil {
			wu.Metadata["state_id"] = issue.State.ID
			wu.Metadata["state_name"] = issue.State.Name
		}

		if issue.Team != nil {
			wu.Metadata["team_key"] = issue.Team.Key
		}

		result = append(result, wu)
	}

	return result, nil
}
