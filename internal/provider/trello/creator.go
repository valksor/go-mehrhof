package trello

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// CreateWorkUnit implements the provider.WorkUnitCreator interface.
// It creates a new card in Trello.
func (p *Provider) CreateWorkUnit(ctx context.Context, opts provider.CreateWorkUnitOptions) (*provider.WorkUnit, error) {
	if p.boardID == "" {
		return nil, ErrNoBoardConfigured
	}

	// Find the "To Do" list as default target
	targetList, err := p.client.FindListByName(ctx, p.boardID, "To Do")
	if err != nil {
		// Try to get first list as fallback
		lists, listErr := p.client.GetBoardLists(ctx, p.boardID)
		if listErr != nil || len(lists) == 0 {
			return nil, fmt.Errorf("find target list: %w", err)
		}
		targetList = &lists[0]
	}

	// Create card
	card, err := p.client.CreateCard(ctx, targetList.ID, opts.Title, opts.Description)
	if err != nil {
		return nil, fmt.Errorf("create card: %w", err)
	}

	// Add labels if specified
	if len(opts.Labels) > 0 {
		for _, label := range opts.Labels {
			if err := p.client.AddLabel(ctx, card.ID, label); err != nil {
				// Ignore label errors - label might not exist
				continue
			}
		}
	}

	// Get the full card with list info
	return p.Fetch(ctx, card.ID)
}
