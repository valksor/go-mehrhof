package events

import "time"

// Type identifies event categories
type Type string

const (
	TypeStateChanged   Type = "state_changed"
	TypeProgress       Type = "progress"
	TypeError          Type = "error"
	TypeFileChanged    Type = "file_changed"
	TypeAgentMessage   Type = "agent_message"
	TypeCheckpoint     Type = "checkpoint"
	TypeBlueprintReady Type = "blueprint_ready"

	// GitHub-related events
	TypeBranchCreated Type = "branch_created"
	TypePlanCompleted Type = "plan_completed"
	TypeImplementDone Type = "implement_done"
	TypePRCreated     Type = "pr_created"
)

// Event is the base event structure
type Event struct {
	Type      Type
	Timestamp time.Time
	Data      map[string]any
}

// Eventer interface for typed events
type Eventer interface {
	ToEvent() Event
}

// StateChangedEvent when workflow state changes
type StateChangedEvent struct {
	From      string
	To        string
	Event     string // Triggering event
	TaskID    string
	Timestamp time.Time
}

func (e StateChangedEvent) ToEvent() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return Event{
		Type:      TypeStateChanged,
		Timestamp: e.Timestamp,
		Data: map[string]any{
			"from":    e.From,
			"to":      e.To,
			"event":   e.Event,
			"task_id": e.TaskID,
		},
	}
}

// ProgressEvent for progress updates
type ProgressEvent struct {
	TaskID    string
	Phase     string
	Message   string
	Current   int
	Total     int
	Timestamp time.Time
}

func (e ProgressEvent) ToEvent() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return Event{
		Type:      TypeProgress,
		Timestamp: e.Timestamp,
		Data: map[string]any{
			"task_id": e.TaskID,
			"phase":   e.Phase,
			"message": e.Message,
			"current": e.Current,
			"total":   e.Total,
		},
	}
}

// ErrorEvent for errors
type ErrorEvent struct {
	TaskID    string
	Error     error
	Fatal     bool
	Timestamp time.Time
}

func (e ErrorEvent) ToEvent() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	errMsg := ""
	if e.Error != nil {
		errMsg = e.Error.Error()
	}
	return Event{
		Type:      TypeError,
		Timestamp: e.Timestamp,
		Data: map[string]any{
			"task_id": e.TaskID,
			"error":   errMsg,
			"fatal":   e.Fatal,
		},
	}
}

// FileChangedEvent when files are modified
type FileChangedEvent struct {
	TaskID    string
	Path      string
	Operation string // create, update, delete
	Timestamp time.Time
}

func (e FileChangedEvent) ToEvent() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return Event{
		Type:      TypeFileChanged,
		Timestamp: e.Timestamp,
		Data: map[string]any{
			"task_id":   e.TaskID,
			"path":      e.Path,
			"operation": e.Operation,
		},
	}
}

// CheckpointEvent when a checkpoint is created
type CheckpointEvent struct {
	TaskID    string
	Commit    string
	Message   string
	Timestamp time.Time
}

func (e CheckpointEvent) ToEvent() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return Event{
		Type:      TypeCheckpoint,
		Timestamp: e.Timestamp,
		Data: map[string]any{
			"task_id": e.TaskID,
			"commit":  e.Commit,
			"message": e.Message,
		},
	}
}

// AgentMessageEvent for agent output
type AgentMessageEvent struct {
	TaskID    string
	Content   string
	Role      string // assistant, tool, system
	Timestamp time.Time
}

func (e AgentMessageEvent) ToEvent() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return Event{
		Type:      TypeAgentMessage,
		Timestamp: e.Timestamp,
		Data: map[string]any{
			"task_id": e.TaskID,
			"content": e.Content,
			"role":    e.Role,
		},
	}
}

// BlueprintReadyEvent when a blueprint is ready
type BlueprintReadyEvent struct {
	TaskID      string
	BlueprintID string
	Timestamp   time.Time
}

func (e BlueprintReadyEvent) ToEvent() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return Event{
		Type:      TypeBlueprintReady,
		Timestamp: e.Timestamp,
		Data: map[string]any{
			"task_id":      e.TaskID,
			"blueprint_id": e.BlueprintID,
		},
	}
}

// BranchCreatedEvent when a task branch is created
type BranchCreatedEvent struct {
	TaskID    string
	Branch    string
	Timestamp time.Time
}

func (e BranchCreatedEvent) ToEvent() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return Event{
		Type:      TypeBranchCreated,
		Timestamp: e.Timestamp,
		Data: map[string]any{
			"task_id": e.TaskID,
			"branch":  e.Branch,
		},
	}
}

// PlanCompletedEvent when planning phase completes
type PlanCompletedEvent struct {
	TaskID          string
	SpecificationID int
	Timestamp       time.Time
}

func (e PlanCompletedEvent) ToEvent() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return Event{
		Type:      TypePlanCompleted,
		Timestamp: e.Timestamp,
		Data: map[string]any{
			"task_id":          e.TaskID,
			"specification_id": e.SpecificationID,
		},
	}
}

// ImplementDoneEvent when implementation phase completes
type ImplementDoneEvent struct {
	TaskID    string
	DiffStat  string
	Timestamp time.Time
}

func (e ImplementDoneEvent) ToEvent() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return Event{
		Type:      TypeImplementDone,
		Timestamp: e.Timestamp,
		Data: map[string]any{
			"task_id":   e.TaskID,
			"diff_stat": e.DiffStat,
		},
	}
}

// PRCreatedEvent when a pull request is created
type PRCreatedEvent struct {
	TaskID    string
	PRNumber  int
	PRURL     string
	Timestamp time.Time
}

func (e PRCreatedEvent) ToEvent() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return Event{
		Type:      TypePRCreated,
		Timestamp: e.Timestamp,
		Data: map[string]any{
			"task_id":   e.TaskID,
			"pr_number": e.PRNumber,
			"pr_url":    e.PRURL,
		},
	}
}
