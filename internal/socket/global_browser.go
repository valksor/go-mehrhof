package socket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"time"

	"github.com/valksor/kvelmo/internal/browser"
)

// --- Browser Handlers ---

// BrowserParams holds common browser operation params.
type BrowserParams struct {
	WorktreeID  string `json:"worktree_id,omitempty"`
	SessionName string `json:"session_name,omitempty"`
}

func (g *GlobalSocket) getBrowserOpts(params BrowserParams) *browser.ExecOptions {
	opts := &browser.ExecOptions{
		SessionName: params.SessionName,
	}

	// Get worktree path if specified
	if params.WorktreeID != "" {
		g.mu.RLock()
		if wt, ok := g.worktrees[params.WorktreeID]; ok {
			opts.WorktreePath = wt.Path
		}
		g.mu.RUnlock()
	}

	return opts
}

func (g *GlobalSocket) handleBrowserSnapshot(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Snapshot(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserEvalParams holds params for browser.eval.
type BrowserEvalParams struct {
	BrowserParams

	JS string `json:"js"`
}

func (g *GlobalSocket) handleBrowserEval(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserEvalParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.JS == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "js is required"), nil
	}

	result, err := browser.Eval(ctx, g.getBrowserOpts(params.BrowserParams), params.JS)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserConsole(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Console(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserNetwork(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Network(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserScreenshotParams holds params for browser.screenshot.
type BrowserScreenshotParams struct {
	BrowserParams

	Path     string `json:"path,omitempty"`
	FullPage bool   `json:"full_page,omitempty"`
	Element  string `json:"element,omitempty"`
	Format   string `json:"format,omitempty"`
	Quality  int    `json:"quality,omitempty"`
}

func (g *GlobalSocket) handleBrowserScreenshot(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserScreenshotParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil
		}
	}

	screenshotOpts := &browser.ScreenshotOptions{
		Path:     params.Path,
		FullPage: params.FullPage,
		Element:  params.Element,
		Format:   params.Format,
		Quality:  params.Quality,
	}

	result, err := browser.Screenshot(ctx, g.getBrowserOpts(params.BrowserParams), screenshotOpts)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	if params.WorktreeID == "" {
		return NewResultResponse(req.ID, result)
	}

	// worktree_id provided — store via screenshots.capture
	g.mu.RLock()
	wt, ok := g.worktrees[params.WorktreeID]
	g.mu.RUnlock()
	if !ok {
		return NewResultResponse(req.ID, result)
	}

	imageData, err := os.ReadFile(result.Path)
	if err != nil {
		return NewResultResponse(req.ID, result)
	}

	format := params.Format
	if format == "" {
		format = "png"
	}

	captureParams, err := json.Marshal(map[string]any{
		"source": "user",
		"format": format,
		"data":   base64.StdEncoding.EncodeToString(imageData),
	})
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "failed to marshal params"), nil
	}

	client, err := NewClient(wt.SocketPath, WithTimeout(5*time.Second))
	if err != nil {
		return NewResultResponse(req.ID, result)
	}
	defer func() { _ = client.Close() }()

	captureResp, err := client.Call(ctx, "screenshots.capture", captureParams)
	if err != nil || captureResp == nil || captureResp.Result == nil {
		return NewResultResponse(req.ID, result)
	}

	var ss map[string]any
	if err := json.Unmarshal(captureResp.Result, &ss); err != nil {
		return NewResultResponse(req.ID, result)
	}

	return NewResultResponse(req.ID, map[string]any{
		"id":    ss["id"],
		keyPath: result.Path,
	})
}

// BrowserNavigateParams holds params for browser.navigate.
type BrowserNavigateParams struct {
	BrowserParams

	URL string `json:"url"`
}

func (g *GlobalSocket) handleBrowserNavigate(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserNavigateParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.URL == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "url is required"), nil
	}

	result, err := browser.Navigate(ctx, g.getBrowserOpts(params.BrowserParams), params.URL)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}
