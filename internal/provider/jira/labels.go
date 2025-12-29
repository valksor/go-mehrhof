package jira

import (
	"context"
)

// AddLabels adds labels to a Jira issue
func (p *Provider) AddLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}

	// Update base URL if detected from reference
	if ref.BaseURL != "" && p.baseURL == "" {
		p.baseURL = ref.BaseURL
		p.client.SetBaseURL(ref.BaseURL)
	}

	// First, fetch the current issue to get its existing labels
	issue, err := p.client.GetIssue(ctx, ref.IssueKey)
	if err != nil {
		return err
	}

	// Merge existing labels with new labels (avoid duplicates)
	existingLabels := make(map[string]bool)
	for _, label := range issue.Fields.Labels {
		existingLabels[label] = true
	}

	// Add new labels
	for _, label := range labels {
		if !existingLabels[label] {
			issue.Fields.Labels = append(issue.Fields.Labels, label)
		}
	}

	// Update the issue with all labels
	updateInput := UpdateIssueInput{}
	updateInput.Fields.Labels = issue.Fields.Labels

	return p.client.UpdateIssue(ctx, ref.IssueKey, updateInput)
}

// RemoveLabels removes labels from a Jira issue
func (p *Provider) RemoveLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}

	// Update base URL if detected from reference
	if ref.BaseURL != "" && p.baseURL == "" {
		p.baseURL = ref.BaseURL
		p.client.SetBaseURL(ref.BaseURL)
	}

	// First, fetch the current issue to get its existing labels
	issue, err := p.client.GetIssue(ctx, ref.IssueKey)
	if err != nil {
		return err
	}

	// Build a set of label names to remove
	labelsToRemove := make(map[string]bool)
	for _, label := range labels {
		labelsToRemove[label] = true
	}

	// Keep labels that aren't in the remove list
	var keptLabels []string
	for _, label := range issue.Fields.Labels {
		if !labelsToRemove[label] {
			keptLabels = append(keptLabels, label)
		}
	}

	// Update the issue with remaining labels
	updateInput := UpdateIssueInput{}
	updateInput.Fields.Labels = keptLabels

	return p.client.UpdateIssue(ctx, ref.IssueKey, updateInput)
}
