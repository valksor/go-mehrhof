package storage

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkspaceConfig holds workspace-specific configuration that users can customize.
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
	Storage   StorageSettings             `yaml:"storage,omitempty"`
}

// PluginsConfig holds plugin-related configuration.
type PluginsConfig struct {
	// Enabled lists the plugin names that should be loaded
	// Only plugins in this list will be activated
	Enabled []string `yaml:"enabled,omitempty"`

	// Config holds plugin-specific configuration keyed by plugin name
	// Each plugin receives its configuration during initialization
	Config map[string]map[string]any `yaml:"config,omitempty"`
}

// GitHubSettings holds GitHub provider configuration.
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

// GitHubCommentsSettings controls automated GitHub issue commenting.
type GitHubCommentsSettings struct {
	Enabled         bool `yaml:"enabled"`           // Master switch (default: false)
	OnBranchCreated bool `yaml:"on_branch_created"` // Post when branch is created
	OnPlanDone      bool `yaml:"on_plan_done"`      // Post summary of planned implementation
	OnImplementDone bool `yaml:"on_implement_done"` // Post changelog with files changed
	OnPRCreated     bool `yaml:"on_pr_created"`     // Post PR link
}

// WrikeSettings holds Wrike provider configuration.
type WrikeSettings struct {
	Token  string `yaml:"token,omitempty"`  // Wrike API token (env vars take priority)
	Host   string `yaml:"host,omitempty"`   // API base URL override (default: https://www.wrike.com/api/v4)
	Folder string `yaml:"folder,omitempty"` // Default folder ID for task lookup
}

// GitLabSettings holds GitLab provider configuration.
type GitLabSettings struct {
	Token         string `yaml:"token,omitempty"`          // GitLab token (env vars take priority)
	Host          string `yaml:"host,omitempty"`           // GitLab host (default: https://gitlab.com)
	ProjectPath   string `yaml:"project_path,omitempty"`   // Default project path (e.g., group/project)
	BranchPattern string `yaml:"branch_pattern,omitempty"` // Default: "issue/{key}-{slug}"
	CommitPrefix  string `yaml:"commit_prefix,omitempty"`  // Default: "[#{key}]"
}

// NotionSettings holds Notion provider configuration.
type NotionSettings struct {
	Token               string `yaml:"token,omitempty"`                // Notion token (env vars take priority)
	DatabaseID          string `yaml:"database_id,omitempty"`          // Default database ID
	StatusProperty      string `yaml:"status_property,omitempty"`      // Property name for status (default: Status)
	DescriptionProperty string `yaml:"description_property,omitempty"` // Property name for description
	LabelsProperty      string `yaml:"labels_property,omitempty"`      // Property name for labels (default: Tags)
}

// JiraSettings holds Jira provider configuration.
type JiraSettings struct {
	Token   string `yaml:"token,omitempty"`    // Jira API token (env vars take priority)
	Email   string `yaml:"email,omitempty"`    // Email for Cloud auth
	BaseURL string `yaml:"base_url,omitempty"` // Base URL (optional, auto-detected)
	Project string `yaml:"project,omitempty"`  // Default project key
}

// LinearSettings holds Linear provider configuration.
type LinearSettings struct {
	Token string `yaml:"token,omitempty"` // Linear API key (env vars take priority)
	Team  string `yaml:"team,omitempty"`  // Default team key
}

// YouTrackSettings holds YouTrack provider configuration.
type YouTrackSettings struct {
	Token string `yaml:"token,omitempty"` // YouTrack token (env vars take priority)
	Host  string `yaml:"host,omitempty"`  // YouTrack host
}

// AgentAliasConfig defines a user-defined agent alias that wraps an existing agent
// with custom environment variables and CLI arguments.
type AgentAliasConfig struct {
	Extends     string            `yaml:"extends"`               // Base agent name to wrap
	Description string            `yaml:"description,omitempty"` // Human-readable description
	Env         map[string]string `yaml:"env,omitempty"`         // Environment variables to pass
	Args        []string          `yaml:"args,omitempty"`        // CLI arguments to pass
}

// GitSettings holds git-related configuration.
type GitSettings struct {
	CommitPrefix  string `yaml:"commit_prefix"`
	BranchPattern string `yaml:"branch_pattern"`
	AutoCommit    bool   `yaml:"auto_commit"`
	SignCommits   bool   `yaml:"sign_commits"`
}

// StepAgentConfig holds agent configuration for a specific workflow step.
type StepAgentConfig struct {
	Name string            `yaml:"name,omitempty"` // Agent name or alias
	Env  map[string]string `yaml:"env,omitempty"`  // Step-specific env vars
	Args []string          `yaml:"args,omitempty"` // Step-specific CLI args
}

// AgentSettings holds agent-related configuration.
type AgentSettings struct {
	Default    string                     `yaml:"default"`
	Timeout    int                        `yaml:"timeout"`
	MaxRetries int                        `yaml:"max_retries"`
	Steps      map[string]StepAgentConfig `yaml:"steps,omitempty"` // Per-step agent configuration
}

// WorkflowSettings holds workflow-related configuration.
type WorkflowSettings struct {
	AutoInit             bool `yaml:"auto_init"`
	SessionRetentionDays int  `yaml:"session_retention_days"`
	DeleteWorkOnFinish   bool `yaml:"delete_work_on_finish"`  // Delete work dirs on finish (default: false)
	DeleteWorkOnAbandon  bool `yaml:"delete_work_on_abandon"` // Delete work dirs on abandon (default: true)
}

// UpdateSettings holds update-related configuration.
type UpdateSettings struct {
	Enabled       bool `yaml:"enabled"`        // Enable automatic update checks
	CheckInterval int  `yaml:"check_interval"` // Hours between checks (default: 24)
}

// StorageSettings holds storage-related configuration.
type StorageSettings struct {
	WorkDir string `yaml:"work_dir,omitempty"` // Path to work directory (relative to project root)
}

// ProvidersSettings holds provider-related configuration.
type ProvidersSettings struct {
	Default string `yaml:"default,omitempty"` // Default provider for bare references (e.g., "file", "directory", "github")
}

// NewDefaultWorkspaceConfig creates a WorkspaceConfig with default values.
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
			DeleteWorkOnFinish:   false, // Keep work dirs by default on finish
			DeleteWorkOnAbandon:  true,  // Delete work dirs by default on abandon
		},
		Providers: ProvidersSettings{
			Default: "file",
		},
		Update: UpdateSettings{
			Enabled:       true,
			CheckInterval: 24,
		},
		Storage: StorageSettings{
			WorkDir: ".mehrhof/work", // Default: .mehrhof/work (relative to project root)
		},
		Env: make(map[string]string),
	}
}

// GetEnvForAgent returns env vars for a specific agent, stripping the prefix.
// E.g., for agent "claude": CLAUDE_FOO=bar → FOO=bar.
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

// SaveConfig saves the workspace configuration to .mehrhof/config.yaml.
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

	// Add storage section comment if storage work_dir is default/empty
	if cfg.Storage.WorkDir == "" || cfg.Storage.WorkDir == ".mehrhof/work" {
		content += `
# Storage settings
# Configure where task work directories are stored (relative to project root)
# Example:
# storage:
#     work_dir: .mehrhof/work    # Default location
#     work_dir: tasks/           # Alternative: store in project root tasks/ directory
#     work_dir: .task-work       # Alternative: hidden directory in project root
`
	}

	// Add workflow cleanup settings comment
	if !cfg.Workflow.DeleteWorkOnFinish && cfg.Workflow.DeleteWorkOnAbandon {
		content += `
# Workflow cleanup settings
# Control whether work directories are deleted when tasks finish/abandon
# Example:
# workflow:
#     delete_work_on_finish: false   # Keep work dirs after finish (default)
#     delete_work_on_abandon: true   # Delete work dirs on abandon (default)
`
	}

	if err := os.WriteFile(w.ConfigPath(), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// LoadConfig loads the workspace configuration from .mehrhof/config.yaml.
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
