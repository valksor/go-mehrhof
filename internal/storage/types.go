package storage

import "time"

// ActiveTask represents the currently active task (stored in .active_task).
type ActiveTask struct {
	ID           string    `yaml:"id"`
	Ref          string    `yaml:"ref"`      // e.g., "dir:.mehrhof/my-feature" or "file:task.md"
	WorkDir      string    `yaml:"work_dir"` // e.g., ".mehrhof/work/abc12345"
	State        string    `yaml:"state"`    // idle, planning, implementing, reviewing, done
	Branch       string    `yaml:"branch,omitempty"`
	UseGit       bool      `yaml:"use_git"`
	WorktreePath string    `yaml:"worktree_path,omitempty"` // path to git worktree if using worktrees
	Started      time.Time `yaml:"started"`
}

// TaskWork represents the work directory structure (.mehrhof/work/<id>/).
type TaskWork struct {
	Version  string       `yaml:"version"`
	Metadata WorkMetadata `yaml:"metadata"`
	Source   SourceInfo   `yaml:"source"`
	Git      GitInfo      `yaml:"git,omitempty"`
	Agent    AgentInfo    `yaml:"agent,omitempty"`
	Costs    CostStats    `yaml:"costs,omitempty"`
}

// WorkMetadata holds task identification.
type WorkMetadata struct {
	ID        string    `yaml:"id"`
	Title     string    `yaml:"title,omitempty"`
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at"`

	// Naming fields for branch/commit customization
	ExternalKey string `yaml:"external_key,omitempty"` // User-facing key (e.g., "FEATURE-123")
	TaskType    string `yaml:"task_type,omitempty"`    // Task type (e.g., "feature", "fix")
	Slug        string `yaml:"slug,omitempty"`         // URL-safe title slug
}

// SourceInfo tracks the original source (read-only reference).
// Hybrid storage: metadata in YAML, actual file content in source/ directory.
type SourceInfo struct {
	Type    string    `yaml:"type"`              // directory, file, github, youtrack
	Ref     string    `yaml:"ref"`               // original reference
	ReadAt  time.Time `yaml:"read_at"`           // when source was read
	Files   []string  `yaml:"files,omitempty"`   // relative paths to source files (e.g., "source/task.md")
	Content string    `yaml:"content,omitempty"` // kept for backwards compat, empty for new tasks
}

// GitInfo holds git-related information.
type GitInfo struct {
	Branch       string    `yaml:"branch,omitempty"`
	BaseBranch   string    `yaml:"base_branch,omitempty"`
	WorktreePath string    `yaml:"worktree_path,omitempty"` // path to worktree if using worktrees
	CreatedAt    time.Time `yaml:"created_at,omitempty"`

	// Resolved naming for commits/branches
	CommitPrefix  string `yaml:"commit_prefix,omitempty"`  // Resolved prefix (e.g., "[FEATURE-123]")
	BranchPattern string `yaml:"branch_pattern,omitempty"` // Template used to generate branch
}

// StepAgentInfo holds per-step agent resolution info.
type StepAgentInfo struct {
	Name      string            `yaml:"name,omitempty"`       // Resolved agent name for this step
	Source    string            `yaml:"source,omitempty"`     // Where specified: "cli-step", "cli", "task-step", "task", "workspace-step", "workspace", "auto"
	InlineEnv map[string]string `yaml:"inline_env,omitempty"` // Resolved inline env vars for this step
	Args      []string          `yaml:"args,omitempty"`       // CLI args for this step
}

// AgentInfo holds the agent configuration used for this task.
type AgentInfo struct {
	Name      string                   `yaml:"name,omitempty"`       // Default resolved agent name
	Source    string                   `yaml:"source,omitempty"`     // Where agent was specified: "cli", "task", "workspace", "auto"
	InlineEnv map[string]string        `yaml:"inline_env,omitempty"` // Original inline env vars from task
	Args      []string                 `yaml:"args,omitempty"`       // CLI args for the task
	Steps     map[string]StepAgentInfo `yaml:"steps,omitempty"`      // Per-step agent resolution
}

// SpecificationStatus constants.
const (
	SpecificationStatusDraft        = "draft"
	SpecificationStatusReady        = "ready"
	SpecificationStatusImplementing = "implementing"
	SpecificationStatusDone         = "done"
)

// Specification represents a specification file (specification-N.md)
// These are stored as markdown files with optional YAML frontmatter.
type Specification struct {
	Number      int       `yaml:"-"`
	Title       string    `yaml:"title,omitempty"`
	Description string    `yaml:"-"` // Parsed from markdown content
	Status      string    `yaml:"status,omitempty"`
	CreatedAt   time.Time `yaml:"created_at,omitempty"`
	UpdatedAt   time.Time `yaml:"updated_at,omitempty"`
	CompletedAt time.Time `yaml:"completed_at,omitempty"`
	Sections    []string  `yaml:"-"` // Parsed from markdown content
	Content     string    `yaml:"-"` // Raw markdown content (without frontmatter)
}

// Note represents a user note added via the note command.
type Note struct {
	Timestamp time.Time `yaml:"timestamp"`
	Content   string    `yaml:"content"`
	State     string    `yaml:"state,omitempty"` // state when note was added
}

// NotesFile represents the notes.md structure.
type NotesFile struct {
	Notes []Note `yaml:"notes"`
}

// Session records an interaction session.
type Session struct {
	Version   string          `yaml:"version"`
	Kind      string          `yaml:"kind"`
	Metadata  SessionMetadata `yaml:"metadata"`
	Usage     *UsageInfo      `yaml:"usage,omitempty"`
	Exchanges []Exchange      `yaml:"exchanges,omitempty"`
}

// SessionMetadata holds session identification.
type SessionMetadata struct {
	StartedAt time.Time `yaml:"started_at"`
	EndedAt   time.Time `yaml:"ended_at,omitempty"`
	Type      string    `yaml:"type"` // planning, implementing, reviewing, checkpointing
	Agent     string    `yaml:"agent"`
	State     string    `yaml:"state,omitempty"` // task state when session started
}

// UsageInfo tracks token/cost usage.
type UsageInfo struct {
	InputTokens  int     `yaml:"input_tokens"`
	OutputTokens int     `yaml:"output_tokens"`
	CachedTokens int     `yaml:"cached_tokens,omitempty"`
	CostUSD      float64 `yaml:"cost_usd,omitempty"`
}

// CostStats tracks cumulative token/cost usage across all workflow steps.
type CostStats struct {
	TotalInputTokens  int                      `yaml:"total_input_tokens"`
	TotalOutputTokens int                      `yaml:"total_output_tokens"`
	TotalCachedTokens int                      `yaml:"total_cached_tokens,omitempty"`
	TotalCostUSD      float64                  `yaml:"total_cost_usd"`
	ByStep            map[string]StepCostStats `yaml:"by_step,omitempty"`
}

// StepCostStats tracks usage for a specific workflow step.
type StepCostStats struct {
	InputTokens  int     `yaml:"input_tokens"`
	OutputTokens int     `yaml:"output_tokens"`
	CachedTokens int     `yaml:"cached_tokens,omitempty"`
	CostUSD      float64 `yaml:"cost_usd"`
	Calls        int     `yaml:"calls"` // Number of agent calls in this step
}

// Exchange represents a single message in a session.
type Exchange struct {
	Role         string       `yaml:"role"` // user, agent, system
	Timestamp    time.Time    `yaml:"timestamp"`
	Content      string       `yaml:"content"`
	FilesChanged []FileChange `yaml:"files_changed,omitempty"`
}

// FileChange records a file modification.
type FileChange struct {
	Path      string `yaml:"path"`
	Operation string `yaml:"operation"` // create, update, delete
}

// Checkpoint records a git checkpoint for undo/redo.
type Checkpoint struct {
	ID        string    `yaml:"id"`
	Commit    string    `yaml:"commit"`
	Message   string    `yaml:"message"`
	State     string    `yaml:"state"` // task state at checkpoint
	CreatedAt time.Time `yaml:"created_at"`
}

// NewActiveTask creates a new active task.
func NewActiveTask(id, ref, workDir string) *ActiveTask {
	return &ActiveTask{
		ID:      id,
		Ref:     ref,
		WorkDir: workDir,
		State:   "idle",
		UseGit:  false,
		Started: time.Now(),
	}
}

// NewTaskWork creates a new task work structure.
func NewTaskWork(id string, source SourceInfo) *TaskWork {
	now := time.Now()
	return &TaskWork{
		Version: "1",
		Metadata: WorkMetadata{
			ID:        id,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Source: source,
	}
}

// NewSession creates a new session with defaults.
func NewSession(sessionType, agent, state string) *Session {
	now := time.Now()
	return &Session{
		Version: "1",
		Kind:    "Session",
		Metadata: SessionMetadata{
			StartedAt: now,
			Type:      sessionType,
			Agent:     agent,
			State:     state,
		},
		Exchanges: make([]Exchange, 0),
	}
}
