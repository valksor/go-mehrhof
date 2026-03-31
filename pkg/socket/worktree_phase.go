package socket

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"time"

	"github.com/valksor/kvelmo/pkg/conductor"
)

// --- Task Lifecycle Handlers ---

type StartParams struct {
	Source               string                  `json:"source"` // e.g., "github:owner/repo#123"
	UseWorktreeIsolation bool                    `json:"use_worktree_isolation,omitempty"`
	AutoAdvance          bool                    `json:"auto_advance,omitempty"`  // Auto-progress through plan → implement → review
	SkipPhases           []string                `json:"skip_phases,omitempty"`   // Per-invocation phases to skip (e.g., ["simplify", "optimize"])
	ContextItems         []conductor.ContextItem `json:"context_items,omitempty"` // Attached context references (@-mentions)
}

func (w *WorktreeSocket) handleStart(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params StartParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, -32602, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	if params.Source == "" {
		return NewErrorResponse(req.ID, -32602, "source is required"), nil
	}

	if err := w.conductor.Start(ctx, params.Source); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	// Enable auto-advance if requested
	if params.AutoAdvance {
		w.conductor.SetAutoAdvance(true)
	}

	// Set per-invocation skip phases
	if len(params.SkipPhases) > 0 {
		w.conductor.SetSkipPhases(params.SkipPhases)
	}

	// Attach context items (@-mentions) to the work unit
	if len(params.ContextItems) > 0 {
		w.conductor.SetContextItems(params.ContextItems)
	}

	// Worktree isolation is handled inside conductor.Start(). Log if active.
	if wu := w.conductor.WorkUnit(); wu != nil && wu.WorktreePath != "" {
		slog.Info("handleStart: worktree isolation active", "path", wu.WorktreePath, "branch", wu.Branch)
	}

	// Auto-advance: trigger next phase immediately after start.
	// If "plan" is in skip_phases, jump straight to implement.
	if params.AutoAdvance {
		skipPlan := slices.Contains(params.SkipPhases, "plan")
		go func() { //nolint:contextcheck // intentionally uses background context for async auto-advance
			bgCtx := context.Background()
			if skipPlan {
				if _, err := w.conductor.Implement(bgCtx); err != nil {
					slog.Warn("auto-advance: implement after start failed", "error", err)
				}
			} else {
				if _, err := w.conductor.Plan(bgCtx); err != nil {
					slog.Warn("auto-advance: plan after start failed", "error", err)
				}
			}
		}()
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "started",
		"state":  w.conductor.State(),
	})
}

type PlanParams struct {
	Prompt string `json:"prompt,omitempty"`  // Additional context for planning
	DryRun bool   `json:"dry_run,omitempty"` // Simulate without executing agent
}

func (w *WorktreeSocket) handlePlan(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params PlanParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil
		}
	}

	// Apply dry-run mode for this call
	prevDryRun := w.conductor.DryRunEnabled()
	if params.DryRun {
		w.conductor.SetDryRun(true)
		defer w.conductor.SetDryRun(prevDryRun)
	}

	// Submit planning job
	jobID, err := w.conductor.Plan(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "planning",
		"job_id": jobID,
		"state":  w.conductor.State(),
	})
}

type ImplementParams struct {
	Prompt string `json:"prompt,omitempty"`  // Additional context for implementation
	DryRun bool   `json:"dry_run,omitempty"` // Simulate without executing agent
}

func (w *WorktreeSocket) handleImplement(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params ImplementParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil
		}
	}

	// Apply dry-run mode for this call
	prevDryRun := w.conductor.DryRunEnabled()
	if params.DryRun {
		w.conductor.SetDryRun(true)
		defer w.conductor.SetDryRun(prevDryRun)
	}

	// Submit implementation job
	jobID, err := w.conductor.Implement(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "implementing",
		"job_id": jobID,
		"state":  w.conductor.State(),
	})
}

type OptimizeParams struct {
	Prompt string `json:"prompt,omitempty"`  // Additional context for optimization
	DryRun bool   `json:"dry_run,omitempty"` // Simulate without executing agent
}

func (w *WorktreeSocket) handleOptimize(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params OptimizeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil
		}
	}

	// Apply dry-run mode for this call
	prevDryRun := w.conductor.DryRunEnabled()
	if params.DryRun {
		w.conductor.SetDryRun(true)
		defer w.conductor.SetDryRun(prevDryRun)
	}

	// Submit optimization job
	jobID, err := w.conductor.Optimize(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "optimizing",
		"job_id": jobID,
		"state":  w.conductor.State(),
	})
}

type SimplifyParams struct {
	DryRun bool `json:"dry_run,omitempty"` // Simulate without executing agent
}

func (w *WorktreeSocket) handleSimplify(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params SimplifyParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil
		}
	}

	// Apply dry-run mode for this call
	prevDryRun := w.conductor.DryRunEnabled()
	if params.DryRun {
		w.conductor.SetDryRun(true)
		defer w.conductor.SetDryRun(prevDryRun)
	}

	jobID, err := w.conductor.Simplify(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "simplifying",
		"job_id": jobID,
		"state":  w.conductor.State(),
	})
}

func (w *WorktreeSocket) handleShutdown(ctx context.Context, req *Request) (*Response, error) {
	// Send response before shutting down.
	go func() {
		time.Sleep(50 * time.Millisecond)
		if err := w.server.Stop(); err != nil {
			slog.Error("shutdown: failed to stop server", "error", err)
		}
	}()

	return NewResultResponse(req.ID, map[string]string{"status": "shutting_down"})
}

type ReviewParams struct {
	Approve bool   `json:"approve"`
	Reject  bool   `json:"reject"`
	Message string `json:"message,omitempty"`
	Fix     bool   `json:"fix,omitempty"`   // Auto-fix issues after entering review
	Force   bool   `json:"force,omitempty"` // Allow re-entering review from already-reviewed state
}

func (w *WorktreeSocket) handleReview(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params ReviewParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if err := w.conductor.Review(ctx, params.Fix); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	// Record review result if approve/reject specified
	if params.Approve || params.Reject {
		w.conductor.AddReview(params.Approve, params.Message)
	}

	return NewResultResponse(req.ID, map[string]any{
		"status":  "reviewing",
		"state":   w.conductor.State(),
		"message": params.Message,
	})
}

type SubmitParams struct {
	Title        string            `json:"title,omitempty"`
	Body         string            `json:"body,omitempty"`
	Draft        bool              `json:"draft,omitempty"`
	Reviewers    []string          `json:"reviewers,omitempty"`
	Labels       []string          `json:"labels,omitempty"`
	DeleteBranch bool              `json:"delete_branch,omitempty"` // Delete local branch after submit
	DryRun       bool              `json:"dry_run,omitempty"`       // Preview PR without creating it
	Sections     map[string]string `json:"sections,omitempty"`      // Per-PR custom sections (not yet wired into PR body generation)
}

func (w *WorktreeSocket) handleSubmit(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params SubmitParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	// Dry-run: preview PR without creating it
	if params.DryRun {
		preview, err := w.conductor.PreviewSubmit(ctx)
		if err != nil {
			return NewErrorResponse(req.ID, -32603, err.Error()), nil
		}

		return NewResultResponse(req.ID, map[string]any{
			"status":  "preview",
			"preview": preview,
		})
	}

	if err := w.conductor.Submit(ctx, params.DeleteBranch); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "submitted",
		"state":  w.conductor.State(),
	})
}

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
