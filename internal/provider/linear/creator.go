package linear

import (
	"context"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// CreateWorkUnit creates a new Linear issue
// Note: Linear requires a team ID (not team key) for creating issues.
// The team key from config must be resolved to a team ID first.
func (p *Provider) CreateWorkUnit(ctx context.Context, opts provider.CreateWorkUnitOptions) (*provider.WorkUnit, error) {
	// For now, we need to use the team key. In production, you'd resolve this to a team ID
	// Since we don't have a team lookup function yet, we'll require the team to be configured
	if p.team == "" {
		return nil, fmt.Errorf("%w: specify linear.team in config", ErrTeamRequired)
	}

	// Build the create input
	// Note: Linear requires teamId (not team key). For now, we'll pass the team key
	// and the API will handle it or we'll need to add a team lookup function.
	input := CreateIssueInput{
		Title:       opts.Title,
		Description: opts.Description,
	}

	// Set priority if specified
	if opts.Priority != provider.PriorityNormal {
		input.Priority = mapProviderPriorityToLinear(opts.Priority)
	}

	// Note: Linear expects label IDs and assignee IDs, not names
	// In production, you would resolve names to IDs first
	if len(opts.Labels) > 0 {
		input.LabelIDs = opts.Labels
	}
	if len(opts.Assignees) > 0 {
		// Use first assignee for now
		input.AssigneeID = opts.Assignees[0]
	}

	// Create the issue
	issue, err := p.client.CreateIssue(ctx, input)
	if err != nil {
		return nil, err
	}

	// Convert to WorkUnit
	wu := &provider.WorkUnit{
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
		TaskType:    inferTaskTypeFromLabels(opts.Labels),
		Slug:        naming.Slugify(issue.Title, 50),
		Metadata:    buildMetadata(issue),
	}

	return wu, nil
}

// inferTaskTypeFromLabels determines task type from label names.
func inferTaskTypeFromLabels(labels []string) string {
	for _, label := range labels {
		switch lower(label) {
		case "bug", "bugfix", "fix":
			return "fix"
		case "feature", "enhancement":
			return "feature"
		case "docs", "documentation":
			return "docs"
		case "refactor":
			return "refactor"
		case "chore":
			return "chore"
		case "test":
			return "test"
		case "ci":
			return "ci"
		}
	}
	return "issue"
}

// lower is a helper for lowercase conversion.
func lower(s string) string {
	var result []rune
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}
