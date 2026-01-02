package wrike

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// UpdateStatus implements the provider.StatusUpdater interface.
// It changes the status of a Wrike task.
func (p *Provider) UpdateStatus(ctx context.Context, workUnitID string, status provider.Status) error {
	// Convert provider status to Wrike status
	wrikeStatus := mapProviderStatusToWrike(status)

	// Update the task via API
	if err := p.client.UpdateTaskStatus(ctx, workUnitID, wrikeStatus); err != nil {
		return fmt.Errorf("update task status: %w", err)
	}

	return nil
}

// mapProviderStatusToWrike converts a provider.Status to a Wrike status string.
func mapProviderStatusToWrike(status provider.Status) string {
	switch status {
	case provider.StatusOpen:
		return "Active"
	case provider.StatusInProgress:
		return "Active"
	case provider.StatusReview:
		return "Active" // Wrike doesn't have a native review status
	case provider.StatusDone:
		return "Completed"
	case provider.StatusClosed:
		return "Cancelled"
	default:
		return "Active"
	}
}
