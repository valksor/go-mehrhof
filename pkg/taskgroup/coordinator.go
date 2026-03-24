package taskgroup

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Coordinator manages the lifecycle of task groups.
type Coordinator struct {
	mu     sync.RWMutex
	store  *Store
	groups map[string]*Group
}

// NewCoordinator creates a Coordinator backed by the given store.
// Existing groups are loaded from disk on creation.
func NewCoordinator(store *Store) *Coordinator {
	c := &Coordinator{
		store:  store,
		groups: make(map[string]*Group),
	}
	groups, err := store.LoadAll()
	if err != nil {
		slog.Warn("failed to load task groups", "error", err)
	}
	for _, g := range groups {
		c.groups[g.ID] = g
	}
	return c
}

// CreateGroup creates a new task group with a generated ID.
func (c *Coordinator) CreateGroup(label string) (*Group, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	now := time.Now()
	g := &Group{
		ID:        id,
		Label:     label,
		Tasks:     []TaskRef{},
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	c.mu.Lock()
	c.groups[g.ID] = g
	c.mu.Unlock()

	if err := c.store.Save(g); err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	slog.Info("task group created", "id", g.ID, "label", label)
	return g, nil
}

// GetGroup returns a group by ID.
func (c *Coordinator) GetGroup(id string) (*Group, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	g, ok := c.groups[id]
	if !ok {
		return nil, fmt.Errorf("get group: group %q not found", id)
	}
	return g, nil
}

// ListGroups returns all groups.
func (c *Coordinator) ListGroups() []*Group {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*Group, 0, len(c.groups))
	for _, g := range c.groups {
		result = append(result, g)
	}
	return result
}

// AddTask adds a task reference to a group and persists the change.
func (c *Coordinator) AddTask(groupID string, ref TaskRef) error {
	c.mu.Lock()
	g, ok := c.groups[groupID]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("add task to group: group %q not found", groupID)
	}
	if err := g.AddTask(ref); err != nil {
		c.mu.Unlock()
		return err
	}
	c.mu.Unlock()

	if err := c.store.Save(g); err != nil {
		return fmt.Errorf("add task to group: %w", err)
	}
	slog.Info("task added to group", "group_id", groupID, "task_id", ref.TaskID)
	return nil
}

// UpdateTaskState updates a task's state in whichever group contains it.
func (c *Coordinator) UpdateTaskState(taskID, state string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, g := range c.groups {
		if g.ContainsTask(taskID) {
			g.UpdateTaskState(taskID, state)
			_ = c.store.Save(g) // best-effort persist
			return
		}
	}
}

// CanSubmit checks if a task belonging to a group is allowed to submit.
// Returns true if syncSubmit is disabled or the task's group has all members ready.
// If the task is not in any group, submission is always allowed.
func (c *Coordinator) CanSubmit(taskID string, syncSubmit bool) (bool, error) {
	if !syncSubmit {
		return true, nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, g := range c.groups {
		if g.ContainsTask(taskID) {
			if g.AllReady() {
				return true, nil
			}
			return false, fmt.Errorf("cannot submit: task group %q (%s) has members not yet ready", g.ID, g.Label)
		}
	}
	// Not in any group — always allowed
	return true, nil
}

// SubmitGroup marks all tasks in a group as submitted.
func (c *Coordinator) SubmitGroup(id string) error {
	c.mu.Lock()
	g, ok := c.groups[id]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("submit group: group %q not found", id)
	}
	if !g.AllReady() {
		c.mu.Unlock()
		return fmt.Errorf("submit group: not all tasks are ready in group %q", id)
	}
	g.Status = StatusSubmitted
	g.UpdatedAt = time.Now()
	c.mu.Unlock()

	if err := c.store.Save(g); err != nil {
		return fmt.Errorf("submit group: %w", err)
	}
	slog.Info("task group submitted", "id", id)
	return nil
}

// RemoveGroup deletes a group from memory and disk.
func (c *Coordinator) RemoveGroup(id string) error {
	c.mu.Lock()
	if _, ok := c.groups[id]; !ok {
		c.mu.Unlock()
		return fmt.Errorf("remove group: group %q not found", id)
	}
	delete(c.groups, id)
	c.mu.Unlock()

	if err := c.store.Delete(id); err != nil {
		return fmt.Errorf("remove group: %w", err)
	}
	slog.Info("task group removed", "id", id)
	return nil
}

// FindGroupForTask returns the group that contains the given task, or nil.
func (c *Coordinator) FindGroupForTask(taskID string) *Group {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, g := range c.groups {
		if g.ContainsTask(taskID) {
			return g
		}
	}
	return nil
}

// generateID produces a short random hex ID.
func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
