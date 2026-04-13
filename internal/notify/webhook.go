package notify

import "time"

// Format specifies the webhook payload format.
type Format string

const (
	// FormatGeneric sends the Payload directly as JSON.
	FormatGeneric Format = "generic"
	// FormatSlack sends a Slack Block Kit formatted payload.
	FormatSlack Format = "slack"
)

// WebhookEndpoint configures a single webhook destination.
type WebhookEndpoint struct {
	URL    string   `yaml:"url"    json:"url"`
	Format Format   `yaml:"format" json:"format"`
	Events []string `yaml:"events" json:"events"`
}

// Payload carries event data for webhook dispatch.
type Payload struct {
	Event         string    `json:"event"`
	Timestamp     time.Time `json:"timestamp"`
	TaskID        string    `json:"task_id"`
	TaskTitle     string    `json:"task_title"`
	State         string    `json:"state"`
	PreviousState string    `json:"previous_state"`
	Error         string    `json:"error,omitempty"`
	Message       string    `json:"message"`
	ProjectPath   string    `json:"project_path"`
}
