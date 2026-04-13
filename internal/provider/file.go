package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileProvider struct{}

func NewFileProvider() *FileProvider {
	return &FileProvider{}
}

func (p *FileProvider) Name() string {
	return "file"
}

func (p *FileProvider) FetchTask(ctx context.Context, path string) (*Task, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Parse title from first line (if markdown)
	lines := strings.Split(string(content), "\n")
	title := filepath.Base(path)
	description := string(content)

	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		if after, ok := strings.CutPrefix(firstLine, "# "); ok {
			title = after
			description = strings.Join(lines[1:], "\n")
		}
	}

	return &Task{
		ID:          absPath,
		Title:       title,
		Description: strings.TrimSpace(description),
		URL:         "file://" + absPath,
		Source:      "file",
	}, nil
}

func (p *FileProvider) UpdateStatus(ctx context.Context, id string, status string) error {
	// Files don't have status to update
	return nil
}
