package youtrack

import (
	"context"
	"fmt"
	"io"
)

// DownloadAttachment downloads an attachment.
func (p *Provider) DownloadAttachment(ctx context.Context, workUnitID, attachmentID string) (io.ReadCloser, error) {
	rc, _, err := p.client.DownloadAttachment(ctx, attachmentID)
	if err != nil {
		return nil, fmt.Errorf("download attachment: %w", err)
	}
	return rc, nil
}
