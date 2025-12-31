package asana

import (
	"context"
	"fmt"
	"strings"
)

// AddLabels implements the provider.LabelManager interface.
// In Asana, labels are called tags and require GIDs.
func (p *Provider) AddLabels(ctx context.Context, workUnitID string, labels []string) error {
	// Get current task to check existing tags
	task, err := p.client.GetTask(ctx, workUnitID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	// Create set of existing tag names (case-insensitive)
	existingTags := make(map[string]struct{})
	for _, tag := range task.Tags {
		existingTags[strings.ToLower(tag.Name)] = struct{}{}
	}

	// Get workspace tags to find GIDs
	workspaceTags, err := p.client.GetWorkspaceTags(ctx)
	if err != nil {
		return fmt.Errorf("get workspace tags: %w", err)
	}

	// Build tag name -> GID map
	tagGIDs := make(map[string]string)
	for _, tag := range workspaceTags {
		tagGIDs[strings.ToLower(tag.Name)] = tag.GID
	}

	// Add each label that doesn't already exist
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}

		labelLower := strings.ToLower(label)

		// Skip if already tagged
		if _, exists := existingTags[labelLower]; exists {
			continue
		}

		// Find or create the tag
		tagGID, exists := tagGIDs[labelLower]
		if !exists {
			// Create the tag in the workspace
			newTag, err := p.client.CreateTag(ctx, label)
			if err != nil {
				return fmt.Errorf("create tag %s: %w", label, err)
			}
			tagGID = newTag.GID
		}

		// Add tag to task
		if err := p.client.AddTagToTask(ctx, workUnitID, tagGID); err != nil {
			return fmt.Errorf("add tag %s to task: %w", label, err)
		}
	}

	return nil
}

// RemoveLabels implements the provider.LabelManager interface.
// Removes specified labels (tags) from the Asana task.
func (p *Provider) RemoveLabels(ctx context.Context, workUnitID string, labels []string) error {
	// Get current task
	task, err := p.client.GetTask(ctx, workUnitID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	// Create set of labels to remove (case-insensitive)
	toRemove := make(map[string]struct{})
	for _, label := range labels {
		toRemove[strings.ToLower(strings.TrimSpace(label))] = struct{}{}
	}

	// Remove matching tags
	for _, tag := range task.Tags {
		if _, remove := toRemove[strings.ToLower(tag.Name)]; remove {
			if err := p.client.RemoveTagFromTask(ctx, workUnitID, tag.GID); err != nil {
				return fmt.Errorf("remove tag %s from task: %w", tag.Name, err)
			}
		}
	}

	return nil
}
