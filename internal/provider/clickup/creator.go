package clickup

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// CreateWorkUnit implements the provider.WorkUnitCreator interface.
// It creates a new task in ClickUp.
func (p *Provider) CreateWorkUnit(ctx context.Context, opts provider.CreateWorkUnitOptions) (*provider.WorkUnit, error) {
	listID := p.config.DefaultList
	if listID == "" {
		return nil, ErrListRequired
	}

	// Build task create request
	reqBody := map[string]any{
		"name":        opts.Title,
		"description": opts.Description,
	}

	// Set priority
	if opts.Priority != provider.PriorityNormal {
		reqBody["priority"] = mapProviderPriorityToClickUp(opts.Priority)
	}

	// Add tags from labels
	if len(opts.Labels) > 0 {
		reqBody["tags"] = opts.Labels
	}

	// Set parent for subtask
	if opts.ParentID != "" {
		ref, err := ParseReference(opts.ParentID)
		if err == nil {
			parentID := ref.TaskID
			if parentID == "" {
				parentID = ref.CustomID
			}
			reqBody["parent"] = parentID
		}
	}

	// Create task
	task, err := p.client.CreateTask(ctx, listID, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	return p.taskToWorkUnit(task), nil
}

// mapProviderPriorityToClickUp converts provider.Priority to ClickUp priority number
func mapProviderPriorityToClickUp(priority provider.Priority) int {
	switch priority {
	case provider.PriorityCritical:
		return 1 // urgent
	case provider.PriorityHigh:
		return 2 // high
	case provider.PriorityNormal:
		return 3 // normal
	case provider.PriorityLow:
		return 4 // low
	default:
		return 3
	}
}
