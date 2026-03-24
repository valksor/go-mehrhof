package socket

import (
	"context"

	"github.com/valksor/kvelmo/pkg/discovery"
)

// handleDiscoveryScan scans the worktree for runnable commands (Makefile targets,
// package.json scripts, Taskfile tasks, bin/ executables).
func (w *WorktreeSocket) handleDiscoveryScan(_ context.Context, req *Request) (*Response, error) {
	commands := discovery.DiscoverTools(w.path)
	if commands == nil {
		commands = []string{}
	}

	return NewResultResponse(req.ID, map[string]any{
		"commands": commands,
		"count":    len(commands),
	})
}
