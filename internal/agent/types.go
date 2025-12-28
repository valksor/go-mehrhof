package agent

import "time"

// EventType identifies the type of streaming event
type EventType string

const (
	EventText       EventType = "text"
	EventToolUse    EventType = "tool_use"
	EventToolResult EventType = "tool_result"
	EventFile       EventType = "file"
	EventError      EventType = "error"
	EventUsage      EventType = "usage"
	EventComplete   EventType = "complete"
)

// Event represents a streaming event from an agent
type Event struct {
	Type      EventType
	Timestamp time.Time
	Data      map[string]any
	Raw       []byte
	ToolCall  *ToolCall // Standardized tool call info (if EventToolUse)
	Text      string    // Extracted text content (if EventText)
}

// Response is the aggregated result from an agent run
type Response struct {
	Files    []FileChange
	Summary  string
	Messages []string
	Usage    *UsageStats
	Duration time.Duration
	Question *Question // Pending question if agent asked one
}

// Question represents a question from the agent to the user
type Question struct {
	Text    string
	Options []QuestionOption
}

// QuestionOption represents an answer option
type QuestionOption struct {
	Label       string
	Description string
}

// ToolCall represents a standardized tool call for display
type ToolCall struct {
	Name        string         // Tool name (Read, Write, Bash, etc.)
	Description string         // Human-readable description
	Input       map[string]any // Tool input parameters
}

// FileChange represents a file modification
type FileChange struct {
	Path      string `yaml:"path"`
	Operation FileOp `yaml:"operation"`
	Content   string `yaml:"content,omitempty"`
}

// FileOp is the type of file operation
type FileOp string

const (
	FileOpCreate FileOp = "create"
	FileOpUpdate FileOp = "update"
	FileOpDelete FileOp = "delete"
)

// UsageStats tracks token usage and cost
type UsageStats struct {
	InputTokens  int     `yaml:"input_tokens"`
	OutputTokens int     `yaml:"output_tokens"`
	CachedTokens int     `yaml:"cached_tokens,omitempty"`
	CostUSD      float64 `yaml:"cost_usd,omitempty"`
}

// Config holds agent configuration
type Config struct {
	Command     []string
	Environment map[string]string
	Args        []string // Additional CLI arguments
	Timeout     time.Duration
	RetryCount  int
	RetryDelay  time.Duration
	WorkDir     string
}

// NewConfig creates a default config
func NewConfig() Config {
	return Config{
		Timeout:    30 * time.Minute,
		RetryCount: 3,
		RetryDelay: time.Second,
	}
}
