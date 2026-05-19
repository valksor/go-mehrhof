package socket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/valksor/kvelmo/metrics"
)

// timeSeriesStore is set via SetTimeSeriesStore to enable metrics.history queries.
var timeSeriesStore *metrics.TimeSeriesStore

// SetTimeSeriesStore sets the global time-series store for metrics history.
func SetTimeSeriesStore(ts *metrics.TimeSeriesStore) {
	timeSeriesStore = ts
}

func (g *GlobalSocket) handleMetricsHistory(_ context.Context, req *Request) (*Response, error) {
	if timeSeriesStore == nil {
		return NewResultResponse(req.ID, map[string]any{
			keyEntries: []any{},
			keyEnabled: false,
		})
	}

	var params struct {
		From string `json:"from"` // RFC3339
		To   string `json:"to"`   // RFC3339 (optional, defaults to now)
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	var from, to time.Time
	if params.From != "" {
		var err error
		from, err = time.Parse(time.RFC3339, params.From)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid from time: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	} else {
		// Default to last 24 hours
		from = time.Now().Add(-24 * time.Hour)
	}
	if params.To != "" {
		var err error
		to, err = time.Parse(time.RFC3339, params.To)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid to time: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	entries, err := timeSeriesStore.Query(from, to)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		keyEntries: entries,
		keyCount:   len(entries),
		keyEnabled: true,
	})
}
