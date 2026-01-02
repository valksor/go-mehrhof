package notion

import (
	"context"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// AddComment adds a comment to a Notion page.
func (p *Provider) AddComment(ctx context.Context, workUnitID, body string) (string, error) {
	input := &AddCommentInput{
		Parent: CommentParent{
			BlockID: workUnitID,
		},
		RichText: []RichText{
			{
				Type: "text",
				Text: &TextContent{
					Content: body,
				},
				PlainText: body,
			},
		},
	}

	comment, err := p.client.AddComment(ctx, input)
	if err != nil {
		return "", err
	}

	return comment.ID, nil
}

// FetchComments retrieves comments for a Notion page.
func (p *Provider) FetchComments(ctx context.Context, id string) ([]provider.Comment, error) {
	comments, err := p.client.GetComments(ctx, id)
	if err != nil {
		return nil, err
	}

	result := make([]provider.Comment, 0, len(comments))
	for _, c := range comments {
		// Extract comment text from rich text
		body := ""
		for _, rt := range c.RichText {
			body += rt.PlainText
		}

		// Extract author info from created_by (which is a RichText in Notion's API)
		var author provider.Person
		if c.CreatedBy.Type == "person" && len(c.RichText) > 0 {
			// Notion's comment API returns limited user info
			author = provider.Person{
				Name: "Notion User",
			}
		}

		result = append(result, provider.Comment{
			ID:        c.ID,
			Body:      body,
			CreatedAt: c.CreatedTime,
			UpdatedAt: c.LastEditedTime,
			Author:    author,
		})
	}

	return result, nil
}
