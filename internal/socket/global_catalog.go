package socket

import (
	"context"
	"encoding/json"

	"github.com/valksor/kvelmo/internal/catalog"
	"github.com/valksor/kvelmo/internal/page"
)

var catalogInstance *catalog.Catalog

// SetCatalog sets the global catalog instance.
func SetCatalog(c *catalog.Catalog) {
	catalogInstance = c
}

func (g *GlobalSocket) handleCatalogList(_ context.Context, req *Request) (*Response, error) {
	if catalogInstance == nil {
		return NewResultResponse(req.ID, map[string]any{"templates": []any{}})
	}

	var params struct {
		Page    int `json:"page,omitempty"`
		PerPage int `json:"per_page,omitempty"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	templates, err := catalogInstance.List()
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	pg := page.NewPage(templates, paginationWithDefault(params.Page, params.PerPage))

	return NewResultResponse(req.ID, map[string]any{
		"templates": pg.Items,
		keyTotal:    pg.Total,
		"page":      pg.PageNum,
		"per_page":  pg.PerPage,
		"has_next":  pg.HasNext,
	})
}

func (g *GlobalSocket) handleCatalogGet(_ context.Context, req *Request) (*Response, error) {
	if catalogInstance == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "catalog not configured"), nil
	}

	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	tmpl, err := catalogInstance.Get(params.Name)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, tmpl)
}

func (g *GlobalSocket) handleCatalogImport(_ context.Context, req *Request) (*Response, error) {
	if catalogInstance == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "catalog not configured"), nil
	}

	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	if err := catalogInstance.Import(params.Path); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{"success": true})
}
