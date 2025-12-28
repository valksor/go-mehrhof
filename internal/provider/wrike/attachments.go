package wrike

import (
	"context"
	"fmt"
	"io"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// DownloadAttachment downloads an attachment by ID
func (p *Provider) DownloadAttachment(ctx context.Context, workUnitID, attachmentID string) (io.ReadCloser, error) {
	rc, _, err := p.client.DownloadAttachment(ctx, attachmentID)
	if err != nil {
		return nil, fmt.Errorf("download attachment: %w", err)
	}
	return rc, nil
}

// mapAttachments converts Wrike attachments to provider attachments
func mapAttachments(wrikeAttachments []Attachment) []provider.Attachment {
	result := make([]provider.Attachment, 0, len(wrikeAttachments))
	for _, a := range wrikeAttachments {
		result = append(result, provider.Attachment{
			ID:        a.ID,
			Name:      a.Name,
			URL:       "", // Wrike uses download endpoint, not direct URLs
			CreatedAt: a.CreatedDate,
		})
	}
	return result
}
