package vcs

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Checkpoint represents a saved state for undo/redo.
type Checkpoint struct {
	ID        string    // Commit hash
	TaskID    string    // Associated task ID
	Number    int       // Checkpoint number within task
	Message   string    // Checkpoint description
	Timestamp time.Time // When checkpoint was created
}

// CheckpointPrefix is the tag prefix for checkpoints.
const CheckpointPrefix = "task-checkpoint"

// checkpointTagRe matches checkpoint tags: task-checkpoint/<taskID>/<number>.
var checkpointTagRe = regexp.MustCompile(`^task-checkpoint/([^/]+)/(\d+)$`)

// CreateCheckpoint creates a checkpoint for a task with default prefix [taskID].
func (g *Git) CreateCheckpoint(taskID, message string) (*Checkpoint, error) {
	defaultPrefix := fmt.Sprintf("[%s]", taskID)
	return g.CreateCheckpointWithPrefix(taskID, message, defaultPrefix)
}

// CreateCheckpointWithPrefix creates a checkpoint for a task with a custom commit prefix.
func (g *Git) CreateCheckpointWithPrefix(taskID, message, commitPrefix string) (*Checkpoint, error) {
	// Get next checkpoint number
	existing, err := g.ListCheckpoints(taskID)
	if err != nil {
		return nil, err
	}

	number := 1
	if len(existing) > 0 {
		number = existing[len(existing)-1].Number + 1
	}

	// Create commit if there are changes
	hasChanges, err := g.HasChanges()
	if err != nil {
		return nil, err
	}

	var commitHash string
	if hasChanges {
		if err := g.AddAll(); err != nil {
			return nil, fmt.Errorf("stage changes: %w", err)
		}
		commitMsg := fmt.Sprintf("%s checkpoint %d: %s", commitPrefix, number, message)
		commitHash, err = g.Commit(commitMsg)
		if err != nil {
			return nil, fmt.Errorf("create commit: %w", err)
		}
	} else {
		// Use current HEAD for empty checkpoint
		commitHash, err = g.RevParse("HEAD")
		if err != nil {
			return nil, err
		}
	}

	// Create tag for checkpoint
	tagName := fmt.Sprintf("%s/%s/%d", CheckpointPrefix, taskID, number)
	_, err = g.run("tag", tagName, commitHash)
	if err != nil {
		return nil, fmt.Errorf("create checkpoint tag: %w", err)
	}

	return &Checkpoint{
		ID:        commitHash,
		TaskID:    taskID,
		Number:    number,
		Message:   message,
		Timestamp: time.Now(),
	}, nil
}

// ListCheckpoints returns all checkpoints for a task.
func (g *Git) ListCheckpoints(taskID string) ([]*Checkpoint, error) {
	prefix := fmt.Sprintf("%s/%s/", CheckpointPrefix, taskID)
	out, err := g.run("tag", "-l", prefix+"*")
	if err != nil {
		return nil, err
	}

	var checkpoints []*Checkpoint
	for _, tag := range strings.Split(out, "\n") {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		matches := checkpointTagRe.FindStringSubmatch(tag)
		if matches == nil || matches[1] != taskID {
			continue
		}

		num, _ := strconv.Atoi(matches[2])

		// Get commit info for this tag
		hash, err := g.RevParse(tag)
		if err != nil {
			continue
		}

		msg, _ := g.GetCommitMessage(hash)

		cp := &Checkpoint{
			ID:      hash,
			TaskID:  taskID,
			Number:  num,
			Message: msg,
		}

		// Get timestamp
		out, err := g.run("log", "-1", "--format=%aI", hash)
		if err == nil {
			cp.Timestamp, _ = time.Parse(time.RFC3339, strings.TrimSpace(out))
		}

		checkpoints = append(checkpoints, cp)
	}

	// Sort by number
	slices.SortFunc(checkpoints, func(a, b *Checkpoint) int {
		return cmp.Compare(a.Number, b.Number)
	})

	return checkpoints, nil
}

// GetCheckpoint returns a specific checkpoint.
func (g *Git) GetCheckpoint(taskID string, number int) (*Checkpoint, error) {
	checkpoints, err := g.ListCheckpoints(taskID)
	if err != nil {
		return nil, err
	}

	for _, cp := range checkpoints {
		if cp.Number == number {
			return cp, nil
		}
	}

	return nil, fmt.Errorf("checkpoint %d not found for task %s", number, taskID)
}

// GetLatestCheckpoint returns the most recent checkpoint.
func (g *Git) GetLatestCheckpoint(taskID string) (*Checkpoint, error) {
	checkpoints, err := g.ListCheckpoints(taskID)
	if err != nil {
		return nil, err
	}

	if len(checkpoints) == 0 {
		return nil, fmt.Errorf("no checkpoints for task %s", taskID)
	}

	return checkpoints[len(checkpoints)-1], nil
}

// RestoreCheckpoint reverts to a checkpoint state.
func (g *Git) RestoreCheckpoint(taskID string, number int) error {
	cp, err := g.GetCheckpoint(taskID, number)
	if err != nil {
		return err
	}

	// Hard reset to checkpoint
	if err := g.ResetHard(cp.ID); err != nil {
		return fmt.Errorf("restore checkpoint: %w", err)
	}

	return nil
}

// CanUndo checks if undo is possible for a task.
func (g *Git) CanUndo(taskID string) (bool, error) {
	checkpoints, err := g.ListCheckpoints(taskID)
	if err != nil {
		return false, err
	}

	if len(checkpoints) < 2 {
		return false, nil
	}

	// Check if current HEAD is at the latest checkpoint
	head, err := g.RevParse("HEAD")
	if err != nil {
		return false, err
	}

	latest := checkpoints[len(checkpoints)-1]
	return head == latest.ID, nil
}

// CanRedo checks if redo is possible for a task.
func (g *Git) CanRedo(taskID string) (bool, error) {
	checkpoints, err := g.ListCheckpoints(taskID)
	if err != nil {
		return false, err
	}

	if len(checkpoints) < 2 {
		return false, nil
	}

	// Check if we're not at the latest checkpoint
	head, err := g.RevParse("HEAD")
	if err != nil {
		return false, err
	}

	latest := checkpoints[len(checkpoints)-1]
	return head != latest.ID, nil
}

// Undo reverts to the previous checkpoint.
func (g *Git) Undo(taskID string) (*Checkpoint, error) {
	checkpoints, err := g.ListCheckpoints(taskID)
	if err != nil {
		return nil, err
	}

	if len(checkpoints) < 2 {
		return nil, fmt.Errorf("nothing to undo")
	}

	head, err := g.RevParse("HEAD")
	if err != nil {
		return nil, err
	}

	// Find current position
	currentIdx := -1
	for i, cp := range checkpoints {
		if cp.ID == head {
			currentIdx = i
			break
		}
	}

	if currentIdx < 1 {
		return nil, fmt.Errorf("cannot undo: at earliest checkpoint")
	}

	// Restore previous checkpoint
	previous := checkpoints[currentIdx-1]
	if err := g.ResetHard(previous.ID); err != nil {
		return nil, fmt.Errorf("undo: %w", err)
	}

	return previous, nil
}

// Redo moves forward to the next checkpoint.
func (g *Git) Redo(taskID string) (*Checkpoint, error) {
	checkpoints, err := g.ListCheckpoints(taskID)
	if err != nil {
		return nil, err
	}

	if len(checkpoints) < 2 {
		return nil, fmt.Errorf("nothing to redo")
	}

	head, err := g.RevParse("HEAD")
	if err != nil {
		return nil, err
	}

	// Find current position
	currentIdx := -1
	for i, cp := range checkpoints {
		if cp.ID == head {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 || currentIdx >= len(checkpoints)-1 {
		return nil, fmt.Errorf("cannot redo: at latest checkpoint")
	}

	// Restore next checkpoint
	next := checkpoints[currentIdx+1]
	if err := g.ResetHard(next.ID); err != nil {
		return nil, fmt.Errorf("redo: %w", err)
	}

	return next, nil
}

// DeleteCheckpoint removes a checkpoint tag.
func (g *Git) DeleteCheckpoint(taskID string, number int) error {
	tagName := fmt.Sprintf("%s/%s/%d", CheckpointPrefix, taskID, number)
	_, err := g.run("tag", "-d", tagName)
	if err != nil {
		return fmt.Errorf("delete checkpoint: %w", err)
	}
	return nil
}

// DeleteAllCheckpoints removes all checkpoints for a task.
func (g *Git) DeleteAllCheckpoints(taskID string) error {
	checkpoints, err := g.ListCheckpoints(taskID)
	if err != nil {
		return err
	}

	for _, cp := range checkpoints {
		if err := g.DeleteCheckpoint(taskID, cp.Number); err != nil {
			// Continue on error, best effort
			continue
		}
	}

	return nil
}

// ChangeSummary holds statistics about changes.
type ChangeSummary struct {
	Added    []string // New files
	Modified []string // Changed files
	Deleted  []string // Removed files
	Total    int      // Total changed files
}

// GetChangeSummary returns a summary of staged and unstaged changes.
func (g *Git) GetChangeSummary() (*ChangeSummary, error) {
	// Get status output (porcelain format for parsing)
	out, err := g.run("status", "--porcelain", "-uall")
	if err != nil {
		return nil, err
	}

	summary := &ChangeSummary{
		Added:    make([]string, 0),
		Modified: make([]string, 0),
		Deleted:  make([]string, 0),
	}

	for _, line := range strings.Split(out, "\n") {
		if len(line) < 3 {
			continue
		}

		status := line[:2]
		file := strings.TrimSpace(line[3:])

		// Handle renamed files (R status shows "old -> new")
		if strings.Contains(file, " -> ") {
			parts := strings.SplitN(file, " -> ", 2)
			if len(parts) == 2 {
				file = parts[1]
			}
		}

		switch {
		case status[0] == 'A' || status[1] == 'A' || status[0] == '?' || status[1] == '?':
			summary.Added = append(summary.Added, file)
		case status[0] == 'D' || status[1] == 'D':
			summary.Deleted = append(summary.Deleted, file)
		case status[0] == 'M' || status[1] == 'M' || status[0] == 'R' || status[1] == 'R':
			summary.Modified = append(summary.Modified, file)
		default:
			// Any other change counts as modified
			if status != "  " {
				summary.Modified = append(summary.Modified, file)
			}
		}
	}

	summary.Total = len(summary.Added) + len(summary.Modified) + len(summary.Deleted)
	return summary, nil
}

// GenerateAutoSummary creates an automatic commit message based on changes.
func (g *Git) GenerateAutoSummary() (string, error) {
	summary, err := g.GetChangeSummary()
	if err != nil {
		return "", err
	}

	if summary.Total == 0 {
		return "no changes", nil
	}

	// Build a descriptive message
	var parts []string

	if len(summary.Added) > 0 {
		if len(summary.Added) == 1 {
			parts = append(parts, fmt.Sprintf("add %s", summary.Added[0]))
		} else {
			parts = append(parts, fmt.Sprintf("add %d files", len(summary.Added)))
		}
	}

	if len(summary.Modified) > 0 {
		if len(summary.Modified) == 1 {
			parts = append(parts, fmt.Sprintf("update %s", summary.Modified[0]))
		} else {
			parts = append(parts, fmt.Sprintf("update %d files", len(summary.Modified)))
		}
	}

	if len(summary.Deleted) > 0 {
		if len(summary.Deleted) == 1 {
			parts = append(parts, fmt.Sprintf("remove %s", summary.Deleted[0]))
		} else {
			parts = append(parts, fmt.Sprintf("remove %d files", len(summary.Deleted)))
		}
	}

	return strings.Join(parts, ", "), nil
}

// CreateCheckpointAutoSummary creates a checkpoint with auto-generated message.
func (g *Git) CreateCheckpointAutoSummary(taskID string) (*Checkpoint, error) {
	msg, err := g.GenerateAutoSummary()
	if err != nil {
		msg = "checkpoint"
	}
	return g.CreateCheckpoint(taskID, msg)
}

// CheckpointTracker provides higher-level checkpoint tracking.
type CheckpointTracker struct {
	git    *Git
	taskID string
}

// NewCheckpointTracker creates a tracker for a task.
func NewCheckpointTracker(git *Git, taskID string) *CheckpointTracker {
	return &CheckpointTracker{
		git:    git,
		taskID: taskID,
	}
}

// Save creates a new checkpoint.
func (t *CheckpointTracker) Save(message string) (*Checkpoint, error) {
	return t.git.CreateCheckpoint(t.taskID, message)
}

// SaveAuto creates a checkpoint with auto-generated message.
func (t *CheckpointTracker) SaveAuto() (*Checkpoint, error) {
	return t.git.CreateCheckpointAutoSummary(t.taskID)
}

// UndoAvailable checks if undo is possible.
func (t *CheckpointTracker) UndoAvailable() bool {
	can, _ := t.git.CanUndo(t.taskID)
	return can
}

// RedoAvailable checks if redo is possible.
func (t *CheckpointTracker) RedoAvailable() bool {
	can, _ := t.git.CanRedo(t.taskID)
	return can
}

// Undo reverts to the previous state.
func (t *CheckpointTracker) Undo() (*Checkpoint, error) {
	return t.git.Undo(t.taskID)
}

// Redo moves forward to the next state.
func (t *CheckpointTracker) Redo() (*Checkpoint, error) {
	return t.git.Redo(t.taskID)
}

// List returns all checkpoints.
func (t *CheckpointTracker) List() ([]*Checkpoint, error) {
	return t.git.ListCheckpoints(t.taskID)
}
