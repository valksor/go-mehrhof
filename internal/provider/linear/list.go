package linear

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// List retrieves issues from Linear
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	teamKey := p.team

	// If team key not configured, try to extract from labels filter
	if teamKey == "" && len(opts.Labels) > 0 {
		for _, label := range opts.Labels {
			// Check if label looks like a team key (e.g., "ENG")
			if strings.Contains(label, "-") {
				// Extract team key from issue ID format like "ENG-123"
				parts := strings.Split(label, "-")
				if len(parts) == 2 {
					teamKey = parts[0]
					break
				}
			}
		}
	}

	if teamKey == "" {
		return nil, fmt.Errorf("%w: specify linear.team in config or include team key in labels", ErrTeamRequired)
	}

	// Build filters
	filters := ListFilters{}

	// Map status filter to Linear state name
	if opts.Status != "" {
		filters.State = mapProviderStatusToLinearStateName(opts.Status)
	}

	// Fetch issues from Linear
	issues, err := p.client.ListIssues(ctx, teamKey, filters)
	if err != nil {
		return nil, err
	}

	// Apply label filter if specified (post-filter since Linear API filtering is limited)
	var filtered []*Issue
	if len(opts.Labels) > 0 {
		for _, issue := range issues {
			if matchesLabels(issue, opts.Labels) {
				filtered = append(filtered, issue)
			}
		}
	} else {
		filtered = issues
	}

	// Apply offset
	if opts.Offset > 0 && opts.Offset < len(filtered) {
		filtered = filtered[opts.Offset:]
	} else if opts.Offset > 0 {
		filtered = []*Issue{}
	}

	// Apply limit
	if opts.Limit > 0 && opts.Limit < len(filtered) {
		filtered = filtered[:opts.Limit]
	}

	// Convert to WorkUnits
	result := make([]*provider.WorkUnit, 0, len(filtered))
	for _, issue := range filtered {
		wu := issueToWorkUnit(issue)
		result = append(result, wu)
	}

	return result, nil
}

// matchesLabels checks if an issue matches the given labels
func matchesLabels(issue *Issue, labels []string) bool {
	if len(labels) == 0 {
		return true
	}

	issueLabelNames := make(map[string]bool)
	for _, label := range issue.Labels {
		issueLabelNames[label.Name] = true
	}

	for _, filterLabel := range labels {
		if !issueLabelNames[filterLabel] {
			return false
		}
	}
	return true
}

// issueToWorkUnit converts an Issue to a WorkUnit without fetching nested data
// Used by List for efficiency when listing multiple issues
func issueToWorkUnit(issue *Issue) *provider.WorkUnit {
	return &provider.WorkUnit{
		ID:          issue.ID,
		ExternalID:  issue.Identifier,
		Provider:    ProviderName,
		Title:       issue.Title,
		Description: issue.Description,
		Status:      mapLinearStatus(issue.State),
		Priority:    mapLinearPriority(issue.Priority),
		Labels:      extractLabelNames(issue.Labels),
		Assignees:   mapAssignees(issue.Assignee),
		CreatedAt:   issue.CreatedAt,
		UpdatedAt:   issue.UpdatedAt,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: issue.Identifier,
			SyncedAt:  time.Now(),
		},
		ExternalKey: issue.Identifier,
		TaskType:    "issue",
		Slug:        naming.Slugify(issue.Title, 50),
		Metadata:    buildMetadata(issue),
	}
}
