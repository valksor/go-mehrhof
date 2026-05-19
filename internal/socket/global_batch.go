package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
)

// BatchParams is the request for tasks.batch.
type BatchParams struct {
	Action string            `json:"action"`           // "plan", "implement", "review", "submit", "abort", "reset", "stop"
	Filter map[string]string `json:"filter,omitempty"` // Optional: {"state": "reviewing"} to filter targets
}

// BatchResultItem is the result for a single worktree in a batch operation.
type BatchResultItem struct {
	Path    string `json:"path"`
	State   string `json:"state"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func (g *GlobalSocket) handleBatch(ctx context.Context, req *Request) (*Response, error) {
	var params BatchParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.Action == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "action is required"), nil
	}

	// Validate action — the action name doubles as the RPC method name.
	// pause/resume are state-machine events but have no batchable worktree RPC
	// handler yet; omit them so batch requests fail loud at the dispatcher
	// rather than fail at the worktree with method-not-found.
	validActions := []string{
		actionPlan, actionImplement, actionReview, actionSubmit,
		actionAbort, actionReset, actionStop,
	}
	if !slices.Contains(validActions, params.Action) {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, fmt.Sprintf("invalid action %q (valid: plan, implement, review, submit, abort, reset, stop)", params.Action)), nil
	}
	rpcMethod := params.Action

	// Get all registered worktrees
	g.mu.RLock()
	worktrees := make([]WorktreeInfo, 0, len(g.worktrees))
	for _, wt := range g.worktrees {
		worktrees = append(worktrees, *wt)
	}
	g.mu.RUnlock()

	// Dispatch action to each matching worktree
	var results []BatchResultItem
	stateFilter := params.Filter["state"]
	tagFilter := params.Filter["tag"]
	matchFilter := params.Filter["match"]

	for _, wt := range worktrees {
		// Apply path match filter before connecting
		if matchFilter != "" && !strings.Contains(wt.Path, matchFilter) {
			continue
		}

		// Query worktree status to check state/tag filters
		client, err := NewClient(wt.SocketPath, WithTimeout(3*time.Second))
		if err != nil {
			results = append(results, BatchResultItem{
				Path:  wt.Path,
				Error: "connect failed: " + err.Error(),
			})

			continue
		}

		statusCtx, statusCancel := context.WithTimeout(ctx, 3*time.Second)
		resp, err := client.Call(statusCtx, "status", nil)
		statusCancel()
		if err != nil {
			_ = client.Close()
			results = append(results, BatchResultItem{
				Path:  wt.Path,
				Error: "status check failed: " + err.Error(),
			})

			continue
		}

		var status struct {
			State string   `json:"state"`
			Tags  []string `json:"tags"`
		}
		if err := json.Unmarshal(resp.Result, &status); err != nil {
			_ = client.Close()

			continue
		}

		// Apply state filter
		if stateFilter != "" && status.State != stateFilter {
			_ = client.Close()

			continue
		}

		// Apply tag filter (case-insensitive to match archive filtering)
		if tagFilter != "" && !slices.ContainsFunc(status.Tags, func(t string) bool {
			return strings.EqualFold(t, tagFilter)
		}) {
			_ = client.Close()

			continue
		}

		// Skip idle worktrees
		if status.State == string(StateNone) {
			_ = client.Close()

			continue
		}

		// Dispatch action
		actionCtx, actionCancel := context.WithTimeout(ctx, 10*time.Second)
		_, err = client.Call(actionCtx, rpcMethod, nil)
		actionCancel()
		_ = client.Close()

		item := BatchResultItem{
			Path:    wt.Path,
			State:   status.State,
			Success: err == nil,
		}
		if err != nil {
			item.Error = err.Error()
			slog.Warn("batch action failed", "action", params.Action, "path", wt.Path, "error", err)
		}
		results = append(results, item)
	}

	return NewResultResponse(req.ID, map[string]any{
		"action":  params.Action,
		"results": results,
		keyTotal:  len(results),
	})
}
