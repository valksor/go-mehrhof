package conductor

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"slices"
	"time"
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
// Must be called with c.mu held (read or write).
func (c *Conductor) checkApproval(event Event) error {
	s := c.getEffectiveSettings()
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
