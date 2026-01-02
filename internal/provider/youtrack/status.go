package youtrack

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// UpdateStatus changes the status of an issue via the State custom field.
func (p *Provider) UpdateStatus(ctx context.Context, workUnitID string, status provider.Status) error {
	// Map provider status to YouTrack state name
	stateName := statusToYouTrackState(status)

	// Build custom field update for State
	updates := map[string]interface{}{
		"customFields": []map[string]interface{}{
			{
				"name":  "State",
				"$type": "SingleEnumIssueCustomField",
				"value": map[string]string{
					"name": stateName,
				},
			},
		},
	}

	_, err := p.client.UpdateIssue(ctx, workUnitID, updates)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return nil
}

// statusToYouTrackState maps provider status to YouTrack state name.
// This is a default mapping - users may need customization based on their workflow.
func statusToYouTrackState(status provider.Status) string {
	switch status {
	case provider.StatusOpen:
		return "New"
	case provider.StatusInProgress:
		return "In Progress"
	case provider.StatusReview:
		return "Review"
	case provider.StatusDone:
		return "Done"
	case provider.StatusClosed:
		return "Obsolete"
	default:
		return "New"
	}
}
