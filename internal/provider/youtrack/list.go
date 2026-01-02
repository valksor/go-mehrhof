package youtrack

import (
	"context"
	"fmt"
	"strings"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// List retrieves issues from YouTrack.
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	// Build query from options
	query := buildQuery(opts)

	issues, err := p.client.GetIssuesByQuery(ctx, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	result := make([]*provider.WorkUnit, 0, len(issues))
	for _, issue := range issues {
		result = append(result, p.issueToWorkUnit(&issue, nil, nil))
	}

	return result, nil
}

// buildQuery constructs YouTrack query from ListOptions.
func buildQuery(opts provider.ListOptions) string {
	var parts []string

	// Status filter
	if opts.Status != "" {
		switch opts.Status {
		case provider.StatusOpen:
			parts = append(parts, "Unresolved")
		case provider.StatusDone, provider.StatusClosed:
			parts = append(parts, "Resolved")
		}
	}

	// Label/Tag filter - YouTrack uses tag: syntax
	for _, label := range opts.Labels {
		parts = append(parts, fmt.Sprintf("tag: %s", label))
	}

	// Order by
	if opts.OrderBy != "" {
		order := "asc"
		if opts.OrderDir == "desc" {
			order = "desc"
		}
		parts = append(parts, fmt.Sprintf("sort by: %s %s", opts.OrderBy, order))
	}

	return strings.Join(parts, " ")
}
