package screenshot

import "time"

// Screenshot represents a captured screenshot with metadata.
type Screenshot struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Path      string    `json:"path"`
	Filename  string    `json:"filename"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"` // "agent" or "user"
	Step      string    `json:"step,omitempty"`
	Agent     string    `json:"agent,omitempty"`
	Format    string    `json:"format"` // "png" or "jpeg"
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	SizeBytes int64     `json:"size_bytes"`
}

// Source constants for screenshot origin.
const (
	SourceAgent = "agent"
	SourceUser  = "user"
)

// Format constants for image formats.
const (
	FormatPNG  = "png"
	FormatJPEG = "jpeg"
)

// SaveOptions configures how a screenshot is saved.
type SaveOptions struct {
	Source string // "agent" or "user"
	Step   string // workflow step (optional)
	Agent  string // agent name if agent-captured (optional)
	Format string // "png" or "jpeg", defaults to "png"
}
