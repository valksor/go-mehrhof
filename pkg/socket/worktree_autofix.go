package socket

import (
	"context"
)

// handleAutoFixStatus returns the current auto-fix loop state.
func (w *WorktreeSocket) handleAutoFixStatus(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	status := w.conductor.GetAutoFixStatus()

	return NewResultResponse(req.ID, status)
}
