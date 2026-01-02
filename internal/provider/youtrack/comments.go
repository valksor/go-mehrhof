package youtrack

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// FetchComments retrieves comments for an issue.
func (p *Provider) FetchComments(ctx context.Context, workUnitID string) ([]provider.Comment, error) {
	comments, err := p.client.GetComments(ctx, workUnitID)
	if err != nil {
		return nil, fmt.Errorf("fetch comments: %w", err)
	}

	result := make([]provider.Comment, 0, len(comments))
	for _, c := range comments {
		if c.Deleted {
			continue
		}
		result = append(result, provider.Comment{
			ID:        c.ID,
			Author:    provider.Person{ID: c.Author.ID, Name: c.Author.FullName},
			Body:      c.Text,
			CreatedAt: timeFromMillis(c.Created),
			UpdatedAt: timeFromMillis(c.Updated),
		})
	}

	return result, nil
}

// AddComment adds a comment to an issue.
func (p *Provider) AddComment(ctx context.Context, workUnitID string, body string) (*provider.Comment, error) {
	comment, err := p.client.AddComment(ctx, workUnitID, body)
	if err != nil {
		return nil, fmt.Errorf("add comment: %w", err)
	}

	return &provider.Comment{
		ID:        comment.ID,
		Author:    provider.Person{ID: comment.Author.ID, Name: comment.Author.FullName},
		Body:      comment.Text,
		CreatedAt: timeFromMillis(comment.Created),
	}, nil
}
