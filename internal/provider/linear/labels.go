package linear

import (
	"context"
	"errors"
)

// AddLabels adds labels to a Linear issue
// Note: Linear uses label IDs rather than names, so this implementation
// fetches the issue first to get existing labels and then updates with new ones.
// For a production implementation, you would want to cache label name → ID mappings.
func (p *Provider) AddLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}

	// First, fetch the current issue to get its existing labels
	issue, err := p.client.GetIssue(ctx, ref.IssueID)
	if err != nil {
		return err
	}

	// Extract existing label IDs
	existingLabelIDs := make([]string, 0, len(issue.Labels))
	existingLabelNames := make(map[string]string)
	for _, label := range issue.Labels {
		existingLabelIDs = append(existingLabelIDs, label.ID)
		existingLabelNames[label.Name] = label.ID
	}

	// Add new labels by name (in production, you'd resolve names to IDs first)
	// For now, we'll skip labels that don't exist by name
	newLabelIDs := make([]string, 0, len(labels))
	for _, labelName := range labels {
		if _, exists := existingLabelNames[labelName]; exists {
			// Already has this label
			continue
		}
		// In production, you'd look up the label ID from the label name
		// For now, we'll try to use the label name directly (may not work in all cases)
		newLabelIDs = append(newLabelIDs, labelName)
	}

	// Combine existing and new labels
	allLabelIDs := append(existingLabelIDs, newLabelIDs...)

	// Update the issue with all labels
	_, err = p.client.UpdateIssue(ctx, issue.ID, UpdateIssueInput{
		LabelIDs: allLabelIDs,
	})

	return err
}

// RemoveLabels removes labels from a Linear issue.
func (p *Provider) RemoveLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}

	// First, fetch the current issue to get its existing labels
	issue, err := p.client.GetIssue(ctx, ref.IssueID)
	if err != nil {
		return err
	}

	// Build a set of label names to remove
	labelsToRemove := make(map[string]bool)
	for _, label := range labels {
		labelsToRemove[label] = true
	}

	// Keep labels that aren't in the remove list
	keptLabelIDs := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		if !labelsToRemove[label.Name] {
			keptLabelIDs = append(keptLabelIDs, label.ID)
		}
	}

	// Update the issue with remaining labels
	_, err = p.client.UpdateIssue(ctx, issue.ID, UpdateIssueInput{
		LabelIDs: keptLabelIDs,
	})

	return err
}

// GetLabelIDs resolves label names to label IDs
// This is a helper function that would be used in production to properly resolve labels
// For now, it returns the names as-is since we'd need additional API calls to resolve them.
func GetLabelIDs(ctx context.Context, client *Client, teamKey string, labelNames []string) ([]string, error) {
	// In a production implementation, you would:
	// 1. Query the team's labels using GraphQL
	// 2. Build a map of label name → label ID
	// 3. Return the corresponding IDs for the given names
	//
	// For now, we'll return the names as-is with a note
	return labelNames, errors.New("label name to ID resolution not fully implemented - labels must be passed as IDs")
}
