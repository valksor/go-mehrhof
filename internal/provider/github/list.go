package github

import (
	"context"
	"fmt"
	"time"

	gh "github.com/google/go-github/v67/github"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// List retrieves issues from the repository.
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	owner := p.owner
	repo := p.repo

	if owner == "" || repo == "" {
		return nil, ErrRepoNotConfigured
	}

	p.client.SetOwnerRepo(owner, repo)

	// Build GitHub API options
	ghOpts := &gh.IssueListByRepoOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	// Map status filter
	if opts.Status != "" {
		switch opts.Status {
		case provider.StatusOpen:
			ghOpts.State = "open"
		case provider.StatusClosed, provider.StatusDone:
			ghOpts.State = "closed"
		case provider.StatusInProgress, provider.StatusReview:
			// GitHub doesn't have these states, use all
			ghOpts.State = "all"
		}
	} else {
		ghOpts.State = "open" // Default to open issues
	}

	// Map labels filter
	if len(opts.Labels) > 0 {
		ghOpts.Labels = opts.Labels
	}

	// Pagination support
	var allIssues []*gh.Issue
	for {
		issues, resp, err := p.client.ListIssuesByRepository(ctx, ghOpts)
		if err != nil {
			return nil, err
		}
		allIssues = append(allIssues, issues...)

		// Apply limit if specified
		if opts.Limit > 0 && len(allIssues) >= opts.Limit {
			allIssues = allIssues[:opts.Limit]
			break
		}

		if resp.NextPage == 0 {
			break
		}
		ghOpts.Page = resp.NextPage
	}

	// Apply offset if specified
	if opts.Offset > 0 && opts.Offset < len(allIssues) {
		allIssues = allIssues[opts.Offset:]
	} else if opts.Offset > 0 {
		allIssues = []*gh.Issue{}
	}

	// Convert to WorkUnits
	result := make([]*provider.WorkUnit, 0, len(allIssues))
	for _, issue := range allIssues {
		// Skip pull requests from issues list
		if issue.IsPullRequest() {
			continue
		}

		wu := &provider.WorkUnit{
			ID:          fmt.Sprintf("%d", issue.GetNumber()),
			ExternalID:  fmt.Sprintf("%s/%s#%d", owner, repo, issue.GetNumber()),
			Provider:    ProviderName,
			Title:       issue.GetTitle(),
			Description: issue.GetBody(),
			Status:      mapGitHubState(issue.GetState()),
			Priority:    inferPriorityFromLabels(issue.Labels),
			Labels:      extractLabelNames(issue.Labels),
			Assignees:   mapAssignees(issue.Assignees),
			CreatedAt:   issue.GetCreatedAt().Time,
			UpdatedAt:   issue.GetUpdatedAt().Time,
			Source: provider.SourceInfo{
				Type:      ProviderName,
				Reference: fmt.Sprintf("%s/%s#%d", owner, repo, issue.GetNumber()),
				SyncedAt:  time.Now(),
			},
			ExternalKey: fmt.Sprintf("%d", issue.GetNumber()),
			TaskType:    inferTypeFromLabels(issue.Labels),
			Slug:        naming.Slugify(issue.GetTitle(), 50),
			Metadata: map[string]any{
				"html_url":     issue.GetHTMLURL(),
				"owner":        owner,
				"repo":         repo,
				"issue_number": issue.GetNumber(),
			},
		}
		result = append(result, wu)
	}

	return result, nil
}

// ListIssuesByRepository wraps the GitHub API call for listing issues.
func (c *Client) ListIssuesByRepository(ctx context.Context, opts *gh.IssueListByRepoOptions) ([]*gh.Issue, *gh.Response, error) {
	return c.gh.Issues.ListByRepo(ctx, c.owner, c.repo, opts)
}
