// Package taskgroup manages cross-repo task groups for synchronized lifecycle.
// Groups link tasks across repositories so they can be coordinated
// (e.g., submit all at once when every member reaches the reviewing state).
package taskgroup

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// Valid group statuses.
const (
	StatusActive    = "active"
	StatusReady     = "ready"
	StatusSubmitted = "submitted"
	StatusCompleted = "completed"
)

// States that qualify as "ready" for synchronized submission.
// A task must be in one of these states for the group to allow submit.
var readyStates = []string{
	"reviewing", "submitted", "completed",
}

// TaskRef identifies a task in a specific project.
type TaskRef struct {
	ProjectDir string `json:"project_dir"`
	TaskID     string `json:"task_id"`
	State      string `json:"state"` // mirrors conductor state
}

// Group links tasks across repositories for synchronized lifecycle.
type Group struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Tasks     []TaskRef `json:"tasks"`
	Status    string    `json:"status"` // active, ready, submitted, completed
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AllReady returns true when all tasks have reached the reviewing state or later.
func (g *Group) AllReady() bool {
	if len(g.Tasks) == 0 {
		return false
	}
	for _, t := range g.Tasks {
		if !slices.Contains(readyStates, t.State) {
			return false
		}
	}
	return true
}

// AddTask adds a task reference to the group.
// Returns an error if the task ID already exists in the group.
func (g *Group) AddTask(ref TaskRef) error {
	for _, t := range g.Tasks {
		if t.TaskID == ref.TaskID {
			return fmt.Errorf("add task: task %q already in group", ref.TaskID)
		}
	}
	g.Tasks = append(g.Tasks, ref)
	g.UpdatedAt = time.Now()
	g.refreshStatus()
	return nil
}

// RemoveTask removes a task by ID.
// Returns an error if the task is not found in the group.
func (g *Group) RemoveTask(taskID string) error {
	idx := slices.IndexFunc(g.Tasks, func(t TaskRef) bool {
		return t.TaskID == taskID
	})
	if idx < 0 {
		return fmt.Errorf("remove task: task %q not found in group", taskID)
	}
	g.Tasks = slices.Delete(g.Tasks, idx, idx+1)
	g.UpdatedAt = time.Now()
	g.refreshStatus()
	return nil
}

// UpdateTaskState updates a task's state within the group.
// If the task is not found, this is a no-op.
func (g *Group) UpdateTaskState(taskID, state string) {
	for i := range g.Tasks {
		if g.Tasks[i].TaskID == taskID {
			g.Tasks[i].State = state
			g.UpdatedAt = time.Now()
			g.refreshStatus()
			return
		}
	}
}

// ContainsTask returns true if the group contains a task with the given ID.
func (g *Group) ContainsTask(taskID string) bool {
	return slices.ContainsFunc(g.Tasks, func(t TaskRef) bool {
		return t.TaskID == taskID
	})
}

// refreshStatus recalculates the group status from member states.
func (g *Group) refreshStatus() {
	if g.AllReady() && g.Status == StatusActive {
		g.Status = StatusReady
	}
}

// ValidateStatus checks that the status string is a known value.
func ValidateStatus(s string) error {
	switch s {
	case StatusActive, StatusReady, StatusSubmitted, StatusCompleted:
		return nil
	default:
		return errors.New("unknown group status: " + s)
	}
}
