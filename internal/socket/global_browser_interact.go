package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/valksor/kvelmo/internal/browser"
)

// BrowserClickParams holds params for browser.click.
type BrowserClickParams struct {
	BrowserParams

	Selector string `json:"selector"`
}

func (g *GlobalSocket) handleBrowserClick(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserClickParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Click(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserTypeParams holds params for browser.type.
type BrowserTypeParams struct {
	BrowserParams

	Selector string `json:"selector"`
	Text     string `json:"text"`
}

func (g *GlobalSocket) handleBrowserType(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserTypeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Type(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector, params.Text)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserWaitParams holds params for browser.wait.
type BrowserWaitParams struct {
	BrowserParams

	Selector  string `json:"selector"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

func (g *GlobalSocket) handleBrowserWait(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserWaitParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Wait(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector, params.TimeoutMs)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserInstall(ctx context.Context, req *Request) (*Response, error) {
	if err := browser.Install(ctx); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("install runtime: %v", err)), nil
	}
	if err := browser.InstallBrowsers(ctx); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("install browsers: %v", err)), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"ok":      true,
		"message": "Browser runtime installed successfully",
		"path":    browser.Paths(),
	})
}

func (g *GlobalSocket) handleBrowserStatus(ctx context.Context, req *Request) (*Response, error) {
	installed := browser.IsInstalled()
	result := map[string]any{
		"installed":   installed,
		"runtime_dir": browser.Paths(),
		"binary_path": browser.BinaryPath(),
	}

	if installed {
		version, err := browser.Version()
		if err != nil {
			result["version_error"] = err.Error()
		} else {
			result["version"] = version
		}
	}

	cfg, err := browser.LoadConfig()
	if err != nil {
		result["config_error"] = err.Error()
	} else {
		result["config"] = cfg
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserConfigGet(ctx context.Context, req *Request) (*Response, error) {
	cfg, err := browser.LoadConfig()
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, cfg)
}

// BrowserConfigSetParams holds params for browser.config.set.
type BrowserConfigSetParams struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (g *GlobalSocket) handleBrowserConfigSet(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserConfigSetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	cfg, err := browser.LoadConfig()
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("load config: %v", err)), nil
	}

	switch strings.ToLower(params.Key) {
	case "headless":
		cfg.Headless = params.Value == "true" || params.Value == "1" || params.Value == "yes"
	case "browser":
		if params.Value != "chromium" && params.Value != "firefox" && params.Value != "webkit" {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "browser must be chromium, firefox, or webkit"), nil
		}
		cfg.Browser = params.Value
	case "profile":
		cfg.Profile = params.Value
	case "timeout":
		var timeout int
		if _, scanErr := fmt.Sscanf(params.Value, "%d", &timeout); scanErr != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "timeout must be a number"), nil //nolint:nilerr // JSON-RPC error response
		}
		cfg.Timeout = timeout
	default:
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "unknown config key: "+params.Key), nil
	}

	if err := cfg.Save(); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("save config: %v", err)), nil
	}

	return NewResultResponse(req.ID, cfg)
}

// BrowserFillParams holds params for browser.fill.
type BrowserFillParams struct {
	BrowserParams

	Selector string `json:"selector"`
	Value    string `json:"value"`
}

func (g *GlobalSocket) handleBrowserFill(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserFillParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Fill(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector, params.Value)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserSelectParams holds params for browser.select.
type BrowserSelectParams struct {
	BrowserParams

	Selector string   `json:"selector"`
	Values   []string `json:"values"`
}

func (g *GlobalSocket) handleBrowserSelect(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserSelectParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	if len(params.Values) == 0 {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "values is required"), nil
	}

	result, err := browser.Select(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector, params.Values...)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserHoverParams holds params for browser.hover.
type BrowserHoverParams struct {
	BrowserParams

	Selector string `json:"selector"`
}

func (g *GlobalSocket) handleBrowserHover(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserHoverParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Hover(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserFocusParams holds params for browser.focus.
type BrowserFocusParams struct {
	BrowserParams

	Selector string `json:"selector"`
}

func (g *GlobalSocket) handleBrowserFocus(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserFocusParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Focus(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserScrollParams holds params for browser.scroll.
type BrowserScrollParams struct {
	BrowserParams

	Direction string `json:"direction"` // up, down, left, right
	Amount    int    `json:"amount,omitempty"`
	Selector  string `json:"selector,omitempty"`
}

func (g *GlobalSocket) handleBrowserScroll(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserScrollParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Direction == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "direction is required"), nil
	}

	switch params.Direction {
	case "up", "down", "left", "right":
		// valid
	default:
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "direction must be one of: up, down, left, right"), nil
	}

	result, err := browser.Scroll(ctx, g.getBrowserOpts(params.BrowserParams), params.Direction, params.Amount, params.Selector)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserPressParams holds params for browser.press.
type BrowserPressParams struct {
	BrowserParams

	Key      string `json:"key"`
	Selector string `json:"selector,omitempty"`
}

func (g *GlobalSocket) handleBrowserPress(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserPressParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Key == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "key is required"), nil
	}

	result, err := browser.Press(ctx, g.getBrowserOpts(params.BrowserParams), params.Key, params.Selector)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserBack(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Back(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserForward(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Forward(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserReload(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Reload(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserDialogParams holds params for browser.dialog.
type BrowserDialogParams struct {
	BrowserParams

	Action string `json:"action"` // accept or dismiss
	Text   string `json:"text,omitempty"`
}

func (g *GlobalSocket) handleBrowserDialog(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserDialogParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Action != "accept" && params.Action != "dismiss" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "action must be 'accept' or 'dismiss'"), nil
	}

	result, err := browser.Dialog(ctx, g.getBrowserOpts(params.BrowserParams), params.Action, params.Text)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserUploadParams holds params for browser.upload.
type BrowserUploadParams struct {
	BrowserParams

	Selector string   `json:"selector"`
	Files    []string `json:"files"`
}

func (g *GlobalSocket) handleBrowserUpload(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserUploadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	if len(params.Files) == 0 {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "files is required"), nil
	}

	result, err := browser.Upload(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector, params.Files)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserPDFParams holds params for browser.pdf.
type BrowserPDFParams struct {
	BrowserParams

	Path      string `json:"path,omitempty"`
	Format    string `json:"format,omitempty"`
	Landscape bool   `json:"landscape,omitempty"`
}

func (g *GlobalSocket) handleBrowserPDF(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserPDFParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	pdfOpts := &browser.PDFOptions{
		Path:      params.Path,
		Format:    params.Format,
		Landscape: params.Landscape,
	}

	result, err := browser.GeneratePDF(ctx, g.getBrowserOpts(params.BrowserParams), pdfOpts)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}
