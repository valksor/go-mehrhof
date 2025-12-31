package trello

import (
	"context"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// FetchSubtasks implements the provider.SubtaskFetcher interface.
// It converts Trello checklist items to subtasks.
func (p *Provider) FetchSubtasks(ctx context.Context, workUnitID string) ([]*provider.WorkUnit, error) {
	// Get the card with checklists
	card, err := p.client.GetCard(ctx, workUnitID)
	if err != nil {
		return nil, fmt.Errorf("get card: %w", err)
	}

	if len(card.Checklists) == 0 {
		return nil, nil
	}

	// Convert checklist items to WorkUnits
	var result []*provider.WorkUnit
	for _, checklist := range card.Checklists {
		for _, item := range checklist.CheckItems {
			// Determine status from item state
			status := provider.StatusOpen
			if item.State == "complete" {
				status = provider.StatusDone
			}

			wu := &provider.WorkUnit{
				ID:          item.ID,
				ExternalID:  item.ID,
				ExternalKey: item.ID,
				Provider:    ProviderName,
				Title:       item.Name,
				Status:      status,
				Priority:    provider.PriorityNormal,
				TaskType:    "subtask",
				CreatedAt:   time.Time{}, // Trello doesn't provide creation time for items
				Source: provider.SourceInfo{
					Type:      ProviderName,
					Reference: fmt.Sprintf("trello:%s/checkitem/%s", workUnitID, item.ID),
					SyncedAt:  time.Now(),
				},
				Metadata: map[string]any{
					"parent_id":      workUnitID,
					"is_subtask":     true,
					"checklist_id":   checklist.ID,
					"checklist_name": checklist.Name,
					"state":          item.State,
				},
			}
			result = append(result, wu)
		}
	}

	return result, nil
}
