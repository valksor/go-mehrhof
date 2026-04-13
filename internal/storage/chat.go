package storage

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// ChatMessage represents a message in the chat history.
// This mirrors internal/socket.ChatMessage but is storage-independent.
type ChatMessage struct {
	ID        string   `json:"id"`
	Role      string   `json:"role"` // "user", "assistant", "system"
	Content   string   `json:"content"`
	Mentions  []string `json:"mentions,omitempty"`  // File paths mentioned with @
	Timestamp string   `json:"timestamp,omitempty"` // RFC3339 format
	JobID     string   `json:"job_id,omitempty"`    // Job ID if this message triggered a job
}

// ChatHistory holds the chat messages for a task.
type ChatHistory struct {
	TaskID    string        `json:"task_id"`
	Messages  []ChatMessage `json:"messages"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// ChatStore manages chat history persistence for tasks.
type ChatStore struct {
	store *Store
	mu    sync.RWMutex
}

// NewChatStore creates a new ChatStore.
func NewChatStore(store *Store) *ChatStore {
	return &ChatStore{store: store}
}

// SaveMessage saves a chat message for a task.
// Creates the chat history file if it doesn't exist.
func (c *ChatStore) SaveMessage(taskID string, msg ChatMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure timestamp
	if msg.Timestamp == "" {
		msg.Timestamp = time.Now().Format(time.RFC3339)
	}

	// Load existing history
	history, err := c.loadHistoryLocked(taskID)
	if err != nil {
		// Create new history if file doesn't exist
		history = &ChatHistory{
			TaskID:   taskID,
			Messages: []ChatMessage{},
		}
	}

	// Append message
	history.Messages = append(history.Messages, msg)
	history.UpdatedAt = time.Now()

	// Save
	return c.saveHistoryLocked(taskID, history)
}

// LoadHistory loads the chat history for a task.
// Returns empty history if no file exists.
func (c *ChatStore) LoadHistory(taskID string) (*ChatHistory, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.loadHistoryLocked(taskID)
}

// loadHistoryLocked loads history without acquiring lock (caller must hold lock).
func (c *ChatStore) loadHistoryLocked(taskID string) (*ChatHistory, error) {
	path := c.store.ChatFile(taskID)

	var history ChatHistory
	if err := LoadJSON(path, &history); err != nil {
		if os.IsNotExist(err) {
			return &ChatHistory{
				TaskID:   taskID,
				Messages: []ChatMessage{},
			}, nil
		}

		return nil, fmt.Errorf("load chat history: %w", err)
	}

	return &history, nil
}

// saveHistoryLocked saves history without acquiring lock (caller must hold lock).
func (c *ChatStore) saveHistoryLocked(taskID string, history *ChatHistory) error {
	path := c.store.ChatFile(taskID)

	// Ensure directory exists
	if err := EnsureDir(c.store.WorkDir(taskID)); err != nil {
		return fmt.Errorf("create work directory: %w", err)
	}

	if err := SaveJSON(path, history); err != nil {
		return fmt.Errorf("save chat history: %w", err)
	}

	return nil
}

// ClearHistory clears the chat history for a task.
// Removes all messages but keeps the file.
func (c *ChatStore) ClearHistory(taskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	history := &ChatHistory{
		TaskID:    taskID,
		Messages:  []ChatMessage{},
		UpdatedAt: time.Now(),
	}

	return c.saveHistoryLocked(taskID, history)
}

// DeleteHistory removes the chat history file for a task.
func (c *ChatStore) DeleteHistory(taskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.store.ChatFile(taskID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove chat history: %w", err)
	}

	return nil
}

// MessageCount returns the number of messages in the chat history.
func (c *ChatStore) MessageCount(taskID string) (int, error) {
	history, err := c.LoadHistory(taskID)
	if err != nil {
		return 0, err
	}

	return len(history.Messages), nil
}

// GetLastMessage returns the last message in the chat history.
// Returns nil if no messages exist.
func (c *ChatStore) GetLastMessage(taskID string) (*ChatMessage, error) {
	history, err := c.LoadHistory(taskID)
	if err != nil {
		return nil, err
	}
	if len(history.Messages) == 0 {
		return nil, nil //nolint:nilnil // Documented behavior: nil means no messages
	}

	return &history.Messages[len(history.Messages)-1], nil
}

// GetMessagesByRole returns all messages with the given role.
func (c *ChatStore) GetMessagesByRole(taskID, role string) ([]ChatMessage, error) {
	history, err := c.LoadHistory(taskID)
	if err != nil {
		return nil, err
	}

	var messages []ChatMessage
	for _, msg := range history.Messages {
		if msg.Role == role {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}
