package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/valksor/kvelmo/pkg/eventlog"
	"github.com/valksor/kvelmo/pkg/page"
)

// handleEventlogQuery returns event log entries matching optional filters.
func (w *WorktreeSocket) handleEventlogQuery(_ context.Context, req *Request) (*Response, error) {
	var params struct {
		Type    eventlog.EventType `json:"type,omitempty"`
		Phase   string             `json:"phase,omitempty"`
		Since   string             `json:"since,omitempty"`
		Until   string             `json:"until,omitempty"`
		Page    int                `json:"page,omitempty"`
		PerPage int                `json:"per_page,omitempty"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	opts := eventlog.QueryOptions{
		Type:  params.Type,
		Phase: params.Phase,
	}

	if params.Since != "" {
		dur, err := time.ParseDuration(params.Since)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, fmt.Sprintf("invalid since duration: %s", err)), nil
		}

		opts.Since = time.Now().Add(-dur)
	}

	if params.Until != "" {
		t, err := time.Parse(time.RFC3339, params.Until)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, fmt.Sprintf("invalid until time: %s", err)), nil
		}

		opts.Until = t
	}

	dir := filepath.Join(w.path, ".kvelmo")

	entries, err := eventlog.Query(dir, opts)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("query eventlog: %s", err)), nil
	}

	if entries == nil {
		entries = []eventlog.Entry{}
	}

	pg := page.NewPage(entries, paginationWithDefault(params.Page, params.PerPage))

	return NewResultResponse(req.ID, map[string]any{
		"entries":  pg.Items,
		"total":    pg.Total,
		"page":     pg.PageNum,
		"per_page": pg.PerPage,
		"has_next": pg.HasNext,
	})
}
