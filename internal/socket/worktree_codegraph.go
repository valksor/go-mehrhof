package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/valksor/kvelmo/internal/codegraph"
)

func (w *WorktreeSocket) codegraphDB(ctx context.Context) (*codegraph.Graph, error) {
	w.codegraphOnce.Do(func() {
		dbPath := filepath.Join(w.path, ".kvelmo", "codegraph.db")
		w.codegraphInst, w.codegraphInitErr = codegraph.New(ctx, dbPath)
	})
	if w.codegraphInitErr != nil {
		return nil, w.codegraphInitErr
	}

	return w.codegraphInst, nil
}

func (w *WorktreeSocket) handleCodegraphStats(ctx context.Context, req *Request) (*Response, error) {
	g, err := w.codegraphDB(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("open codegraph: %s", err)), nil
	}

	return NewResultResponse(req.ID, g.Stats(ctx))
}

func (w *WorktreeSocket) handleCodegraphIndex(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Path string `json:"path"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	dir := params.Path
	if dir == "" {
		dir = w.path
	} else {
		validated, err := ValidatePath(w.path, dir)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "path must be within worktree"), nil //nolint:nilerr // JSON-RPC error response
		}
		dir = validated
	}

	g, err := w.codegraphDB(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("open codegraph: %s", err)), nil
	}

	if err := g.IndexDirectory(ctx, dir); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("index failed: %s", err)), nil
	}

	stats := g.Stats(ctx)

	return NewResultResponse(req.ID, map[string]any{
		"files":   stats["files"],
		"symbols": stats["symbols"],
		"edges":   stats["edges"],
	})
}

func (w *WorktreeSocket) handleCodegraphSearch(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Name    string `json:"name"`
		Pattern bool   `json:"pattern"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.Name == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "name is required"), nil
	}

	g, err := w.codegraphDB(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("open codegraph: %s", err)), nil
	}

	var symbols []codegraph.Symbol
	if params.Pattern {
		symbols, err = g.QuerySymbolPattern(ctx, params.Name)
	} else {
		symbols, err = g.QuerySymbol(ctx, params.Name)
	}
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("search failed: %s", err)), nil
	}

	if symbols == nil {
		symbols = []codegraph.Symbol{}
	}

	return NewResultResponse(req.ID, map[string]any{"symbols": symbols})
}

func (w *WorktreeSocket) handleCodegraphCallers(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Name string `json:"name"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.Name == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "name is required"), nil
	}

	g, err := w.codegraphDB(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("open codegraph: %s", err)), nil
	}

	callers, err := g.QueryCallersOf(ctx, params.Name)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("query failed: %s", err)), nil
	}

	if callers == nil {
		callers = []codegraph.Symbol{}
	}

	return NewResultResponse(req.ID, map[string]any{"callers": callers})
}

func (w *WorktreeSocket) handleCodegraphDeps(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Package string `json:"package"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.Package == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "package is required"), nil
	}

	g, err := w.codegraphDB(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("open codegraph: %s", err)), nil
	}

	deps, err := g.QueryDependenciesOf(ctx, params.Package)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("query failed: %s", err)), nil
	}

	if deps == nil {
		deps = []string{}
	}

	return NewResultResponse(req.ID, map[string]any{"dependencies": deps})
}
