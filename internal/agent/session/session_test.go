package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp/test-task")

	expectedDir := filepath.Join("/tmp/test-task", SessionDir)
	if m.baseDir != expectedDir {
		t.Errorf("NewManager() baseDir = %q, want %q", m.baseDir, expectedDir)
	}
}

func TestState_ToSummary(t *testing.T) {
	now := time.Now()
	state := &State{
		ID:             "test-id-12345",
		TaskID:         "task-123",
		AgentName:      "claude",
		Phase:          "implementing",
		Status:         StatusInterrupted,
		StartedAt:      now.Add(-1 * time.Hour),
		CheckpointedAt: now,
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
		},
		Error: "timeout",
	}

	summary := state.ToSummary()

	if summary.ID != state.ID {
		t.Errorf("ToSummary().ID = %q, want %q", summary.ID, state.ID)
	}
	if summary.TaskID != state.TaskID {
		t.Errorf("ToSummary().TaskID = %q, want %q", summary.TaskID, state.TaskID)
	}
	if summary.MessageCount != 2 {
		t.Errorf("ToSummary().MessageCount = %d, want 2", summary.MessageCount)
	}
	if summary.Error != "timeout" {
		t.Errorf("ToSummary().Error = %q, want %q", summary.Error, "timeout")
	}
}

func TestState_IsRecoverable(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		messages []Message
		want     bool
	}{
		{
			name:   "active session",
			status: StatusActive,
			want:   false,
		},
		{
			name:   "completed session",
			status: StatusCompleted,
			want:   false,
		},
		{
			name:   "interrupted session",
			status: StatusInterrupted,
			want:   true,
		},
		{
			name:   "recoverable session",
			status: StatusRecoverable,
			want:   true,
		},
		{
			name:   "failed session with messages",
			status: StatusFailed,
			messages: []Message{
				{Role: "user", Content: "test"},
			},
			want: true,
		},
		{
			name:   "failed session without messages",
			status: StatusFailed,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &State{
				Status:   tt.status,
				Messages: tt.messages,
			}

			got := state.IsRecoverable()
			if got != tt.want {
				t.Errorf("IsRecoverable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestState_GetLastUserMessage(t *testing.T) {
	state := &State{
		Messages: []Message{
			{Role: "user", Content: "First"},
			{Role: "assistant", Content: "Response"},
			{Role: "user", Content: "Second"},
		},
	}

	msg := state.GetLastUserMessage()
	if msg == nil {
		t.Fatal("GetLastUserMessage() returned nil")
	}
	if msg.Content != "Second" {
		t.Errorf("GetLastUserMessage().Content = %q, want %q", msg.Content, "Second")
	}
}

func TestState_GetLastUserMessage_NoUserMessages(t *testing.T) {
	state := &State{
		Messages: []Message{
			{Role: "assistant", Content: "Response"},
		},
	}

	msg := state.GetLastUserMessage()
	if msg != nil {
		t.Errorf("GetLastUserMessage() = %v, want nil", msg)
	}
}

func TestState_GetLastAssistantMessage(t *testing.T) {
	state := &State{
		Messages: []Message{
			{Role: "user", Content: "First"},
			{Role: "assistant", Content: "First Response"},
			{Role: "user", Content: "Second"},
			{Role: "assistant", Content: "Second Response"},
		},
	}

	msg := state.GetLastAssistantMessage()
	if msg == nil {
		t.Fatal("GetLastAssistantMessage() returned nil")
	}
	if msg.Content != "Second Response" {
		t.Errorf("GetLastAssistantMessage().Content = %q, want %q", msg.Content, "Second Response")
	}
}

func TestManager_SaveAndLoad(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	m := NewManager(tempDir)

	// Create a state
	state := &State{
		ID:        generateID(),
		TaskID:    "test-task",
		AgentName: "claude",
		Phase:     "implementing",
		Status:    StatusActive,
		StartedAt: time.Now(),
		Messages: []Message{
			{Role: "user", Content: "Implement feature X"},
			{Role: "assistant", Content: "I'll help with that"},
		},
		Context: SessionContext{
			Specifications: []string{"spec-1.md"},
			FilesRead:      []string{"main.go"},
			GitBranch:      "feature/test",
		},
		Usage: &agent.UsageStats{
			InputTokens:  1000,
			OutputTokens: 500,
		},
	}

	// Save
	err := m.Save(state)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	loaded, err := m.Load(state.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify
	if loaded.ID != state.ID {
		t.Errorf("Loaded.ID = %q, want %q", loaded.ID, state.ID)
	}
	if loaded.TaskID != state.TaskID {
		t.Errorf("Loaded.TaskID = %q, want %q", loaded.TaskID, state.TaskID)
	}
	if loaded.AgentName != state.AgentName {
		t.Errorf("Loaded.AgentName = %q, want %q", loaded.AgentName, state.AgentName)
	}
	if len(loaded.Messages) != len(state.Messages) {
		t.Errorf("Loaded.Messages len = %d, want %d", len(loaded.Messages), len(state.Messages))
	}
	if loaded.Context.GitBranch != state.Context.GitBranch {
		t.Errorf("Loaded.Context.GitBranch = %q, want %q", loaded.Context.GitBranch, state.Context.GitBranch)
	}
}

func TestManager_List(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(tempDir)

	// Save multiple sessions
	for range 3 {
		state := &State{
			ID:        generateID(),
			TaskID:    "test-task",
			AgentName: "claude",
			Phase:     "implementing",
			Status:    StatusActive,
			StartedAt: time.Now(),
		}
		if err := m.Save(state); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List
	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("List() returned %d sessions, want 3", len(sessions))
	}

	// Verify sorted by newest first
	for i := range len(sessions) - 1 {
		if sessions[i].CheckpointedAt.Before(sessions[i+1].CheckpointedAt) {
			t.Error("List() sessions not sorted by newest first")
		}
	}
}

func TestManager_ListRecoverable(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(tempDir)

	// Save sessions with different statuses
	statuses := []Status{StatusActive, StatusCompleted, StatusInterrupted, StatusRecoverable}
	for _, status := range statuses {
		state := &State{
			ID:        generateID(),
			TaskID:    "test-task",
			AgentName: "claude",
			Phase:     "implementing",
			Status:    status,
			StartedAt: time.Now(),
		}
		if err := m.Save(state); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// List recoverable
	recoverable, err := m.ListRecoverable()
	if err != nil {
		t.Fatalf("ListRecoverable() error = %v", err)
	}

	if len(recoverable) != 2 { // Interrupted + Recoverable
		t.Errorf("ListRecoverable() returned %d sessions, want 2", len(recoverable))
	}
}

func TestManager_Delete(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(tempDir)

	// Save a session
	state := &State{
		ID:        generateID(),
		TaskID:    "test-task",
		AgentName: "claude",
		Phase:     "implementing",
		Status:    StatusActive,
		StartedAt: time.Now(),
	}
	if err := m.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify it exists
	_, err := m.Load(state.ID)
	if err != nil {
		t.Fatalf("Load() after save error = %v", err)
	}

	// Delete
	if err := m.Delete(state.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify it's gone
	_, err = m.Load(state.ID)
	if err == nil {
		t.Error("Load() should fail after delete")
	}
}

func TestManager_Clean(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(tempDir)

	// Save 5 sessions
	for range 5 {
		state := &State{
			ID:        generateID(),
			TaskID:    "test-task",
			AgentName: "claude",
			Phase:     "implementing",
			Status:    StatusCompleted, // Completed can be cleaned
			StartedAt: time.Now(),
		}
		if err := m.Save(state); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Clean, keeping only 2
	removed, err := m.Clean(24*time.Hour, 2)
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}

	if removed != 3 {
		t.Errorf("Clean() removed %d, want 3", removed)
	}

	// Verify only 2 remain
	sessions, _ := m.List()
	if len(sessions) != 2 {
		t.Errorf("After Clean(), %d sessions remain, want 2", len(sessions))
	}
}

func TestManager_LoadLatest(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(tempDir)

	// Save sessions with different statuses
	states := []struct {
		status Status
	}{
		{StatusCompleted},   // Not recoverable
		{StatusInterrupted}, // Recoverable - should be returned
	}

	var interruptedID string
	for _, s := range states {
		state := &State{
			ID:        generateID(),
			TaskID:    "test-task",
			AgentName: "claude",
			Phase:     "implementing",
			Status:    s.status,
			StartedAt: time.Now(),
		}
		if s.status == StatusInterrupted {
			interruptedID = state.ID
		}
		if err := m.Save(state); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Load latest recoverable
	latest, err := m.LoadLatest()
	if err != nil {
		t.Fatalf("LoadLatest() error = %v", err)
	}

	if latest.ID != interruptedID {
		t.Errorf("LoadLatest().ID = %q, want %q", latest.ID, interruptedID)
	}
}

func TestManager_MarkInterrupted(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(tempDir)

	// Save an active session
	state := &State{
		ID:        generateID(),
		TaskID:    "test-task",
		AgentName: "claude",
		Phase:     "implementing",
		Status:    StatusActive,
		StartedAt: time.Now(),
	}
	if err := m.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Mark as interrupted
	err := m.MarkInterrupted(state.ID, "connection timeout")
	if err != nil {
		t.Fatalf("MarkInterrupted() error = %v", err)
	}

	// Load and verify
	loaded, _ := m.Load(state.ID)
	if loaded.Status != StatusInterrupted {
		t.Errorf("Status = %q, want %q", loaded.Status, StatusInterrupted)
	}
	if loaded.Error != "connection timeout" {
		t.Errorf("Error = %q, want %q", loaded.Error, "connection timeout")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == id2 {
		t.Error("generateID() should produce unique IDs")
	}

	if len(id1) < 10 {
		t.Errorf("generateID() = %q, expected longer ID", id1)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"implementing", "implementing"},
		{"Planning Phase", "planning-phase"},
		{"test/file:name", "testfilename"},
		{"--double--hyphens--", "double-hyphens"},
		{"MixedCase123", "mixedcase123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestManager_EmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(tempDir)

	// List on empty directory
	sessions, err := m.List()
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("List() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("List() on empty dir returned %d sessions, want 0", len(sessions))
	}

	// LoadLatest on empty directory
	_, err = m.LoadLatest()
	if err == nil {
		t.Error("LoadLatest() on empty dir should return error")
	}
}
