package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v67/github"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// CreateWorkUnit creates a new GitHub issue
func (p *Provider) CreateWorkUnit(ctx context.Context, opts provider.CreateWorkUnitOptions) (*provider.WorkUnit, error) {
	owner := p.owner
	repo := p.repo

	if owner == "" || repo == "" {
		return nil, ErrRepoNotConfigured
	}

	p.client.SetOwnerRepo(owner, repo)

	// Build GitHub issue request
	issueReq := &github.IssueRequest{
		Title:  &opts.Title,
		Body:   &opts.Description,
		Labels: &opts.Labels,
	}

	// Add assignees if specified
	if len(opts.Assignees) > 0 {
		issueReq.Assignees = &opts.Assignees
	}

	// Create the issue
	issue, _, err := p.client.CreateIssue(ctx, issueReq)
	if err != nil {
		return nil, err
	}

	// Convert to WorkUnit
	wu := &provider.WorkUnit{
		ID:          fmt.Sprintf("%d", issue.GetNumber()),
		ExternalID:  fmt.Sprintf("%s/%s#%d", owner, repo, issue.GetNumber()),
		Provider:    ProviderName,
		Title:       issue.GetTitle(),
		Description: issue.GetBody(),
		Status:      mapGitHubState(issue.GetState()),
		Priority:    opts.Priority,
		Labels:      opts.Labels,
		Assignees:   mapGitHubAssignees(opts.Assignees),
		CreatedAt:   issue.GetCreatedAt().Time,
		UpdatedAt:   issue.GetUpdatedAt().Time,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: fmt.Sprintf("%s/%s#%d", owner, repo, issue.GetNumber()),
			SyncedAt:  time.Now(),
		},
		ExternalKey: fmt.Sprintf("%d", issue.GetNumber()),
		TaskType:    inferTaskTypeFromLabels(opts.Labels),
		Slug:        naming.Slugify(opts.Title, 50),
		Metadata: map[string]any{
			"html_url":     issue.GetHTMLURL(),
			"owner":        owner,
			"repo":         repo,
			"issue_number": issue.GetNumber(),
		},
	}

	return wu, nil
}

// CreateIssue wraps the GitHub API call for creating an issue
func (c *Client) CreateIssue(ctx context.Context, issue *github.IssueRequest) (*github.Issue, *github.Response, error) {
	return c.gh.Issues.Create(ctx, c.owner, c.repo, issue)
}

// mapGitHubAssignees converts assignee usernames to Person structs
func mapGitHubAssignees(assignees []string) []provider.Person {
	persons := make([]provider.Person, len(assignees))
	for i, a := range assignees {
		persons[i] = provider.Person{
			Name: a,
		}
	}
	return persons
}

// inferTaskTypeFromLabels determines task type from label names
func inferTaskTypeFromLabels(labels []string) string {
	for _, label := range labels {
		name := lower(label)
		if t, ok := labelTypeMap[name]; ok {
			return t
		}
	}
	return "issue"
}

// lower is a helper for lowercase conversion
func lower(s string) string {
	// Simple lowercase implementation
	var result []rune
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}
