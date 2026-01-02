package bitbucket

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// taskListPattern matches GitHub/GitLab-style task list items
// Matches: - [ ] Task item or - [x] Completed task.
var taskListPattern = regexp.MustCompile(`(?m)^[\s]*[-*]\s*\[([ xX])\]\s*(.+)$`)

// FetchSubtasks implements the provider.SubtaskFetcher interface.
// For Bitbucket, this parses task list items (- [ ] item) from the issue body.
// Note: Bitbucket doesn't have native subtasks, so we parse markdown task lists.
func (p *Provider) FetchSubtasks(ctx context.Context, workUnitID string) ([]*provider.WorkUnit, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, err
	}

	workspace := ref.Workspace
	repoSlug := ref.RepoSlug
	if workspace == "" {
		workspace = p.config.Workspace
	}
	if repoSlug == "" {
		repoSlug = p.config.RepoSlug
	}

	if workspace == "" || repoSlug == "" {
		return nil, ErrRepoNotConfigured
	}

	p.client.SetWorkspaceRepo(workspace, repoSlug)

	// Fetch the issue
	issue, err := p.client.GetIssue(ctx, ref.IssueID)
	if err != nil {
		return nil, fmt.Errorf("fetch issue: %w", err)
	}

	// Parse task list from body
	if issue.Content == nil || issue.Content.Raw == "" {
		return nil, nil
	}

	return parseTaskListToWorkUnits(issue.Content.Raw, workUnitID), nil
}

// parseTaskListToWorkUnits extracts task list items from markdown and converts them to WorkUnits.
func parseTaskListToWorkUnits(body, parentID string) []*provider.WorkUnit {
	matches := taskListPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	workUnits := make([]*provider.WorkUnit, 0, len(matches))
	for i, match := range matches {
		if len(match) < 3 {
			continue
		}

		completed := strings.ToLower(match[1]) == "x"
		title := strings.TrimSpace(match[2])

		status := provider.StatusOpen
		if completed {
			status = provider.StatusDone
		}

		// Generate a synthetic ID for the subtask
		subtaskID := fmt.Sprintf("%s:task-%d", parentID, i)

		wu := &provider.WorkUnit{
			ID:          subtaskID,
			ExternalID:  subtaskID,
			Provider:    ProviderName,
			Title:       title,
			Description: "",
			Status:      status,
			Priority:    provider.PriorityNormal,
			Labels:      []string{},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Source: provider.SourceInfo{
				Type:      ProviderName,
				Reference: parentID,
				SyncedAt:  time.Now(),
			},
			ExternalKey: fmt.Sprintf("task-%d", i),
			TaskType:    "subtask",
			Slug:        naming.Slugify(title, 50),
			Metadata: map[string]any{
				"parent_id":   parentID,
				"is_subtask":  true,
				"task_index":  i,
				"completed":   completed,
				"parsed_from": "task_list",
			},
		}

		workUnits = append(workUnits, wu)
	}

	return workUnits
}
