package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

// WorkspaceConfig holds workspace-specific configuration that users can customize
type WorkspaceConfig struct {
	Git       GitSettings                 `yaml:"git"`
	Agent     AgentSettings               `yaml:"agent"`
	Workflow  WorkflowSettings            `yaml:"workflow"`
	Providers ProvidersSettings           `yaml:"providers,omitempty"`
	Env       map[string]string           `yaml:"env,omitempty"`
	Agents    map[string]AgentAliasConfig `yaml:"agents,omitempty"`
	GitHub    *GitHubSettings             `yaml:"github,omitempty"`
	Wrike     *WrikeSettings              `yaml:"wrike,omitempty"`
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
#     sonnet-fast:
#         extends: claude
#         description: "Claude Sonnet with limited turns"
#         args: ["--model", "claude-sonnet-4-20250514", "--max-turns", "3"]
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

// Specification methods

// SpecificationsDir returns the specifications directory path
func (w *Workspace) SpecificationsDir(taskID string) string {
	return filepath.Join(w.WorkPath(taskID), specsDirName)
}

// SpecificationPath returns the path for a specification file
func (w *Workspace) SpecificationPath(taskID string, number int) string {
	filename := fmt.Sprintf("specification-%d.md", number)
	return filepath.Join(w.SpecificationsDir(taskID), filename)
}

// SaveSpecification saves a specification file (markdown)
func (w *Workspace) SaveSpecification(taskID string, number int, content string) error {
	specPath := w.SpecificationPath(taskID, number)
	return os.WriteFile(specPath, []byte(content), 0o644)
}

// LoadSpecification loads a specification file content
func (w *Workspace) LoadSpecification(taskID string, number int) (string, error) {
	specPath := w.SpecificationPath(taskID, number)
	data, err := os.ReadFile(specPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ListSpecifications returns all specification numbers for a task
func (w *Workspace) ListSpecifications(taskID string) ([]int, error) {
	specsDir := w.SpecificationsDir(taskID)

	entries, err := os.ReadDir(specsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []int{}, nil
		}
		return nil, fmt.Errorf("read specifications directory: %w", err)
	}

	pattern := regexp.MustCompile(`^specification-(\d+)\.md$`)
	var numbers []int

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := pattern.FindStringSubmatch(entry.Name())
		if matches != nil {
			num, _ := strconv.Atoi(matches[1])
			numbers = append(numbers, num)
		}
	}

	sort.Ints(numbers)
	return numbers, nil
}

// NextSpecificationNumber returns the next available specification number
func (w *Workspace) NextSpecificationNumber(taskID string) (int, error) {
	specifications, err := w.ListSpecifications(taskID)
	if err != nil {
		return 0, err
	}

	if len(specifications) == 0 {
		return 1, nil
	}

	return specifications[len(specifications)-1] + 1, nil
}

// GatherSpecificationsContent reads all specifications and returns combined content
func (w *Workspace) GatherSpecificationsContent(taskID string) (string, error) {
	specifications, err := w.ListSpecifications(taskID)
	if err != nil {
		return "", err
	}

	var parts []string
	for _, num := range specifications {
		content, err := w.LoadSpecification(taskID, num)
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("### Specification %d\n\n%s", num, content))
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

// GetLatestSpecificationContent returns only the most recent specification content
func (w *Workspace) GetLatestSpecificationContent(taskID string) (string, int, error) {
	specifications, err := w.ListSpecifications(taskID)
	if err != nil {
		return "", 0, err
	}

	if len(specifications) == 0 {
		return "", 0, nil
	}

	// specifications are sorted, so last one is the latest
	latestNum := specifications[len(specifications)-1]
	content, err := w.LoadSpecification(taskID, latestNum)
	if err != nil {
		return "", 0, err
	}

	return content, latestNum, nil
}

// ParseSpecification parses a specification file with optional YAML frontmatter
func (w *Workspace) ParseSpecification(taskID string, number int) (*Specification, error) {
	content, err := w.LoadSpecification(taskID, number)
	if err != nil {
		return nil, err
	}

	spec := &Specification{
		Number: number,
		Status: SpecificationStatusDraft, // default status
	}

	// Check for YAML frontmatter (starts with ---)
	if strings.HasPrefix(content, "---\n") {
		// Find the closing ---
		endIdx := strings.Index(content[4:], "\n---")
		if endIdx > 0 {
			frontmatter := content[4 : 4+endIdx]
			spec.Content = strings.TrimSpace(content[4+endIdx+4:])

			// Parse frontmatter
			if err := yaml.Unmarshal([]byte(frontmatter), spec); err != nil {
				// Ignore frontmatter parsing errors, just use content
				spec.Content = content
			}
		} else {
			spec.Content = content
		}
	} else {
		spec.Content = content
	}

	// Extract title from first heading
	lines := strings.Split(spec.Content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			spec.Title = strings.TrimPrefix(line, "# ")
			break
		}
	}

	return spec, nil
}

// SaveSpecificationWithMeta saves a specification with YAML frontmatter
func (w *Workspace) SaveSpecificationWithMeta(taskID string, spec *Specification) error {
	// Ensure timestamps
	now := time.Now()
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = now
	}
	spec.UpdatedAt = now

	// Build frontmatter
	frontmatter, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal specification frontmatter: %w", err)
	}

	// Combine frontmatter and content
	var content strings.Builder
	content.WriteString("---\n")
	content.Write(frontmatter)
	content.WriteString("---\n\n")
	content.WriteString(spec.Content)

	return w.SaveSpecification(taskID, spec.Number, content.String())
}

// UpdateSpecificationStatus updates the status of a specification file
func (w *Workspace) UpdateSpecificationStatus(taskID string, number int, status string) error {
	spec, err := w.ParseSpecification(taskID, number)
	if err != nil {
		return err
	}

	spec.Status = status
	spec.UpdatedAt = time.Now()

	// Set completion timestamp if done
	if status == SpecificationStatusDone && spec.CompletedAt.IsZero() {
		spec.CompletedAt = time.Now()
	}

	return w.SaveSpecificationWithMeta(taskID, spec)
}

// ListSpecificationsWithStatus returns all specifications with their parsed status
func (w *Workspace) ListSpecificationsWithStatus(taskID string) ([]*Specification, error) {
	numbers, err := w.ListSpecifications(taskID)
	if err != nil {
		return nil, err
	}

	specifications := make([]*Specification, 0, len(numbers))
	for _, num := range numbers {
		specification, err := w.ParseSpecification(taskID, num)
		if err != nil {
			// Include specification with error status
			specifications = append(specifications, &Specification{Number: num, Status: "error"})
			continue
		}
		specifications = append(specifications, specification)
	}

	return specifications, nil
}

// GetSpecificationsSummary returns a summary of specification statuses
func (w *Workspace) GetSpecificationsSummary(taskID string) (map[string]int, error) {
	specifications, err := w.ListSpecificationsWithStatus(taskID)
	if err != nil {
		return nil, err
	}

	summary := map[string]int{
		SpecificationStatusDraft:        0,
		SpecificationStatusReady:        0,
		SpecificationStatusImplementing: 0,
		SpecificationStatusDone:         0,
	}

	for _, specification := range specifications {
		summary[specification.Status]++
	}

	return summary, nil
}

// Session methods

// SessionsDir returns the sessions directory path
func (w *Workspace) SessionsDir(taskID string) string {
	return filepath.Join(w.WorkPath(taskID), sessionsDirName)
}

// SessionPath returns the path for a session file
func (w *Workspace) SessionPath(taskID, filename string) string {
	return filepath.Join(w.SessionsDir(taskID), filename)
}

// CreateSession creates a new session
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

// LoadSession loads a session by filename
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

// SaveSession saves a session
func (w *Workspace) SaveSession(taskID, filename string, session *Session) error {
	sessionFile := w.SessionPath(taskID, filename)

	data, err := yaml.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	return os.WriteFile(sessionFile, data, 0o644)
}

// ListSessions returns all sessions for a task
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
func (w *Workspace) GetSourceContent(taskID string) (string, error) {
	work, err := w.LoadWork(taskID)
	if err != nil {
		return "", err
	}

	var parts []string

	// Single file source
	if work.Source.Content != "" {
		parts = append(parts, work.Source.Content)
	}

	// Multiple files from directory
	for _, f := range work.Source.Files {
		parts = append(parts, fmt.Sprintf("### %s\n\n%s", f.Path, f.Content))
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

// PendingQuestion represents a question from the agent awaiting user response
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

// QuestionOption represents an answer option
type QuestionOption struct {
	Label       string `yaml:"label"`
	Description string `yaml:"description,omitempty"`
}

const pendingQuestionFile = "pending_question.yaml"

// PendingQuestionPath returns the path to pending question file
func (w *Workspace) PendingQuestionPath(taskID string) string {
	return filepath.Join(w.WorkPath(taskID), pendingQuestionFile)
}

// HasPendingQuestion checks if there's a pending question
func (w *Workspace) HasPendingQuestion(taskID string) bool {
	_, err := os.Stat(w.PendingQuestionPath(taskID))
	return err == nil
}

// SavePendingQuestion saves a pending question
func (w *Workspace) SavePendingQuestion(taskID string, q *PendingQuestion) error {
	data, err := yaml.Marshal(q)
	if err != nil {
		return fmt.Errorf("marshal question: %w", err)
	}
	return os.WriteFile(w.PendingQuestionPath(taskID), data, 0o644)
}

// LoadPendingQuestion loads a pending question
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

// ClearPendingQuestion removes the pending question file
func (w *Workspace) ClearPendingQuestion(taskID string) error {
	err := os.Remove(w.PendingQuestionPath(taskID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Planned directory methods (standalone planning without a task)

// PlannedRoot returns the .mehrhof/planned directory path
func (w *Workspace) PlannedRoot() string {
	return filepath.Join(w.taskRoot, plannedDirName)
}

// PlannedPath returns the path for a specific plan
func (w *Workspace) PlannedPath(planID string) string {
	return filepath.Join(w.PlannedRoot(), planID)
}

// Plan represents a standalone planning session
type Plan struct {
	Version string      `yaml:"version"`
	ID      string      `yaml:"id"`
	Title   string      `yaml:"title,omitempty"`
	Seed    string      `yaml:"seed,omitempty"`
	Created time.Time   `yaml:"created"`
	Updated time.Time   `yaml:"updated"`
	History []PlanEntry `yaml:"history,omitempty"`
}

// PlanEntry represents an entry in the planning conversation
type PlanEntry struct {
	Timestamp time.Time `yaml:"timestamp"`
	Role      string    `yaml:"role"`
	Content   string    `yaml:"content"`
}

const (
	planFileName        = "plan.yaml"
	planHistoryFileName = "plan-history.md"
)

// CreatePlan creates a new standalone plan
func (w *Workspace) CreatePlan(planID, seed string) (*Plan, error) {
	planPath := w.PlannedPath(planID)

	// Create plan directory
	if err := os.MkdirAll(planPath, 0o755); err != nil {
		return nil, fmt.Errorf("create plan directory: %w", err)
	}

	now := time.Now()
	plan := &Plan{
		Version: "1",
		ID:      planID,
		Seed:    seed,
		Created: now,
		Updated: now,
		History: make([]PlanEntry, 0),
	}

	// Save plan.yaml
	if err := w.SavePlan(plan); err != nil {
		return nil, fmt.Errorf("save plan: %w", err)
	}

	// Create initial plan-history.md
	historyPath := filepath.Join(planPath, planHistoryFileName)
	header := fmt.Sprintf("# Planning Session\n\nCreated: %s\n", now.Format("2006-01-02 15:04:05"))
	if seed != "" {
		header += fmt.Sprintf("\nSeed Topic: %s\n", seed)
	}
	header += "\n---\n\n"
	if err := os.WriteFile(historyPath, []byte(header), 0o644); err != nil {
		return nil, fmt.Errorf("create history file: %w", err)
	}

	return plan, nil
}

// SavePlan saves a plan's metadata
func (w *Workspace) SavePlan(plan *Plan) error {
	plan.Updated = time.Now()
	planFile := filepath.Join(w.PlannedPath(plan.ID), planFileName)

	data, err := yaml.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	return os.WriteFile(planFile, data, 0o644)
}

// LoadPlan loads a plan by ID
func (w *Workspace) LoadPlan(planID string) (*Plan, error) {
	planFile := filepath.Join(w.PlannedPath(planID), planFileName)

	data, err := os.ReadFile(planFile)
	if err != nil {
		return nil, fmt.Errorf("read plan file: %w", err)
	}

	var plan Plan
	if err := yaml.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parse plan file: %w", err)
	}

	return &plan, nil
}

// AppendPlanHistory appends an entry to the plan history (both YAML and markdown)
func (w *Workspace) AppendPlanHistory(planID, role, content string) error {
	plan, err := w.LoadPlan(planID)
	if err != nil {
		return err
	}

	entry := PlanEntry{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	plan.History = append(plan.History, entry)

	// Save updated plan.yaml
	if err := w.SavePlan(plan); err != nil {
		return err
	}

	// Append to markdown history
	historyPath := filepath.Join(w.PlannedPath(planID), planHistoryFileName)
	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer func() { _ = f.Close() }()

	roleLabel := "User"
	if role == "assistant" {
		roleLabel = "Assistant"
	}
	_, err = fmt.Fprintf(f, "## %s (%s)\n\n%s\n\n---\n\n",
		roleLabel, entry.Timestamp.Format("15:04:05"), content)
	return err
}

// ListPlans returns all plan IDs
func (w *Workspace) ListPlans() ([]string, error) {
	plannedRoot := w.PlannedRoot()
	if _, err := os.Stat(plannedRoot); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(plannedRoot)
	if err != nil {
		return nil, fmt.Errorf("read planned directory: %w", err)
	}

	var planIDs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			planFile := filepath.Join(plannedRoot, entry.Name(), planFileName)
			if _, err := os.Stat(planFile); err == nil {
				planIDs = append(planIDs, entry.Name())
			}
		}
	}

	return planIDs, nil
}

// DeletePlan removes a plan directory
func (w *Workspace) DeletePlan(planID string) error {
	return os.RemoveAll(w.PlannedPath(planID))
}

// GeneratePlanID generates a unique plan ID based on timestamp
func GeneratePlanID() string {
	return time.Now().Format("2006-01-02-150405")
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
