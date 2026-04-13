package conductor

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

var queueCounter atomic.Int64

// QueuedTask represents a task waiting in the queue.
type QueuedTask struct {
	ID             string    `json:"id"`
	Source         string    `json:"source"`          // Provider source reference (e.g., "github:owner/repo#123")
	Title          string    `json:"title"`           // Display title (may be empty until loaded)
	Priority       int       `json:"priority"`        // 1=highest, 5=lowest, 0=default (treated as 3)
	PreferredAgent string    `json:"preferred_agent"` // Agent to use (empty = default)
	AddedAt        time.Time `json:"added_at"`
	Position       int       `json:"position"` // 1-based position in queue
}

// QueueTask adds a task source to the queue.
// The task will be loaded when it reaches the front and the current task finishes.
func (c *Conductor) QueueTask(source string, title string) (*QueuedTask, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	task := &QueuedTask{
		ID:      fmt.Sprintf("q-%d-%d", time.Now().UnixNano(), queueCounter.Add(1)),
		Source:  source,
		Title:   title,
		AddedAt: time.Now(),
	}

	c.taskQueue = append(c.taskQueue, task)
	task.Position = len(c.taskQueue)

	c.emit(ConductorEvent{
		Type:    "task_queued",
		State:   c.machine.State(),
		Message: fmt.Sprintf("Task queued at position %d", task.Position),
		Data:    mustMarshalJSON(task),
	})

	return task, nil
}

// DequeueTask removes a task from the queue by ID.
func (c *Conductor) DequeueTask(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, t := range c.taskQueue {
		if t.ID == id {
			c.taskQueue = append(c.taskQueue[:i], c.taskQueue[i+1:]...)

			c.emit(ConductorEvent{
				Type:    "task_dequeued",
				State:   c.machine.State(),
				Message: "Task removed from queue",
			})

			return nil
		}
	}

	return fmt.Errorf("queued task %s not found", id)
}

// ListQueue returns the current task queue.
func (c *Conductor) ListQueue() []QueuedTask {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]QueuedTask, len(c.taskQueue))
	for i, t := range c.taskQueue {
		result[i] = *t
		result[i].Position = i + 1
	}

	return result
}

// QueueLength returns the number of tasks in the queue.
func (c *Conductor) QueueLength() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.taskQueue)
}

// ReorderQueue moves a task to a new position (1-based).
func (c *Conductor) ReorderQueue(id string, newPosition int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if newPosition < 1 || newPosition > len(c.taskQueue) {
		return fmt.Errorf("position %d out of range (1-%d)", newPosition, len(c.taskQueue))
	}

	// Find the task
	fromIdx := -1
	for i, t := range c.taskQueue {
		if t.ID == id {
			fromIdx = i

			break
		}
	}
	if fromIdx == -1 {
		return fmt.Errorf("queued task %s not found", id)
	}

	// Remove and reinsert using a new slice to avoid append aliasing
	task := c.taskQueue[fromIdx]
	remaining := make([]*QueuedTask, 0, len(c.taskQueue))
	remaining = append(remaining, c.taskQueue[:fromIdx]...)
	remaining = append(remaining, c.taskQueue[fromIdx+1:]...)
	toIdx := newPosition - 1
	reordered := make([]*QueuedTask, 0, len(c.taskQueue))
	reordered = append(reordered, remaining[:toIdx]...)
	reordered = append(reordered, task)
	reordered = append(reordered, remaining[toIdx:]...)
	c.taskQueue = reordered

	return nil
}

// popNextTask removes and returns the first task from the queue, or nil if empty.
// Caller must hold c.mu.
func (c *Conductor) popNextTask() *QueuedTask {
	if len(c.taskQueue) == 0 {
		return nil
	}

	next := c.taskQueue[0]
	c.taskQueue = c.taskQueue[1:]

	return next
}

// MarshalQueue returns the queue as JSON (for persistence).
func (c *Conductor) MarshalQueue() (json.RawMessage, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return json.Marshal(c.taskQueue)
}
