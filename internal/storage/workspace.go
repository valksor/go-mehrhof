package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	taskDirName     = ".mehrhof"
	workSubDirName  = "work"
	plannedDirName  = "planned"
	activeTaskFile  = ".active_task"
	workFileName    = "work.yaml"
	notesFileName   = "notes.md"
	specsDirName    = "specifications"
	sessionsDirName = "sessions"
	configFileName  = "config.yaml"
	envFileName     = ".env"
)

// Workspace manages task storage within a repository
type Workspace struct {
	root     string // Repository root
	taskRoot string // .mehrhof directory
	workRoot string // .mehrhof/work directory
}

// OpenWorkspace opens or creates a workspace in the given directory
func OpenWorkspace(repoRoot string) (*Workspace, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	taskRoot := filepath.Join(absRoot, taskDirName)
	workRoot := filepath.Join(taskRoot, workSubDirName)

	return &Workspace{
		root:     absRoot,
		taskRoot: taskRoot,
		workRoot: workRoot,
	}, nil
}

// Root returns the repository root path
func (w *Workspace) Root() string {
	return w.root
}

// TaskRoot returns the .mehrhof directory path
func (w *Workspace) TaskRoot() string {
	return w.taskRoot
}

// WorkRoot returns the .mehrhof/work directory path
func (w *Workspace) WorkRoot() string {
	return w.workRoot
}

// ConfigPath returns the path to the config file
func (w *Workspace) ConfigPath() string {
	return filepath.Join(w.taskRoot, configFileName)
}

// HasConfig returns true if the config file exists
func (w *Workspace) HasConfig() bool {
	_, err := os.Stat(w.ConfigPath())
	return err == nil
}

// EnvPath returns the path to the .env file
func (w *Workspace) EnvPath() string {
	return filepath.Join(w.taskRoot, envFileName)
}

// LoadEnv reads the .env file and returns key-value pairs.
// Returns empty map if file doesn't exist.
func (w *Workspace) LoadEnv() (map[string]string, error) {
	data, err := os.ReadFile(w.EnvPath())
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("read .env: %w", err)
	}

	result := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result, nil
}

// WorkspaceConfig holds workspace-specific configuration that users can customize
type WorkspaceConfig struct {
	Git       GitSettings                 `yaml:"git"`
	Agent     AgentSettings               `yaml:"agent"`
	Workflow  WorkflowSettings            `yaml:"workflow"`
	Providers ProvidersSettings           `yaml:"providers,omitempty"`
	Env       map[string]string           `yaml:"env,omitempty"`
	Agents    map[string]AgentAliasConfig `yaml:"agents,omitempty"`
	GitHub    *GitHubSettings             `yaml:"github,omitempty"`
	GitLab    *GitLabSettings             `yaml:"gitlab,omitempty"`
	Notion    *NotionSettings             `yaml:"notion,omitempty"`
	Jira      *JiraSettings               `yaml:"jira,omitempty"`
	Linear    *LinearSettings             `yaml:"linear,omitempty"`
	Wrike     *WrikeSettings              `yaml:"wrike,omitempty"`
	YouTrack  *YouTrackSettings           `yaml:"youtrack,omitempty"`
	Plugins   PluginsConfig               `yaml:"plugins,omitempty"`
	Update    UpdateSettings              `yaml:"update,omitempty"`
}

// PluginsConfig holds plugin-related configuration
type PluginsConfig struct {
	// Enabled lists the plugin names that should be loaded
	// Only plugins in this list will be activated
	Enabled []string `yaml:"enabled,omitempty"`

	// Config holds plugin-specific configuration keyed by plugin name
	// Each plugin receives its configuration during initialization
	Config map[string]map[string]any `yaml:"config,omitempty"`
}

// GitHubSettings holds GitHub provider configuration
type GitHubSettings struct {
	Token         string                  `yaml:"token,omitempty"`          // GitHub token (env vars take priority)
	Owner         string                  `yaml:"owner,omitempty"`          // Repository owner (auto-detected from git remote)
	Repo          string                  `yaml:"repo,omitempty"`           // Repository name
	BranchPattern string                  `yaml:"branch_pattern,omitempty"` // Default: "issue/{key}-{slug}"
	CommitPrefix  string                  `yaml:"commit_prefix,omitempty"`  // Default: "[#{key}]"
	TargetBranch  string                  `yaml:"target_branch,omitempty"`  // Default: detected from repo
	DraftPR       bool                    `yaml:"draft_pr,omitempty"`       // Create PRs as draft
	Comments      *GitHubCommentsSettings `yaml:"comments,omitempty"`
}

// GitHubCommentsSettings controls automated GitHub issue commenting
type GitHubCommentsSettings struct {
	Enabled         bool `yaml:"enabled"`           // Master switch (default: false)
	OnBranchCreated bool `yaml:"on_branch_created"` // Post when branch is created
	OnPlanDone      bool `yaml:"on_plan_done"`      // Post summary of planned implementation
	OnImplementDone bool `yaml:"on_implement_done"` // Post changelog with files changed
	OnPRCreated     bool `yaml:"on_pr_created"`     // Post PR link
}

// WrikeSettings holds Wrike provider configuration
type WrikeSettings struct {
	Token  string `yaml:"token,omitempty"`  // Wrike API token (env vars take priority)
	Host   string `yaml:"host,omitempty"`   // API base URL override (default: https://www.wrike.com/api/v4)
	Folder string `yaml:"folder,omitempty"` // Default folder ID for task lookup
}

// GitLabSettings holds GitLab provider configuration
type GitLabSettings struct {
	Token         string `yaml:"token,omitempty"`          // GitLab token (env vars take priority)
	Host          string `yaml:"host,omitempty"`           // GitLab host (default: https://gitlab.com)
	ProjectPath   string `yaml:"project_path,omitempty"`   // Default project path (e.g., group/project)
	BranchPattern string `yaml:"branch_pattern,omitempty"` // Default: "issue/{key}-{slug}"
	CommitPrefix  string `yaml:"commit_prefix,omitempty"`  // Default: "[#{key}]"
}

// NotionSettings holds Notion provider configuration
type NotionSettings struct {
	Token               string `yaml:"token,omitempty"`                // Notion token (env vars take priority)
	DatabaseID          string `yaml:"database_id,omitempty"`          // Default database ID
	StatusProperty      string `yaml:"status_property,omitempty"`      // Property name for status (default: Status)
	DescriptionProperty string `yaml:"description_property,omitempty"` // Property name for description
	LabelsProperty      string `yaml:"labels_property,omitempty"`      // Property name for labels (default: Tags)
}

// JiraSettings holds Jira provider configuration
type JiraSettings struct {
	Token   string `yaml:"token,omitempty"`    // Jira API token (env vars take priority)
	Email   string `yaml:"email,omitempty"`    // Email for Cloud auth
	BaseURL string `yaml:"base_url,omitempty"` // Base URL (optional, auto-detected)
	Project string `yaml:"project,omitempty"`  // Default project key
}

// LinearSettings holds Linear provider configuration
type LinearSettings struct {
	Token string `yaml:"token,omitempty"` // Linear API key (env vars take priority)
	Team  string `yaml:"team,omitempty"`  // Default team key
}

// YouTrackSettings holds YouTrack provider configuration
type YouTrackSettings struct {
	Token string `yaml:"token,omitempty"` // YouTrack token (env vars take priority)
	Host  string `yaml:"host,omitempty"`  // YouTrack host
}

// AgentAliasConfig defines a user-defined agent alias that wraps an existing agent
// with custom environment variables and CLI arguments
type AgentAliasConfig struct {
	Extends     string            `yaml:"extends"`               // Base agent name to wrap
	Description string            `yaml:"description,omitempty"` // Human-readable description
	Env         map[string]string `yaml:"env,omitempty"`         // Environment variables to pass
	Args        []string          `yaml:"args,omitempty"`        // CLI arguments to pass
}

// GitSettings holds git-related configuration
type GitSettings struct {
	CommitPrefix  string `yaml:"commit_prefix"`
	BranchPattern string `yaml:"branch_pattern"`
	AutoCommit    bool   `yaml:"auto_commit"`
	SignCommits   bool   `yaml:"sign_commits"`
}

// StepAgentConfig holds agent configuration for a specific workflow step
type StepAgentConfig struct {
	Name string            `yaml:"name,omitempty"` // Agent name or alias
	Env  map[string]string `yaml:"env,omitempty"`  // Step-specific env vars
	Args []string          `yaml:"args,omitempty"` // Step-specific CLI args
}

// AgentSettings holds agent-related configuration
type AgentSettings struct {
	Default    string                     `yaml:"default"`
	Timeout    int                        `yaml:"timeout"`
	MaxRetries int                        `yaml:"max_retries"`
	Steps      map[string]StepAgentConfig `yaml:"steps,omitempty"` // Per-step agent configuration
}

// WorkflowSettings holds workflow-related configuration
type WorkflowSettings struct {
	AutoInit             bool `yaml:"auto_init"`
	SessionRetentionDays int  `yaml:"session_retention_days"`
}

// UpdateSettings holds update-related configuration
type UpdateSettings struct {
	Enabled       bool `yaml:"enabled"`        // Enable automatic update checks
	CheckInterval int  `yaml:"check_interval"` // Hours between checks (default: 24)
}

// ProvidersSettings holds provider-related configuration
type ProvidersSettings struct {
	Default string `yaml:"default,omitempty"` // Default provider for bare references (e.g., "file", "directory", "github")
}

// NewDefaultWorkspaceConfig creates a WorkspaceConfig with default values
func NewDefaultWorkspaceConfig() *WorkspaceConfig {
	return &WorkspaceConfig{
		Git: GitSettings{
			AutoCommit:    true,
			CommitPrefix:  "[{key}]",
			BranchPattern: "{type}/{key}--{slug}",
			SignCommits:   false,
		},
		Agent: AgentSettings{
			Default:    "claude",
			Timeout:    300,
			MaxRetries: 3,
		},
		Workflow: WorkflowSettings{
			AutoInit:             true,
			SessionRetentionDays: 30,
		},
		Providers: ProvidersSettings{
			Default: "file",
		},
		Update: UpdateSettings{
			Enabled:       true,
			CheckInterval: 24,
		},
		Env: make(map[string]string),
	}
}

// GetEnvForAgent returns env vars for a specific agent, stripping the prefix.
// E.g., for agent "claude": CLAUDE_FOO=bar → FOO=bar
func (cfg *WorkspaceConfig) GetEnvForAgent(agentName string) map[string]string {
	prefix := strings.ToUpper(agentName) + "_"
	result := make(map[string]string)
	for k, v := range cfg.Env {
		if strings.HasPrefix(k, prefix) {
			stripped := strings.TrimPrefix(k, prefix)
			result[stripped] = v
		}
	}
	return result
}

// SaveConfig saves the workspace configuration to .mehrhof/config.yaml
func (w *Workspace) SaveConfig(cfg *WorkspaceConfig) error {
	// Ensure .mehrhof directory exists
	if err := os.MkdirAll(w.taskRoot, 0o755); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Add header comment
	header := `# Task workspace configuration
# Edit this file to customize task behavior
# Run 'task init' to regenerate with defaults

`
	// Add env section comment if env is empty (to show users how to use it)
	content := header + string(data)
	if len(cfg.Env) == 0 {
		content += `
# Environment variables passed to agents (filtered by agent name prefix)
# Prefix is stripped when passed: CLAUDE_FOO=bar -> FOO=bar
# Example:
# env:
#     CLAUDE_ANTHROPIC_API_KEY: your-key # passed to claude as ANTHROPIC_API_KEY
`
	}

	// Add providers section comment if providers.default is empty
	if cfg.Providers.Default == "" {
		content += `
# Provider settings
# Set a default provider for bare task references (without scheme prefix)
# Example:
# providers:
#     default: file    # "task.md" becomes "file:task.md"
`
	}

	// Add agents section comment if agents is empty
	if len(cfg.Agents) == 0 {
		content += `
# User-defined agent aliases
# Aliases wrap existing agents with custom environment variables and CLI arguments
# Use 'mehr agents list' to see all available agents
# Example:
# agents:
#     opus:
#         extends: claude                       # base agent to wrap
#         description: "Claude Opus model"      # shown in 'mehr agents list'
#         args: ["--model", "claude-opus-4-20250514"]  # CLI flags to pass
#     claude-fast:
#         extends: claude
#         description: "Claude with limited turns"
#         args: ["--max-turns", "3"]
#     glm:
#         extends: claude
#         description: "Claude with GLM key"
#         env:
#             ANTHROPIC_API_KEY: "${GLM_API_KEY}" # ${VAR} references system env
`
	}

	// Add plugins section comment if plugins is empty
	if len(cfg.Plugins.Enabled) == 0 {
		content += `
# Plugin configuration
# Plugins must be explicitly enabled to be loaded
# Use 'mehr plugins list' to see all discovered plugins
# Example:
# plugins:
#     enabled:
#         - jira                           # Enable the jira plugin
#         - youtrack                       # Enable the youtrack plugin
#     config:                              # Plugin-specific configuration
#         jira:
#             url: "https://company.atlassian.net"
#             project: "PROJ"
#         youtrack:
#             url: "https://youtrack.company.com"
`
	}

	if err := os.WriteFile(w.ConfigPath(), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// LoadConfig loads the workspace configuration from .mehrhof/config.yaml
func (w *Workspace) LoadConfig() (*WorkspaceConfig, error) {
	data, err := os.ReadFile(w.ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			// Return defaults if config doesn't exist
			return NewDefaultWorkspaceConfig(), nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := NewDefaultWorkspaceConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return cfg, nil
}

// EnsureInitialized ensures that the workspace directory exists.
// Note: Does NOT update .gitignore - use UpdateGitignore() for that (typically only in init command).
func (w *Workspace) EnsureInitialized() error {
	// Create .mehrhof/work directory (also creates .mehrhof/)
	if err := os.MkdirAll(w.workRoot, 0o755); err != nil {
		return fmt.Errorf("create workspace directories: %w", err)
	}

	return nil
}

// UpdateGitignore adds standard mehrhof entries to .gitignore.
// This should only be called from the init command.
func (w *Workspace) UpdateGitignore() error {
	gitignorePath := filepath.Join(w.root, ".gitignore")

	// Read existing .gitignore
	var content string
	if _, err := os.Stat(gitignorePath); err == nil {
		data, err := os.ReadFile(gitignorePath)
		if err != nil {
			return fmt.Errorf("read .gitignore: %w", err)
		}
		content = string(data)
	}

	// Define entries to add
	entries := []string{
		taskDirName + "/work/",
		taskDirName + "/" + envFileName,
		activeTaskFile,
	}

	modified := false
	lines := strings.Split(content, "\n")

	for _, entry := range entries {
		found := false
		for _, line := range lines {
			if strings.TrimSpace(line) == entry {
				found = true
				break
			}
		}

		if !found {
			if len(content) > 0 && !strings.HasSuffix(content, "\n") {
				content += "\n"
				lines = append(lines, "")
			}
			content += entry + "\n"
			lines = append(lines, entry)
			modified = true
		}
	}

	if modified {
		if err := os.WriteFile(gitignorePath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write .gitignore: %w", err)
		}
	}

	return nil
}

// ActiveTask methods

// ActiveTaskPath returns the path to .active_task file
func (w *Workspace) ActiveTaskPath() string {
	return filepath.Join(w.root, activeTaskFile)
}

// HasActiveTask checks if there's an active task
func (w *Workspace) HasActiveTask() bool {
	_, err := os.Stat(w.ActiveTaskPath())
	return err == nil
}

// LoadActiveTask loads the active task reference
func (w *Workspace) LoadActiveTask() (*ActiveTask, error) {
	data, err := os.ReadFile(w.ActiveTaskPath())
	if err != nil {
		return nil, fmt.Errorf("read active task: %w", err)
	}

	var active ActiveTask
	if err := yaml.Unmarshal(data, &active); err != nil {
		return nil, fmt.Errorf("parse active task: %w", err)
	}

	return &active, nil
}

// SaveActiveTask saves the active task reference using atomic write pattern
func (w *Workspace) SaveActiveTask(active *ActiveTask) error {
	data, err := yaml.Marshal(active)
	if err != nil {
		return fmt.Errorf("marshal active task: %w", err)
	}

	// Use atomic write pattern: write to temp file, then rename
	path := w.ActiveTaskPath()
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write active task: %w", err)
	}
	// Atomic rename is guaranteed to be atomic on POSIX systems
	if err := os.Rename(tmpPath, path); err != nil {
		// Clean up temp file on error, log if cleanup fails
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			slog.Warn("failed to clean up temp file after rename error", "path", tmpPath, "error", removeErr)
		}
		return fmt.Errorf("save active task: %w", err)
	}

	return nil
}

// ClearActiveTask removes the active task file
func (w *Workspace) ClearActiveTask() error {
	err := os.Remove(w.ActiveTaskPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// UpdateActiveTaskState updates just the state field
func (w *Workspace) UpdateActiveTaskState(state string) error {
	active, err := w.LoadActiveTask()
	if err != nil {
		return err
	}
	active.State = state
	return w.SaveActiveTask(active)
}

// Work directory methods

// WorkPath returns the path for a specific task's work directory
func (w *Workspace) WorkPath(taskID string) string {
	return filepath.Join(w.workRoot, taskID)
}

// WorkExists checks if a work directory exists
func (w *Workspace) WorkExists(taskID string) bool {
	workPath := w.WorkPath(taskID)
	info, err := os.Stat(workPath)
	return err == nil && info.IsDir()
}

// GenerateTaskID generates a unique task ID
func GenerateTaskID() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("task-%06x", time.Now().UnixNano()&0xffffff)
	}
	return hex.EncodeToString(bytes)
}

// CreateWork creates a new work directory with initial structure
func (w *Workspace) CreateWork(taskID string, source SourceInfo) (*TaskWork, error) {
	workPath := w.WorkPath(taskID)

	// Create work directory structure
	dirs := []string{
		workPath,
		filepath.Join(workPath, specsDirName),
		filepath.Join(workPath, sessionsDirName),
		filepath.Join(workPath, "source"), // Source files directory
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	// Create work metadata
	work := NewTaskWork(taskID, source)

	// Save work.yaml
	if err := w.SaveWork(work); err != nil {
		return nil, fmt.Errorf("save work: %w", err)
	}

	// Create empty notes.md
	notesPath := filepath.Join(workPath, notesFileName)
	if err := os.WriteFile(notesPath, []byte("# Notes\n\n"), 0o644); err != nil {
		return nil, fmt.Errorf("create notes file: %w", err)
	}

	return work, nil
}

// LoadWork loads a task's work metadata
func (w *Workspace) LoadWork(taskID string) (*TaskWork, error) {
	workFile := filepath.Join(w.WorkPath(taskID), workFileName)

	data, err := os.ReadFile(workFile)
	if err != nil {
		return nil, fmt.Errorf("read work file: %w", err)
	}

	var work TaskWork
	if err := yaml.Unmarshal(data, &work); err != nil {
		return nil, fmt.Errorf("parse work file: %w", err)
	}

	return &work, nil
}

// SaveWork saves a task's work metadata using atomic write pattern
func (w *Workspace) SaveWork(work *TaskWork) error {
	work.Metadata.UpdatedAt = time.Now()

	workFile := filepath.Join(w.WorkPath(work.Metadata.ID), workFileName)

	data, err := yaml.Marshal(work)
	if err != nil {
		return fmt.Errorf("marshal work: %w", err)
	}

	// Use atomic write pattern: write to temp file, then rename
	tmpFile := workFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("write work file: %w", err)
	}
	// Atomic rename is guaranteed to be atomic on POSIX systems
	if err := os.Rename(tmpFile, workFile); err != nil {
		// Clean up temp file on error, log if cleanup fails
		if removeErr := os.Remove(tmpFile); removeErr != nil {
			slog.Warn("failed to clean up temp file after rename error", "path", tmpFile, "error", removeErr)
		}
		return fmt.Errorf("save work: %w", err)
	}

	return nil
}

// AddUsage adds token usage stats to a task's work and saves it
func (w *Workspace) AddUsage(taskID, step string, inputTokens, outputTokens, cachedTokens int, costUSD float64) error {
	work, err := w.LoadWork(taskID)
	if err != nil {
		return fmt.Errorf("load work: %w", err)
	}

	// Initialize ByStep map if needed
	if work.Costs.ByStep == nil {
		work.Costs.ByStep = make(map[string]StepCostStats)
	}

	// Update totals
	work.Costs.TotalInputTokens += inputTokens
	work.Costs.TotalOutputTokens += outputTokens
	work.Costs.TotalCachedTokens += cachedTokens
	work.Costs.TotalCostUSD += costUSD

	// Update step stats
	stepStats := work.Costs.ByStep[step]
	stepStats.InputTokens += inputTokens
	stepStats.OutputTokens += outputTokens
	stepStats.CachedTokens += cachedTokens
	stepStats.CostUSD += costUSD
	stepStats.Calls++
	work.Costs.ByStep[step] = stepStats

	return w.SaveWork(work)
}

// DeleteWork removes a work directory
func (w *Workspace) DeleteWork(taskID string) error {
	workPath := w.WorkPath(taskID)
	return os.RemoveAll(workPath)
}

// ListWorks returns all task IDs in the work directory
func (w *Workspace) ListWorks() ([]string, error) {
	if _, err := os.Stat(w.workRoot); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(w.workRoot)
	if err != nil {
		return nil, fmt.Errorf("read work directory: %w", err)
	}

	var taskIDs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			// Check if it has a work.yaml
			workFile := filepath.Join(w.workRoot, entry.Name(), workFileName)
			if _, err := os.Stat(workFile); err == nil {
				taskIDs = append(taskIDs, entry.Name())
			}
		}
	}

	return taskIDs, nil
}

// Notes methods

// NotesPath returns the path to notes.md
func (w *Workspace) NotesPath(taskID string) string {
	return filepath.Join(w.WorkPath(taskID), notesFileName)
}

// AppendNote adds a note to notes.md
func (w *Workspace) AppendNote(taskID, content, state string) error {
	notesPath := w.NotesPath(taskID)

	// Read existing content
	existing, _ := os.ReadFile(notesPath)

	// Format new note
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	stateTag := ""
	if state != "" {
		stateTag = fmt.Sprintf(" [%s]", state)
	}
	newNote := fmt.Sprintf("\n## %s%s\n\n%s\n", timestamp, stateTag, content)

	// Use strings.Builder for efficient concatenation
	var b strings.Builder
	b.Grow(len(existing) + len(newNote))
	b.Write(existing)
	b.WriteString(newNote)
	return os.WriteFile(notesPath, []byte(b.String()), 0o644)
}

// ReadNotes reads the notes file content
func (w *Workspace) ReadNotes(taskID string) (string, error) {
	data, err := os.ReadFile(w.NotesPath(taskID))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Worktree and locking methods

const locksDirName = "locks"

// LocksDir returns the path to the locks directory
func (w *Workspace) LocksDir() string {
	return filepath.Join(w.taskRoot, locksDirName)
}

// TaskLockPath returns the path to the lock file for a task
func (w *Workspace) TaskLockPath(taskID string) string {
	return filepath.Join(w.LocksDir(), taskID+".lock")
}

// WithTaskLock executes a function while holding an exclusive lock on the task.
// This prevents concurrent processes from modifying the same task simultaneously.
func (w *Workspace) WithTaskLock(taskID string, fn func() error) error {
	return WithLock(w.TaskLockPath(taskID), fn)
}

// WithTaskLockTimeout executes a function while holding a task lock,
// with a timeout for acquiring the lock.
func (w *Workspace) WithTaskLockTimeout(taskID string, timeout time.Duration, fn func() error) error {
	return WithLockTimeout(w.TaskLockPath(taskID), timeout, fn)
}

// FindTaskByWorktreePath finds a task by its worktree path.
// This is used to auto-detect the active task when running commands from within a worktree.
// Returns nil if no task is found with the given worktree path.
func (w *Workspace) FindTaskByWorktreePath(worktreePath string) (*ActiveTask, error) {
	// Normalize the path for comparison
	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		return nil, fmt.Errorf("resolve worktree path: %w", err)
	}

	// List all tasks and check their worktree paths
	taskIDs, err := w.ListWorks()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	for _, taskID := range taskIDs {
		work, err := w.LoadWork(taskID)
		if err != nil {
			continue // Skip tasks that can't be loaded
		}

		if work.Git.WorktreePath == "" {
			continue // Task doesn't have a worktree
		}

		// Normalize and compare paths
		taskWorktreePath, err := filepath.Abs(work.Git.WorktreePath)
		if err != nil {
			continue
		}

		if taskWorktreePath == absPath {
			// Found the task, build an ActiveTask from the work metadata
			active := &ActiveTask{
				ID:           work.Metadata.ID,
				Ref:          work.Source.Ref,
				WorkDir:      w.WorkPath(taskID),
				State:        "", // Will be loaded from .active_task if available
				Branch:       work.Git.Branch,
				UseGit:       work.Git.Branch != "",
				WorktreePath: work.Git.WorktreePath,
				Started:      work.Metadata.CreatedAt,
			}

			// Try to load the current state from .active_task if this is the active task
			if w.HasActiveTask() {
				existing, err := w.LoadActiveTask()
				if err == nil && existing.ID == taskID {
					active.State = existing.State
				}
			}

			return active, nil
		}
	}

	return nil, nil // No task found for this worktree
}

// ListTasksWithWorktrees returns all tasks that have associated worktrees.
// This is useful for listing parallel tasks across multiple terminals.
func (w *Workspace) ListTasksWithWorktrees() ([]*TaskWork, error) {
	taskIDs, err := w.ListWorks()
	if err != nil {
		return nil, err
	}

	var tasks []*TaskWork
	for _, taskID := range taskIDs {
		work, err := w.LoadWork(taskID)
		if err != nil {
			continue
		}

		if work.Git.WorktreePath != "" {
			tasks = append(tasks, work)
		}
	}

	return tasks, nil
}
