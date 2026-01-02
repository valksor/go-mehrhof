package jira

import (
	"context"
	"strings"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// UpdateStatus changes the status of a Jira issue via workflow transitions
// Jira requires using workflow transitions rather than directly setting status.
func (p *Provider) UpdateStatus(ctx context.Context, workUnitID string, status provider.Status) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}

	// Update base URL if detected from reference
	if ref.BaseURL != "" && p.baseURL == "" {
		p.baseURL = ref.BaseURL
		p.client.SetBaseURL(ref.BaseURL)
	}

	// Get available transitions for this issue
	transitions, err := p.client.GetTransitions(ctx, ref.IssueKey)
	if err != nil {
		return err
	}

	// Map provider status to possible Jira transition names
	possibleNames := mapProviderStatusToJiraTransitions(status)

	// Find matching transition
	var transitionID string
	for _, transition := range transitions {
		for _, name := range possibleNames {
			if strings.EqualFold(transition.Name, name) {
				transitionID = transition.ID

				break
			}
		}
		if transitionID != "" {
			break
		}
	}

	if transitionID == "" {
		return ErrNoTransition
	}

	// Execute the transition
	return p.client.DoTransition(ctx, ref.IssueKey, transitionID)
}

// GetAvailableStatuses returns a list of possible status names for a provider status.
// This is useful for debugging or showing available options.
func GetAvailableStatuses(status provider.Status) []string {
	return mapProviderStatusToJiraTransitions(status)
}
