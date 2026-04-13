package socket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/valksor/kvelmo/internal/activitylog"
	"github.com/valksor/kvelmo/internal/report"
)

// handleReportGenerate produces a structured compliance report from activity
// log entries and current worktree task data.
func (g *GlobalSocket) handleReportGenerate(_ context.Context, req *Request) (*Response, error) {
	var params struct {
		Format string `json:"format"`
		Since  string `json:"since"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	since := 30 * 24 * time.Hour
	if params.Since != "" {
		if d, err := parseSinceDuration(params.Since); err == nil {
			since = d
		}
	}

	// Collect activity entries.
	var entries []activitylog.Entry
	if g.server.activityLogger != nil {
		if adapter, ok := g.server.activityLogger.(*activityLogAdapter); ok {
			if e, err := adapter.log.Query(activitylog.QueryOptions{Since: since}); err == nil {
				entries = e
			}
		}
	}

	// Collect task info from active worktrees.
	g.mu.RLock()
	tasks := make([]report.TaskInfo, 0, len(g.worktrees))
	for _, wt := range g.worktrees {
		tasks = append(tasks, report.TaskInfo{
			ID:    wt.ID,
			Path:  wt.Path,
			State: wt.State,
		})
	}
	g.mu.RUnlock()

	period := params.Since
	if period == "" {
		period = "30d"
	}

	r := report.Generate(entries, tasks, period)
	md := report.FormatMarkdown(r)

	return NewResultResponse(req.ID, map[string]any{
		"markdown": md,
		"report":   r,
	})
}
