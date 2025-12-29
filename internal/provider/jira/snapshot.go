package jira

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// Snapshot captures the issue content from Jira
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	// Update base URL if detected from reference
	if ref.BaseURL != "" && p.baseURL == "" {
		p.baseURL = ref.BaseURL
		p.client.SetBaseURL(ref.BaseURL)
	}

	// Fetch the issue
	issue, err := p.client.GetIssue(ctx, ref.IssueKey)
	if err != nil {
		return nil, err
	}

	// Fetch comments
	comments, _ := p.client.GetComments(ctx, ref.IssueKey)

	var content strings.Builder

	// Header
	content.WriteString(fmt.Sprintf("# %s\n\n", issue.Key))
	content.WriteString(fmt.Sprintf("## %s\n\n", issue.Fields.Summary))

	// Metadata
	content.WriteString("### Metadata\n\n")
	content.WriteString(fmt.Sprintf("- **Key:** %s\n", issue.Key))
	content.WriteString(fmt.Sprintf("- **Status:** %s\n", issue.Fields.Status.Name))
	if issue.Fields.Priority != nil {
		content.WriteString(fmt.Sprintf("- **Priority:** %s\n", issue.Fields.Priority.Name))
	}

	if issue.Fields.Project != nil {
		content.WriteString(fmt.Sprintf("- **Project:** %s (%s)\n", issue.Fields.Project.Name, issue.Fields.Project.Key))
	}

	if issue.Fields.Issuetype != nil {
		content.WriteString(fmt.Sprintf("- **Type:** %s\n", issue.Fields.Issuetype.Name))
	}

	if len(issue.Fields.Labels) > 0 {
		content.WriteString(fmt.Sprintf("- **Labels:** %s\n", strings.Join(issue.Fields.Labels, ", ")))
	}

	if issue.Fields.Assignee != nil {
		content.WriteString(fmt.Sprintf("- **Assignee:** %s\n", issue.Fields.Assignee.DisplayName))
	}

	if issue.Fields.Sprint != nil {
		content.WriteString(fmt.Sprintf("- **Sprint:** %s\n", issue.Fields.Sprint.Name))
	}

	content.WriteString(fmt.Sprintf("- **Created:** %s\n", issue.Fields.Created.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("- **Updated:** %s\n", issue.Fields.Updated.Format(time.RFC3339)))

	// Construct browse URL
	browseURL := ""
	if p.baseURL != "" {
		browseURL = fmt.Sprintf("%s/browse/%s", strings.TrimSuffix(p.baseURL, "/rest/api/3"), issue.Key)
	}
	if browseURL != "" {
		content.WriteString(fmt.Sprintf("- **URL:** %s\n\n", browseURL))
	} else {
		content.WriteString(fmt.Sprintf("- **URL:** %s\n\n", issue.Self))
	}

	// Description
	if issue.Fields.Description != "" {
		content.WriteString("## Description\n\n")
		content.WriteString(issue.Fields.Description)
		content.WriteString("\n\n")
	}

	// Comments
	if len(comments) > 0 {
		content.WriteString("## Comments\n\n")
		for _, c := range comments {
			authorName := "Unknown"
			if c.Author != nil {
				authorName = c.Author.DisplayName
			}
			content.WriteString(fmt.Sprintf("### %s - %s\n\n", authorName, c.Created.Format(time.RFC3339)))
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
