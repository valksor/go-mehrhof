package storage

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/meta"
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
	Description  string    `yaml:"description,omitempty" json:"description,omitempty"` // Task description for search
	Branch       string    `yaml:"branch,omitempty" json:"branch,omitempty"`
	Source       string    `yaml:"source,omitempty" json:"source,omitempty"`
	FinalState   string    `yaml:"final_state" json:"final_state"` // "finished", "abandoned", etc.
	StartedAt    time.Time `yaml:"started_at" json:"started_at"`
	CompletedAt  time.Time `yaml:"completed_at" json:"completed_at"`
	Duration     string    `yaml:"duration,omitempty" json:"duration,omitempty"`           // Human-readable duration (e.g., "2h15m")
	FilesTouched []string  `yaml:"files_touched,omitempty" json:"files_touched,omitempty"` // Files modified during task
	Tags         []string  `yaml:"tags,omitempty" json:"tags,omitempty"`                   // Task tags for filtering
	PhasesRun    []string  `yaml:"phases_run,omitempty" json:"phases_run,omitempty"`       // Phases that were executed (for pattern detection)
	AgentUsed    string    `yaml:"agent_used,omitempty" json:"agent_used,omitempty"`       // Primary agent used
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

	return SaveYAML(s.ArchiveFile(), tasks)
}

// ListArchivedTasks returns all archived tasks, newest first.
func (s *Store) ListArchivedTasks() ([]ArchivedTask, error) {
	var tasks []ArchivedTask
	if err := LoadYAML(s.ArchiveFile(), &tasks); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	slices.SortFunc(tasks, func(a, b ArchivedTask) int {
		return b.CompletedAt.Compare(a.CompletedAt)
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
				!strings.Contains(strings.ToLower(t.Source), q) &&
				!strings.Contains(strings.ToLower(t.Description), q) {
				continue
			}
		}
		if opts.Tag != "" && !slices.ContainsFunc(t.Tags, func(tag string) bool {
			return strings.EqualFold(tag, opts.Tag)
		}) {
			continue
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
