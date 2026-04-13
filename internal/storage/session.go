package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionEntry represents a saved agent session for resume capability.
type SessionEntry struct {
	SessionID string    `json:"session_id"`
	AgentType string    `json:"agent_type"` // "claude", "codex", etc.
	TaskID    string    `json:"task_id"`
	Phase     string    `json:"phase"` // "planning", "implementing", "reviewing"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Sessions holds all session entries for a project.
type Sessions struct {
	Entries   map[string]SessionEntry `json:"entries"` // Keyed by "taskID:phase"
	UpdatedAt time.Time               `json:"updated_at"`
}

// SessionStore manages session persistence for resume functionality.
type SessionStore struct {
	store *Store
	mu    sync.RWMutex
}

// NewSessionStore creates a new SessionStore.
func NewSessionStore(store *Store) *SessionStore {
	return &SessionStore{store: store}
}

// sessionKey creates a unique key for a task/phase combination.
func sessionKey(taskID, phase string) string {
	return taskID + ":" + phase
}

// SaveSession saves or updates a session entry.
func (s *SessionStore) SaveSession(entry SessionEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions, err := s.loadSessionsLocked()
	if err != nil {
		sessions = &Sessions{
			Entries: make(map[string]SessionEntry),
		}
	}

	now := time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	key := sessionKey(entry.TaskID, entry.Phase)
	sessions.Entries[key] = entry
	sessions.UpdatedAt = now

	return s.saveSessionsLocked(sessions)
}

// GetSession retrieves a session for a task and phase.
// Returns nil if no session exists.
func (s *SessionStore) GetSession(taskID, phase string) (*SessionEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions, err := s.loadSessionsLocked()
	if err != nil {
		return nil, nil //nolint:nilnil // No sessions file yet, not an error
	}

	key := sessionKey(taskID, phase)
	if entry, ok := sessions.Entries[key]; ok {
		return &entry, nil
	}

	return nil, nil //nolint:nilnil // Documented behavior: nil means session not found
}

// GetSessionByID retrieves a session by its session ID.
func (s *SessionStore) GetSessionByID(sessionID string) (*SessionEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions, err := s.loadSessionsLocked()
	if err != nil {
		return nil, nil //nolint:nilnil // No sessions file yet, not an error
	}

	for _, entry := range sessions.Entries {
		if entry.SessionID == sessionID {
			return &entry, nil
		}
	}

	return nil, nil //nolint:nilnil // Documented behavior: nil means session not found
}

// DeleteSession removes a session entry.
func (s *SessionStore) DeleteSession(taskID, phase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions, err := s.loadSessionsLocked()
	if err != nil {
		return nil // No sessions file
	}

	key := sessionKey(taskID, phase)
	delete(sessions.Entries, key)
	sessions.UpdatedAt = time.Now()

	return s.saveSessionsLocked(sessions)
}

// DeleteSessionsForTask removes all sessions for a task.
func (s *SessionStore) DeleteSessionsForTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions, err := s.loadSessionsLocked()
	if err != nil {
		return nil
	}

	for key, entry := range sessions.Entries {
		if entry.TaskID == taskID {
			delete(sessions.Entries, key)
		}
	}
	sessions.UpdatedAt = time.Now()

	return s.saveSessionsLocked(sessions)
}

// ListSessions returns all sessions.
func (s *SessionStore) ListSessions() ([]SessionEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions, err := s.loadSessionsLocked()
	if err != nil {
		return []SessionEntry{}, nil
	}

	entries := make([]SessionEntry, 0, len(sessions.Entries))
	for _, entry := range sessions.Entries {
		entries = append(entries, entry)
	}

	return entries, nil
}

// ListSessionsForTask returns all sessions for a specific task.
func (s *SessionStore) ListSessionsForTask(taskID string) ([]SessionEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions, err := s.loadSessionsLocked()
	if err != nil {
		return []SessionEntry{}, nil
	}

	var entries []SessionEntry
	for _, entry := range sessions.Entries {
		if entry.TaskID == taskID {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// HasSession checks if a session exists for a task and phase.
func (s *SessionStore) HasSession(taskID, phase string) (bool, error) {
	session, err := s.GetSession(taskID, phase)
	if err != nil {
		return false, err
	}

	return session != nil, nil
}

// UpdateSessionTimestamp updates the UpdatedAt timestamp for a session.
func (s *SessionStore) UpdateSessionTimestamp(taskID, phase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions, err := s.loadSessionsLocked()
	if err != nil {
		return fmt.Errorf("load sessions: %w", err)
	}

	key := sessionKey(taskID, phase)
	if entry, ok := sessions.Entries[key]; ok {
		entry.UpdatedAt = time.Now()
		sessions.Entries[key] = entry
		sessions.UpdatedAt = time.Now()

		return s.saveSessionsLocked(sessions)
	}

	return fmt.Errorf("session not found for %s", key)
}

// loadSessionsLocked loads sessions from file (caller must hold lock).
func (s *SessionStore) loadSessionsLocked() (*Sessions, error) {
	path := s.store.SessionsFile()

	var sessions Sessions
	if err := LoadJSON(path, &sessions); err != nil {
		if os.IsNotExist(err) {
			return &Sessions{
				Entries: make(map[string]SessionEntry),
			}, nil
		}

		return nil, fmt.Errorf("load sessions: %w", err)
	}

	if sessions.Entries == nil {
		sessions.Entries = make(map[string]SessionEntry)
	}

	return &sessions, nil
}

// saveSessionsLocked saves sessions to file (caller must hold lock).
func (s *SessionStore) saveSessionsLocked(sessions *Sessions) error {
	path := s.store.SessionsFile()

	// Ensure directory exists (sessions live in a subdirectory of projectRoot)
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("create sessions directory: %w", err)
	}

	if err := SaveJSON(path, sessions); err != nil {
		return fmt.Errorf("save sessions: %w", err)
	}

	return nil
}

// CleanOldSessions removes sessions older than the specified duration.
func (s *SessionStore) CleanOldSessions(maxAge time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions, err := s.loadSessionsLocked()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for key, entry := range sessions.Entries {
		if entry.UpdatedAt.Before(cutoff) {
			delete(sessions.Entries, key)
			removed++
		}
	}

	if removed > 0 {
		sessions.UpdatedAt = time.Now()
		if err := s.saveSessionsLocked(sessions); err != nil {
			return 0, err
		}
	}

	return removed, nil
}
