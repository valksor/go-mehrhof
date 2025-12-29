package jira

import (
	"context"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// CreateWorkUnit creates a new Jira issue
func (p *Provider) CreateWorkUnit(ctx context.Context, opts provider.CreateWorkUnitOptions) (*provider.WorkUnit, error) {
	// Determine project key
	projectKey := p.defaultProject
	if projectKey == "" {
		// Try to extract from parent ID (e.g., "PROJ-123")
		if opts.ParentID != "" {
			ref, err := ParseReference(opts.ParentID)
			if err == nil {
				projectKey = ref.ProjectKey
			}
		}
	}

	if projectKey == "" {
		return nil, fmt.Errorf("%w: specify jira.project in config or provide parent issue", ErrProjectRequired)
	}

	// Build the create input
	input := CreateIssueInput{}
	input.Fields.Project = &Project{
		Key: projectKey,
	}
	input.Fields.Summary = opts.Title
	input.Fields.Description = opts.Description

	// Set issue type - default to "Task" or use custom field
	issueType := "Task"
	if opts.CustomFields != nil {
		if t, ok := opts.CustomFields["issue_type"].(string); ok {
			issueType = t
		}
	}
	input.Fields.IssueType = &IssueType{
		Name: issueType,
	}

	// Set priority if specified
	if opts.Priority != provider.PriorityNormal {
		priorityName := mapProviderPriorityToJira(opts.Priority)
		input.Fields.Priority = &Priority{
			Name: priorityName,
		}
	}

	// Set labels
	if len(opts.Labels) > 0 {
		input.Fields.Labels = opts.Labels
	}

	// Set assignee (first one)
	if len(opts.Assignees) > 0 {
		input.Fields.Assignee = &User{
			AccountID: opts.Assignees[0],
		}
	}

	// Create the issue
	issue, err := p.client.CreateIssue(ctx, input)
	if err != nil {
		return nil, err
	}

	// Convert to WorkUnit
	wu := &provider.WorkUnit{
		ID:          issue.ID,
		ExternalID:  issue.Key,
		Provider:    ProviderName,
		Title:       issue.Fields.Summary,
		Description: issue.Fields.Description,
		Status:      mapJiraStatus(issue.Fields.Status.Name),
		Priority:    mapJiraPriority(issue.Fields.Priority),
		Labels:      issue.Fields.Labels,
		Assignees:   mapAssignees(issue.Fields.Assignee),
		CreatedAt:   issue.Fields.Created,
		UpdatedAt:   issue.Fields.Updated,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: issue.Key,
			SyncedAt:  time.Now(),
		},
		ExternalKey: issue.Key,
		TaskType:    inferTaskTypeFromLabels(opts.Labels),
		Slug:        naming.Slugify(issue.Fields.Summary, 50),
		Metadata:    buildMetadata(issue),
	}

	return wu, nil
}
