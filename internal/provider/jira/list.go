package jira

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// List retrieves issues from Jira.
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	projectKey := p.defaultProject

	// Try to extract project key from labels filter (user may pass project as label)
	if projectKey == "" && len(opts.Labels) > 0 {
		for _, label := range opts.Labels {
			if looksLikeProjectKey(label) {
				projectKey = label
				break
			}
		}
	}

	if projectKey == "" {
		return nil, fmt.Errorf("%w: specify jira.project in config or include project key in labels", ErrProjectRequired)
	}

	// Build JQL query
	jql := buildJQL(projectKey, opts)

	// Calculate pagination
	startAt := opts.Offset
	if startAt < 0 {
		startAt = 0
	}
	maxResults := opts.Limit
	if maxResults <= 0 {
		maxResults = 50 // Default page size
	}

	// Fetch issues from Jira
	issues, _, err := p.client.ListIssues(ctx, jql, startAt, maxResults)
	if err != nil {
		return nil, err
	}

	// Convert to WorkUnits
	result := make([]*provider.WorkUnit, 0, len(issues))
	for _, issue := range issues {
		wu := issueToWorkUnit(issue)
		result = append(result, wu)
	}

	return result, nil
}

// buildJQL constructs a JQL query from list options.
func buildJQL(projectKey string, opts provider.ListOptions) string {
	var jqlParts []string

	// Project filter
	jqlParts = append(jqlParts, fmt.Sprintf("project = %s", projectKey))

	// Status filter
	if opts.Status != "" {
		jqlParts = append(jqlParts, fmt.Sprintf("status = \"%s\"", opts.Status))
	}

	// Labels filter
	if len(opts.Labels) > 0 {
		// Filter out project key from labels
		var labels []string
		for _, label := range opts.Labels {
			if !looksLikeProjectKey(label) {
				labels = append(labels, label)
			}
		}
		if len(labels) > 0 {
			quotedLabels := make([]string, len(labels))
			for i, label := range labels {
				quotedLabels[i] = fmt.Sprintf("\"%s\"", label)
			}
			jqlParts = append(jqlParts, fmt.Sprintf("labels in (%s)", strings.Join(quotedLabels, ", ")))
		}
	}

	// Build base query with filters
	jql := strings.Join(jqlParts, " AND ")

	// Append ordering (no AND before ORDER BY)
	orderBy := "created"
	if opts.OrderBy != "" {
		orderBy = opts.OrderBy
	}
	orderDir := "DESC"
	if opts.OrderDir == "asc" {
		orderDir = "ASC"
	}
	jql = fmt.Sprintf("%s ORDER BY %s %s", jql, orderBy, orderDir)

	return jql
}

// looksLikeProjectKey checks if a string looks like a Jira project key.
func looksLikeProjectKey(s string) bool {
	// Project keys are typically 2-10 uppercase letters/numbers
	if len(s) < 2 || len(s) > 10 {
		return false
	}
	for _, r := range s {
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		if !isUpper && !isDigit {
			return false
		}
	}
	return true
}

// issueToWorkUnit converts an Issue to a WorkUnit without fetching nested data.
// Used by List for efficiency when listing multiple issues.
func issueToWorkUnit(issue *Issue) *provider.WorkUnit {
	return &provider.WorkUnit{
		ID:          issue.ID,
		ExternalID:  issue.Key,
		Provider:    ProviderName,
		Title:       issue.Fields.Summary,
		Description: issue.Fields.Description,
		Status:      mapJiraStatus(issue.Fields.Status.Name),
		Priority:    mapJiraPriority(issue.Fields.Priority),
		Labels:      issue.Fields.Labels,
		Assignees:   mapAssignees(issue.Fields.Assignee),
		CreatedAt:   issue.Fields.Created,
		UpdatedAt:   issue.Fields.Updated,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: issue.Key,
			SyncedAt:  time.Now(),
		},
		ExternalKey: issue.Key,
		TaskType:    inferTaskTypeFromLabels(issue.Fields.Labels),
		Slug:        naming.Slugify(issue.Fields.Summary, 50),
		Metadata:    buildMetadata(issue),
	}
}
