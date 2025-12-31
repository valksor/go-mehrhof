package github

import (
	"context"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// FetchSubtasks implements the provider.SubtaskFetcher interface.
// It parses task list items from the GitHub issue body.
func (p *Provider) FetchSubtasks(ctx context.Context, workUnitID string) ([]*provider.WorkUnit, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	// Determine owner/repo
	owner := ref.Owner
	repo := ref.Repo
	if owner == "" {
		owner = p.owner
	}
	if repo == "" {
		repo = p.repo
	}
	if owner == "" || repo == "" {
		return nil, ErrRepoNotConfigured
	}

	// Update client with correct owner/repo
	p.client.SetOwnerRepo(owner, repo)

	// Fetch the issue
	issue, err := p.client.GetIssue(ctx, ref.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}

	// Parse task list from issue body
	tasks := ParseTaskList(issue.GetBody())
	if len(tasks) == 0 {
		return nil, nil
	}

	// Convert to WorkUnits
	result := make([]*provider.WorkUnit, 0, len(tasks))
	for i, task := range tasks {
		// Determine status from completion state
		status := provider.StatusOpen
		if task.Completed {
			status = provider.StatusDone
		}

		// Create a unique ID for this task item
		taskID := fmt.Sprintf("%s/%s#%d-task-%d", owner, repo, ref.IssueNumber, i+1)

		wu := &provider.WorkUnit{
			ID:          taskID,
			ExternalID:  taskID,
			ExternalKey: fmt.Sprintf("%d-task-%d", ref.IssueNumber, i+1),
			Provider:    ProviderName,
			Title:       task.Text,
			Status:      status,
			Priority:    provider.PriorityNormal,
			TaskType:    "subtask",
			Slug:        naming.Slugify(task.Text, 50),
			CreatedAt:   issue.GetCreatedAt().Time,
			UpdatedAt:   issue.GetUpdatedAt().Time,
			Source: provider.SourceInfo{
				Type:      ProviderName,
				Reference: taskID,
				SyncedAt:  time.Now(),
			},
			Metadata: map[string]any{
				"parent_id":    workUnitID,
				"is_subtask":   true,
				"line_number":  task.Line,
				"completed":    task.Completed,
				"issue_number": ref.IssueNumber,
				"owner":        owner,
				"repo":         repo,
			},
		}

		result = append(result, wu)
	}

	return result, nil
}
