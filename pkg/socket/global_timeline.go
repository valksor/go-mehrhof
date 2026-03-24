package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valksor/kvelmo/pkg/activitylog"
	"github.com/valksor/kvelmo/pkg/timeline"
)

// activityLogForTimeline extracts the activitylog.Log from the server's
// activity logger adapter. Returns nil if no activity log is configured.
func (g *GlobalSocket) activityLogForTimeline() *activitylog.Log {
	if g.server.activityLogger == nil {
		return nil
	}
	adapter, ok := g.server.activityLogger.(*activityLogAdapter)
	if !ok {
		return nil
	}

	return adapter.log
}

// handleTimelineRecent returns recent timeline activities in reverse chronological order.
func (g *GlobalSocket) handleTimelineRecent(_ context.Context, req *Request) (*Response, error) {
	log := g.activityLogForTimeline()
	if log == nil {
		return NewResultResponse(req.ID, map[string]any{"activities": []any{}, "count": 0})
	}

	var params struct {
		Since string `json:"since"` // Duration string e.g. "1h", "30m"
		Limit int    `json:"limit"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	opts := activitylog.QueryOptions{
		Limit: params.Limit,
	}
	if opts.Limit <= 0 {
		opts.Limit = 50
	}

	if params.Since != "" {
		d, err := time.ParseDuration(params.Since)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, fmt.Sprintf("invalid since duration: %s", err)), nil
		}
		opts.Since = d
	}

	svc := timeline.New(log)
	activities, err := svc.RecentActivity(opts)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"activities": activities,
		"count":      len(activities),
	})
}

// handleTimelineTask returns chronological timeline activities for a specific task.
func (g *GlobalSocket) handleTimelineTask(_ context.Context, req *Request) (*Response, error) {
	log := g.activityLogForTimeline()
	if log == nil {
		return NewResultResponse(req.ID, map[string]any{"activities": []any{}, "count": 0})
	}

	var params struct {
		TraceID string `json:"trace_id"`
		Limit   int    `json:"limit"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.TraceID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "trace_id is required"), nil
	}

	if params.Limit <= 0 {
		params.Limit = 100
	}

	svc := timeline.New(log)
	activities, err := svc.TaskTimeline(params.TraceID, params.Limit)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"activities": activities,
		"count":      len(activities),
	})
}

// handleTimelineSummary returns event counts grouped by type.
func (g *GlobalSocket) handleTimelineSummary(_ context.Context, req *Request) (*Response, error) {
	log := g.activityLogForTimeline()
	if log == nil {
		return NewResultResponse(req.ID, map[string]any{"summary": map[string]int{}})
	}

	var params struct {
		Since string `json:"since"` // Duration string e.g. "24h"
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	sinceStr := params.Since
	if sinceStr == "" {
		sinceStr = "24h"
	}

	d, err := time.ParseDuration(sinceStr)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, fmt.Sprintf("invalid since duration: %s", err)), nil
	}

	since := time.Now().Add(-d)

	svc := timeline.New(log)
	summary, err := svc.ProjectSummary(since)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"summary": summary,
	})
}
