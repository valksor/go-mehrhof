package storage

import (
	"os"
	"path/filepath"
	"time"
)

// TaskHistoryEntry records a single state machine transition for audit persistence.
type TaskHistoryEntry struct {
	From      string    `yaml:"from"`
	To        string    `yaml:"to"`
	Event     string    `yaml:"event"`
	Timestamp time.Time `yaml:"timestamp"`
}

// TaskState is the on-disk snapshot of a WorkUnit and its state machine state.
// Written as pure YAML to <workdir>/<task-id>/task.yaml on every mutation.
// This is the single source of truth for task state across socket restarts.
type TaskState struct {
	State             string                        `yaml:"state"`
	ID                string                        `yaml:"id"`
	ExternalID        string                        `yaml:"external_id,omitempty"`
	Title             string                        `yaml:"title"`
	Description       string                        `yaml:"description,omitempty"`
	Branch            string                        `yaml:"branch,omitempty"`
	WorktreePath      string                        `yaml:"worktree_path,omitempty"`
	Specifications    []string                      `yaml:"specifications,omitempty"`
	Checkpoints       []string                      `yaml:"checkpoints,omitempty"`
	RedoStack         []string                      `yaml:"redo_stack,omitempty"`
	Jobs              []string                      `yaml:"jobs,omitempty"`
	Metadata          map[string]string             `yaml:"metadata,omitempty"`
	Source            *TaskSource                   `yaml:"source,omitempty"`
	Hierarchy         *TaskHierarchy                `yaml:"hierarchy,omitempty"`
	Approvals         map[string]TaskApprovalRecord `yaml:"approvals,omitempty"`
	ChecklistChecked  []string                      `yaml:"checklist_checked,omitempty"`
	Tags              []string                      `yaml:"tags,omitempty"`
	Priority          int                           `yaml:"priority,omitempty"`
	DependsOn         []string                      `yaml:"depends_on,omitempty"`
	QualityGatePassed *bool                         `yaml:"quality_gate_passed,omitempty"`
	VarPoolPath       string                        `yaml:"var_pool_path,omitempty"`
	History           []TaskHistoryEntry            `yaml:"history,omitempty"`
	CreatedAt         time.Time                     `yaml:"created_at"`
	UpdatedAt         time.Time                     `yaml:"updated_at"`
}

// TaskApprovalRecord mirrors conductor.ApprovalRecord without an import cycle.
type TaskApprovalRecord struct {
	ApprovedBy string    `yaml:"approved_by,omitempty"`
	ApprovedAt time.Time `yaml:"approved_at"`
}

// TaskSource mirrors conductor.Source without creating an import cycle.
type TaskSource struct {
	Provider  string `yaml:"provider"`
	Reference string `yaml:"reference"`
	URL       string `yaml:"url,omitempty"`
	Content   string `yaml:"content,omitempty"`
}

// TaskHierarchySummary mirrors conductor.TaskSummary without an import cycle.
type TaskHierarchySummary struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description,omitempty"`
	URL         string `yaml:"url,omitempty"`
	Status      string `yaml:"status,omitempty"`
}

// TaskHierarchy mirrors conductor.HierarchyContext without an import cycle.
type TaskHierarchy struct {
	Parent   *TaskHierarchySummary  `yaml:"parent,omitempty"`
	Siblings []TaskHierarchySummary `yaml:"siblings,omitempty"`
}

// SaveTaskState writes ts to <workdir>/<ts.ID>/task.yaml atomically.
func (s *Store) SaveTaskState(ts *TaskState) error {
	if err := EnsureDir(s.WorkDir(ts.ID)); err != nil {
		return err
	}

	return SaveYAML(s.TaskStateFile(ts.ID), ts)
}

// LoadTaskState reads and parses task.yaml for the given task ID.
// Returns os.ErrNotExist (wrapped) if the file does not exist.
func (s *Store) LoadTaskState(taskID string) (*TaskState, error) {
	var ts TaskState
	if err := LoadYAML(s.TaskStateFile(taskID), &ts); err != nil {
		return nil, err
	}

	return &ts, nil
}

// TaskStateExists reports whether task.yaml exists for the given task ID.
func (s *Store) TaskStateExists(taskID string) bool {
	_, err := os.Stat(s.TaskStateFile(taskID))

	return err == nil
}

// DeleteTaskState removes task.yaml for the given task ID.
// Called when a task is abandoned or deleted to prevent stale state from being restored.
func (s *Store) DeleteTaskState(taskID string) error {
	err := os.Remove(s.TaskStateFile(taskID))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// FindActiveTask returns the task ID whose task.yaml was most recently modified.
// Returns ("", nil) if no task.yaml files exist.
func (s *Store) FindActiveTask() (string, error) {
	workRoot := s.WorkRoot()
	entries, err := os.ReadDir(workRoot)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	var newest string
	var newestTime time.Time
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		f := filepath.Join(workRoot, e.Name(), "task.yaml")
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if info.ModTime().After(newestTime) {
			newestTime = info.ModTime()
			newest = e.Name()
		}
	}

	return newest, nil
}
