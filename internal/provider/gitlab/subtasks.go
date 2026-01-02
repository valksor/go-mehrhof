package gitlab

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// FetchSubtasks implements the provider.SubtaskFetcher interface.
// It parses task list items from the GitLab issue description.
func (p *Provider) FetchSubtasks(ctx context.Context, workUnitID string) ([]*provider.WorkUnit, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	// Determine project
	projectPath := ref.ProjectPath
	if projectPath == "" {
		projectPath = p.projectPath
		if projectPath == "" {
			projectPath = p.config.ProjectPath
		}
	}

	// Set up client with correct project
	if ref.ProjectID > 0 {
		p.client.SetProjectID(ref.ProjectID)
	} else if projectPath != "" {
		p.client.SetProjectPath(projectPath)
	} else {
		return nil, ErrProjectNotConfigured
	}

	// Fetch the issue
	issue, err := p.client.GetIssue(ctx, ref.IssueIID)
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}

	// Parse task list from issue description
	tasks := ParseTaskList(issue.Description)
	if len(tasks) == 0 {
		return nil, nil
	}

	// Determine display project path
	displayProject := projectPath
	if displayProject == "" && ref.ProjectID > 0 {
		displayProject = strconv.FormatInt(ref.ProjectID, 10)
	} else if displayProject == "" && issue.ProjectID != 0 {
		displayProject = strconv.FormatInt(issue.ProjectID, 10)
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
		taskID := fmt.Sprintf("%s#%d-task-%d", displayProject, ref.IssueIID, i+1)

		var createdAt, updatedAt time.Time
		if issue.CreatedAt != nil {
			createdAt = *issue.CreatedAt
		}
		if issue.UpdatedAt != nil {
			updatedAt = *issue.UpdatedAt
		}

		wu := &provider.WorkUnit{
			ID:          taskID,
			ExternalID:  taskID,
			ExternalKey: fmt.Sprintf("%d-task-%d", ref.IssueIID, i+1),
			Provider:    ProviderName,
			Title:       task.Text,
			Status:      status,
			Priority:    provider.PriorityNormal,
			TaskType:    "subtask",
			Slug:        naming.Slugify(task.Text, 50),
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
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
				"issue_iid":    ref.IssueIID,
				"project_path": displayProject,
				"project_id":   issue.ProjectID,
			},
		}

		result = append(result, wu)
	}

	return result, nil
}
