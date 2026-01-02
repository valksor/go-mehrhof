package notion

import (
	"context"
	"fmt"
	"strings"
)

// AddLabels adds labels to a Notion page.
func (p *Provider) AddLabels(ctx context.Context, workUnitID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}

	// Get the page first to find existing labels
	page, err := p.client.GetPage(ctx, workUnitID)
	if err != nil {
		return err
	}

	// Find the labels property and get existing labels
	prop, ok := GetProperty(*page, p.labelsProperty)
	if !ok {
		return fmt.Errorf("labels property %q not found on page", p.labelsProperty)
	}

	// Collect existing labels
	existingLabels := make(map[string]bool)
	if prop.MultiSelect != nil {
		for _, opt := range prop.MultiSelect.Options {
			existingLabels[strings.ToLower(opt.Name)] = true
		}
	}

	// Add new labels (avoiding duplicates)
	allLabels := make([]string, 0)
	if prop.MultiSelect != nil {
		for _, opt := range prop.MultiSelect.Options {
			allLabels = append(allLabels, opt.Name)
		}
	}
	for _, label := range labels {
		if !existingLabels[strings.ToLower(label)] {
			allLabels = append(allLabels, label)
		}
	}

	// Build update input
	update := &UpdatePageInput{
		Properties: map[string]Property{
			p.labelsProperty: MakeMultiSelectProperty(allLabels),
		},
	}

	_, err = p.client.UpdatePage(ctx, workUnitID, update)
	return err
}

// RemoveLabels removes labels from a Notion page.
func (p *Provider) RemoveLabels(ctx context.Context, workUnitID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}

	// Get the page first to find existing labels
	page, err := p.client.GetPage(ctx, workUnitID)
	if err != nil {
		return err
	}

	// Find the labels property and get existing labels
	prop, ok := GetProperty(*page, p.labelsProperty)
	if !ok {
		return fmt.Errorf("labels property %q not found on page", p.labelsProperty)
	}

	// Build set of labels to remove (case-insensitive)
	toRemove := make(map[string]bool)
	for _, label := range labels {
		toRemove[strings.ToLower(label)] = true
	}

	// Filter out labels to remove
	keptLabels := make([]string, 0)
	if prop.MultiSelect != nil {
		for _, opt := range prop.MultiSelect.Options {
			if !toRemove[strings.ToLower(opt.Name)] {
				keptLabels = append(keptLabels, opt.Name)
			}
		}
	}

	// Build update input
	update := &UpdatePageInput{
		Properties: map[string]Property{
			p.labelsProperty: MakeMultiSelectProperty(keptLabels),
		},
	}

	_, err = p.client.UpdatePage(ctx, workUnitID, update)
	return err
}
