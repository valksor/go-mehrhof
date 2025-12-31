package provider

import "time"

// WorkUnit represents a task from any provider
type WorkUnit struct {
	ID          string
	ExternalID  string // Provider-specific ID
	Provider    string // Provider name
	Title       string
	Description string
	Status      Status
	Priority    Priority
	Labels      []string
	Assignees   []Person
	Comments    []Comment
	Attachments []Attachment
	Subtasks    []string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Source      SourceInfo

	// Naming fields for branch/commit customization
	ExternalKey string // User-facing key (e.g., "FEATURE-123") for branches/commits
	TaskType    string // Task type (e.g., "feature", "fix", "task")
	Slug        string // URL-safe title slug for branch names

	// Agent configuration from task source
	AgentConfig *AgentConfig // Per-task agent configuration (optional)
}

// SourceInfo tracks where the work unit came from
type SourceInfo struct {
	Type      string    // Provider type
	Reference string    // Original reference
	SyncedAt  time.Time // Last sync time
}

// StepAgentConfig holds agent configuration for a specific workflow step
type StepAgentConfig struct {
	Name string            // Agent name or alias
	Env  map[string]string // Step-specific env vars
	Args []string          // Step-specific CLI args
}

// AgentConfig holds per-task agent configuration from the task source
type AgentConfig struct {
	Name  string                     // Agent name or alias (e.g., "glm", "claude")
	Env   map[string]string          // Inline environment variables
	Args  []string                   // CLI arguments
	Steps map[string]StepAgentConfig // Per-step agent overrides
}

// Status represents work unit status
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusReview     Status = "review"
	StatusDone       Status = "done"
	StatusClosed     Status = "closed"
)

// Priority represents work unit priority
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// String returns priority as string
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "normal"
	}
}

// Person represents a user/assignee
type Person struct {
	ID    string
	Name  string
	Email string
}

// Comment represents a comment on a work unit
type Comment struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	Author    Person
	ID        string
	Body      string
}

// Attachment represents a file attachment
type Attachment struct {
	CreatedAt   time.Time
	ID          string
	Name        string
	URL         string
	ContentType string
	Size        int64
}

// Capability identifies provider capabilities
type Capability string

const (
	CapRead               Capability = "read"
	CapList               Capability = "list"
	CapDownloadAttachment Capability = "download_attachment"
	CapFetchComments      Capability = "fetch_comments"
	CapComment            Capability = "comment"
	CapUpdateStatus       Capability = "update_status"
	CapManageLabels       Capability = "manage_labels"
	CapSnapshot           Capability = "snapshot"
	CapCreatePR           Capability = "create_pr"
	CapLinkBranch         Capability = "link_branch"
	CapCreateWorkUnit     Capability = "create_work_unit"
	CapFetchSubtasks      Capability = "fetch_subtasks"
)

// CapabilitySet is a set of capabilities
type CapabilitySet map[Capability]bool

// Has checks if capability is present
func (cs CapabilitySet) Has(cap Capability) bool {
	return cs[cap]
}

// InferCapabilities uses type assertions to determine capabilities
func InferCapabilities(p any) CapabilitySet {
	caps := make(CapabilitySet)

	if _, ok := p.(Reader); ok {
		caps[CapRead] = true
	}
	if _, ok := p.(Lister); ok {
		caps[CapList] = true
	}
	if _, ok := p.(AttachmentDownloader); ok {
		caps[CapDownloadAttachment] = true
	}
	if _, ok := p.(CommentFetcher); ok {
		caps[CapFetchComments] = true
	}
	if _, ok := p.(Commenter); ok {
		caps[CapComment] = true
	}
	if _, ok := p.(StatusUpdater); ok {
		caps[CapUpdateStatus] = true
	}
	if _, ok := p.(LabelManager); ok {
		caps[CapManageLabels] = true
	}
	if _, ok := p.(Snapshotter); ok {
		caps[CapSnapshot] = true
	}
	if _, ok := p.(PRCreator); ok {
		caps[CapCreatePR] = true
	}
	if _, ok := p.(BranchLinker); ok {
		caps[CapLinkBranch] = true
	}
	if _, ok := p.(WorkUnitCreator); ok {
		caps[CapCreateWorkUnit] = true
	}
	if _, ok := p.(SubtaskFetcher); ok {
		caps[CapFetchSubtasks] = true
	}

	return caps
}

// Config holds provider configuration
type Config struct {
	options map[string]any
}

// NewConfig creates a new config
func NewConfig() Config {
	return Config{options: make(map[string]any)}
}

// Set sets an option
func (c Config) Set(key string, value any) Config {
	c.options[key] = value
	return c
}

// Get gets an option
func (c Config) Get(key string) any {
	return c.options[key]
}

// GetString gets a string option
func (c Config) GetString(key string) string {
	if v, ok := c.options[key].(string); ok {
		return v
	}
	return ""
}

// GetBool gets a bool option
func (c Config) GetBool(key string) bool {
	if v, ok := c.options[key].(bool); ok {
		return v
	}
	return false
}
