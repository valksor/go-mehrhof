package jira

import (
	"context"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// AddComment adds a comment to a Jira issue
func (p *Provider) AddComment(ctx context.Context, workUnitID string, body string) (*provider.Comment, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, err
	}

	// Update base URL if detected from reference
	if ref.BaseURL != "" && p.baseURL == "" {
		p.baseURL = ref.BaseURL
		p.client.SetBaseURL(ref.BaseURL)
	}

	// Add the comment
	comment, err := p.client.AddComment(ctx, ref.IssueKey, body)
	if err != nil {
		return nil, err
	}

	// Convert to provider Comment
	var author provider.Person
	if comment.Author != nil {
		author = provider.Person{
			ID:   comment.Author.AccountID,
			Name: comment.Author.DisplayName,
		}
	}

	return &provider.Comment{
		ID:        comment.ID,
		Body:      comment.Body,
		CreatedAt: comment.Created,
		UpdatedAt: comment.Updated,
		Author:    author,
	}, nil
}

// FetchComments retrieves comments for a Jira issue
func (p *Provider) FetchComments(ctx context.Context, workUnitID string) ([]provider.Comment, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, err
	}

	// Update base URL if detected from reference
	if ref.BaseURL != "" && p.baseURL == "" {
		p.baseURL = ref.BaseURL
		p.client.SetBaseURL(ref.BaseURL)
	}

	// Get comments
	comments, err := p.client.GetComments(ctx, ref.IssueKey)
	if err != nil {
		return nil, err
	}

	return mapComments(comments), nil
}
