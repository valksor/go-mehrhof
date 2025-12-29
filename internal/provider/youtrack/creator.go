package youtrack

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// CreateWorkUnit creates a new YouTrack issue
func (p *Provider) CreateWorkUnit(ctx context.Context, opts provider.CreateWorkUnitOptions) (*provider.WorkUnit, error) {
	// Map priority and type to custom fields
	customFields := p.buildCustomFields(opts)

	// For project ID, use a default or extract from custom fields
	projectID := "0-0" // Default project ID, should be configurable
	if pid, ok := opts.CustomFields["project_id"].(string); ok {
		projectID = pid
	}

	issue, err := p.client.CreateIssue(ctx, projectID, opts.Title, opts.Description, customFields)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}

	return p.issueToWorkUnit(issue, nil, nil), nil
}

// buildCustomFields builds custom fields array from CreateWorkUnitOptions
func (p *Provider) buildCustomFields(opts provider.CreateWorkUnitOptions) []map[string]interface{} {
	var fields []map[string]interface{}

	// Priority
	fields = append(fields, map[string]interface{}{
		"name":  "Priority",
		"$type": "SingleEnumIssueCustomField",
		"value": map[string]string{"name": priorityToYouTrack(opts.Priority)},
	})

	// Type - can be inferred or set to default
	taskType := "Task"
	if opts.CustomFields != nil {
		if t, ok := opts.CustomFields["type"].(string); ok {
			taskType = t
		}
	}
	fields = append(fields, map[string]interface{}{
		"name":  "Type",
		"$type": "SingleEnumIssueCustomField",
		"value": map[string]string{"name": taskType},
	})

	return fields
}

// priorityToYouTrack maps provider Priority to YouTrack priority name
func priorityToYouTrack(p provider.Priority) string {
	switch p {
	case provider.PriorityCritical:
		return "Critical"
	case provider.PriorityHigh:
		return "High"
	case provider.PriorityLow:
		return "Low"
	default:
		return "Normal"
	}
}
