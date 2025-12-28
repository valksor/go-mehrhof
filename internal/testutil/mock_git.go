// Package testutil provides shared testing utilities for go-mehrhof tests.
package testutil

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/valksor/go-mehrhof/internal/vcs"
)

// MockGit is a mock implementation of vcs.Git for testing.
type MockGit struct {
	mu sync.Mutex

	// Config
	RootDir    string
	isWorktree bool
	MainRepo   string

	// Branch state
	currentBranch string
	BaseBranch    string
	Branches      map[string]bool

	// Checkpoints (commits)
	Checkpoints     map[string][]vcs.Checkpoint
	CheckpointIndex map[string]int // Current position for undo/redo

	// Worktrees
	Worktrees map[string]string // path -> branch

	// Call tracking
	CreatesBranch     []string
	CreatesWorktree   []string
	DeletesBranch     []string
	DeletesWorktree   []string
	Checkouts         []string
	Undos             []string
	Redos             []string
	CreateCheckpoints []CheckpointCreateParams

	// Error simulation
	BranchError     error
	CheckoutError   error
	HasChangesError error
	HasChangesValue bool
	UndoError       error
	RedoError       error

	// File operations
	FilesChanged map[string]bool
}

// CheckpointCreateParams holds parameters for checkpoint creation.
type CheckpointCreateParams struct {
	TaskID  string
	Message string
	Prefix  string
}

// NewMockGit creates a new mock Git instance.
func NewMockGit(rootDir string) *MockGit {
	return &MockGit{
		RootDir:           rootDir,
		isWorktree:        false,
		BaseBranch:        "main",
		currentBranch:     "main",
		Branches:          map[string]bool{"main": true},
		Checkpoints:       make(map[string][]vcs.Checkpoint),
		CheckpointIndex:   make(map[string]int),
		Worktrees:         make(map[string]string),
		CreatesBranch:     make([]string, 0),
		CreatesWorktree:   make([]string, 0),
		DeletesBranch:     make([]string, 0),
		DeletesWorktree:   make([]string, 0),
		Checkouts:         make([]string, 0),
		Undos:             make([]string, 0),
		Redos:             make([]string, 0),
		CreateCheckpoints: make([]CheckpointCreateParams, 0),
		FilesChanged:      make(map[string]bool),
	}
}

// Root returns the root directory of the git repository.
func (m *MockGit) Root() string {
	return m.RootDir
}

// IsWorktree returns true if the current directory is a git worktree.
func (m *MockGit) IsWorktree() bool {
	return m.isWorktree
}

// GetMainWorktreePath returns the path to the main git repository.
func (m *MockGit) GetMainWorktreePath() (string, error) {
	if m.MainRepo != "" {
		return m.MainRepo, nil
	}
	if m.isWorktree {
		return filepath.Dir(m.RootDir), nil
	}
	return m.RootDir, nil
}

// CurrentBranch returns the current branch name.
func (m *MockGit) CurrentBranch() (string, error) {
	return m.currentBranch, nil
}

// GetBaseBranch returns the base branch name.
func (m *MockGit) GetBaseBranch() (string, error) {
	return m.BaseBranch, nil
}

// CreateBranch creates a new branch.
func (m *MockGit) CreateBranch(name, base string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.BranchError != nil {
		return m.BranchError
	}

	m.Branches[name] = true
	m.CreatesBranch = append(m.CreatesBranch, name)
	return nil
}

// Checkout switches to a branch.
func (m *MockGit) Checkout(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.CheckoutError != nil {
		return m.CheckoutError
	}

	m.currentBranch = name
	m.Checkouts = append(m.Checkouts, name)
	return nil
}

// DeleteBranch deletes a branch.
func (m *MockGit) DeleteBranch(name string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.Branches, name)
	m.DeletesBranch = append(m.DeletesBranch, name)
	return nil
}

// BranchExists checks if a branch exists.
func (m *MockGit) BranchExists(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Branches[name]
}

// GetWorktreePath returns the worktree path for a task ID.
func (m *MockGit) GetWorktreePath(taskID string) string {
	return filepath.Join(m.RootDir, "..", "worktrees", taskID)
}

// EnsureWorktreesDir creates the worktrees directory.
func (m *MockGit) EnsureWorktreesDir() error {
	return nil // No-op for mock
}

// CreateWorktreeNewBranch creates a new worktree with a new branch.
func (m *MockGit) CreateWorktreeNewBranch(path, branch, base string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Worktrees[path] = branch
	m.Branches[branch] = true
	m.CreatesWorktree = append(m.CreatesWorktree, path)
	return nil
}

// RemoveWorktree removes a worktree.
func (m *MockGit) RemoveWorktree(path string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.Worktrees, path)
	m.DeletesWorktree = append(m.DeletesWorktree, path)
	return nil
}

// ListWorktrees lists all worktrees.
func (m *MockGit) ListWorktrees() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	paths := make([]string, 0, len(m.Worktrees))
	for path := range m.Worktrees {
		paths = append(paths, path)
	}
	return paths, nil
}

// HasChanges checks if there are uncommitted changes.
func (m *MockGit) HasChanges() (bool, error) {
	if m.HasChangesError != nil {
		return false, m.HasChangesError
	}
	return m.HasChangesValue, nil
}

// CreateCheckpointWithPrefix creates a checkpoint with a custom prefix.
func (m *MockGit) CreateCheckpointWithPrefix(taskID, message, prefix string) (vcs.Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	num := len(m.Checkpoints[taskID]) + 1
	checkpoint := vcs.Checkpoint{
		Number:  num,
		ID:      fmt.Sprintf("commit-%s-%d", taskID, num),
		Message: prefix + " " + message,
	}

	m.Checkpoints[taskID] = append(m.Checkpoints[taskID], checkpoint)
	m.CheckpointIndex[taskID] = len(m.Checkpoints[taskID])
	m.CreateCheckpoints = append(m.CreateCheckpoints, CheckpointCreateParams{
		TaskID:  taskID,
		Message: message,
		Prefix:  prefix,
	})

	return checkpoint, nil
}

// ListCheckpoints lists all checkpoints for a task.
func (m *MockGit) ListCheckpoints(taskID string) ([]vcs.Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Checkpoints[taskID], nil
}

// CanUndo checks if undo is possible.
func (m *MockGit) CanUndo(taskID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx, ok := m.CheckpointIndex[taskID]
	return ok && idx > 1, nil
}

// CanRedo checks if redo is possible.
func (m *MockGit) CanRedo(taskID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx, ok := m.CheckpointIndex[taskID]
	checkpoints := m.Checkpoints[taskID]
	return ok && idx < len(checkpoints), nil
}

// Undo undoes to the previous checkpoint.
func (m *MockGit) Undo(taskID string) (vcs.Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.UndoError != nil {
		return vcs.Checkpoint{}, m.UndoError
	}

	idx := m.CheckpointIndex[taskID]
	if idx <= 1 {
		return vcs.Checkpoint{}, fmt.Errorf("nothing to undo")
	}

	idx--
	m.CheckpointIndex[taskID] = idx
	m.Undos = append(m.Undos, taskID)

	return m.Checkpoints[taskID][idx-1], nil
}

// Redo redoes to the next checkpoint.
func (m *MockGit) Redo(taskID string) (vcs.Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.RedoError != nil {
		return vcs.Checkpoint{}, m.RedoError
	}

	idx := m.CheckpointIndex[taskID]
	checkpoints := m.Checkpoints[taskID]

	if idx >= len(checkpoints) {
		return vcs.Checkpoint{}, fmt.Errorf("nothing to redo")
	}

	idx++
	m.CheckpointIndex[taskID] = idx
	m.Redos = append(m.Redos, taskID)

	return checkpoints[idx-1], nil
}

// DeleteAllCheckpoints deletes all checkpoints for a task.
func (m *MockGit) DeleteAllCheckpoints(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.Checkpoints, taskID)
	delete(m.CheckpointIndex, taskID)
	return nil
}

// PushBranch pushes a branch to remote.
func (m *MockGit) PushBranch(branch, remote string, setUpstream bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// No-op for mock
	return nil
}

// MergeBranch merges a branch.
func (m *MockGit) MergeBranch(branch string, fastForward bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// No-op for mock
	return nil
}

// MergeSquash performs a squash merge.
func (m *MockGit) MergeSquash(branch string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// No-op for mock
	return nil
}

// Commit creates a commit.
func (m *MockGit) Commit(message string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a fake commit ID
	commitID := fmt.Sprintf("commit-%d", len(m.Checkpoints)+1)
	return commitID, nil
}

// Diff returns git diff output.
func (m *MockGit) Diff(args ...string) (string, error) {
	// Return mock diff output
	return " file1.txt | 1 +\n 1 file changed, 1 insertion(+)", nil
}

// AddFileChanged marks a file as changed.
func (m *MockGit) AddFileChanged(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FilesChanged[path] = true
}

// ClearFileChanges clears all tracked file changes.
func (m *MockGit) ClearFileChanges() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FilesChanged = make(map[string]bool)
}

// SetWorktreeMode sets whether this is a worktree.
func (m *MockGit) SetWorktreeMode(isWorktree bool, mainRepo string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isWorktree = isWorktree
	m.MainRepo = mainRepo
}

// SetHasChanges sets the return value for HasChanges.
func (m *MockGit) SetHasChanges(hasChanges bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.HasChangesValue = hasChanges
}

// CreateCheckpoint is a convenience method for CreateCheckpointWithPrefix.
func (m *MockGit) CreateCheckpoint(taskID, message string) (vcs.Checkpoint, error) {
	return m.CreateCheckpointWithPrefix(taskID, message, "["+taskID+"]")
}
