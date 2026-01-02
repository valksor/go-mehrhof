package vcs

import (
	"cmp"
	"context"
	"errors"
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
func (g *Git) CreateCheckpoint(ctx context.Context, taskID, message string) (*Checkpoint, error) {
	defaultPrefix := fmt.Sprintf("[%s]", taskID)

	return g.CreateCheckpointWithPrefix(ctx, taskID, message, defaultPrefix)
}

// CreateCheckpointWithPrefix creates a checkpoint for a task with a custom commit prefix.
func (g *Git) CreateCheckpointWithPrefix(ctx context.Context, taskID, message, commitPrefix string) (*Checkpoint, error) {
	// Get next checkpoint number
	existing, err := g.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, err
	}

	number := 1
	if len(existing) > 0 {
		number = existing[len(existing)-1].Number + 1
	}

	// Create commit if there are changes
	hasChanges, err := g.HasChanges(ctx)
	if err != nil {
		return nil, err
	}

	var commitHash string
	if hasChanges {
		if err := g.AddAll(ctx); err != nil {
			return nil, fmt.Errorf("stage changes: %w", err)
		}
		commitMsg := fmt.Sprintf("%s checkpoint %d: %s", commitPrefix, number, message)
		commitHash, err = g.Commit(ctx, commitMsg)
		if err != nil {
			return nil, fmt.Errorf("create commit: %w", err)
		}
	} else {
		// Use current HEAD for empty checkpoint
		commitHash, err = g.RevParse(ctx, "HEAD")
		if err != nil {
			return nil, err
		}
	}

	// Create tag for checkpoint
	tagName := fmt.Sprintf("%s/%s/%d", CheckpointPrefix, taskID, number)
	_, err = g.run(ctx, "tag", tagName, commitHash)
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
func (g *Git) ListCheckpoints(ctx context.Context, taskID string) ([]*Checkpoint, error) {
	prefix := fmt.Sprintf("%s/%s/", CheckpointPrefix, taskID)
	out, err := g.run(ctx, "tag", "-l", prefix+"*")
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
		hash, err := g.RevParse(ctx, tag)
		if err != nil {
			continue
		}

		msg, _ := g.GetCommitMessage(ctx, hash)

		cp := &Checkpoint{
			ID:      hash,
			TaskID:  taskID,
			Number:  num,
			Message: msg,
		}

		// Get timestamp
		out, err := g.run(ctx, "log", "-1", "--format=%aI", hash)
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
func (g *Git) GetCheckpoint(ctx context.Context, taskID string, number int) (*Checkpoint, error) {
	checkpoints, err := g.ListCheckpoints(ctx, taskID)
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
func (g *Git) GetLatestCheckpoint(ctx context.Context, taskID string) (*Checkpoint, error) {
	checkpoints, err := g.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if len(checkpoints) == 0 {
		return nil, fmt.Errorf("no checkpoints for task %s", taskID)
	}

	return checkpoints[len(checkpoints)-1], nil
}

// RestoreCheckpoint reverts to a checkpoint state.
func (g *Git) RestoreCheckpoint(ctx context.Context, taskID string, number int) error {
	cp, err := g.GetCheckpoint(ctx, taskID, number)
	if err != nil {
		return err
	}

	// Hard reset to checkpoint
	if err := g.ResetHard(ctx, cp.ID); err != nil {
		return fmt.Errorf("restore checkpoint: %w", err)
	}

	return nil
}

// CanUndo checks if undo is possible for a task.
func (g *Git) CanUndo(ctx context.Context, taskID string) (bool, error) {
	checkpoints, err := g.ListCheckpoints(ctx, taskID)
	if err != nil {
		return false, err
	}

	if len(checkpoints) < 2 {
		return false, nil
	}

	// Check if current HEAD is at the latest checkpoint
	head, err := g.RevParse(ctx, "HEAD")
	if err != nil {
		return false, err
	}

	latest := checkpoints[len(checkpoints)-1]

	return head == latest.ID, nil
}

// CanRedo checks if redo is possible for a task.
func (g *Git) CanRedo(ctx context.Context, taskID string) (bool, error) {
	checkpoints, err := g.ListCheckpoints(ctx, taskID)
	if err != nil {
		return false, err
	}

	if len(checkpoints) < 2 {
		return false, nil
	}

	// Check if we're not at the latest checkpoint
	head, err := g.RevParse(ctx, "HEAD")
	if err != nil {
		return false, err
	}

	latest := checkpoints[len(checkpoints)-1]

	return head != latest.ID, nil
}

// Undo reverts to the previous checkpoint.
func (g *Git) Undo(ctx context.Context, taskID string) (*Checkpoint, error) {
	checkpoints, err := g.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if len(checkpoints) < 2 {
		return nil, errors.New("nothing to undo")
	}

	head, err := g.RevParse(ctx, "HEAD")
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
		return nil, errors.New("cannot undo: at earliest checkpoint")
	}

	// Restore previous checkpoint
	previous := checkpoints[currentIdx-1]
	if err := g.ResetHard(ctx, previous.ID); err != nil {
		return nil, fmt.Errorf("undo: %w", err)
	}

	return previous, nil
}

// Redo moves forward to the next checkpoint.
func (g *Git) Redo(ctx context.Context, taskID string) (*Checkpoint, error) {
	checkpoints, err := g.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if len(checkpoints) < 2 {
		return nil, errors.New("nothing to redo")
	}

	head, err := g.RevParse(ctx, "HEAD")
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
		return nil, errors.New("cannot redo: at latest checkpoint")
	}

	// Restore next checkpoint
	next := checkpoints[currentIdx+1]
	if err := g.ResetHard(ctx, next.ID); err != nil {
		return nil, fmt.Errorf("redo: %w", err)
	}

	return next, nil
}

// DeleteCheckpoint removes a checkpoint tag.
func (g *Git) DeleteCheckpoint(ctx context.Context, taskID string, number int) error {
	tagName := fmt.Sprintf("%s/%s/%d", CheckpointPrefix, taskID, number)
	_, err := g.run(ctx, "tag", "-d", tagName)
	if err != nil {
		return fmt.Errorf("delete checkpoint: %w", err)
	}

	return nil
}

// DeleteAllCheckpoints removes all checkpoints for a task.
func (g *Git) DeleteAllCheckpoints(ctx context.Context, taskID string) error {
	checkpoints, err := g.ListCheckpoints(ctx, taskID)
	if err != nil {
		return err
	}

	for _, cp := range checkpoints {
		if err := g.DeleteCheckpoint(ctx, taskID, cp.Number); err != nil {
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
func (g *Git) GetChangeSummary(ctx context.Context) (*ChangeSummary, error) {
	// Get status output (porcelain format for parsing)
	out, err := g.run(ctx, "status", "--porcelain", "-uall")
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
func (g *Git) GenerateAutoSummary(ctx context.Context) (string, error) {
	summary, err := g.GetChangeSummary(ctx)
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
			parts = append(parts, "add "+summary.Added[0])
		} else {
			parts = append(parts, fmt.Sprintf("add %d files", len(summary.Added)))
		}
	}

	if len(summary.Modified) > 0 {
		if len(summary.Modified) == 1 {
			parts = append(parts, "update "+summary.Modified[0])
		} else {
			parts = append(parts, fmt.Sprintf("update %d files", len(summary.Modified)))
		}
	}

	if len(summary.Deleted) > 0 {
		if len(summary.Deleted) == 1 {
			parts = append(parts, "remove "+summary.Deleted[0])
		} else {
			parts = append(parts, fmt.Sprintf("remove %d files", len(summary.Deleted)))
		}
	}

	return strings.Join(parts, ", "), nil
}

// CreateCheckpointAutoSummary creates a checkpoint with auto-generated message.
func (g *Git) CreateCheckpointAutoSummary(ctx context.Context, taskID string) (*Checkpoint, error) {
	msg, err := g.GenerateAutoSummary(ctx)
	if err != nil {
		msg = "checkpoint"
	}

	return g.CreateCheckpoint(ctx, taskID, msg)
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
func (t *CheckpointTracker) Save(ctx context.Context, message string) (*Checkpoint, error) {
	return t.git.CreateCheckpoint(ctx, t.taskID, message)
}

// SaveAuto creates a checkpoint with auto-generated message.
func (t *CheckpointTracker) SaveAuto(ctx context.Context) (*Checkpoint, error) {
	return t.git.CreateCheckpointAutoSummary(ctx, t.taskID)
}

// UndoAvailable checks if undo is possible.
func (t *CheckpointTracker) UndoAvailable(ctx context.Context) bool {
	can, _ := t.git.CanUndo(ctx, t.taskID)

	return can
}

// RedoAvailable checks if redo is possible.
func (t *CheckpointTracker) RedoAvailable(ctx context.Context) bool {
	can, _ := t.git.CanRedo(ctx, t.taskID)

	return can
}

// Undo reverts to the previous state.
func (t *CheckpointTracker) Undo(ctx context.Context) (*Checkpoint, error) {
	return t.git.Undo(ctx, t.taskID)
}

// Redo moves forward to the next state.
func (t *CheckpointTracker) Redo(ctx context.Context) (*Checkpoint, error) {
	return t.git.Redo(ctx, t.taskID)
}

// List returns all checkpoints.
func (t *CheckpointTracker) List(ctx context.Context) ([]*Checkpoint, error) {
	return t.git.ListCheckpoints(ctx, t.taskID)
}
