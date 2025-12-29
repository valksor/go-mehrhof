package notion

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// Snapshot captures the page content from Notion
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	// Fetch the page
	page, err := p.client.GetPage(ctx, ref.PageID)
	if err != nil {
		return nil, err
	}

	// Fetch page content blocks
	blocks, err := p.client.GetPageContent(ctx, ref.PageID)
	if err != nil {
		blocks = []Block{} // Continue without content
	}

	// Fetch comments
	comments, _ := p.client.GetComments(ctx, ref.PageID)

	var content strings.Builder

	// Header
	content.WriteString(fmt.Sprintf("# %s\n\n", extractTitle(*page)))

	// Metadata
	content.WriteString("### Metadata\n\n")
	content.WriteString(fmt.Sprintf("- **ID:** %s\n", page.ID))
	content.WriteString(fmt.Sprintf("- **URL:** %s\n", page.URL))

	if prop, ok := GetProperty(*page, p.statusProperty); ok {
		if prop.Status != nil {
			content.WriteString(fmt.Sprintf("- **Status:** %s\n", prop.Status.Name))
		} else if prop.Select != nil {
			content.WriteString(fmt.Sprintf("- **Status:** %s\n", prop.Select.Name))
		}
	}

	if labels := extractLabelsFromPage(*page, p.labelsProperty); len(labels) > 0 {
		content.WriteString(fmt.Sprintf("- **Labels:** %s\n", strings.Join(labels, ", ")))
	}

	if assignees := extractAssignees(*page); len(assignees) > 0 {
		assigneeNames := make([]string, len(assignees))
		for i, a := range assignees {
			assigneeNames[i] = a.Name
		}
		content.WriteString(fmt.Sprintf("- **Assignees:** %s\n", strings.Join(assigneeNames, ", ")))
	}

	content.WriteString(fmt.Sprintf("- **Created:** %s\n", page.CreatedTime.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("- **Updated:** %s\n\n", page.LastEditedTime.Format(time.RFC3339)))

	// Description/Content
	if len(blocks) > 0 {
		content.WriteString("## Content\n\n")
		content.WriteString(BlocksToMarkdown(blocks))
		content.WriteString("\n")
	}

	// Comments
	if len(comments) > 0 {
		content.WriteString("## Comments\n\n")
		for _, c := range comments {
			// Extract comment text
			body := ""
			for _, rt := range c.RichText {
				body += rt.PlainText
			}

			// Get author name
			authorName := "Unknown"
			if c.CreatedBy.Type == "person" {
				authorName = "Notion User"
			}

			content.WriteString(fmt.Sprintf("### %s - %s\n\n", authorName, c.CreatedTime.Format(time.RFC3339)))
			content.WriteString(body)
			content.WriteString("\n\n---\n\n")
		}
	}

	return &provider.Snapshot{
		Type:    ProviderName,
		Ref:     id,
		Content: content.String(),
	}, nil
}
