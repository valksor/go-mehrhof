package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Session methods

// SessionsDir returns the sessions directory path.
func (w *Workspace) SessionsDir(taskID string) string {
	return filepath.Join(w.WorkPath(taskID), sessionsDirName)
}

// SessionPath returns the path for a session file.
func (w *Workspace) SessionPath(taskID, filename string) string {
	return filepath.Join(w.SessionsDir(taskID), filename)
}

// CreateSession creates a new session.
func (w *Workspace) CreateSession(taskID, sessionType, agent, state string) (*Session, string, error) {
	session := NewSession(sessionType, agent, state)

	// Generate filename from timestamp
	filename := session.Metadata.StartedAt.Format("2006-01-02T15-04-05") + "-" + sessionType + ".yaml"
	sessionFile := w.SessionPath(taskID, filename)

	data, err := yaml.Marshal(session)
	if err != nil {
		return nil, "", fmt.Errorf("marshal session: %w", err)
	}

	if err := os.WriteFile(sessionFile, data, 0o644); err != nil {
		return nil, "", fmt.Errorf("write session file: %w", err)
	}

	return session, filename, nil
}

// LoadSession loads a session by filename.
func (w *Workspace) LoadSession(taskID, filename string) (*Session, error) {
	sessionFile := w.SessionPath(taskID, filename)

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var session Session
	if err := yaml.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session file: %w", err)
	}

	return &session, nil
}

// SaveSession saves a session.
func (w *Workspace) SaveSession(taskID, filename string, session *Session) error {
	sessionFile := w.SessionPath(taskID, filename)

	data, err := yaml.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	return os.WriteFile(sessionFile, data, 0o644)
}

// ListSessions returns all sessions for a task.
func (w *Workspace) ListSessions(taskID string) ([]*Session, error) {
	sessDir := w.SessionsDir(taskID)
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		session, err := w.LoadSession(taskID, entry.Name())
		if err != nil {
			continue // Skip invalid sessions
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetSourceContent returns combined source content for prompts
// Reads from actual files in source/ directory (hybrid storage).
func (w *Workspace) GetSourceContent(taskID string) (string, error) {
	work, err := w.LoadWork(taskID)
	if err != nil {
		return "", err
	}

	workPath := w.WorkPath(taskID)
	var parts []string

	// Read from source files (new hybrid storage)
	for _, filePath := range work.Source.Files {
		fullPath := filepath.Join(workPath, filePath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			// Log but continue - file might be missing
			continue
		}
		// Extract filename for heading
		filename := filepath.Base(filePath)
		parts = append(parts, fmt.Sprintf("### %s\n\n%s", filename, string(content)))
	}

	// Fallback: read from embedded content (backwards compatibility)
	if len(parts) == 0 && work.Source.Content != "" {
		parts = append(parts, work.Source.Content)
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

// PendingQuestion represents a question from the agent awaiting user response.
type PendingQuestion struct {
	Question string           `yaml:"question"`
	Options  []QuestionOption `yaml:"options,omitempty"`
	Phase    string           `yaml:"phase"`
	AskedAt  time.Time        `yaml:"asked_at"`
	// Context preservation fields - save agent's exploration context when exiting with a question
	ContextSummary string   `yaml:"context_summary,omitempty"` // Brief summary for prompt inclusion
	FullContext    string   `yaml:"full_context,omitempty"`    // Complete agent output for --full-context flag
	ExploredFiles  []string `yaml:"explored_files,omitempty"`  // Files referenced during exploration
}

// QuestionOption represents an answer option.
type QuestionOption struct {
	Label       string `yaml:"label"`
	Description string `yaml:"description,omitempty"`
}

const pendingQuestionFile = "pending_question.yaml"

// PendingQuestionPath returns the path to pending question file.
func (w *Workspace) PendingQuestionPath(taskID string) string {
	return filepath.Join(w.WorkPath(taskID), pendingQuestionFile)
}

// HasPendingQuestion checks if there's a pending question.
func (w *Workspace) HasPendingQuestion(taskID string) bool {
	_, err := os.Stat(w.PendingQuestionPath(taskID))

	return err == nil
}

// SavePendingQuestion saves a pending question.
func (w *Workspace) SavePendingQuestion(taskID string, q *PendingQuestion) error {
	data, err := yaml.Marshal(q)
	if err != nil {
		return fmt.Errorf("marshal question: %w", err)
	}

	return os.WriteFile(w.PendingQuestionPath(taskID), data, 0o644)
}

// LoadPendingQuestion loads a pending question.
func (w *Workspace) LoadPendingQuestion(taskID string) (*PendingQuestion, error) {
	data, err := os.ReadFile(w.PendingQuestionPath(taskID))
	if err != nil {
		return nil, err
	}
	var q PendingQuestion
	if err := yaml.Unmarshal(data, &q); err != nil {
		return nil, fmt.Errorf("parse question: %w", err)
	}

	return &q, nil
}

// ClearPendingQuestion removes the pending question file.
func (w *Workspace) ClearPendingQuestion(taskID string) error {
	err := os.Remove(w.PendingQuestionPath(taskID))
	if os.IsNotExist(err) {
		return nil
	}

	return err
}
