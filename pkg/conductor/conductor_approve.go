package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"slices"
	"time"

	"github.com/valksor/kvelmo/pkg/graph"
	"github.com/valksor/kvelmo/pkg/riskeval"
)

// approverIdentity returns a best-effort user identity string for audit purposes.
// If a configured identity is provided (from settings), it takes precedence.
func approverIdentity(configuredIdentity string) string {
	if configuredIdentity != "" {
		return configuredIdentity
	}
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if h, err := os.Hostname(); err == nil {
		return h
	}

	return "unknown"
}

// checkApproval verifies that the given event has been approved when required by policy.
// When risk-based approval is enabled, low-risk changes are auto-approved and high-risk
// changes require review regardless of explicit approval settings.
// Must be called with c.mu held (read or write).
func (c *Conductor) checkApproval(event Event) error {
	s := c.getEffectiveSettings()

	// Risk-based approval: evaluate risk and decide automatically.
	rba := s.Workflow.Policy.RiskBasedApproval
	if rba != nil && rba.Enabled && s.Workflow.Policy.ApprovalRequired[string(event)] {
		score := c.evaluateRisk()
		c.emitRiskEvaluated(score)

		autoThreshold := rba.AutoApproveThreshold
		if autoThreshold == 0 {
			autoThreshold = riskeval.DefaultLowThreshold
		}
		highThreshold := rba.HighRiskThreshold
		if highThreshold == 0 {
			highThreshold = riskeval.DefaultHighThreshold
		}

		if score.Score < autoThreshold {
			slog.Info("risk-based auto-approval",
				"event", event, "score", score.Score, "level", score.Level)
			return nil
		}

		if score.Score >= highThreshold {
			// High risk: always require explicit approval, no bypass.
			record, ok := c.workUnit.Approvals[string(event)]
			if !ok || record.ApprovedAt.IsZero() {
				c.emit(ConductorEvent{
					Type:    "approval_required",
					State:   c.machine.State(),
					Message: fmt.Sprintf("High-risk change (score=%.2f): approval required for %s", score.Score, event),
				})
				return fmt.Errorf("cannot %s: high-risk change (score=%.2f) requires explicit approval. Run: kvelmo approve %s", event, score.Score, event)
			}
			return nil
		}

		// Between thresholds: fall through to normal approval check.
	}

	if !s.Workflow.Policy.ApprovalRequired[string(event)] {
		return nil
	}

	record, ok := c.workUnit.Approvals[string(event)]
	if !ok || record.ApprovedAt.IsZero() {
		c.emit(ConductorEvent{
			Type:    "approval_required",
			State:   c.machine.State(),
			Message: fmt.Sprintf("Approval required for: %s", event),
		})

		return fmt.Errorf("cannot %s: explicit approval required. Run: kvelmo approve %s", event, event)
	}

	return nil
}

// evaluateRisk computes the risk score for the current work unit.
// Must be called with c.mu held (read or write).
func (c *Conductor) evaluateRisk() riskeval.RiskScore {
	s := c.getEffectiveSettings()

	input := riskeval.Input{
		SensitivePaths: s.Workflow.Policy.SensitivePaths,
	}

	// Gather diff stats from git if available.
	if c.git != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		stat, err := c.git.DiffNumStatAgainst(ctx, "")
		if err == nil {
			input.DiffLinesAdded = stat.Added
			input.DiffLinesRemoved = stat.Removed
			input.FilesChanged = stat.Files
			input.TotalFilesChanged = len(stat.Files)

			// Count test files.
			for _, f := range stat.Files {
				if isTestFile(f) {
					input.TestFilesChanged++
				}
			}
		} else {
			slog.Debug("risk evaluation: failed to get diff stats", "error", err)
		}
	}

	return riskeval.Evaluate(input)
}

// EvaluateRisk computes the risk score for the current work unit (exported for RPC).
func (c *Conductor) EvaluateRisk() riskeval.RiskScore {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.workUnit == nil {
		return riskeval.RiskScore{Level: riskeval.LevelLow}
	}

	return c.evaluateRisk()
}

// emitRiskEvaluated emits a risk_evaluated event with score and factors.
func (c *Conductor) emitRiskEvaluated(score riskeval.RiskScore) {
	data, err := json.Marshal(score)
	if err != nil {
		slog.Warn("failed to marshal risk score", "error", err)
		return
	}

	c.emit(ConductorEvent{
		Type:    "risk_evaluated",
		State:   c.machine.State(),
		Message: fmt.Sprintf("Risk evaluated: %.2f (%s)", score.Score, score.Level),
		Data:    data,
	})
}

// isTestFile returns true if the file path looks like a test file.
func isTestFile(path string) bool {
	// Go test files
	if len(path) > 8 && path[len(path)-8:] == "_test.go" {
		return true
	}
	// JS/TS test files
	for _, suffix := range []string{".test.ts", ".test.tsx", ".test.js", ".test.jsx", ".spec.ts", ".spec.tsx", ".spec.js", ".spec.jsx"} {
		if len(path) > len(suffix) && path[len(path)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

// Approve marks a transition event as approved by a human.
// Used when policy requires explicit approval for specific transitions.
func (c *Conductor) Approve(event string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return errors.New("no task loaded")
	}

	if c.workUnit.Approvals == nil {
		c.workUnit.Approvals = make(map[string]ApprovalRecord)
	}
	s := c.getEffectiveSettings()
	c.workUnit.Approvals[event] = ApprovalRecord{
		ApprovedBy: approverIdentity(s.Identity),
		ApprovedAt: time.Now(),
	}
	c.workUnit.UpdatedAt = time.Now()
	c.persistState()

	c.emit(ConductorEvent{
		Type:    "transition_approved",
		State:   c.machine.State(),
		Message: "Approved: " + event,
	})

	return nil
}

// CheckReviewItem marks a review checklist item as checked.
func (c *Conductor) CheckReviewItem(item string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return errors.New("no task loaded")
	}

	if !slices.Contains(c.workUnit.ChecklistChecked, item) {
		c.workUnit.ChecklistChecked = append(c.workUnit.ChecklistChecked, item)
	}
	c.workUnit.UpdatedAt = time.Now()
	c.persistState()

	return nil
}

// UncheckReviewItem removes a review checklist item.
func (c *Conductor) UncheckReviewItem(item string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return errors.New("no task loaded")
	}

	c.workUnit.ChecklistChecked = slices.DeleteFunc(c.workUnit.ChecklistChecked, func(s string) bool {
		return s == item
	})
	c.workUnit.UpdatedAt = time.Now()
	c.persistState()

	return nil
}

// ApproveNode approves a pending graph node approval gate.
func (c *Conductor) ApproveNode(nodeID string) error {
	c.mu.RLock()
	sched := c.activeScheduler
	c.mu.RUnlock()

	if sched == nil {
		return errors.New("no active graph execution")
	}

	if !sched.ApproveNode(graph.NodeID(nodeID)) {
		return fmt.Errorf("no pending approval for node %q", nodeID)
	}

	c.emit(ConductorEvent{
		Type:    "node_approved",
		NodeID:  nodeID,
		Message: "Node approved: " + nodeID,
	})

	return nil
}

// RejectNode rejects a pending graph node approval gate.
func (c *Conductor) RejectNode(nodeID string) error {
	c.mu.RLock()
	sched := c.activeScheduler
	c.mu.RUnlock()

	if sched == nil {
		return errors.New("no active graph execution")
	}

	if !sched.RejectNode(graph.NodeID(nodeID)) {
		return fmt.Errorf("no pending approval for node %q", nodeID)
	}

	c.emit(ConductorEvent{
		Type:    "node_rejected",
		NodeID:  nodeID,
		Message: "Node rejected: " + nodeID,
	})

	return nil
}

// ReviewChecklistStatus returns the configured checklist items and which are checked.
func (c *Conductor) ReviewChecklistStatus() ([]string, []string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s := c.getEffectiveSettings()
	required := s.Workflow.Policy.ReviewChecklist
	var checked []string
	if c.workUnit != nil {
		checked = c.workUnit.ChecklistChecked
	}

	return required, checked
}
