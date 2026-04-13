// Package storage provides persistent storage for specs, plans, reviews, and chat history.
// Storage location is configurable: either in ~/.valksor/kvelmo/ (home) or .valksor/ (project).
package storage

import (
	"os"
	"path/filepath"

	"github.com/valksor/kvelmo/meta"
)

// WorkDir returns the work directory for a task.
// If saveInProject is true, returns .valksor/work/<task-id>/
// Otherwise returns ~/.valksor/kvelmo/work/<task-id>/.
func WorkDir(projectRoot, taskID string, saveInProject bool) string {
	if saveInProject {
		return filepath.Join(projectRoot, meta.OrgDir, "work", taskID)
	}
	home, _ := os.UserHomeDir()

	return filepath.Join(home, meta.GlobalDir, "work", taskID)
}

// SpecificationsDir returns the specifications directory for a task.
func SpecificationsDir(projectRoot, taskID string, saveInProject bool) string {
	return filepath.Join(WorkDir(projectRoot, taskID, saveInProject), "specifications")
}

// PlansDir returns the plans directory for a task.
func PlansDir(projectRoot, taskID string, saveInProject bool) string {
	return filepath.Join(WorkDir(projectRoot, taskID, saveInProject), "plans")
}

// ReviewsDir returns the reviews directory for a task.
func ReviewsDir(projectRoot, taskID string, saveInProject bool) string {
	return filepath.Join(WorkDir(projectRoot, taskID, saveInProject), "reviews")
}

// ChatFile returns the chat history file path for a task.
func ChatFile(projectRoot, taskID string, saveInProject bool) string {
	return filepath.Join(WorkDir(projectRoot, taskID, saveInProject), "chat.json")
}

// TaskStateFile returns the path to task.yaml for a given task ID.
func TaskStateFile(projectRoot, taskID string, saveInProject bool) string {
	return filepath.Join(WorkDir(projectRoot, taskID, saveInProject), "task.yaml")
}

// SessionsFile returns the sessions file path for a project.
// Sessions are always stored in the project directory.
func SessionsFile(projectRoot string) string {
	return filepath.Join(projectRoot, meta.OrgDir, "sessions.json")
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// Store wraps storage operations with configuration.
type Store struct {
	projectRoot   string
	saveInProject bool
}

// NewStore creates a new Store with the given configuration.
func NewStore(projectRoot string, saveInProject bool) *Store {
	return &Store{
		projectRoot:   projectRoot,
		saveInProject: saveInProject,
	}
}

// WorkDir returns the work directory for a task.
func (s *Store) WorkDir(taskID string) string {
	return WorkDir(s.projectRoot, taskID, s.saveInProject)
}

// SpecificationsDir returns the specifications directory for a task.
func (s *Store) SpecificationsDir(taskID string) string {
	return SpecificationsDir(s.projectRoot, taskID, s.saveInProject)
}

// PlansDir returns the plans directory for a task.
func (s *Store) PlansDir(taskID string) string {
	return PlansDir(s.projectRoot, taskID, s.saveInProject)
}

// ReviewsDir returns the reviews directory for a task.
func (s *Store) ReviewsDir(taskID string) string {
	return ReviewsDir(s.projectRoot, taskID, s.saveInProject)
}

// ChatFile returns the chat history file path for a task.
func (s *Store) ChatFile(taskID string) string {
	return ChatFile(s.projectRoot, taskID, s.saveInProject)
}

// TaskStateFile returns the path to task.yaml for a given task ID.
func (s *Store) TaskStateFile(taskID string) string {
	return TaskStateFile(s.projectRoot, taskID, s.saveInProject)
}

// WorkRoot returns the directory that contains all task work directories.
func (s *Store) WorkRoot() string {
	return filepath.Dir(s.WorkDir("_"))
}

// SessionsFile returns the sessions file path for the project.
func (s *Store) SessionsFile() string {
	return SessionsFile(s.projectRoot)
}

// ProjectRoot returns the project root path.
func (s *Store) ProjectRoot() string {
	return s.projectRoot
}

// SaveInProject returns whether storage is project-local.
func (s *Store) SaveInProject() bool {
	return s.saveInProject
}
