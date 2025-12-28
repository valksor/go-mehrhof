// Package testutil provides shared testing utilities for go-mehrhof tests.
package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/valksor/go-mehrhof/internal/storage"
)

// MockWorkspace is a mock implementation of storage.Workspace for testing.
type MockWorkspace struct {
	mu sync.Mutex

	// Root directory
	RootDir string

	// Tasks
	Tasks     map[string]*storage.ActiveTask
	TaskWorks map[string]*storage.TaskWork
	Specs     map[string]map[int]string // taskID -> number -> content
	Notes     map[string]string
	Sessions  map[string][]*storage.Session
	Questions map[string]*storage.PendingQuestion
	Config    *storage.WorkspaceConfig

	// Call tracking
	SaveActiveTaskCalls     int
	LoadActiveTaskCalls     int
	ClearActiveTaskCalls    int
	CreateWorkCalls         int
	SaveWorkCalls           int
	LoadWorkCalls           int
	DeleteWorkCalls         int
	SaveSpecificationCalls  int
	LoadSpecificationCalls  int
	ListSpecificationsCalls int
	SaveSessionCalls        int
	SaveNoteCalls           int
	ReadNotesCalls          int

	// Error simulation
	SaveActiveTaskError    error
	LoadActiveTaskError    error
	CreateWorkError        error
	SaveWorkError          error
	LoadWorkError          error
	SaveSpecificationError error
	SaveNoteError          error
	SaveSessionError       error
}

// NewMockWorkspace creates a new mock workspace.
func NewMockWorkspace(rootDir string) *MockWorkspace {
	return &MockWorkspace{
		RootDir:   rootDir,
		Tasks:     make(map[string]*storage.ActiveTask),
		TaskWorks: make(map[string]*storage.TaskWork),
		Specs:     make(map[string]map[int]string),
		Notes:     make(map[string]string),
		Sessions:  make(map[string][]*storage.Session),
		Questions: make(map[string]*storage.PendingQuestion),
		Config:    SampleWorkspaceConfig(),
	}
}

// Root returns the workspace root directory.
func (m *MockWorkspace) Root() string {
	return m.RootDir
}

// WorkPath returns the work directory path for a task.
func (m *MockWorkspace) WorkPath(taskID string) string {
	return filepath.Join(m.RootDir, ".mehrhof", "work", taskID)
}

// EnsureInitialized ensures the workspace is initialized.
func (m *MockWorkspace) EnsureInitialized() error {
	if err := os.MkdirAll(m.RootDir, 0o755); err != nil {
		return err
	}
	mehrhofDir := filepath.Join(m.RootDir, ".mehrhof")
	return os.MkdirAll(mehrhofDir, 0o755)
}

// IsInitialized checks if the workspace is initialized.
func (m *MockWorkspace) IsInitialized() bool {
	_, err := os.Stat(filepath.Join(m.RootDir, ".mehrhof"))
	return err == nil
}

// LoadConfig loads the workspace configuration.
func (m *MockWorkspace) LoadConfig() (*storage.WorkspaceConfig, error) {
	return m.Config, nil
}

// SaveConfig saves the workspace configuration.
func (m *MockWorkspace) SaveConfig(cfg *storage.WorkspaceConfig) error {
	m.Config = cfg
	return nil
}

// HasActiveTask checks if there's an active task.
func (m *MockWorkspace) HasActiveTask() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, task := range m.Tasks {
		if task != nil {
			return true
		}
	}
	return false
}

// LoadActiveTask loads the active task.
func (m *MockWorkspace) LoadActiveTask() (*storage.ActiveTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LoadActiveTaskCalls++

	if m.LoadActiveTaskError != nil {
		return nil, m.LoadActiveTaskError
	}

	for _, task := range m.Tasks {
		if task != nil {
			return task, nil
		}
	}
	return nil, fmt.Errorf("no active task")
}

// SaveActiveTask saves the active task.
func (m *MockWorkspace) SaveActiveTask(task *storage.ActiveTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SaveActiveTaskCalls++

	if m.SaveActiveTaskError != nil {
		return m.SaveActiveTaskError
	}

	m.Tasks[task.ID] = task
	return nil
}

// ClearActiveTask clears the active task.
func (m *MockWorkspace) ClearActiveTask() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ClearActiveTaskCalls++

	m.Tasks = make(map[string]*storage.ActiveTask)
	return nil
}

// CreateWork creates a new task work directory.
func (m *MockWorkspace) CreateWork(taskID string, source storage.SourceInfo) (*storage.TaskWork, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreateWorkCalls++

	if m.CreateWorkError != nil {
		return nil, m.CreateWorkError
	}

	work := &storage.TaskWork{
		Version: "1",
		Metadata: storage.WorkMetadata{
			ID: taskID,
		},
		Source: source,
	}

	m.TaskWorks[taskID] = work
	m.Specs[taskID] = make(map[int]string)
	return work, nil
}

// LoadWork loads task work by ID.
func (m *MockWorkspace) LoadWork(taskID string) (*storage.TaskWork, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LoadWorkCalls++

	if m.LoadWorkError != nil {
		return nil, m.LoadWorkError
	}

	work, ok := m.TaskWorks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found")
	}
	return work, nil
}

// SaveWork saves task work.
func (m *MockWorkspace) SaveWork(work *storage.TaskWork) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SaveWorkCalls++

	if m.SaveWorkError != nil {
		return m.SaveWorkError
	}

	m.TaskWorks[work.Metadata.ID] = work
	return nil
}

// DeleteWork deletes task work.
func (m *MockWorkspace) DeleteWork(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteWorkCalls++

	delete(m.TaskWorks, taskID)
	delete(m.Specs, taskID)
	delete(m.Notes, taskID)
	return nil
}

// ListTasks lists all tasks.
func (m *MockWorkspace) ListTasks() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := make([]string, 0, len(m.TaskWorks))
	for id := range m.TaskWorks {
		ids = append(ids, id)
	}
	return ids, nil
}

// FindTaskByWorktreePath finds a task by its worktree path.
func (m *MockWorkspace) FindTaskByWorktreePath(path string) (*storage.ActiveTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.Tasks {
		if task != nil && task.WorktreePath == path {
			return task, nil
		}
	}
	return nil, fmt.Errorf("task not found")
}

// GetSourceContent retrieves the source content for a task.
func (m *MockWorkspace) GetSourceContent(taskID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	work, ok := m.TaskWorks[taskID]
	if !ok {
		return "", nil
	}
	return work.Source.Content, nil
}

// NextSpecificationNumber returns the next specification number.
func (m *MockWorkspace) NextSpecificationNumber(taskID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	specs := m.Specs[taskID]
	if len(specs) == 0 {
		return 1, nil
	}
	maxNum := 0
	for num := range specs {
		if num > maxNum {
			maxNum = num
		}
	}
	return maxNum + 1, nil
}

// SaveSpecification saves a specification.
func (m *MockWorkspace) SaveSpecification(taskID string, num int, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SaveSpecificationCalls++

	if m.SaveSpecificationError != nil {
		return m.SaveSpecificationError
	}

	if m.Specs[taskID] == nil {
		m.Specs[taskID] = make(map[int]string)
	}
	m.Specs[taskID][num] = content
	return nil
}

// LoadSpecification loads a specification.
func (m *MockWorkspace) LoadSpecification(taskID string, num int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LoadSpecificationCalls++

	specs, ok := m.Specs[taskID]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	content, ok := specs[num]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return content, nil
}

// ListSpecifications lists all specification numbers for a task.
func (m *MockWorkspace) ListSpecifications(taskID string) ([]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListSpecificationsCalls++

	specs := m.Specs[taskID]
	nums := make([]int, 0, len(specs))
	for num := range specs {
		nums = append(nums, num)
	}
	return nums, nil
}

// GatherSpecificationsContent gathers all specifications content.
func (m *MockWorkspace) GatherSpecificationsContent(taskID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	specs := m.Specs[taskID]
	var content string
	for num := 1; num <= len(specs); num++ {
		if specContent, ok := specs[num]; ok {
			content += specContent + "\n\n"
		}
	}
	return content, nil
}

// GetLatestSpecificationContent gets the latest specification content.
func (m *MockWorkspace) GetLatestSpecificationContent(taskID string) (string, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	specs := m.Specs[taskID]
	if len(specs) == 0 {
		return "", 0, nil
	}
	maxNum := 0
	for num := range specs {
		if num > maxNum {
			maxNum = num
		}
	}
	return specs[maxNum], maxNum, nil
}

// AppendNote appends a note to the task.
func (m *MockWorkspace) AppendNote(taskID, content, phase string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SaveNoteCalls++

	if m.SaveNoteError != nil {
		return m.SaveNoteError
	}

	existing := m.Notes[taskID]
	m.Notes[taskID] = existing + "\n\n" + content
	return nil
}

// ReadNotes reads all notes for a task.
func (m *MockWorkspace) ReadNotes(taskID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReadNotesCalls++

	return m.Notes[taskID], nil
}

// CreateSession creates a new agent session.
func (m *MockWorkspace) CreateSession(taskID, phase, agent, state string) (*storage.Session, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	session := &storage.Session{
		Metadata: storage.SessionMetadata{
			StartedAt: now,
			Type:      phase,
			Agent:     agent,
			State:     state,
		},
	}

	filename := filepath.Join("sessions", now.Format("20060102-150405")+"-"+phase+".yaml")
	if m.Sessions[taskID] == nil {
		m.Sessions[taskID] = make([]*storage.Session, 0)
	}
	m.Sessions[taskID] = append(m.Sessions[taskID], session)

	return session, filename, nil
}

// SaveSession saves a session.
func (m *MockWorkspace) SaveSession(taskID string, filename string, session *storage.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SaveSessionCalls++

	if m.SaveSessionError != nil {
		return m.SaveSessionError
	}

	// Update the session in the list
	sessions := m.Sessions[taskID]
	for i, s := range sessions {
		// Match by started time as a proxy
		if s.Metadata.StartedAt.Equal(session.Metadata.StartedAt) {
			sessions[i] = session
			return nil
		}
	}
	return nil
}

// ListSessions lists all sessions for a task.
func (m *MockWorkspace) ListSessions(taskID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessions := m.Sessions[taskID]
	filenames := make([]string, len(sessions))
	for i, s := range sessions {
		filenames[i] = s.Metadata.StartedAt.Format("20060102-150405") + "-" + s.Metadata.Type + ".yaml"
	}
	return filenames, nil
}

// LoadSession loads a session by filename.
func (m *MockWorkspace) LoadSession(taskID, filename string) (*storage.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessions := m.Sessions[taskID]
	for _, s := range sessions {
		fn := s.Metadata.StartedAt.Format("20060102-150405") + "-" + s.Metadata.Type + ".yaml"
		if fn == filename {
			return s, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

// SavePendingQuestion saves a pending agent question.
func (m *MockWorkspace) SavePendingQuestion(taskID string, pq *storage.PendingQuestion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Questions[taskID] = pq
	return nil
}

// LoadPendingQuestion loads a pending question.
func (m *MockWorkspace) LoadPendingQuestion(taskID string) (*storage.PendingQuestion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Questions[taskID], nil
}

// HasPendingQuestion checks if there's a pending question.
func (m *MockWorkspace) HasPendingQuestion(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Questions[taskID] != nil
}

// ClearPendingQuestion clears a pending question.
func (m *MockWorkspace) ClearPendingQuestion(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.Questions, taskID)
	return nil
}

// SetActiveTask sets an active task directly (helper for testing).
func (m *MockWorkspace) SetActiveTask(task *storage.ActiveTask) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Tasks[task.ID] = task
}

// SetTaskWork sets task work directly (helper for testing).
func (m *MockWorkspace) SetTaskWork(work *storage.TaskWork) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TaskWorks[work.Metadata.ID] = work
}

// AddSpecification adds a specification directly (helper for testing).
func (m *MockWorkspace) AddSpecification(taskID string, num int, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Specs[taskID] == nil {
		m.Specs[taskID] = make(map[int]string)
	}
	m.Specs[taskID][num] = content
}

// SetNotes sets notes directly (helper for testing).
func (m *MockWorkspace) SetNotes(taskID, notes string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Notes[taskID] = notes
}

// SetConfig sets the workspace config directly (helper for testing).
func (m *MockWorkspace) SetConfig(cfg *storage.WorkspaceConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Config = cfg
}

// GetTaskCount returns the number of tasks.
func (m *MockWorkspace) GetTaskCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.TaskWorks)
}

// GetSpecificationCount returns the number of specifications for a task.
func (m *MockWorkspace) GetSpecificationCount(taskID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Specs[taskID])
}
