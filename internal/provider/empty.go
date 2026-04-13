package provider

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// EmptyProvider allows starting a task without an external source.
// Users can manually provide the task description.
type EmptyProvider struct{}

// NewEmptyProvider creates a new empty provider.
func NewEmptyProvider() *EmptyProvider {
	return &EmptyProvider{}
}

func (p *EmptyProvider) Name() string {
	return "empty"
}

// FetchTask creates a task from the provided description.
// The id parameter is used as the task title/description.
func (p *EmptyProvider) FetchTask(ctx context.Context, id string) (*Task, error) {
	if id == "" {
		return nil, errors.New("description required for empty provider")
	}

	// Generate a unique ID based on timestamp
	taskID := fmt.Sprintf("manual-%d", time.Now().Unix())

	return &Task{
		ID:          taskID,
		Title:       truncateTitle(id),
		Description: id,
		URL:         "",
		Labels:      []string{"manual"},
		Source:      "empty",
	}, nil
}

func (p *EmptyProvider) UpdateStatus(ctx context.Context, id string, status string) error {
	// No external system to update
	return nil
}

// truncateTitle truncates a description to create a title.
func truncateTitle(desc string) string {
	// Use first line or first 80 chars
	title := desc
	if idx := findNewline(title); idx > 0 {
		title = title[:idx]
	}
	if len(title) > 80 {
		title = title[:77] + "..."
	}

	return title
}

func findNewline(s string) int {
	for i, c := range s {
		if c == '\n' || c == '\r' {
			return i
		}
	}

	return -1
}
