package memory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// DocumentOutcome tracks the result of the task that produced a document.
type DocumentOutcome struct {
	Success            bool `json:"success"`
	PRMerged           bool `json:"pr_merged"`
	CIPassedFirstTry   bool `json:"ci_passed_first_try"`
	HumanChangesNeeded bool `json:"human_changes_needed"`
}

// OutcomeScoreBoost returns a score modifier based on the outcome.
// Successful tasks get +0.1, failed get -0.05, nil outcome gets 0.
// Partial success (e.g. succeeded but required human changes) gets a reduced boost.
func OutcomeScoreBoost(outcome *DocumentOutcome) float64 {
	if outcome == nil {
		return 0
	}

	if !outcome.Success {
		return -0.05
	}

	boost := 0.1

	// Reduce boost if human intervention was needed — the solution was not fully autonomous.
	if outcome.HumanChangesNeeded {
		boost -= 0.03
	}

	// Extra boost for clean CI on first try — high-confidence solution.
	if outcome.CIPassedFirstTry {
		boost += 0.02
	}

	// Extra boost for merged PRs — validated in production.
	if outcome.PRMerged {
		boost += 0.02
	}

	return boost
}

// LinkOutcome finds all documents with matching taskID and sets their Outcome field.
// This persists the outcome data alongside the document on disk.
func (vs *VectorStore) LinkOutcome(ctx context.Context, taskID string, outcome DocumentOutcome) error {
	if taskID == "" {
		return errors.New("link outcome: taskID is required")
	}

	vs.mu.Lock()
	var toUpdate []*Document
	for _, doc := range vs.documents {
		if doc.TaskID == taskID {
			doc.Outcome = &outcome
			toUpdate = append(toUpdate, doc)
		}
	}
	// Copy for persistence outside the lock.
	copies := make([]Document, len(toUpdate))
	for i, doc := range toUpdate {
		copies[i] = *doc
	}
	vs.mu.Unlock()

	if len(copies) == 0 {
		slog.Debug("link outcome: no documents found for task", "task_id", taskID)

		return nil
	}

	var firstErr error
	for i := range copies {
		if err := vs.persist(&copies[i]); err != nil {
			slog.Warn("link outcome: persist failed", "id", copies[i].ID, "error", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("persist outcome for %s: %w", copies[i].ID, err)
			}
		}
	}

	if firstErr == nil {
		slog.Info("linked outcome to documents", "task_id", taskID, "count", len(copies), "success", outcome.Success)
	}

	return firstErr
}

// GetDocumentsForTask returns all documents associated with a task ID.
// Results include outcome data when present.
func (vs *VectorStore) GetDocumentsForTask(_ context.Context, taskID string) []*Document {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	var results []*Document
	for _, doc := range vs.documents {
		if doc.TaskID == taskID {
			cp := *doc
			results = append(results, &cp)
		}
	}

	return results
}
