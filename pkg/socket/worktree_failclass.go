package socket

import (
	"context"

	"github.com/valksor/kvelmo/pkg/findings"
	"github.com/valksor/kvelmo/pkg/quality"
)

// handleFailclassStats returns failure classification statistics for the current
// task's quality gate findings.
func (w *WorktreeSocket) handleFailclassStats(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewResultResponse(req.ID, quality.FailClassifierStats{})
	}

	// Build findings from the quality gate error if present.
	var ff []findings.Finding
	if wu.QualityGateError != "" {
		ff = append(ff, findings.Finding{
			Message:  wu.QualityGateError,
			Category: findings.CategoryQuality,
			Source:   "quality_gate",
		})
	}

	classifier := quality.NewFailClassifier(nil)
	classified := classifier.Classify(ff)
	stats := classifier.Stats(classified)

	return NewResultResponse(req.ID, stats)
}
