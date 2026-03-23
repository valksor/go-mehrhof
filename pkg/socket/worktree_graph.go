package socket

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"

	"github.com/valksor/kvelmo/pkg/codegraph"
)

var (
	// graphCache stores per-worktree graph instances to avoid re-opening the DB
	// on every RPC call. The key is the worktree path.
	graphCache   = make(map[string]*codegraph.Graph)
	graphCacheMu sync.Mutex
)

// getOrCreateGraph returns a codegraph.Graph for the worktree, creating one if needed.
func getOrCreateGraph(ctx context.Context, worktreePath string) (*codegraph.Graph, error) {
	graphCacheMu.Lock()
	defer graphCacheMu.Unlock()

	if g, ok := graphCache[worktreePath]; ok {
		return g, nil
	}

	dbPath := filepath.Join(worktreePath, ".kvelmo", "codegraph.db")
	g, err := codegraph.New(ctx, dbPath)
	if err != nil {
		return nil, err
	}

	graphCache[worktreePath] = g

	return g, nil
}

// handleGraphIndex indexes the project directory (or a subdirectory) for the code graph.
func (w *WorktreeSocket) handleGraphIndex(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Dir string `json:"dir"` // optional subdirectory relative to worktree root
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	g, err := getOrCreateGraph(ctx, w.path)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "open graph: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	dir := w.path
	if params.Dir != "" {
		dir = filepath.Join(w.path, params.Dir)
	}

	if err := g.IndexDirectory(ctx, dir); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "index: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	return NewResultResponse(req.ID, g.Stats(ctx))
}

// handleGraphQuery queries symbols by name (exact or pattern with %).
func (w *WorktreeSocket) handleGraphQuery(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Name    string `json:"name"`
		Pattern string `json:"pattern"` // LIKE pattern, e.g. "Config%"
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.Name == "" && params.Pattern == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "name or pattern required"), nil
	}

	g, err := getOrCreateGraph(ctx, w.path)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "open graph: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	var symbols []codegraph.Symbol
	if params.Pattern != "" {
		symbols, err = g.QuerySymbolPattern(ctx, params.Pattern)
	} else {
		symbols, err = g.QuerySymbol(ctx, params.Name)
	}
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "query: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	if symbols == nil {
		symbols = []codegraph.Symbol{}
	}

	return NewResultResponse(req.ID, map[string]any{"symbols": symbols})
}

// handleGraphCallers finds callers of a function/method.
func (w *WorktreeSocket) handleGraphCallers(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Name string `json:"name"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.Name == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "name required"), nil
	}

	g, err := getOrCreateGraph(ctx, w.path)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "open graph: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	callers, err := g.QueryCallersOf(ctx, params.Name)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "query callers: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	if callers == nil {
		callers = []codegraph.Symbol{}
	}

	return NewResultResponse(req.ID, map[string]any{"callers": callers})
}

// handleGraphStats returns index statistics.
func (w *WorktreeSocket) handleGraphStats(ctx context.Context, req *Request) (*Response, error) {
	g, err := getOrCreateGraph(ctx, w.path)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "open graph: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	return NewResultResponse(req.ID, g.Stats(ctx))
}
