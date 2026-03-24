package socket

import (
	"context"
	"encoding/json"
)

// handleAdversarialRun manually triggers an adversarial review.
func (w *WorktreeSocket) handleAdversarialRun(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	result, err := w.conductor.RunAdversarialReview(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"findings": result,
		"count":    len(result),
	})
}

// handleAdversarialResults returns results from the most recent adversarial review.
func (w *WorktreeSocket) handleAdversarialResults(_ context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	result := w.conductor.GetAdversarialFindings()
	if result == nil {
		// Return empty array instead of null for cleaner JSON.
		return NewResultResponse(req.ID, map[string]any{
			"findings": json.RawMessage("[]"),
			"count":    0,
		})
	}

	return NewResultResponse(req.ID, map[string]any{
		"findings": result,
		"count":    len(result),
	})
}
