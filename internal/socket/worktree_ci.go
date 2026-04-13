package socket

import (
	"context"
)

// handleCIStatus returns the CI pipeline status for the current task's PR.
func (w *WorktreeSocket) handleCIStatus(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewResultResponse(req.ID, map[string]any{
			"state":   "unknown",
			"message": "no active task",
		})
	}

	if wu.PRID == "" {
		return NewResultResponse(req.ID, map[string]any{
			"state":   "unknown",
			"message": "no PR submitted yet",
		})
	}

	// CI watcher integration point — if a watcher is active, return its status.
	// For now, return the PR ID so external tools can query CI status directly.
	return NewResultResponse(req.ID, map[string]any{
		"state":   "unknown",
		"pr_id":   wu.PRID,
		"message": "CI watcher not active — use provider's CI dashboard to check status",
	})
}
