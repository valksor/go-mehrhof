package socket

import (
	"context"

	"github.com/valksor/kvelmo/internal/conductor"
)

// handleHooksList returns all configured workflow transition hooks.
func (w *WorktreeSocket) handleHooksList(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	hooks := w.conductor.ListHooks()
	if hooks == nil {
		hooks = make(map[string][]conductor.TransitionHookStatus)
	}

	return NewResultResponse(req.ID, hooks)
}
