package socket

import (
	"context"

	"github.com/valksor/kvelmo/internal/policy"
)

// handlePolicyCheck evaluates workflow policies against the current task state.
func (w *WorktreeSocket) handlePolicyCheck(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewResultResponse(req.ID, map[string]any{
			"violations": []any{},
			keyMessage:   "no active task",
		})
	}

	cfg := w.conductor.GetEffectiveSettings()
	policyCfg := policy.Settings{
		RequiredPhases:      cfg.Workflow.Policy.RequiredPhases,
		SensitivePaths:      cfg.Workflow.Policy.SensitivePaths,
		MinSpecSections:     cfg.Workflow.Policy.MinSpecSections,
		RequireSecurityScan: cfg.Workflow.Policy.RequireSecurityScan,
	}
	for _, dr := range cfg.Workflow.Policy.DocRequirements {
		policyCfg.DocRequirements = append(policyCfg.DocRequirements, policy.DocRequirement{
			Trigger:  dr.Trigger,
			Requires: dr.Requires,
		})
	}

	state := string(w.conductor.Machine().State())
	violations := policy.Evaluate(policyCfg, "", state, wu.Specifications, nil)
	unified := policy.ViolationsToFindings(violations)

	return NewResultResponse(req.ID, map[string]any{
		"violations": violations,
		keyFindings:  unified,
		"blocking":   policy.HasBlockingViolation(violations),
	})
}
