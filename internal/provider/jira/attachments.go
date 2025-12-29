package jira

import (
	"context"
	"io"
)

// DownloadAttachment downloads an attachment from Jira
func (p *Provider) DownloadAttachment(ctx context.Context, workUnitID, attachmentID string) (io.ReadCloser, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, err
	}

	// Update base URL if detected from reference
	if ref.BaseURL != "" && p.baseURL == "" {
		p.baseURL = ref.BaseURL
		p.client.SetBaseURL(ref.BaseURL)
	}

	// Get attachments for the issue
	attachments, err := p.client.GetAttachments(ctx, ref.IssueKey)
	if err != nil {
		return nil, err
	}

	// Find the attachment by ID
	var attachmentURL string
	for _, att := range attachments {
		if att.ID == attachmentID {
			attachmentURL = att.Content
			break
		}
	}

	if attachmentURL == "" {
		return nil, ErrIssueNotFound
	}

	// Download the attachment
	reader, _, err := p.client.DownloadAttachment(ctx, attachmentURL)
	if err != nil {
		return nil, err
	}

	return reader, nil
}
