package socket

import (
	"context"
	"encoding/json"

	"github.com/valksor/kvelmo/settings"
)

// handleConfigCheck compares global and project settings, reporting drift.
func (g *GlobalSocket) handleConfigCheck(_ context.Context, req *Request) (*Response, error) {
	_, global, project, err := settings.LoadEffective("")
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "load settings: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	globalJSON, err := json.Marshal(global)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "marshal global: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	projectJSON, err := json.Marshal(project)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "marshal project: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	var globalMap, projectMap map[string]any
	if err := json.Unmarshal(globalJSON, &globalMap); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "parse global: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}
	if err := json.Unmarshal(projectJSON, &projectMap); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "parse project: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	drifts := settings.CheckConfigDrift(globalMap, projectMap)

	return NewResultResponse(req.ID, map[string]any{
		"drifts": drifts,
		"count":  len(drifts),
	})
}
