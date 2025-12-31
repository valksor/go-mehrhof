package storage

import (
	"fmt"
	"path/filepath"
	"time"
)

const locksDirName = "locks"

// LocksDir returns the path to the locks directory
func (w *Workspace) LocksDir() string {
	return filepath.Join(w.taskRoot, locksDirName)
}

// TaskLockPath returns the path to the lock file for a task
func (w *Workspace) TaskLockPath(taskID string) string {
	return filepath.Join(w.LocksDir(), taskID+".lock")
}

// WithTaskLock executes a function while holding an exclusive lock on the task.
// This prevents concurrent processes from modifying the same task simultaneously.
func (w *Workspace) WithTaskLock(taskID string, fn func() error) error {
	return WithLock(w.TaskLockPath(taskID), fn)
}

// WithTaskLockTimeout executes a function while holding a task lock,
// with a timeout for acquiring the lock.
func (w *Workspace) WithTaskLockTimeout(taskID string, timeout time.Duration, fn func() error) error {
	return WithLockTimeout(w.TaskLockPath(taskID), timeout, fn)
}

// FindTaskByWorktreePath finds a task by its worktree path.
// This is used to auto-detect the active task when running commands from within a worktree.
// Returns nil if no task is found with the given worktree path.
func (w *Workspace) FindTaskByWorktreePath(worktreePath string) (*ActiveTask, error) {
	// Normalize the path for comparison
	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		return nil, fmt.Errorf("resolve worktree path: %w", err)
	}

	// List all tasks and check their worktree paths
	taskIDs, err := w.ListWorks()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	for _, taskID := range taskIDs {
		work, err := w.LoadWork(taskID)
		if err != nil {
			continue // Skip tasks that can't be loaded
		}

		if work.Git.WorktreePath == "" {
			continue // Task doesn't have a worktree
		}

		// Normalize and compare paths
		taskWorktreePath, err := filepath.Abs(work.Git.WorktreePath)
		if err != nil {
			continue
		}

		if taskWorktreePath == absPath {
			// Found the task, build an ActiveTask from the work metadata
			active := &ActiveTask{
				ID:           work.Metadata.ID,
				Ref:          work.Source.Ref,
				WorkDir:      w.WorkPath(taskID),
				State:        "", // Will be loaded from .active_task if available
				Branch:       work.Git.Branch,
				UseGit:       work.Git.Branch != "",
				WorktreePath: work.Git.WorktreePath,
				Started:      work.Metadata.CreatedAt,
			}

			// Try to load the current state from .active_task if this is the active task
			if w.HasActiveTask() {
				existing, err := w.LoadActiveTask()
				if err == nil && existing.ID == taskID {
					active.State = existing.State
				}
			}

			return active, nil
		}
	}

	return nil, nil // No task found for this worktree
}

// ListTasksWithWorktrees returns all tasks that have associated worktrees.
// This is useful for listing parallel tasks across multiple terminals.
func (w *Workspace) ListTasksWithWorktrees() ([]*TaskWork, error) {
	taskIDs, err := w.ListWorks()
	if err != nil {
		return nil, err
	}

	var tasks []*TaskWork
	for _, taskID := range taskIDs {
		work, err := w.LoadWork(taskID)
		if err != nil {
			continue
		}

		if work.Git.WorktreePath != "" {
			tasks = append(tasks, work)
		}
	}

	return tasks, nil
}
