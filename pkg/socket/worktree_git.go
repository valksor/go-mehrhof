package socket

import (
	"context"
	"encoding/json"

	"github.com/valksor/kvelmo/pkg/git"
)

// --- Git Handlers ---

func (w *WorktreeSocket) handleGitStatus(ctx context.Context, req *Request) (*Response, error) {
	if w.repo == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidRequest, "no git repository"), nil
	}

	branch, err := w.repo.CurrentBranch(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	hasChanges, err := w.repo.HasUncommittedChanges(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	files, err := w.repo.DiffFilesWithStatus(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}
	if files == nil {
		files = []git.FileStatus{}
	}

	return NewResultResponse(req.ID, map[string]any{
		"branch":      branch,
		"has_changes": hasChanges,
		"files":       files,
	})
}

type GitDiffParams struct {
	Cached bool `json:"cached"`
}

func (w *WorktreeSocket) handleGitDiff(ctx context.Context, req *Request) (*Response, error) {
	if w.repo == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidRequest, "no git repository"), nil
	}

	var params GitDiffParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	diff, err := w.repo.Diff(ctx, params.Cached)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"diff": diff,
	})
}

type GitDiffAgainstParams struct {
	Ref  string `json:"ref"`
	Stat bool   `json:"stat"`
}

func (w *WorktreeSocket) handleGitDiffAgainst(ctx context.Context, req *Request) (*Response, error) {
	if w.repo == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidRequest, "no git repository"), nil
	}

	var params GitDiffAgainstParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.Ref == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "ref parameter is required"), nil
	}

	diff, err := w.repo.DiffAgainst(ctx, params.Ref, params.Stat)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"diff": diff,
	})
}

type GitLogParams struct {
	Count int `json:"count"`
}

func (w *WorktreeSocket) handleGitLog(ctx context.Context, req *Request) (*Response, error) {
	if w.repo == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidRequest, "no git repository"), nil
	}

	var params GitLogParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}
	if params.Count < 1 {
		params.Count = 10
	}

	entries, err := w.repo.Log(ctx, params.Count)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"entries": entries,
	})
}
