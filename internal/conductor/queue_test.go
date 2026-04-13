package conductor

import (
	"testing"
)

func TestQueueTask(t *testing.T) {
	c := newTestConductor(t)

	task, err := c.QueueTask("github:owner/repo#1", "Fix login bug")
	if err != nil {
		t.Fatalf("QueueTask: %v", err)
	}

	if task.Source != "github:owner/repo#1" {
		t.Errorf("Source = %q, want %q", task.Source, "github:owner/repo#1")
	}
	if task.Title != "Fix login bug" {
		t.Errorf("Title = %q, want %q", task.Title, "Fix login bug")
	}
	if task.Position != 1 {
		t.Errorf("Position = %d, want 1", task.Position)
	}
	if task.ID == "" {
		t.Error("ID is empty")
	}
}

func TestQueueMultiple(t *testing.T) {
	c := newTestConductor(t)

	_, _ = c.QueueTask("github:owner/repo#1", "First")
	_, _ = c.QueueTask("github:owner/repo#2", "Second")
	_, _ = c.QueueTask("github:owner/repo#3", "Third")

	if c.QueueLength() != 3 {
		t.Fatalf("QueueLength = %d, want 3", c.QueueLength())
	}

	queue := c.ListQueue()
	if len(queue) != 3 {
		t.Fatalf("ListQueue len = %d, want 3", len(queue))
	}
	if queue[0].Title != "First" || queue[1].Title != "Second" || queue[2].Title != "Third" {
		t.Errorf("Queue order wrong: %v", queue)
	}
	// Positions should be 1-based
	for i, q := range queue {
		if q.Position != i+1 {
			t.Errorf("queue[%d].Position = %d, want %d", i, q.Position, i+1)
		}
	}
}

func TestDequeueTask(t *testing.T) {
	c := newTestConductor(t)

	t1, _ := c.QueueTask("github:owner/repo#1", "First")
	_, _ = c.QueueTask("github:owner/repo#2", "Second")

	if err := c.DequeueTask(t1.ID); err != nil {
		t.Fatalf("DequeueTask: %v", err)
	}

	if c.QueueLength() != 1 {
		t.Fatalf("QueueLength = %d, want 1", c.QueueLength())
	}

	queue := c.ListQueue()
	if queue[0].Title != "Second" {
		t.Errorf("remaining task = %q, want %q", queue[0].Title, "Second")
	}
}

func TestDequeueTaskNotFound(t *testing.T) {
	c := newTestConductor(t)

	err := c.DequeueTask("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestReorderQueue(t *testing.T) {
	c := newTestConductor(t)

	_, _ = c.QueueTask("github:owner/repo#1", "First")
	_, _ = c.QueueTask("github:owner/repo#2", "Second")
	t3, _ := c.QueueTask("github:owner/repo#3", "Third")

	// Move Third to position 1
	if err := c.ReorderQueue(t3.ID, 1); err != nil {
		t.Fatalf("ReorderQueue: %v", err)
	}

	queue := c.ListQueue()
	if queue[0].Title != "Third" {
		t.Errorf("queue[0] = %q, want Third", queue[0].Title)
	}
	if queue[1].Title != "First" {
		t.Errorf("queue[1] = %q, want First", queue[1].Title)
	}
	if queue[2].Title != "Second" {
		t.Errorf("queue[2] = %q, want Second", queue[2].Title)
	}
}

func TestReorderQueueOutOfRange(t *testing.T) {
	c := newTestConductor(t)

	t1, _ := c.QueueTask("github:owner/repo#1", "First")

	if err := c.ReorderQueue(t1.ID, 0); err == nil {
		t.Error("expected error for position 0")
	}
	if err := c.ReorderQueue(t1.ID, 5); err == nil {
		t.Error("expected error for position > length")
	}
}

func TestPopNextTask(t *testing.T) {
	c := newTestConductor(t)

	_, _ = c.QueueTask("github:owner/repo#1", "First")
	_, _ = c.QueueTask("github:owner/repo#2", "Second")

	// popNextTask requires holding mu
	c.mu.Lock()
	next := c.popNextTask()
	c.mu.Unlock()

	if next == nil {
		t.Fatal("popNextTask returned nil")
	}
	if next.Title != "First" {
		t.Errorf("popped = %q, want First", next.Title)
	}
	if c.QueueLength() != 1 {
		t.Errorf("QueueLength = %d, want 1", c.QueueLength())
	}
}

func TestPopNextTaskEmpty(t *testing.T) {
	c := newTestConductor(t)

	c.mu.Lock()
	next := c.popNextTask()
	c.mu.Unlock()

	if next != nil {
		t.Errorf("expected nil, got %v", next)
	}
}

// newTestConductor creates a minimal conductor for queue testing.
func newTestConductor(t *testing.T) *Conductor {
	t.Helper()

	c, err := New(WithWorkDir(t.TempDir()))
	if err != nil {
		t.Fatalf("New conductor: %v", err)
	}

	return c
}
