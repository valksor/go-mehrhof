package bitbucket

import (
	"context"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// CreateWorkUnit implements the provider.WorkUnitCreator interface.
// It creates a new issue in Bitbucket.
func (p *Provider) CreateWorkUnit(ctx context.Context, opts provider.CreateWorkUnitOptions) (*provider.WorkUnit, error) {
	workspace := p.config.Workspace
	repoSlug := p.config.RepoSlug

	if workspace == "" || repoSlug == "" {
		return nil, ErrRepoNotConfigured
	}

	p.client.SetWorkspaceRepo(workspace, repoSlug)

	// Map priority to Bitbucket format
	priority := mapProviderPriorityToBitbucket(opts.Priority)

	// Determine kind based on labels (if any contain bug/feature hints)
	kind := "task"
	for _, label := range opts.Labels {
		switch label {
		case "bug", "fix":
			kind = "bug"
		case "feature", "enhancement":
			kind = "enhancement"
		}
	}

	// Create the issue
	issue, err := p.client.CreateIssue(ctx, opts.Title, opts.Description, priority, kind)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}

	// Build WorkUnit response
	description := ""
	if issue.Content != nil {
		description = issue.Content.Raw
	}

	webURL := ""
	if issue.Links.HTML != nil {
		webURL = issue.Links.HTML.Href
	}

	return &provider.WorkUnit{
		ID:          fmt.Sprintf("%d", issue.ID),
		ExternalID:  fmt.Sprintf("%s/%s#%d", workspace, repoSlug, issue.ID),
		Provider:    ProviderName,
		Title:       issue.Title,
		Description: description,
		Status:      mapBitbucketState(issue.State),
		Priority:    mapBitbucketPriority(issue.Priority),
		Labels:      []string{},
		Assignees:   mapAssignee(issue.Assignee),
		CreatedAt:   issue.CreatedOn,
		UpdatedAt:   issue.UpdatedOn,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: fmt.Sprintf("%s/%s#%d", workspace, repoSlug, issue.ID),
			SyncedAt:  time.Now(),
		},
		ExternalKey: fmt.Sprintf("%d", issue.ID),
		TaskType:    mapBitbucketKind(issue.Kind),
		Slug:        naming.Slugify(issue.Title, 50),
		Metadata: map[string]any{
			"web_url":        webURL,
			"workspace":      workspace,
			"repo_slug":      repoSlug,
			"issue_id":       issue.ID,
			"kind":           issue.Kind,
			"branch_pattern": p.config.BranchPattern,
			"commit_prefix":  p.config.CommitPrefix,
		},
	}, nil
}

// mapProviderPriorityToBitbucket converts provider.Priority to Bitbucket priority string.
func mapProviderPriorityToBitbucket(priority provider.Priority) string {
	switch priority {
	case provider.PriorityCritical:
		return "critical"
	case provider.PriorityHigh:
		return "major"
	case provider.PriorityNormal:
		return "minor"
	case provider.PriorityLow:
		return "trivial"
	}
	return "minor"
}
