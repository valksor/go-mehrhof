package socket

import (
	"context"
	"encoding/json"
	"slices"
)

// handleTaskTag manages tags on the current task: add, remove, list.
func (w *WorktreeSocket) handleTaskTag(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params struct {
		Action string   `json:"action"` // "add", "remove", "list"
		Tags   []string `json:"tags"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no active task"), nil
	}

	switch params.Action {
	case "add":
		for _, tag := range params.Tags {
			if !slices.Contains(wu.Tags, tag) {
				wu.Tags = append(wu.Tags, tag)
			}
		}
		w.conductor.MarkDirty()

		return NewResultResponse(req.ID, map[string]any{"tags": wu.Tags})

	case "remove":
		for _, tag := range params.Tags {
			wu.Tags = slices.DeleteFunc(wu.Tags, func(t string) bool { return t == tag })
		}
		w.conductor.MarkDirty()

		return NewResultResponse(req.ID, map[string]any{"tags": wu.Tags})

	case "list", "":
		tags := wu.Tags
		if tags == nil {
			tags = []string{}
		}

		return NewResultResponse(req.ID, map[string]any{"tags": tags})

	default:
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "unknown action: "+params.Action), nil
	}
}
