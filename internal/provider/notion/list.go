package notion

import (
	"context"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// List retrieves pages from Notion database
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	databaseID := p.databaseID

	if databaseID == "" {
		return nil, fmt.Errorf("%w: specify notion.database_id in config", ErrDatabaseRequired)
	}

	// Build query request
	req := &DatabaseQueryRequest{}

	// Add status filter if specified
	if opts.Status != "" {
		req.Filter = &Filter{
			Property: p.statusProperty,
			Status: &StatusFilter{
				Equals: mapProviderStatusToNotion(provider.Status(opts.Status)),
			},
		}
	}

	// Add label filter if specified
	if len(opts.Labels) > 0 {
		statusFilter := req.Filter
		if statusFilter != nil {
			// If we already have a status filter, combine with AND
			req.Filter = &Filter{
				And: []Filter{*statusFilter},
			}
		} else {
			req.Filter = &Filter{}
		}

		// For labels, we use multi_select filter
		// Note: We'll need to post-filter since multi_select filter is limited
	}

	// Fetch pages from Notion
	pages, err := p.client.QueryDatabaseAll(ctx, databaseID, req)
	if err != nil {
		return nil, err
	}

	// Apply label filter if specified (post-filter)
	var filtered []*Page
	if len(opts.Labels) > 0 {
		for _, page := range pages {
			if matchesLabels(page, opts.Labels, p.labelsProperty) {
				filtered = append(filtered, &page)
			}
		}
	} else {
		for i := range pages {
			filtered = append(filtered, &pages[i])
		}
	}

	// Apply offset
	if opts.Offset > 0 && opts.Offset < len(filtered) {
		filtered = filtered[opts.Offset:]
	} else if opts.Offset > 0 {
		filtered = []*Page{}
	}

	// Apply limit
	if opts.Limit > 0 && opts.Limit < len(filtered) {
		filtered = filtered[:opts.Limit]
	}

	// Convert to WorkUnits
	result := make([]*provider.WorkUnit, 0, len(filtered))
	for i := range filtered {
		wu := pageToWorkUnit(*filtered[i], p.statusProperty, p.labelsProperty)
		result = append(result, wu)
	}

	return result, nil
}

// matchesLabels checks if a page matches the given labels
func matchesLabels(page Page, labels []string, labelsProperty string) bool {
	if len(labels) == 0 {
		return true
	}

	prop, ok := GetProperty(page, labelsProperty)
	if !ok || prop.MultiSelect == nil {
		return false
	}

	pageLabels := make(map[string]bool)
	for _, opt := range prop.MultiSelect.Options {
		pageLabels[opt.Name] = true
	}

	for _, filterLabel := range labels {
		if !pageLabels[filterLabel] {
			return false
		}
	}
	return true
}

// pageToWorkUnit converts a Page to a WorkUnit without fetching nested data
// Used by List for efficiency when listing multiple pages
func pageToWorkUnit(page Page, statusProperty, labelsProperty string) *provider.WorkUnit {
	return &provider.WorkUnit{
		ID:          page.ID,
		ExternalID:  page.ID,
		Provider:    ProviderName,
		Title:       extractTitle(page),
		Description: "", // Don't fetch blocks for list performance
		Status:      extractStatus(page, statusProperty),
		Priority:    provider.PriorityNormal,
		Labels:      extractLabelsFromPage(page, labelsProperty),
		Assignees:   extractAssignees(page),
		CreatedAt:   page.CreatedTime,
		UpdatedAt:   page.LastEditedTime,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: page.URL,
			SyncedAt:  time.Now(),
		},
		ExternalKey: page.ID[:8], // Use first 8 chars of UUID
		TaskType:    "page",
		Slug:        naming.Slugify(extractTitle(page), 50),
		Metadata:    buildMetadata(page, nil),
	}
}
