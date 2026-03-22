package socket

import (
	"context"
	"encoding/json"
)

// handleApprove marks a transition event as approved by a human.
func (w *WorktreeSocket) handleApprove(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params struct {
		Event string `json:"event"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}
	if params.Event == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "event is required"), nil
	}

	if err := w.conductor.Approve(params.Event); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{"approved": params.Event})
}

// handleApproveNode approves or rejects a pending graph node approval gate.
func (w *WorktreeSocket) handleApproveNode(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params struct {
		NodeID string `json:"node_id"`
		Reject bool   `json:"reject"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}
	if params.NodeID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "node_id is required"), nil
	}

	if params.Reject {
		if err := w.conductor.RejectNode(params.NodeID); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
		}

		return NewResultResponse(req.ID, map[string]any{"rejected": params.NodeID})
	}

	if err := w.conductor.ApproveNode(params.NodeID); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{"approved": params.NodeID})
}

// handleReviewChecklistGet returns the configured checklist items and which are checked.
func (w *WorktreeSocket) handleReviewChecklistGet(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	required, checked := w.conductor.ReviewChecklistStatus()
	if required == nil {
		required = []string{}
	}
	if checked == nil {
		checked = []string{}
	}

	return NewResultResponse(req.ID, map[string]any{
		"required": required,
		"checked":  checked,
	})
}

// handleReviewChecklistCheck marks a review checklist item as checked.
func (w *WorktreeSocket) handleReviewChecklistCheck(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params struct {
		Item string `json:"item"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}
	if params.Item == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "item is required"), nil
	}

	if err := w.conductor.CheckReviewItem(params.Item); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{"checked": params.Item})
}

// handleReviewChecklistUncheck removes a review checklist item.
func (w *WorktreeSocket) handleReviewChecklistUncheck(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params struct {
		Item string `json:"item"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}
	if params.Item == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "item is required"), nil
	}

	if err := w.conductor.UncheckReviewItem(params.Item); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{"unchecked": params.Item})
}
