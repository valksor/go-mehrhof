package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/activitylog"
	"github.com/valksor/kvelmo/pkg/metrics"
)

// handleExport returns task and metrics data for external analysis.
func (g *GlobalSocket) handleExport(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Format  string `json:"format"`
		Since   string `json:"since"`
		Include string `json:"include"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	result := map[string]any{
		"format": params.Format,
	}

	// Include metrics snapshot
	result["metrics"] = metrics.Global().Snapshot()

	// Include active tasks from all worktrees, enriched with checkpoint data
	g.mu.RLock()
	tasks := make([]map[string]any, 0, len(g.worktrees))
	for _, wt := range g.worktrees {
		task := map[string]any{
			"id":    wt.ID,
			"path":  wt.Path,
			"state": wt.State,
		}
		// Query worktree socket for checkpoint data (best-effort)
		if wt.SocketPath != "" {
			if checkpoints := queryWorktreeCheckpoints(ctx, wt.SocketPath); len(checkpoints) > 0 {
				task["checkpoints"] = checkpoints
			}
		}
		tasks = append(tasks, task)
	}
	g.mu.RUnlock()

	result["tasks"] = tasks

	// Include activity log entries if available
	if g.server.activityLogger != nil {
		if adapter, ok := g.server.activityLogger.(*activityLogAdapter); ok {
			since := 7 * 24 * time.Hour // default 7 days
			if params.Since != "" {
				if d, err := parseSinceDuration(params.Since); err == nil {
					since = d
				}
			}
			entries, err := adapter.log.Query(activitylog.QueryOptions{
				Since: since,
			})
			if err == nil {
				result["activity"] = entries
			}
		}
	}

	return NewResultResponse(req.ID, result)
}

// parseSinceDuration parses a duration string supporting both Go duration syntax
// and shorthand "Nd" for days.
func parseSinceDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if numStr, ok := strings.CutSuffix(s, "d"); ok {
		var days int
		if _, err := fmt.Sscanf(numStr, "%d", &days); err == nil {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}

	return 0, fmt.Errorf("invalid duration: %s", s)
}

// queryWorktreeCheckpoints queries a worktree socket for its checkpoint SHAs.
// Returns nil on any error (best-effort enrichment).
func queryWorktreeCheckpoints(ctx context.Context, socketPath string) []string {
	if !SocketExists(socketPath) {
		return nil
	}
	client, err := NewClient(socketPath, WithTimeout(5*time.Second))
	if err != nil {
		return nil
	}
	defer func() { _ = client.Close() }()

	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := client.Call(queryCtx, "checkpoints", nil)
	if err != nil {
		return nil
	}

	var result struct {
		Checkpoints []string `json:"checkpoints"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil
	}

	return result.Checkpoints
}
