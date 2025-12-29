package linear

import (
	"context"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// AddComment adds a comment to a Linear issue
func (p *Provider) AddComment(ctx context.Context, workUnitID string, body string) (*provider.Comment, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, err
	}

	// First, fetch the issue to get its internal ID
	issue, err := p.client.GetIssue(ctx, ref.IssueID)
	if err != nil {
		return nil, err
	}

	// Add the comment
	comment, err := p.client.AddComment(ctx, issue.ID, body)
	if err != nil {
		return nil, err
	}

	// Convert to provider Comment
	var author provider.Person
	if comment.User != nil {
		author = provider.Person{
			ID:   comment.User.ID,
			Name: comment.User.Name,
		}
	}

	return &provider.Comment{
		ID:        comment.ID,
		Body:      comment.Body,
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
		Author:    author,
	}, nil
}

// FetchComments retrieves comments for a Linear issue
func (p *Provider) FetchComments(ctx context.Context, workUnitID string) ([]provider.Comment, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, err
	}

	// First, fetch the issue to get its internal ID
	issue, err := p.client.GetIssue(ctx, ref.IssueID)
	if err != nil {
		return nil, err
	}

	// Get comments
	comments, err := p.client.GetComments(ctx, issue.ID)
	if err != nil {
		return nil, err
	}

	return mapComments(comments), nil
}
