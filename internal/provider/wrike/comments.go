package wrike

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// FetchComments retrieves comments for a work unit
func (p *Provider) FetchComments(ctx context.Context, workUnitID string) ([]provider.Comment, error) {
	comments, err := p.client.GetComments(ctx, workUnitID)
	if err != nil {
		return nil, fmt.Errorf("fetch comments: %w", err)
	}

	result := make([]provider.Comment, 0, len(comments))
	for _, c := range comments {
		author := provider.Person{
			ID:   c.AuthorID,
			Name: c.AuthorName,
		}
		result = append(result, provider.Comment{
			ID:        c.ID,
			Author:    author,
			Body:      c.Text,
			CreatedAt: c.CreatedDate,
			UpdatedAt: c.UpdatedDate,
		})
	}

	return result, nil
}

// AddComment adds a comment to a work unit
func (p *Provider) AddComment(ctx context.Context, workUnitID, body string) (*provider.Comment, error) {
	comment, err := p.client.PostComment(ctx, workUnitID, body)
	if err != nil {
		return nil, fmt.Errorf("post comment: %w", err)
	}

	author := provider.Person{
		ID:   comment.AuthorID,
		Name: comment.AuthorName,
	}

	return &provider.Comment{
		ID:        comment.ID,
		Author:    author,
		Body:      comment.Text,
		CreatedAt: comment.CreatedDate,
		UpdatedAt: comment.UpdatedDate,
	}, nil
}
