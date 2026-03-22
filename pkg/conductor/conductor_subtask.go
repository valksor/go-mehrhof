package conductor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/valksor/kvelmo/pkg/graph"
)

// RunSubTask creates an isolated worktree and runs specified phases.
// This is called by the graph scheduler when a node has SubTask config.
// Returns the combined output from all phases.
func (c *Conductor) RunSubTask(ctx context.Context, config graph.SubTaskConfig) (string, error) {
	// Sub-task execution is not yet implemented. When wired, this will create
	// an isolated worktree, spawn a sub-conductor, and run the requested phases.
	slog.Info("sub-task requested",
		"title", config.Title,
		"phases", config.Phases,
		"branch", config.Branch,
	)

	return "", fmt.Errorf("sub-task execution not yet implemented (title: %s, phases: %v)", config.Title, config.Phases)
}
