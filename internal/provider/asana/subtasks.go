package asana

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// FetchSubtasks implements the provider.SubtaskFetcher interface.
// It retrieves subtasks for a given Asana task.
func (p *Provider) FetchSubtasks(ctx context.Context, workUnitID string) ([]*provider.WorkUnit, error) {
	// Get subtasks via API
	subtasks, err := p.client.GetSubtasks(ctx, workUnitID)
	if err != nil {
		return nil, fmt.Errorf("get subtasks: %w", err)
	}

	if len(subtasks) == 0 {
		return nil, nil
	}

	// Convert to WorkUnits
	result := make([]*provider.WorkUnit, 0, len(subtasks))
	for _, st := range subtasks {
		wu := p.taskToWorkUnit(&st)
		wu.Metadata["parent_id"] = workUnitID
		wu.Metadata["is_subtask"] = true
		wu.TaskType = "subtask"
		result = append(result, wu)
	}

	return result, nil
}
