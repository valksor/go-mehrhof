package storage

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/meta"
	"gopkg.in/yaml.v3"
)

// SearchOptions filters archived tasks.
type SearchOptions struct {
	Query string    `json:"query,omitempty"` // Substring match in title, branch, or source
	Tag   string    `json:"tag,omitempty"`   // Filter by tag (reserved for future use)
	Since time.Time `json:"since,omitempty"` // Only tasks completed after this time
	Until time.Time `json:"until,omitempty"` // Only tasks completed before this time
	State string    `json:"state,omitempty"` // Filter by final_state (e.g., "finished", "abandoned")
	Limit int       `json:"limit,omitempty"` // Max results (0 = unlimited)
	File  string    `json:"file,omitempty"`  // Filter by file touched (substring match)
}

// ArchivedTask is a lightweight record of a completed task.
type ArchivedTask struct {
	ID           string    `yaml:"id" json:"id"`
	Title        string    `yaml:"title" json:"title"`
	Branch       string    `yaml:"branch,omitempty" json:"branch,omitempty"`
	Source       string    `yaml:"source,omitempty" json:"source,omitempty"`
	FinalState   string    `yaml:"final_state" json:"final_state"` // "finished", "abandoned", etc.
	StartedAt    time.Time `yaml:"started_at" json:"started_at"`
	CompletedAt  time.Time `yaml:"completed_at" json:"completed_at"`
	Duration     string    `yaml:"duration,omitempty" json:"duration,omitempty"`           // Human-readable duration (e.g., "2h15m")
	FilesTouched []string  `yaml:"files_touched,omitempty" json:"files_touched,omitempty"` // Files modified during task
}

// ArchiveFile returns the path to the archive index file.
func (s *Store) ArchiveFile() string {
	return filepath.Join(s.projectRoot, meta.OrgDir, "archive.yaml")
}

// ArchiveTask appends a completed task to the archive.
func (s *Store) ArchiveTask(task ArchivedTask) error {
	dir := filepath.Dir(s.ArchiveFile())
	if err := EnsureDir(dir); err != nil {
		return err
	}

	tasks, _ := s.ListArchivedTasks() // ignore error, start fresh if corrupt
	tasks = append(tasks, task)

	data, err := yaml.Marshal(tasks)
	if err != nil {
		return err
	}

	return os.WriteFile(s.ArchiveFile(), data, 0o644)
}

// ListArchivedTasks returns all archived tasks, newest first.
func (s *Store) ListArchivedTasks() ([]ArchivedTask, error) {
	data, err := os.ReadFile(s.ArchiveFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var tasks []ArchivedTask
	if err := yaml.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CompletedAt.After(tasks[j].CompletedAt)
	})

	return tasks, nil
}

// SearchArchivedTasks returns archived tasks matching the given filters.
func (s *Store) SearchArchivedTasks(opts SearchOptions) ([]ArchivedTask, error) {
	tasks, err := s.ListArchivedTasks()
	if err != nil {
		return nil, err
	}

	var filtered []ArchivedTask
	for _, t := range tasks {
		if opts.Query != "" {
			q := strings.ToLower(opts.Query)
			if !strings.Contains(strings.ToLower(t.Title), q) &&
				!strings.Contains(strings.ToLower(t.Branch), q) &&
				!strings.Contains(strings.ToLower(t.Source), q) {
				continue
			}
		}
		if !opts.Since.IsZero() && t.CompletedAt.Before(opts.Since) {
			continue
		}
		if !opts.Until.IsZero() && t.CompletedAt.After(opts.Until) {
			continue
		}
		if opts.State != "" && t.FinalState != opts.State {
			continue
		}
		if opts.File != "" {
			found := false
			for _, f := range t.FilesTouched {
				if strings.Contains(f, opts.File) {
					found = true

					break
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, t)
		if opts.Limit > 0 && len(filtered) >= opts.Limit {
			break
		}
	}

	return filtered, nil
}
