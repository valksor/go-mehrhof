package notion

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// UpdateStatus updates the status of a Notion page.
func (p *Provider) UpdateStatus(ctx context.Context, workUnitID string, status provider.Status) error {
	// Get the page first to find the status property ID
	page, err := p.client.GetPage(ctx, workUnitID)
	if err != nil {
		return err
	}

	// Find the status property by name (case-insensitive)
	var statusPropID string
	for key, prop := range page.Properties {
		if (prop.Type == "status" || prop.Type == "select") && key == p.statusProperty {
			statusPropID = prop.ID

			break
		}
	}

	if statusPropID == "" {
		return fmt.Errorf("status property %q not found on page", p.statusProperty)
	}

	// Build update input
	// Note: We need to use the property ID, not the name
	notionStatus := mapProviderStatusToNotion(status)

	update := &UpdatePageInput{
		Properties: map[string]Property{
			statusPropID: MakeStatusProperty(notionStatus),
		},
	}

	// For status property, we need to use the property name, not ID
	// The API expects property names in the update, not IDs
	update.Properties = map[string]Property{
		p.statusProperty: MakeStatusProperty(notionStatus),
	}

	_, err = p.client.UpdatePage(ctx, workUnitID, update)

	return err
}
