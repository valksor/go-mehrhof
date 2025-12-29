package github

import (
	"context"

	"github.com/google/go-github/v67/github"
)

// AddLabels adds labels to a GitHub issue
func (p *Provider) AddLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}

	owner := ref.Owner
	repo := ref.Repo
	if owner == "" {
		owner = p.owner
	}
	if repo == "" {
		repo = p.repo
	}

	if owner == "" || repo == "" {
		return ErrRepoNotConfigured
	}

	p.client.SetOwnerRepo(owner, repo)

	_, _, err = p.client.AddLabelsToIssue(ctx, ref.IssueNumber, labels)
	return err
}

// RemoveLabels removes labels from a GitHub issue
func (p *Provider) RemoveLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}

	owner := ref.Owner
	repo := ref.Repo
	if owner == "" {
		owner = p.owner
	}
	if repo == "" {
		repo = p.repo
	}

	if owner == "" || repo == "" {
		return ErrRepoNotConfigured
	}

	p.client.SetOwnerRepo(owner, repo)

	// Remove each label individually
	for _, label := range labels {
		_, err := p.client.RemoveLabelForIssue(ctx, ref.IssueNumber, label)
		if err != nil {
			// Continue trying to remove other labels even if one fails
			continue
		}
	}

	return nil
}

// AddLabelsToIssue wraps the GitHub API call for adding labels
func (c *Client) AddLabelsToIssue(ctx context.Context, number int, labels []string) ([]*github.Label, *github.Response, error) {
	return c.gh.Issues.AddLabelsToIssue(ctx, c.owner, c.repo, number, labels)
}

// RemoveLabelForIssue wraps the GitHub API call for removing a label
func (c *Client) RemoveLabelForIssue(ctx context.Context, number int, label string) (*github.Response, error) {
	return c.gh.Issues.RemoveLabelForIssue(ctx, c.owner, c.repo, number, label)
}
