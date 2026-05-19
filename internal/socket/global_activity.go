package socket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/valksor/kvelmo/internal/activitylog"
	"github.com/valksor/kvelmo/internal/timeline"
)

// activityLogAdapter bridges socket.ActivityLogger to activitylog.Log.
type activityLogAdapter struct {
	log *activitylog.Log
}

func (a *activityLogAdapter) Record(entry ActivityEntry) {
	a.log.Record(activitylog.Entry{
		Timestamp:     time.Now(),
		Method:        entry.Method,
		CorrelationID: entry.CorrelationID,
		DurationMs:    entry.DurationMs,
		Error:         entry.Error,
		ParamsSize:    entry.ParamsSize,
		UserID:        entry.UserID,
		TaskID:        entry.TaskID,
		AgentModel:    entry.AgentModel,
		TaskTraceID:   entry.TaskTraceID,
	})
}

// SetActivityLog configures the global socket to record RPC activity.
func (g *GlobalSocket) SetActivityLog(l *activitylog.Log) {
	g.server.SetActivityLogger(&activityLogAdapter{log: l})
}

// handleActivityQuery returns filtered activity log entries.
func (g *GlobalSocket) handleActivityQuery(_ context.Context, req *Request) (*Response, error) {
	if g.server.activityLogger == nil {
		return NewResultResponse(req.ID, map[string]any{keyEntries: []any{}, keyEnabled: false})
	}

	adapter, ok := g.server.activityLogger.(*activityLogAdapter)
	if !ok {
		return NewErrorResponse(req.ID, ErrCodeInternal, "activity log not configured"), nil
	}

	var params struct {
		Since         string `json:"since"`          // Duration string e.g. "1h", "30m"
		MethodPattern string `json:"method_pattern"` // Pipe-separated: "start|plan"
		ErrorsOnly    bool   `json:"errors_only"`
		Limit         int    `json:"limit"`
		Timeline      bool   `json:"timeline"` // Return human-readable timeline activities
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	opts := activitylog.QueryOptions{
		MethodPattern: params.MethodPattern,
		ErrorsOnly:    params.ErrorsOnly,
		Limit:         params.Limit,
	}

	if params.Since != "" {
		d, err := time.ParseDuration(params.Since)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid since duration: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
		opts.Since = d
	}

	// Timeline mode: return human-readable activities instead of raw entries
	if params.Timeline {
		svc := timeline.New(adapter.log)
		activities, err := svc.RecentActivity(opts)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
		}

		return NewResultResponse(req.ID, map[string]any{
			keyEntries: activities,
			keyCount:   len(activities),
			"timeline": true,
			keyEnabled: true,
		})
	}

	entries, err := adapter.log.Query(opts)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		keyEntries: entries,
		keyCount:   len(entries),
		keyEnabled: true,
	})
}
