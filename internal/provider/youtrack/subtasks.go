package youtrack

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// FetchSubtasks implements the provider.SubtaskFetcher interface.
// It retrieves subtasks for a given YouTrack issue.
func (p *Provider) FetchSubtasks(ctx context.Context, workUnitID string) ([]*provider.WorkUnit, error) {
	// First, get the parent issue to get subtask links
	issue, err := p.client.GetIssue(ctx, workUnitID)
	if err != nil {
		return nil, fmt.Errorf("get parent issue: %w", err)
	}

	if len(issue.Subtasks) == 0 {
		return nil, nil
	}

	// Fetch each subtask as a full issue
	result := make([]*provider.WorkUnit, 0, len(issue.Subtasks))
	for _, subtaskLink := range issue.Subtasks {
		// Fetch the subtask issue by its readable ID
		subtaskIssue, err := p.client.GetIssue(ctx, subtaskLink.IDReadable)
		if err != nil {
			// Continue with other subtasks if one fails
			continue
		}

		// Fetch comments and attachments for the subtask
		comments, _ := p.client.GetComments(ctx, subtaskLink.IDReadable)
		attachments, _ := p.client.GetAttachments(ctx, subtaskLink.IDReadable)

		// Convert to WorkUnit using the existing method
		wu := p.issueToWorkUnit(subtaskIssue, comments, attachments)

		// Override task type to indicate it's a subtask
		wu.TaskType = "subtask"

		// Add parent reference to metadata
		wu.Metadata["parent_id"] = workUnitID
		wu.Metadata["is_subtask"] = true

		result = append(result, wu)
	}

	return result, nil
}
