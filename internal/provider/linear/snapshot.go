package linear

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// Snapshot captures the issue content from Linear
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	// Fetch the issue
	issue, err := p.client.GetIssue(ctx, ref.IssueID)
	if err != nil {
		return nil, err
	}

	// Fetch comments
	comments, _ := p.client.GetComments(ctx, issue.ID)

	var content strings.Builder

	// Header
	content.WriteString(fmt.Sprintf("# %s\n\n", issue.Identifier))
	content.WriteString(fmt.Sprintf("## %s\n\n", issue.Title))

	// Metadata
	content.WriteString("### Metadata\n\n")
	content.WriteString(fmt.Sprintf("- **Identifier:** %s\n", issue.Identifier))
	content.WriteString(fmt.Sprintf("- **Status:** %s\n", issue.State.Name))
	content.WriteString(fmt.Sprintf("- **Priority:** %s\n", priorityLabel(issue.Priority)))

	if issue.Team != nil {
		content.WriteString(fmt.Sprintf("- **Team:** %s (%s)\n", issue.Team.Name, issue.Team.Key))
	}

	if len(issue.Labels) > 0 {
		labelNames := make([]string, len(issue.Labels))
		for i, label := range issue.Labels {
			labelNames[i] = label.Name
		}
		content.WriteString(fmt.Sprintf("- **Labels:** %s\n", strings.Join(labelNames, ", ")))
	}

	if issue.Assignee != nil {
		content.WriteString(fmt.Sprintf("- **Assignee:** %s\n", issue.Assignee.Name))
	}

	content.WriteString(fmt.Sprintf("- **Created:** %s\n", issue.CreatedAt.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("- **Updated:** %s\n", issue.UpdatedAt.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("- **URL:** %s\n\n", issue.URL))

	// Description
	if issue.Description != "" {
		content.WriteString("## Description\n\n")
		content.WriteString(issue.Description)
		content.WriteString("\n\n")
	}

	// Comments
	if len(comments) > 0 {
		content.WriteString("## Comments\n\n")
		for _, c := range comments {
			authorName := "Unknown"
			if c.User != nil {
				authorName = c.User.Name
			}
			content.WriteString(fmt.Sprintf("### %s - %s\n\n", authorName, c.CreatedAt.Format(time.RFC3339)))
			content.WriteString(c.Body)
			content.WriteString("\n\n---\n\n")
		}
	}

	return &provider.Snapshot{
		Type:    ProviderName,
		Ref:     id,
		Content: content.String(),
	}, nil
}

// priorityLabel converts Linear priority number to label
func priorityLabel(priority int) string {
	switch priority {
	case 1:
		return "Urgent"
	case 2:
		return "High"
	case 3:
		return "Medium"
	case 4:
		return "Low"
	default:
		return "No priority"
	}
}
