package azuredevops

import (
	"context"
	"fmt"
	"strings"
)

// AddLabels implements the provider.LabelManager interface.
// In Azure DevOps, labels are stored as tags (semicolon-separated string).
func (p *Provider) AddLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	// Get current work item to read existing tags
	workItem, err := p.client.GetWorkItem(ctx, ref.WorkItemID)
	if err != nil {
		return fmt.Errorf("get work item: %w", err)
	}

	// Parse existing tags
	existingTags := parseTags(workItem.Fields.Tags)

	// Create a set of existing tags for deduplication
	tagSet := make(map[string]struct{})
	for _, tag := range existingTags {
		tagSet[tag] = struct{}{}
	}

	// Add new labels (avoiding duplicates)
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label != "" {
			tagSet[label] = struct{}{}
		}
	}

	// Build new tags string
	var allTags []string
	for tag := range tagSet {
		allTags = append(allTags, tag)
	}
	newTagsStr := strings.Join(allTags, "; ")

	// Update work item
	updates := []PatchOperation{
		{
			Op:    "add",
			Path:  "/fields/System.Tags",
			Value: newTagsStr,
		},
	}

	_, err = p.client.UpdateWorkItem(ctx, ref.WorkItemID, updates)
	if err != nil {
		return fmt.Errorf("update tags: %w", err)
	}

	return nil
}

// RemoveLabels implements the provider.LabelManager interface.
// Removes specified labels from Azure DevOps work item tags.
func (p *Provider) RemoveLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	// Get current work item
	workItem, err := p.client.GetWorkItem(ctx, ref.WorkItemID)
	if err != nil {
		return fmt.Errorf("get work item: %w", err)
	}

	// Parse existing tags
	existingTags := parseTags(workItem.Fields.Tags)

	// Create set of labels to remove (case-insensitive)
	toRemove := make(map[string]struct{})
	for _, label := range labels {
		toRemove[strings.ToLower(strings.TrimSpace(label))] = struct{}{}
	}

	// Filter out labels to remove
	var remainingTags []string
	for _, tag := range existingTags {
		if _, remove := toRemove[strings.ToLower(tag)]; !remove {
			remainingTags = append(remainingTags, tag)
		}
	}

	// Build new tags string
	newTagsStr := strings.Join(remainingTags, "; ")

	// Update work item
	updates := []PatchOperation{
		{
			Op:    "add",
			Path:  "/fields/System.Tags",
			Value: newTagsStr,
		},
	}

	_, err = p.client.UpdateWorkItem(ctx, ref.WorkItemID, updates)
	if err != nil {
		return fmt.Errorf("update tags: %w", err)
	}

	return nil
}
