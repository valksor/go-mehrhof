package wrike

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// List implements the provider.Lister interface.
// It lists tasks from the configured folder or space.
// Note: Wrike requires a scope (folder or space) to list tasks.
// Configure via config options: "folder_id" or "space_id"
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	// Get scope from config
	folderID := p.client.folderID
	spaceID := p.client.spaceID

	var tasks []Task
	var err error

	switch {
	case folderID != "":
		tasks, err = p.client.GetTasksInFolder(ctx, folderID)
	case spaceID != "":
		tasks, err = p.client.GetTasksInSpace(ctx, spaceID)
	default:
		return nil, fmt.Errorf("wrike: List requires folder_id or space_id configuration")
	}

	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	// Filter by status if specified
	result := make([]*provider.WorkUnit, 0, len(tasks))
	for _, task := range tasks {
		wu := p.taskToWorkUnit(&task)

		// Apply status filter
		if opts.Status != "" && wu.Status != opts.Status {
			continue
		}

		result = append(result, wu)
	}

	// Apply pagination
	if opts.Offset > 0 {
		if opts.Offset >= len(result) {
			return []*provider.WorkUnit{}, nil
		}
		result = result[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(result) {
		result = result[:opts.Limit]
	}

	return result, nil
}
