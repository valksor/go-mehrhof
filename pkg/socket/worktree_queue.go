package socket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/valksor/kvelmo/pkg/page"
	"github.com/valksor/kvelmo/pkg/storage"
)

// --- Queue Handlers ---

type queueAddParams struct {
	Source string `json:"source"`
	Title  string `json:"title"`
}

func (w *WorktreeSocket) handleQueueAdd(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params queueAddParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	if params.Source == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "source is required"), nil
	}

	task, err := w.conductor.QueueTask(params.Source, params.Title)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, task)
}

type queueRemoveParams struct {
	ID string `json:"id"`
}

func (w *WorktreeSocket) handleQueueRemove(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params queueRemoveParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	if params.ID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "id is required"), nil
	}

	if err := w.conductor.DequeueTask(params.ID); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{"success": true})
}

func (w *WorktreeSocket) handleQueueList(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params struct {
		Page    int `json:"page,omitempty"`
		PerPage int `json:"per_page,omitempty"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	queue := w.conductor.ListQueue()
	pg := page.NewPage(queue, paginationWithDefault(params.Page, params.PerPage))

	return NewResultResponse(req.ID, map[string]any{
		"queue":    pg.Items,
		"total":    pg.Total,
		"page":     pg.PageNum,
		"per_page": pg.PerPage,
		"has_next": pg.HasNext,
	})
}

type queueReorderParams struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
}

func (w *WorktreeSocket) handleQueueReorder(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params queueReorderParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	if params.ID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "id is required"), nil
	}

	if params.Position < 1 {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "position must be >= 1"), nil
	}

	if err := w.conductor.ReorderQueue(params.ID, params.Position); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	// Return updated queue
	queue := w.conductor.ListQueue()

	return NewResultResponse(req.ID, map[string]any{
		"queue": queue,
		"count": len(queue),
	})
}

// --- Task History Handler ---

func (w *WorktreeSocket) handleTaskHistory(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	tasks, err := w.conductor.TaskHistory()
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// --- Task Search Handler ---

type taskSearchParams struct {
	Query string `json:"query,omitempty"`
	Tag   string `json:"tag,omitempty"`
	Since string `json:"since,omitempty"`
	Until string `json:"until,omitempty"`
	State string `json:"state,omitempty"`
	Limit int    `json:"limit,omitempty"`
	File  string `json:"file,omitempty"`
}

func (w *WorktreeSocket) handleTaskSearch(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params taskSearchParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	opts := storage.SearchOptions{
		Query: params.Query,
		Tag:   params.Tag,
		State: params.State,
		Limit: params.Limit,
		File:  params.File,
	}

	if params.Since != "" {
		t, err := time.Parse(time.RFC3339, params.Since)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid since: expected RFC3339"), nil //nolint:nilerr // JSON-RPC error response
		}
		opts.Since = t
	}

	if params.Until != "" {
		t, err := time.Parse(time.RFC3339, params.Until)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid until: expected RFC3339"), nil //nolint:nilerr // JSON-RPC error response
		}
		opts.Until = t
	}

	tasks, err := w.conductor.SearchTaskHistory(opts)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// --- Suggestions Handler ---

func (w *WorktreeSocket) handleSuggestionsList(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	skipSuggestions, agentSuggestions := w.conductor.Suggestions()

	return NewResultResponse(req.ID, map[string]any{
		"skip":  skipSuggestions,
		"agent": agentSuggestions,
	})
}
