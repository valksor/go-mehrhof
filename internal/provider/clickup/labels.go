package clickup

import (
	"context"
	"fmt"
	"strings"
)

// AddLabels implements the provider.LabelManager interface.
// In ClickUp, labels are called tags.
func (p *Provider) AddLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	taskID := ref.TaskID
	if taskID == "" {
		taskID = ref.CustomID
	}

	// Get current task to read existing tags
	task, err := p.client.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	// Build tag set (existing + new)
	tagSet := make(map[string]struct{})
	for _, tag := range task.Tags {
		tagSet[tag.Name] = struct{}{}
	}

	// Add new labels
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label != "" {
			tagSet[label] = struct{}{}
		}
	}

	// Build tag list
	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	// Update task with new tags
	_, err = p.client.UpdateTask(ctx, taskID, map[string]any{
		"tags": tags,
	})
	if err != nil {
		return fmt.Errorf("update tags: %w", err)
	}

	return nil
}

// RemoveLabels implements the provider.LabelManager interface.
// Removes specified labels (tags) from the ClickUp task.
func (p *Provider) RemoveLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	taskID := ref.TaskID
	if taskID == "" {
		taskID = ref.CustomID
	}

	// Get current task
	task, err := p.client.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	// Create set of labels to remove (case-insensitive)
	toRemove := make(map[string]struct{})
	for _, label := range labels {
		toRemove[strings.ToLower(strings.TrimSpace(label))] = struct{}{}
	}

	// Filter out labels to remove
	var remainingTags []string
	for _, tag := range task.Tags {
		if _, remove := toRemove[strings.ToLower(tag.Name)]; !remove {
			remainingTags = append(remainingTags, tag.Name)
		}
	}

	// Update task with remaining tags
	_, err = p.client.UpdateTask(ctx, taskID, map[string]any{
		"tags": remainingTags,
	})
	if err != nil {
		return fmt.Errorf("update tags: %w", err)
	}

	return nil
}
