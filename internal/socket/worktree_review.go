package socket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/provision"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/settings"
)

// --- Review History Handlers ---

// ReviewListResult is the response for review.list.
type ReviewListResult struct {
	Reviews []storage.Review `json:"reviews"`
}

func (w *WorktreeSocket) handleReviewList(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		// No conductor: return empty list for basic sockets
		return NewResultResponse(req.ID, ReviewListResult{Reviews: []storage.Review{}})
	}

	reviews, err := w.conductor.ListReviews()
	if err != nil {
		// No task or no store: return empty list rather than error
		return NewResultResponse(req.ID, ReviewListResult{Reviews: []storage.Review{}})
	}

	return NewResultResponse(req.ID, ReviewListResult{Reviews: reviews})
}

// ReviewViewParams holds params for review.view.
type ReviewViewParams struct {
	Number int `json:"number"`
}

func (w *WorktreeSocket) handleReviewView(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params ReviewViewParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	review, err := w.conductor.GetReview(params.Number)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeNotFound, fmt.Sprintf("review %d not found", params.Number)), nil //nolint:nilerr // JSON-RPC error response
	}

	return NewResultResponse(req.ID, review)
}

// --- Quality Gate Handlers ---

type qualityRespondParams struct {
	PromptID string `json:"prompt_id"`
	Answer   bool   `json:"answer"`
}

func (w *WorktreeSocket) handleQualityRespond(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params qualityRespondParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	if params.PromptID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "prompt_id required"), nil
	}

	if err := w.conductor.RespondToPrompt(params.PromptID, params.Answer); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{"status": "answered"})
}

// handleContextResolve resolves a context reference to its content.
// Used by the web UI for @-mention preview and validation.
func (w *WorktreeSocket) handleContextResolve(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Type string `json:"type"` // "file", "symbol", "commit", "branch", "terminal", "url"
		Ref  string `json:"ref"`  // Reference to resolve
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}
	if params.Type == "" || params.Ref == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "type and ref are required"), nil
	}

	resolver := &conductor.ContextResolver{
		WorktreeRoot: w.path,
		Repo:         w.repo,
		Graph:        w.codegraphInst, // may be nil — symbol resolution falls back to grep
	}

	item := conductor.ContextItem{
		Type: conductor.ContextType(params.Type),
		Ref:  params.Ref,
	}

	resolved, err := resolver.Resolve(ctx, item)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	resp, err := NewResultResponse(req.ID, map[string]string{
		"label":   resolved.Label,
		"content": resolved.Content,
	})
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "encode result: "+err.Error()), nil //nolint:nilerr // JSON-RPC error
	}

	return resp, nil
}

// handleProvisionPreview returns what would be provisioned without executing.
func (w *WorktreeSocket) handleProvisionPreview(_ context.Context, req *Request) (*Response, error) {
	cfg, _, _, err := settings.LoadEffective(w.path)
	if err != nil {
		cfg = settings.DefaultSettings()
	}

	if !settings.BoolValue(cfg.Git.Provision.Enabled, true) {
		return NewResultResponse(req.ID, map[string]string{"status": "disabled"})
	}

	defaults := provision.DefaultOptions(w.path)
	userOpts := provision.Options{
		CopyPatterns:    cfg.Git.Provision.CopyPatterns,
		SymlinkPatterns: cfg.Git.Provision.SymlinkPatterns,
		SetupCommands:   cfg.Git.Provision.SetupCommands,
	}
	merged := provision.MergeOptions(defaults, userOpts)

	result, previewErr := provision.Preview(w.path, merged)
	if previewErr != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, previewErr.Error()), nil
	}

	resp, respErr := NewResultResponse(req.ID, result)
	if respErr != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "encode result: "+respErr.Error()), nil //nolint:nilerr // JSON-RPC error
	}

	return resp, nil
}
