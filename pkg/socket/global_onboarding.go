package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/valksor/kvelmo/pkg/onboarding"
)

// onboardingMu protects concurrent access to the onboarding tracker file.
// The tracker is file-backed with no internal locking, so we serialize
// read-modify-write operations at the handler level.
var onboardingMu sync.Mutex

func withTracker[T any](fn func(*onboarding.Tracker) (T, error)) (T, error) {
	onboardingMu.Lock()
	defer onboardingMu.Unlock()

	return fn(onboarding.New(""))
}

func (g *GlobalSocket) handleOnboardingStatus(_ context.Context, req *Request) (*Response, error) {
	type statusResult struct {
		steps    map[onboarding.Step]bool
		complete bool
	}

	result, err := withTracker(func(t *onboarding.Tracker) (statusResult, error) {
		steps, err := t.Status()
		if err != nil {
			return statusResult{}, err
		}
		complete, err := t.IsComplete()
		if err != nil {
			return statusResult{}, err
		}

		return statusResult{steps: steps, complete: complete}, nil
	})
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("onboarding status: %v", err)), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"steps":    result.steps,
		"complete": result.complete,
	})
}

func (g *GlobalSocket) handleOnboardingComplete(_ context.Context, req *Request) (*Response, error) {
	var params struct {
		Step onboarding.Step `json:"step"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.Step == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "step is required"), nil
	}

	_, err := withTracker(func(t *onboarding.Tracker) (struct{}, error) {
		return struct{}{}, t.Complete(params.Step)
	})
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("onboarding complete: %v", err)), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"completed": params.Step,
	})
}

func (g *GlobalSocket) handleOnboardingReset(_ context.Context, req *Request) (*Response, error) {
	_, err := withTracker(func(t *onboarding.Tracker) (struct{}, error) {
		return struct{}{}, t.Reset()
	})
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("onboarding reset: %v", err)), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"reset": true,
	})
}
