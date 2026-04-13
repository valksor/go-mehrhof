package socket

import (
	"context"
	"encoding/json"

	"github.com/valksor/kvelmo/internal/conductor"
)

// FinishParams holds params for the task.finish handler.
type FinishParams struct {
	DeleteRemote bool `json:"delete_remote,omitempty"` // Delete the remote feature branch
	Force        bool `json:"force,omitempty"`         // Finish even if PR is not merged
}

func (w *WorktreeSocket) handleFinish(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params FinishParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	result, err := w.conductor.Finish(ctx, conductor.FinishOptions{
		DeleteRemoteBranch: params.DeleteRemote,
		Force:              params.Force,
	})
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"previous_branch":       result.PreviousBranch,
		"current_branch":        result.CurrentBranch,
		"branch_deleted":        result.BranchDeleted,
		"remote_branch_deleted": result.RemoteBranchDeleted,
	})
}

func (w *WorktreeSocket) handleRefresh(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	result, err := w.conductor.Refresh(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"task_id":             result.TaskID,
		"branch":              result.Branch,
		"pr_status":           result.PRStatus,
		"pr_merged":           result.PRMerged,
		"pr_url":              result.PRURL,
		"commits_behind_base": result.CommitsBehindBase,
		"action":              result.Action,
		"message":             result.Message,
	})
}

// RemoteApproveParams holds params for the remote.approve handler.
type RemoteApproveParams struct {
	Comment string `json:"comment,omitempty"`
}

func (w *WorktreeSocket) handleRemoteApprove(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params RemoteApproveParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if err := w.conductor.ApprovePR(ctx, params.Comment); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "approved",
		"state":  w.conductor.State(),
	})
}

// RemoteMergeParams holds params for the remote.merge handler.
type RemoteMergeParams struct {
	Method string `json:"method,omitempty"` // merge, squash, rebase (default: rebase)
}

func (w *WorktreeSocket) handleRemoteMerge(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params RemoteMergeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if err := w.conductor.MergePR(ctx, params.Method); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "merged",
		"state":  w.conductor.State(),
	})
}

func (w *WorktreeSocket) handleAbort(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	if err := w.conductor.Abort(ctx); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "aborted",
		"state":  w.conductor.State(),
	})
}

func (w *WorktreeSocket) handleStop(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	if err := w.conductor.Stop(ctx); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "stopped",
		"state":  w.conductor.State(),
	})
}

func (w *WorktreeSocket) handleReset(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	if err := w.conductor.Reset(ctx); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "reset",
		"state":  w.conductor.State(),
	})
}

// --- Abandon Handler ---

// AbandonParams holds params for the abandon handler.
type AbandonParams struct {
	KeepBranch bool `json:"keep_branch,omitempty"`
}

func (w *WorktreeSocket) handleAbandon(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params AbandonParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if err := w.conductor.Abandon(ctx, params.KeepBranch); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "abandoned",
	})
}

// --- Delete Handler ---

// DeleteParams holds params for the delete handler.
type DeleteParams struct {
	DeleteBranch bool `json:"delete_branch,omitempty"`
}

func (w *WorktreeSocket) handleDelete(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params DeleteParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if err := w.conductor.Delete(ctx, params.DeleteBranch); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "deleted",
	})
}

// --- Update Handler ---

// UpdateParams holds params for the update handler.
type UpdateParams struct{}

// UpdateResult is the response for the update handler.
type UpdateResult struct {
	Status           string `json:"status"`
	Changed          bool   `json:"changed"`
	NewSpecification string `json:"new_specification,omitempty"`
}

func (w *WorktreeSocket) handleUpdate(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	changed, specPath, err := w.conductor.UpdateTask(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	result := UpdateResult{
		Status:  "updated",
		Changed: changed,
	}
	if specPath != "" {
		result.NewSpecification = specPath
	}

	return NewResultResponse(req.ID, result)
}
