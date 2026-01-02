package vcs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateCheckpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create some changes
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cp, err := g.CreateCheckpoint(ctx, "task-123", "first checkpoint")
	if err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	if cp.TaskID != "task-123" {
		t.Errorf("TaskID = %q, want %q", cp.TaskID, "task-123")
	}
	if cp.Number != 1 {
		t.Errorf("Number = %d, want 1", cp.Number)
	}
	if cp.ID == "" {
		t.Error("ID should not be empty")
	}
}

func TestCreateMultipleCheckpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// First checkpoint
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cp1, err := g.CreateCheckpoint(ctx, "task-456", "checkpoint 1")
	if err != nil {
		t.Fatalf("CreateCheckpoint 1: %v", err)
	}

	// Second checkpoint
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cp2, err := g.CreateCheckpoint(ctx, "task-456", "checkpoint 2")
	if err != nil {
		t.Fatalf("CreateCheckpoint 2: %v", err)
	}

	if cp1.Number != 1 {
		t.Errorf("first checkpoint number = %d, want 1", cp1.Number)
	}
	if cp2.Number != 2 {
		t.Errorf("second checkpoint number = %d, want 2", cp2.Number)
	}
}

func TestCreateCheckpointNoChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// No changes - should still create checkpoint using current HEAD
	cp, err := g.CreateCheckpoint(ctx, "task-empty", "empty checkpoint")
	if err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	if cp.Number != 1 {
		t.Errorf("Number = %d, want 1", cp.Number)
	}
	if cp.ID == "" {
		t.Error("ID should not be empty even for empty checkpoint")
	}
}

func TestListCheckpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	taskID := "task-list"

	// Create checkpoints
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "cp1"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "cp2"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "cp3"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	checkpoints, err := g.ListCheckpoints(ctx, taskID)
	if err != nil {
		t.Fatalf("ListCheckpoints: %v", err)
	}

	if len(checkpoints) != 3 {
		t.Errorf("expected 3 checkpoints, got %d", len(checkpoints))
	}

	// Should be sorted by number
	for i, cp := range checkpoints {
		expected := i + 1
		if cp.Number != expected {
			t.Errorf("checkpoint %d has number %d, want %d", i, cp.Number, expected)
		}
	}
}

func TestListCheckpointsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	checkpoints, err := g.ListCheckpoints(ctx, "nonexistent-task")
	if err != nil {
		t.Fatalf("ListCheckpoints: %v", err)
	}

	if len(checkpoints) != 0 {
		t.Errorf("expected 0 checkpoints, got %d", len(checkpoints))
	}
}

func TestGetCheckpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	taskID := "task-get"

	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "checkpoint one"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "y.txt"), []byte("y"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cp2, err := g.CreateCheckpoint(ctx, taskID, "checkpoint two")
	if err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	// Get checkpoint 2
	cp, err := g.GetCheckpoint(ctx, taskID, 2)
	if err != nil {
		t.Fatalf("GetCheckpoint: %v", err)
	}

	if cp.Number != 2 {
		t.Errorf("Number = %d, want 2", cp.Number)
	}
	if cp.ID != cp2.ID {
		t.Errorf("ID mismatch: got %q, want %q", cp.ID, cp2.ID)
	}
}

func TestGetCheckpointNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = g.GetCheckpoint(ctx, "task-x", 99)
	if err == nil {
		t.Error("GetCheckpoint should fail for non-existent checkpoint")
	}
}

func TestGetLatestCheckpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	taskID := "task-latest"

	if err := os.WriteFile(filepath.Join(dir, "1.txt"), []byte("1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "first"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2.txt"), []byte("2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cp2, err := g.CreateCheckpoint(ctx, taskID, "second")
	if err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	latest, err := g.GetLatestCheckpoint(ctx, taskID)
	if err != nil {
		t.Fatalf("GetLatestCheckpoint: %v", err)
	}

	if latest.Number != cp2.Number {
		t.Errorf("latest = %d, want %d", latest.Number, cp2.Number)
	}
}

func TestGetLatestCheckpointNone(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = g.GetLatestCheckpoint(ctx, "no-checkpoints")
	if err == nil {
		t.Error("GetLatestCheckpoint should fail when no checkpoints")
	}
}

func TestCanUndo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	taskID := "task-undo"

	// No checkpoints - can't undo
	can, err := g.CanUndo(ctx, taskID)
	if err != nil {
		t.Fatalf("CanUndo: %v", err)
	}
	if can {
		t.Error("should not be able to undo with no checkpoints")
	}

	// Single checkpoint - still can't undo (need at least 2)
	if err := os.WriteFile(filepath.Join(dir, "undo1.txt"), []byte("undo1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "checkpoint 1"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	can, err = g.CanUndo(ctx, taskID)
	if err != nil {
		t.Fatalf("CanUndo: %v", err)
	}
	if can {
		t.Error("should not be able to undo with only 1 checkpoint")
	}

	// Two checkpoints - now can undo
	if err := os.WriteFile(filepath.Join(dir, "undo2.txt"), []byte("undo2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "checkpoint 2"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	can, err = g.CanUndo(ctx, taskID)
	if err != nil {
		t.Fatalf("CanUndo: %v", err)
	}
	if !can {
		t.Error("should be able to undo with 2 checkpoints")
	}
}

func TestCanRedo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	taskID := "task-redo"

	// No checkpoints - can't redo
	can, err := g.CanRedo(ctx, taskID)
	if err != nil {
		t.Fatalf("CanRedo: %v", err)
	}
	if can {
		t.Error("should not be able to redo with no checkpoints")
	}
}

func TestUndoRedo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	taskID := "task-undoredo"

	// Create first checkpoint
	testFile := filepath.Join(dir, "undoredo.txt")
	if err := os.WriteFile(testFile, []byte("v1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "v1"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	// Create second checkpoint
	if err := os.WriteFile(testFile, []byte("v2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "v2"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	// Undo should go back to v1
	cp, err := g.Undo(ctx, taskID)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if cp.Number != 1 {
		t.Errorf("undo returned checkpoint %d, want 1", cp.Number)
	}

	content, _ := os.ReadFile(testFile)
	if string(content) != "v1" {
		t.Errorf("file content = %q, want %q", string(content), "v1")
	}

	// Redo should go back to v2
	cp, err = g.Redo(ctx, taskID)
	if err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if cp.Number != 2 {
		t.Errorf("redo returned checkpoint %d, want 2", cp.Number)
	}

	content, _ = os.ReadFile(testFile)
	if string(content) != "v2" {
		t.Errorf("file content = %q, want %q", string(content), "v2")
	}
}

func TestDeleteCheckpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	taskID := "task-del"

	if err := os.WriteFile(filepath.Join(dir, "del.txt"), []byte("del"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "to delete"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	err = g.DeleteCheckpoint(ctx, taskID, 1)
	if err != nil {
		t.Fatalf("DeleteCheckpoint: %v", err)
	}

	checkpoints, _ := g.ListCheckpoints(ctx, taskID)
	if len(checkpoints) != 0 {
		t.Errorf("expected 0 checkpoints after delete, got %d", len(checkpoints))
	}
}

func TestDeleteAllCheckpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	taskID := "task-delall"

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "cp1"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "cp2"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	err = g.DeleteAllCheckpoints(ctx, taskID)
	if err != nil {
		t.Fatalf("DeleteAllCheckpoints: %v", err)
	}

	checkpoints, _ := g.ListCheckpoints(ctx, taskID)
	if len(checkpoints) != 0 {
		t.Errorf("expected 0 checkpoints, got %d", len(checkpoints))
	}
}

func TestCheckpointTrackerBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tracker := NewCheckpointTracker(g, "tracked-task")

	if tracker.taskID != "tracked-task" {
		t.Errorf("taskID = %q, want %q", tracker.taskID, "tracked-task")
	}

	// Initially no checkpoints
	checkpoints, err := tracker.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(checkpoints) != 0 {
		t.Errorf("expected 0 checkpoints initially, got %d", len(checkpoints))
	}

	// UndoAvailable should be false
	if tracker.UndoAvailable(ctx) {
		t.Error("undo should not be available initially")
	}

	// RedoAvailable should be false
	if tracker.RedoAvailable(ctx) {
		t.Error("redo should not be available initially")
	}
}

func TestCheckpointTagRegexp(t *testing.T) {
	tests := []struct {
		tag      string
		wantTask string
		wantNum  string
	}{
		{"task-checkpoint/abc123/1", "abc123", "1"},
		{"task-checkpoint/task-456/42", "task-456", "42"},
		{"task-checkpoint/my-task/100", "my-task", "100"},
	}

	for _, tt := range tests {
		matches := checkpointTagRe.FindStringSubmatch(tt.tag)
		if matches == nil {
			t.Errorf("tag %q should match", tt.tag)
			continue
		}
		if matches[1] != tt.wantTask {
			t.Errorf("tag %q: task = %q, want %q", tt.tag, matches[1], tt.wantTask)
		}
		if matches[2] != tt.wantNum {
			t.Errorf("tag %q: num = %q, want %q", tt.tag, matches[2], tt.wantNum)
		}
	}
}

func TestCheckpointTagRegexpNoMatch(t *testing.T) {
	nonMatching := []string{
		"not-a-checkpoint",
		"task-checkpoint/",
		"task-checkpoint/task",
		"other-prefix/task/1",
	}

	for _, tag := range nonMatching {
		if checkpointTagRe.MatchString(tag) {
			t.Errorf("tag %q should not match", tag)
		}
	}
}

func TestRestoreCheckpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	taskID := "task-restore"
	testFile := filepath.Join(dir, "restore.txt")

	// Create checkpoints with different content
	if err := os.WriteFile(testFile, []byte("v1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "version 1"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	if err := os.WriteFile(testFile, []byte("v2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "version 2"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	if err := os.WriteFile(testFile, []byte("v3"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := g.CreateCheckpoint(ctx, taskID, "version 3"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	// Restore to checkpoint 1
	err = g.RestoreCheckpoint(ctx, taskID, 1)
	if err != nil {
		t.Fatalf("RestoreCheckpoint: %v", err)
	}

	content, _ := os.ReadFile(testFile)
	if string(content) != "v1" {
		t.Errorf("file content = %q, want %q", string(content), "v1")
	}
}

func TestGetChangeSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create some unstaged/untracked changes
	if err := os.WriteFile(filepath.Join(dir, "added.txt"), []byte("new file"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// GetChangeSummary returns current working directory changes (not per-checkpoint)
	summary, err := g.GetChangeSummary(ctx)
	if err != nil {
		t.Fatalf("GetChangeSummary: %v", err)
	}

	// Should have some changes (added.txt as new, README.md as modified)
	if summary.Total == 0 {
		t.Error("expected some files in summary")
	}
}

func TestGenerateAutoSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create a file change (don't commit it yet)
	if err := os.WriteFile(filepath.Join(dir, "auto.txt"), []byte("auto"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// GenerateAutoSummary() returns a summary of current working directory changes
	summary, err := g.GenerateAutoSummary(ctx)
	if err != nil {
		t.Fatalf("GenerateAutoSummary: %v", err)
	}
	if summary == "" {
		t.Error("expected non-empty auto summary")
	}
}

func TestGenerateAutoSummary_NoChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// No changes
	summary, err := g.GenerateAutoSummary(ctx)
	if err != nil {
		t.Fatalf("GenerateAutoSummary: %v", err)
	}
	if summary != "no changes" {
		t.Errorf("expected 'no changes', got %q", summary)
	}
}

func TestCreateCheckpointAutoSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	taskID := "task-auto"

	// Create a file change
	if err := os.WriteFile(filepath.Join(dir, "autocp.txt"), []byte("auto checkpoint"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cp, err := g.CreateCheckpointAutoSummary(ctx, taskID)
	if err != nil {
		t.Fatalf("CreateCheckpointAutoSummary: %v", err)
	}

	if cp.Number != 1 {
		t.Errorf("checkpoint number = %d, want 1", cp.Number)
	}
	if cp.Message == "" {
		t.Error("checkpoint should have auto-generated message")
	}
}

func TestCheckpointTracker_SaveAndUndoRedo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tracker := NewCheckpointTracker(g, "tracker-test")
	testFile := filepath.Join(dir, "tracker.txt")

	// Save checkpoints
	if err := os.WriteFile(testFile, []byte("v1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err = tracker.Save(ctx, "version 1")
	if err != nil {
		t.Fatalf("Save 1: %v", err)
	}

	if err := os.WriteFile(testFile, []byte("v2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err = tracker.Save(ctx, "version 2")
	if err != nil {
		t.Fatalf("Save 2: %v", err)
	}

	// Check UndoAvailable
	if !tracker.UndoAvailable(ctx) {
		t.Error("undo should be available after 2 saves")
	}

	// Undo
	cp, err := tracker.Undo(ctx)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if cp.Number != 1 {
		t.Errorf("undo returned %d, want 1", cp.Number)
	}

	// Check RedoAvailable
	if !tracker.RedoAvailable(ctx) {
		t.Error("redo should be available after undo")
	}

	// Redo
	cp, err = tracker.Redo(ctx)
	if err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if cp.Number != 2 {
		t.Errorf("redo returned %d, want 2", cp.Number)
	}
}

func TestCheckpointTracker_SaveAuto(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	dir := initTestRepo(t)
	g, err := New(ctx, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tracker := NewCheckpointTracker(g, "auto-save-test")

	// Create a file
	if err := os.WriteFile(filepath.Join(dir, "autosave.txt"), []byte("auto"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cp, err := tracker.SaveAuto(ctx)
	if err != nil {
		t.Fatalf("SaveAuto: %v", err)
	}

	if cp.Number != 1 {
		t.Errorf("checkpoint number = %d, want 1", cp.Number)
	}
}
