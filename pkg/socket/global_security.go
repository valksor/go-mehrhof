package socket

import (
	"context"
	"encoding/json"

	"github.com/valksor/kvelmo/pkg/security"
)

type securityScanParams struct {
	Dir string `json:"dir"`
}

func (g *GlobalSocket) handleSecurityScan(ctx context.Context, req *Request) (*Response, error) {
	var params securityScanParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	if params.Dir == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "dir is required"), nil
	}

	runner := security.NewRunner()
	reports, err := runner.Run(ctx, params.Dir)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	// Convert to unified findings model.
	unified := security.ReportsToFindings(reports)

	var scanners []string
	for _, r := range reports {
		scanners = append(scanners, r.Scanner)
	}

	return NewResultResponse(req.ID, map[string]any{
		"findings": unified,
		"count":    len(unified),
		"scanners": scanners,
	})
}
