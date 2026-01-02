package youtrack

import (
	"context"
	"fmt"
)

// AddLabels adds tags (YouTrack calls them tags) to an issue.
func (p *Provider) AddLabels(ctx context.Context, workUnitID string, labels []string) error {
	// Get existing tags to avoid duplicates
	existingTags, err := p.client.GetTags(ctx, workUnitID)
	if err != nil {
		return fmt.Errorf("get existing tags: %w", err)
	}

	// Build name -> ID map for existing tags
	existingTagNames := make(map[string]bool)
	for _, tag := range existingTags {
		existingTagNames[tag.Name] = true
	}

	// Add each label that doesn't already exist
	for _, label := range labels {
		if existingTagNames[label] {
			continue // Tag already exists on issue
		}
		_, err := p.client.AddTag(ctx, workUnitID, label)
		if err != nil {
			// Log error but continue trying other tags
			fmt.Printf("warning: failed to add tag %s: %v\n", label, err)
		}
	}

	return nil
}

// RemoveLabels removes tags from an issue.
func (p *Provider) RemoveLabels(ctx context.Context, workUnitID string, labels []string) error {
	// Get existing tags to find their IDs
	existingTags, err := p.client.GetTags(ctx, workUnitID)
	if err != nil {
		return fmt.Errorf("get existing tags: %w", err)
	}

	// Build name -> ID map
	tagIDMap := make(map[string]string)
	for _, tag := range existingTags {
		tagIDMap[tag.Name] = tag.ID
	}

	// Remove each specified label
	for _, label := range labels {
		if tagID, exists := tagIDMap[label]; exists {
			err := p.client.RemoveTag(ctx, workUnitID, tagID)
			if err != nil {
				// Log error but continue trying other tags
				fmt.Printf("warning: failed to remove tag %s: %v\n", label, err)
			}
		}
	}

	return nil
}
