package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCheckpointManager(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)
	if m == nil {
		t.Fatal("NewCheckpointManager returned nil")
	}
}

func TestCheckpointManager_Empty(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)

	if m.CanUndo() {
		t.Error("CanUndo() should be false on empty manager")
	}
	if m.CanRedo() {
		t.Error("CanRedo() should be false on empty manager")
	}
	if m.Current() != nil {
		t.Error("Current() should return nil on empty manager")
	}
	if len(m.List()) != 0 {
		t.Errorf("List() should be empty, got %d", len(m.List()))
	}
}

func TestCheckpointManager_Create_NoChanges(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)

	// No changes — Create should still succeed (uses current commit)
	cp, err := m.Create(ctx, "loaded", "initial checkpoint", "")
	if err != nil {
		t.Fatalf("Create() with no changes error = %v", err)
	}
	if cp == nil {
		t.Fatal("Create() returned nil checkpoint")
	}
	if cp.ID != "cp-1" {
		t.Errorf("checkpoint ID = %q, want cp-1", cp.ID)
	}
	if cp.State != "loaded" {
		t.Errorf("checkpoint State = %q, want loaded", cp.State)
	}
	if cp.Message != "initial checkpoint" {
		t.Errorf("checkpoint Message = %q, want 'initial checkpoint'", cp.Message)
	}
	if len(cp.CommitSHA) != 40 {
		t.Errorf("checkpoint CommitSHA length = %d, want 40", len(cp.CommitSHA))
	}
}

func TestCheckpointManager_Create_WithChanges(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)

	// Make a change
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cp, err := m.Create(ctx, "implementing", "add new.txt", "")
	if err != nil {
		t.Fatalf("Create() with changes error = %v", err)
	}
	if cp == nil {
		t.Fatal("Create() returned nil")
	}
	if cp.CommitSHA == "" {
		t.Error("checkpoint CommitSHA should not be empty")
	}
}

func TestCheckpointManager_ListAndCurrent(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)

	// Create two checkpoints
	cp1, err := m.Create(ctx, "loaded", "checkpoint 1", "")
	if err != nil {
		t.Fatalf("Create(checkpoint 1) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cp2, err := m.Create(ctx, "implementing", "checkpoint 2", "")
	if err != nil {
		t.Fatalf("Create(checkpoint 2) error = %v", err)
	}

	checkpoints := m.List()
	if len(checkpoints) != 2 {
		t.Fatalf("List() length = %d, want 2", len(checkpoints))
	}

	current := m.Current()
	if current == nil {
		t.Fatal("Current() should not be nil after creating checkpoints")
	}
	if current.ID != cp2.ID {
		t.Errorf("Current().ID = %q, want %q (latest)", current.ID, cp2.ID)
	}

	_ = cp1 // used for clarity
}

func TestCheckpointManager_Undo(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)

	// Create first checkpoint
	cp1, err := m.Create(ctx, "loaded", "first", "")
	if err != nil {
		t.Fatalf("Create(first): %v", err)
	}

	// Make a change and create second checkpoint
	if err := os.WriteFile(filepath.Join(dir, "second.txt"), []byte("y"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err = m.Create(ctx, "implementing", "second", "")
	if err != nil {
		t.Fatalf("Create(second): %v", err)
	}

	if !m.CanUndo() {
		t.Error("CanUndo() should be true with 2 checkpoints at index 1")
	}

	undone, err := m.Undo(ctx)
	if err != nil {
		t.Fatalf("Undo() error = %v", err)
	}
	if undone == nil {
		t.Fatal("Undo() returned nil")
	}
	if undone.ID != cp1.ID {
		t.Errorf("Undo() returned checkpoint %q, want %q", undone.ID, cp1.ID)
	}
	if m.Current().ID != cp1.ID {
		t.Errorf("Current() after Undo = %q, want %q", m.Current().ID, cp1.ID)
	}
}

func TestCheckpointManager_Undo_NothingToUndo(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)

	// Create only one checkpoint — can't undo
	_, _ = m.Create(ctx, "loaded", "first", "")
	_, err = m.Undo(ctx)
	if err == nil {
		t.Error("Undo() with single checkpoint should return error")
	}
}

func TestCheckpointManager_Redo(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)

	// Create two checkpoints
	_, err = m.Create(ctx, "loaded", "first", "")
	if err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "redo.txt"), []byte("r"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cp2, err := m.Create(ctx, "implementing", "second", "")
	if err != nil {
		t.Fatalf("Create(second) error = %v", err)
	}

	// Undo to get back to first
	_, _ = m.Undo(ctx)

	if !m.CanRedo() {
		t.Error("CanRedo() should be true after Undo")
	}

	redone, err := m.Redo(ctx)
	if err != nil {
		t.Fatalf("Redo() error = %v", err)
	}
	if redone == nil {
		t.Fatal("Redo() returned nil")
	}
	if redone.ID != cp2.ID {
		t.Errorf("Redo() returned checkpoint %q, want %q", redone.ID, cp2.ID)
	}
}

func TestCheckpointManager_Redo_NothingToRedo(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)

	_, _ = m.Create(ctx, "loaded", "first", "")

	_, err = m.Redo(ctx)
	if err == nil {
		t.Error("Redo() with nothing to redo should return error")
	}
}

func TestCheckpointManager_GoTo(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)

	cp1, err := m.Create(ctx, "loaded", "first", "")
	if err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "goto.txt"), []byte("g"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _ = m.Create(ctx, "implementing", "second", "")

	// GoTo the first checkpoint
	result, err := m.GoTo(ctx, cp1.ID)
	if err != nil {
		t.Fatalf("GoTo() error = %v", err)
	}
	if result.ID != cp1.ID {
		t.Errorf("GoTo() result ID = %q, want %q", result.ID, cp1.ID)
	}
	if m.Current().ID != cp1.ID {
		t.Errorf("Current() after GoTo = %q, want %q", m.Current().ID, cp1.ID)
	}
}

func TestCheckpointManager_GoTo_NotFound(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)

	_, err = m.GoTo(ctx, "cp-nonexistent")
	if err == nil {
		t.Error("GoTo() with unknown ID should return error")
	}
}

func TestCheckpointManager_Create_TruncatesHistoryOnUndo(t *testing.T) {
	ctx := context.Background()
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	m := NewCheckpointManager(repo)

	_, _ = m.Create(ctx, "loaded", "first", "")
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _ = m.Create(ctx, "implementing", "second", "")

	// Undo to first
	_, _ = m.Undo(ctx)

	// Now create a new checkpoint — should truncate "second" from history
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	newCp, err := m.Create(ctx, "implementing", "new second", "")
	if err != nil {
		t.Fatalf("Create after Undo: %v", err)
	}
	if newCp == nil {
		t.Fatal("Create after Undo returned nil checkpoint")
	}
	// History should have exactly 2 entries
	if len(m.List()) != 2 {
		t.Errorf("List() length after truncation = %d, want 2", len(m.List()))
	}
	if m.CanRedo() {
		t.Error("CanRedo() should be false after creating new checkpoint")
	}
}
