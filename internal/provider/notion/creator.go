package notion

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// CreateWorkUnitInput holds input for creating a new page
type CreateWorkUnitInput struct {
	Title       string
	Description string
	Status      provider.Status
	Labels      []string
	AssigneeID  string
}

// CreateWorkUnit creates a new Notion page
func (p *Provider) CreateWorkUnit(ctx context.Context, input CreateWorkUnitInput) (*provider.WorkUnit, error) {
	databaseID := p.databaseID
	if databaseID == "" {
		return nil, fmt.Errorf("%w: specify notion.database_id in config", ErrDatabaseRequired)
	}

	// Build properties
	properties := map[string]Property{
		"Name":           MakeTitleProperty(input.Title),
		p.statusProperty: MakeStatusProperty(mapProviderStatusToNotion(input.Status)),
	}

	// Add description if provided
	if input.Description != "" {
		properties[p.descriptionProperty] = MakeRichTextProperty(input.Description)
	}

	// Add labels if provided
	if len(input.Labels) > 0 {
		properties[p.labelsProperty] = MakeMultiSelectProperty(input.Labels)
	}

	// Add assignee if provided
	if input.AssigneeID != "" {
		properties["Assignee"] = Property{
			Type: "people",
			People: &PeopleProp{
				Type: "people",
				People: []User{
					{
						ID: input.AssigneeID,
					},
				},
			},
		}
	}

	createInput := &CreatePageInput{
		Parent: Parent{
			Type:       "database_id",
			DatabaseID: databaseID,
		},
		Properties: properties,
	}

	page, err := p.client.CreatePage(ctx, createInput)
	if err != nil {
		return nil, err
	}

	return &provider.WorkUnit{
		ID:          page.ID,
		ExternalID:  page.ID,
		Provider:    ProviderName,
		Title:       input.Title,
		Description: input.Description,
		Status:      input.Status,
		Priority:    provider.PriorityNormal,
		Labels:      input.Labels,
		CreatedAt:   page.CreatedTime,
		UpdatedAt:   page.LastEditedTime,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: page.URL,
			SyncedAt:  page.CreatedTime,
		},
		ExternalKey: page.ID[:8],
		TaskType:    "page",
	}, nil
}
