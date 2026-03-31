package socket

import (
	"context"
	"encoding/json"
)

// --- Checkpoint Handlers ---

type UndoParams struct {
	Steps int `json:"steps"`
}

func (w *WorktreeSocket) handleUndo(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params UndoParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}
	if params.Steps < 1 {
		params.Steps = 1
	}

	for range params.Steps {
		if err := w.conductor.Undo(ctx); err != nil {
			return NewErrorResponse(req.ID, -32603, err.Error()), nil
		}
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "undone",
		"steps":  params.Steps,
		"state":  w.conductor.State(),
	})
}

type RedoParams struct {
	Steps int `json:"steps"`
}

func (w *WorktreeSocket) handleRedo(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params RedoParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}
	if params.Steps < 1 {
		params.Steps = 1
	}

	for range params.Steps {
		if err := w.conductor.Redo(ctx); err != nil {
			return NewErrorResponse(req.ID, -32603, err.Error()), nil
		}
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "redone",
		"steps":  params.Steps,
		"state":  w.conductor.State(),
	})
}

// CheckpointGotoParams holds params for checkpoint.goto.
type CheckpointGotoParams struct {
	SHA string `json:"sha"`
}

func (w *WorktreeSocket) handleCheckpointGoto(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params CheckpointGotoParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.SHA == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "sha is required"), nil
	}

	if err := w.conductor.GotoCheckpoint(ctx, params.SHA); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "ok",
		"sha":    params.SHA,
		"state":  w.conductor.State(),
	})
}

// handleCheckpointPreview returns the diff between HEAD and a target checkpoint SHA.
func (w *WorktreeSocket) handleCheckpointPreview(ctx context.Context, req *Request) (*Response, error) {
	if w.repo == nil {
		return NewErrorResponse(req.ID, -32600, "no git repository"), nil
	}

	var params CheckpointGotoParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.SHA == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "sha is required"), nil
	}

	diff, err := w.repo.DiffAgainst(ctx, params.SHA, false)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	stat, _ := w.repo.DiffAgainst(ctx, params.SHA, true)

	return NewResultResponse(req.ID, map[string]any{
		"sha":  params.SHA,
		"diff": diff,
		"stat": stat,
	})
}

// CheckpointInfo holds a checkpoint SHA enriched with git commit metadata.
type CheckpointInfo struct {
	SHA       string `json:"sha"`
	Message   string `json:"message"`
	Author    string `json:"author"`
	Timestamp string `json:"timestamp"`
	State     string `json:"state,omitempty"` // Conductor state at checkpoint time (from persisted metadata)
}

func (w *WorktreeSocket) handleCheckpoints(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewResultResponse(req.ID, map[string]any{
			"checkpoints": []CheckpointInfo{},
			"redo_stack":  []CheckpointInfo{},
		})
	}

	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewResultResponse(req.ID, map[string]any{
			"checkpoints": []CheckpointInfo{},
			"redo_stack":  []CheckpointInfo{},
		})
	}

	meta := wu.CheckpointMeta
	enrich := func(shas []string) []CheckpointInfo {
		result := make([]CheckpointInfo, 0, len(shas))
		for _, sha := range shas {
			info := CheckpointInfo{SHA: sha}
			if w.repo != nil {
				if entry, err := w.repo.CommitInfo(ctx, sha); err == nil {
					info.Message = entry.Message
					info.Author = entry.Author
					info.Timestamp = entry.Date
				}
			}
			if m, ok := meta[sha]; ok {
				info.State = m.State
			}
			result = append(result, info)
		}

		return result
	}

	return NewResultResponse(req.ID, map[string]any{
		"checkpoints": enrich(wu.Checkpoints),
		"redo_stack":  enrich(wu.RedoStack),
	})
}
