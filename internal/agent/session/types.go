package session

import (
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

// Status represents the session state
type Status string

const (
	StatusActive      Status = "active"
	StatusCompleted   Status = "completed"
	StatusInterrupted Status = "interrupted"
	StatusFailed      Status = "failed"
	StatusRecoverable Status = "recoverable"
)

// State represents a checkpointed session state
type State struct {
	// Version for format compatibility
	Version string `yaml:"version"`

	// Identifiers
	ID     string `yaml:"id"`
	TaskID string `yaml:"task_id"`

	// Agent info
	AgentName string `yaml:"agent"`
	Model     string `yaml:"model,omitempty"`

	// Timing
	StartedAt      time.Time `yaml:"started_at"`
	CheckpointedAt time.Time `yaml:"checkpointed_at"`
	LastActivityAt time.Time `yaml:"last_activity_at,omitempty"`

	// Workflow state
	Phase  string `yaml:"phase"` // planning, implementing, reviewing, etc.
	Status Status `yaml:"status"`

	// Conversation
	Messages []Message `yaml:"messages"`

	// Context
	Context SessionContext `yaml:"context"`

	// Token usage
	Usage *agent.UsageStats `yaml:"usage,omitempty"`

	// Error information (if interrupted/failed)
	Error string `yaml:"error,omitempty"`
}

// Message represents a conversation message
type Message struct {
	Role      string    `yaml:"role"` // user, assistant, system
	Content   string    `yaml:"content"`
	Timestamp time.Time `yaml:"timestamp"`

	// File changes made in this message (for assistant role)
	FilesModified []FileModification `yaml:"files_modified,omitempty"`

	// Tool calls made in this message
	ToolCalls []ToolCallRecord `yaml:"tool_calls,omitempty"`
}

// FileModification records a file change
type FileModification struct {
	Path      string `yaml:"path"`
	Operation string `yaml:"operation"` // create, update, delete
	Checksum  string `yaml:"checksum,omitempty"`
}

// ToolCallRecord records a tool invocation
type ToolCallRecord struct {
	Name   string         `yaml:"name"`
	Input  map[string]any `yaml:"input,omitempty"`
	Output string         `yaml:"output,omitempty"`
	Error  string         `yaml:"error,omitempty"`
}

// SessionContext holds context information for the session
type SessionContext struct {
	// Task-related files
	Specifications []string `yaml:"specifications,omitempty"`
	Reviews        []string `yaml:"reviews,omitempty"`

	// Files the agent has read
	FilesRead []string `yaml:"files_read,omitempty"`

	// Working directory
	WorkDir string `yaml:"work_dir,omitempty"`

	// Git state at checkpoint
	GitBranch string `yaml:"git_branch,omitempty"`
	GitCommit string `yaml:"git_commit,omitempty"`

	// Additional metadata
	Metadata map[string]any `yaml:"metadata,omitempty"`
}

// Summary returns a brief description of the session
type Summary struct {
	ID             string    `yaml:"id"`
	TaskID         string    `yaml:"task_id"`
	AgentName      string    `yaml:"agent"`
	Phase          string    `yaml:"phase"`
	Status         Status    `yaml:"status"`
	StartedAt      time.Time `yaml:"started_at"`
	CheckpointedAt time.Time `yaml:"checkpointed_at"`
	MessageCount   int       `yaml:"message_count"`
	Error          string    `yaml:"error,omitempty"`
}

// ToSummary converts State to Summary
func (s *State) ToSummary() Summary {
	return Summary{
		ID:             s.ID,
		TaskID:         s.TaskID,
		AgentName:      s.AgentName,
		Phase:          s.Phase,
		Status:         s.Status,
		StartedAt:      s.StartedAt,
		CheckpointedAt: s.CheckpointedAt,
		MessageCount:   len(s.Messages),
		Error:          s.Error,
	}
}

// IsRecoverable checks if the session can be recovered
func (s *State) IsRecoverable() bool {
	switch s.Status {
	case StatusInterrupted, StatusRecoverable:
		return true
	case StatusFailed:
		// Failed sessions can be recovered if they have messages
		return len(s.Messages) > 0
	}
	return false
}

// Age returns how long since the checkpoint was made
func (s *State) Age() time.Duration {
	return time.Since(s.CheckpointedAt)
}

// GetLastUserMessage returns the last user message
func (s *State) GetLastUserMessage() *Message {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == "user" {
			return &s.Messages[i]
		}
	}
	return nil
}

// GetLastAssistantMessage returns the last assistant message
func (s *State) GetLastAssistantMessage() *Message {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == "assistant" {
			return &s.Messages[i]
		}
	}
	return nil
}
