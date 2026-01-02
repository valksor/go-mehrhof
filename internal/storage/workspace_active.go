package storage

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ActiveTaskPath returns the path to .active_task file.
func (w *Workspace) ActiveTaskPath() string {
	return filepath.Join(w.root, activeTaskFile)
}

// HasActiveTask checks if there's an active task.
func (w *Workspace) HasActiveTask() bool {
	_, err := os.Stat(w.ActiveTaskPath())

	return err == nil
}

// LoadActiveTask loads the active task reference.
func (w *Workspace) LoadActiveTask() (*ActiveTask, error) {
	data, err := os.ReadFile(w.ActiveTaskPath())
	if err != nil {
		return nil, fmt.Errorf("read active task: %w", err)
	}

	var active ActiveTask
	if err := yaml.Unmarshal(data, &active); err != nil {
		return nil, fmt.Errorf("parse active task: %w", err)
	}

	return &active, nil
}

// SaveActiveTask saves the active task reference using atomic write pattern.
func (w *Workspace) SaveActiveTask(active *ActiveTask) error {
	data, err := yaml.Marshal(active)
	if err != nil {
		return fmt.Errorf("marshal active task: %w", err)
	}

	// Use atomic write pattern: write to temp file, then rename
	path := w.ActiveTaskPath()
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write active task: %w", err)
	}
	// Atomic rename is guaranteed to be atomic on POSIX systems
	if err := os.Rename(tmpPath, path); err != nil {
		// Clean up temp file on error, log if cleanup fails
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			slog.Warn("failed to clean up temp file after rename error", "path", tmpPath, "error", removeErr)
		}

		return fmt.Errorf("save active task: %w", err)
	}

	return nil
}

// ClearActiveTask removes the active task file.
func (w *Workspace) ClearActiveTask() error {
	err := os.Remove(w.ActiveTaskPath())
	if os.IsNotExist(err) {
		return nil
	}

	return err
}

// UpdateActiveTaskState updates just the state field.
func (w *Workspace) UpdateActiveTaskState(state string) error {
	active, err := w.LoadActiveTask()
	if err != nil {
		return err
	}
	active.State = state

	return w.SaveActiveTask(active)
}
