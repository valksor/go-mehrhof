package socket

import (
	"context"
	"encoding/json"

	"github.com/valksor/kvelmo/pkg/conductor"
)

// handleForkCreate creates a new fork for the current task.
func (w *WorktreeSocket) handleForkCreate(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params struct {
		Label string `json:"label"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.Label == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "label is required"), nil
	}

	info, err := w.conductor.Fork(ctx, params.Label)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, info)
}

// handleForkList returns all active forks for the current task.
func (w *WorktreeSocket) handleForkList(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	forks := w.conductor.ListForks()
	if forks == nil {
		forks = []conductor.ForkInfo{}
	}

	return NewResultResponse(req.ID, map[string]any{"forks": forks})
}

// handleForkCompare returns a comparison of all active forks.
func (w *WorktreeSocket) handleForkCompare(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	comparison, err := w.conductor.CompareForks(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, comparison)
}

// handleForkSelect selects the winning fork and merges it back.
func (w *WorktreeSocket) handleForkSelect(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params struct {
		ForkID string `json:"fork_id"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.ForkID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "fork_id is required"), nil
	}

	if err := w.conductor.SelectFork(ctx, params.ForkID); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]string{"status": "ok"})
}
