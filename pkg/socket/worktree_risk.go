package socket

import (
	"context"
	"encoding/json"

	"github.com/valksor/kvelmo/pkg/eventlog"
)

// handleRiskEvaluate returns the current task risk score.
func (w *WorktreeSocket) handleRiskEvaluate(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no task loaded"), nil
	}

	score := w.conductor.EvaluateRisk(ctx)

	return NewResultResponse(req.ID, score)
}

// handleRiskHistory returns risk scores from recent eventlog entries.
func (w *WorktreeSocket) handleRiskHistory(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	var params struct {
		Limit int `json:"limit"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}

	el := w.conductor.EventLog()
	if el == nil {
		return NewResultResponse(req.ID, map[string]any{"entries": []any{}})
	}

	entries, err := el.Query(eventlog.EventRiskEvaluated, params.Limit)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{"entries": entries})
}
