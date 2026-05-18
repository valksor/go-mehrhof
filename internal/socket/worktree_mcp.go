package socket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/valksor/kvelmo/internal/conductor"
)

// --- MCP-facing handlers (mcp.* methods) ---
//
// These handlers expose a narrow surface that the kvelmo MCP server
// (cmd/kvelmo/commands/mcp.go) translates into MCP tool calls. They are
// invoked by the claudemcp adapter's spawned MCP subprocess on behalf of an
// interactive Claude Code session, so all input is treated as untrusted: the
// worktree boundary is enforced via the existing per-worktree socket.
//
// Naming convention: every method is prefixed with `mcp.` to keep the MCP
// surface clearly grouped and avoid colliding with the established RPCs that
// drive the CLI/web UI.

// MCPTaskGetResult is returned by mcp.task.get.
type MCPTaskGetResult struct {
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	ExternalID  string `json:"external_id,omitempty"`
	State       string `json:"state"`
}

func (w *WorktreeSocket) handleMCPTaskGet(_ context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}
	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewErrorResponse(req.ID, ErrCodeNotFound, "no task loaded"), nil
	}

	return NewResultResponse(req.ID, MCPTaskGetResult{
		ID:          wu.ID,
		Title:       wu.Title,
		Description: wu.Description,
		ExternalID:  wu.ExternalID,
		State:       string(w.conductor.State()),
	})
}

// MCPSpecificationsResult is returned by mcp.task.specifications.
type MCPSpecificationsResult struct {
	Specifications []MCPSpecification `json:"specifications"`
}

type MCPSpecification struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

type mcpSpecificationsParams struct {
	IncludeContent bool `json:"include_content,omitempty"`
}

func (w *WorktreeSocket) handleMCPSpecifications(_ context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}
	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewErrorResponse(req.ID, ErrCodeNotFound, "no task loaded"), nil
	}
	var params mcpSpecificationsParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	const maxSpecBytes = 10 * 1024 * 1024
	out := make([]MCPSpecification, 0, len(wu.Specifications))
	for _, p := range wu.Specifications {
		spec := MCPSpecification{Path: p}
		if params.IncludeContent {
			if info, err := os.Stat(p); err == nil && info.Size() <= maxSpecBytes {
				if data, err := os.ReadFile(p); err == nil {
					spec.Content = string(data)
				}
			}
		}
		out = append(out, spec)
	}

	return NewResultResponse(req.ID, MCPSpecificationsResult{Specifications: out})
}

// MCPFileReadParams holds params for mcp.files.read.
type MCPFileReadParams struct {
	Path string `json:"path"`
}

// MCPFileReadResult is returned by mcp.files.read.
type MCPFileReadResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

func (w *WorktreeSocket) handleMCPFileRead(_ context.Context, req *Request) (*Response, error) {
	var params MCPFileReadParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC handlers surface errors via Response.Error
		}
	}
	if params.Path == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "path is required"), nil
	}

	abs, err := w.resolveWithinWorktree(params.Path)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	info, err := os.Stat(abs)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeNotFound, err.Error()), nil
	}
	if info.IsDir() {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "path is a directory"), nil
	}
	const maxFileBytes = 10 * 1024 * 1024
	if info.Size() > maxFileBytes {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), maxFileBytes)), nil
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, MCPFileReadResult{
		Path:    params.Path,
		Content: string(data),
		Size:    info.Size(),
	})
}

// resolveWithinWorktree ensures the given path stays within the worktree root.
// Delegates to ValidatePath which calls filepath.EvalSymlinks on both sides
// so a symlink inside the worktree pointing at /etc/passwd does not escape.
func (w *WorktreeSocket) resolveWithinWorktree(path string) (string, error) {
	if w.path == "" {
		return "", errors.New("worktree path not configured")
	}

	return ValidatePath(w.path, path)
}

// MCPArtifactsSaveParams holds params for mcp.artifacts.save.
type MCPArtifactsSaveParams struct {
	Kind    string `json:"kind"`
	Content string `json:"content"`
}

// MCPArtifactsSaveResult is returned by mcp.artifacts.save.
type MCPArtifactsSaveResult struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

func (w *WorktreeSocket) handleMCPArtifactsSave(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}
	var params MCPArtifactsSaveParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC handlers surface errors via Response.Error
		}
	}
	if params.Kind == "" || params.Content == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "kind and content are required"), nil
	}
	path, err := w.conductor.SaveArtifact(ctx, params.Kind, params.Content)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, MCPArtifactsSaveResult{Path: path, Kind: params.Kind})
}

// MCPCheckpointParams holds params for mcp.checkpoints.create.
type MCPCheckpointParams struct {
	Message string `json:"message"`
}

// MCPCheckpointResult is returned by mcp.checkpoints.create.
type MCPCheckpointResult struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

func (w *WorktreeSocket) handleMCPCheckpoint(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}
	var params MCPCheckpointParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC handlers surface errors via Response.Error
		}
	}
	if params.Message == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "message is required"), nil
	}
	sha, err := w.conductor.CreateCheckpoint(ctx, params.Message)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, MCPCheckpointResult{SHA: sha, Message: params.Message})
}

// MCPSignalCompleteParams holds params for mcp.signal.complete.
type MCPSignalCompleteParams struct {
	Phase   string `json:"phase"` // "plan", "implement", "simplify", "optimize", "review"
	Summary string `json:"summary,omitempty"`
}

// MCPSignalResult is returned by signal handlers.
type MCPSignalResult struct {
	OK        bool   `json:"ok"`
	State     string `json:"state,omitempty"`
	Finalized bool   `json:"finalized,omitempty"`
}

// phaseToCompletionEvent maps a phase name to its `*Done` conductor event.
func phaseToCompletionEvent(phase string) (conductor.Event, error) {
	switch phase {
	case conductor.PhasePlan:
		return conductor.EventPlanDone, nil
	case conductor.PhaseImplement:
		return conductor.EventImplementDone, nil
	case conductor.PhaseSimplify:
		return conductor.EventSimplifyDone, nil
	case conductor.PhaseOptimize:
		return conductor.EventOptimizeDone, nil
	case conductor.PhaseReview:
		return conductor.EventReviewDone, nil
	}

	return "", fmt.Errorf("unknown phase: %s", phase)
}

// handleMCPSignalComplete acknowledges the LLM's completion signal. The
// actual phase finalization (detect specs, copy to repo, checkpoint, state
// advance) is driven by the existing worker-pool/graph path: the adapter's
// pump emits agent.EventComplete in response to the rendezvous event that
// follows this RPC, which the worker pool consumes, which causes the graph
// scheduler to fire handleGraphCompletion → finalizePhaseLocked → state
// machine dispatch. Doing the work here would duplicate it (and would run
// before the state machine has advanced, leading to a misleading response
// state value).
func (w *WorktreeSocket) handleMCPSignalComplete(_ context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}
	var params MCPSignalCompleteParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC handlers surface errors via Response.Error
		}
	}
	if params.Phase == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "phase is required"), nil
	}
	if _, err := phaseToCompletionEvent(params.Phase); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	return NewResultResponse(req.ID, MCPSignalResult{OK: true, State: string(w.conductor.State())})
}

// MCPSignalFailureParams holds params for mcp.signal.failure.
type MCPSignalFailureParams struct {
	Phase     string `json:"phase"`
	Reason    string `json:"reason,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

// handleMCPSignalFailure acknowledges the LLM's failure signal. Same as
// handleMCPSignalComplete: the actual state-machine failure path is driven
// by the adapter's pump emitting agent.EventError (in response to the
// rendezvous event triggered by this RPC's caller). The worker pool sees
// EventError and the existing failure-policy path takes over.
//
// If the rendezvous dial to the adapter fails (e.g. the adapter has already
// cleaned up its socket), the spawned claude process will exit normally
// after this RPC returns; the pump then emits "claude exited without
// signalling completion" — less specific than the LLM's reason, but the
// task still reaches the failure state via the normal job path.
func (w *WorktreeSocket) handleMCPSignalFailure(_ context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}
	var params MCPSignalFailureParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC handlers surface errors via Response.Error
		}
	}

	return NewResultResponse(req.ID, MCPSignalResult{OK: true, State: string(w.conductor.State())})
}
