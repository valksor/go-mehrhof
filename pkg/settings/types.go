// Package settings provides two-tier configuration management with schema-driven UI generation.
//
// Configuration is stored in two locations:
//   - Global: ~/.valksor/kvelmo/kvelmo.yaml (lower priority)
//   - Project: .valksor/kvelmo.yaml (higher priority, overrides global)
//
// Sensitive fields (tokens) are stored in .env files next to the config:
//   - Global: ~/.valksor/kvelmo/.env
//   - Project: .valksor/.env
//
// Schema tags on struct fields drive automatic React form generation.
package settings

// Settings represents the complete configuration for kvelmo.
// Project settings override global settings when both are present.
type Settings struct {
	Preset       string                 `yaml:"preset,omitempty" json:"preset,omitempty" schema:"label=Preset;desc=Configuration preset to apply defaults (e.g. compliance);options=|compliance|fast|solo;advanced"`
	Identity     string                 `yaml:"identity,omitempty" json:"identity,omitempty" schema:"label=Identity;desc=User identity for approval records and audit trail (defaults to OS username)"`
	Agent        AgentSettings          `yaml:"agent,omitempty" json:"agent,omitempty"`
	Providers    ProviderSettings       `yaml:"providers,omitempty" json:"providers,omitempty"`
	Git          GitSettings            `yaml:"git,omitempty" json:"git,omitempty"`
	Workers      WorkerSettings         `yaml:"workers,omitempty" json:"workers,omitempty"`
	Storage      StorageSettings        `yaml:"storage,omitempty" json:"storage,omitempty"`
	Workflow     WorkflowSettings       `yaml:"workflow,omitempty" json:"workflow,omitempty"`
	Notify       NotifySettings         `yaml:"notify,omitempty" json:"notify,omitempty"`
	Watchdog     WatchdogSettings       `yaml:"watchdog,omitempty" json:"watchdog,omitempty"`
	Security     SecuritySettings       `yaml:"security,omitempty" json:"security,omitempty"`
	UI           UISettings             `yaml:"ui,omitempty" json:"ui,omitempty"`
	TUI          TUISettings            `yaml:"tui,omitempty" json:"tui,omitempty"`
	Environment  string                 `yaml:"environment,omitempty" json:"environment,omitempty" schema:"label=Environment;desc=Deployment environment (dev, staging, prod);options=dev|staging|prod;default=dev"`
	CustomAgents map[string]CustomAgent `yaml:"custom_agents,omitempty" json:"custom_agents,omitempty"`
}

// UISettings configures UI state that persists across sessions.
type UISettings struct {
	OnboardingDismissed bool `yaml:"onboarding_dismissed,omitempty" json:"onboarding_dismissed,omitempty"`
}

// TUISettings configures the terminal user interface.
type TUISettings struct {
	Layout string `yaml:"layout,omitempty" json:"layout,omitempty" schema:"label=TUI Layout;desc=Default layout for kvelmo tui (stacked or dashboard);options=stacked|dashboard;default=stacked"`
}

// AgentSettings configures AI agent behavior.
type AgentSettings struct {
	Default       string            `yaml:"default,omitempty" json:"default,omitempty" schema:"label=Default Agent;desc=Agent used when none specified;options=claude|codex|openai|anthropic|ollama"`
	Allowed       []string          `yaml:"allowed,omitempty" json:"allowed,omitempty" schema:"label=Allowed Agents;desc=Agents permitted for this project;type=multiselect;options=claude|codex|openai|anthropic|ollama"`
	Strategy      string            `yaml:"strategy,omitempty" json:"strategy,omitempty" schema:"label=Default Strategy;desc=Agent reasoning strategy (direct = pass-through, iterative = self-review loop);options=direct|iterative;default=direct"`
	PhaseStrategy map[string]string `yaml:"phase_strategy,omitempty" json:"phase_strategy,omitempty" schema:"label=Per-Phase Strategy;desc=Override strategy per phase (e.g. plan: iterative);type=keyvalue;advanced"`
	PhaseAgent    map[string]string `yaml:"phase_agent,omitempty" json:"phase_agent,omitempty" schema:"label=Per-Phase Agent;desc=Override agent per phase (e.g. plan: gemini, implement: claude);type=keyvalue;advanced"`
	Consensus     *ConsensusConfig  `yaml:"consensus,omitempty" json:"consensus,omitempty"`

	// API agent settings
	OpenAI    OpenAIAgentConfig    `yaml:"openai,omitempty" json:"openai,omitempty"`
	Anthropic AnthropicAgentConfig `yaml:"anthropic,omitempty" json:"anthropic,omitempty"`
	Ollama    OllamaAgentConfig    `yaml:"ollama,omitempty" json:"ollama,omitempty"`
}

// ConsensusConfig configures multi-agent consensus review.
type ConsensusConfig struct {
	Enabled      bool     `yaml:"enabled,omitempty" json:"enabled,omitempty" schema:"label=Enable Consensus;desc=Run reviews with multiple agents and merge findings;default=false"`
	Agents       []string `yaml:"agents,omitempty" json:"agents,omitempty" schema:"label=Consensus Agents;desc=Agent names to use for consensus review;type=tags"`
	MinAgreement int      `yaml:"min_agreement,omitempty" json:"min_agreement,omitempty" schema:"label=Min Agreement;desc=Minimum number of agents that must detect a finding;default=1;min=1"`
}

// CustomAgent defines a user-created agent that wraps a base agent.
// Custom agents are stored in settings.yaml under custom_agents map.
// They automatically appear in agent selection dropdowns in the UI.
type CustomAgent struct {
	Extends     string            `yaml:"extends" json:"extends" schema:"label=Base Agent;desc=Agent to wrap;options=claude|codex|openai|anthropic|ollama;required"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty" schema:"label=Description;desc=Human-readable description"`
	Args        []string          `yaml:"args,omitempty" json:"args,omitempty" schema:"label=CLI Arguments;desc=Additional arguments passed to agent;type=tags"`
	Env         map[string]string `yaml:"env,omitempty" json:"env,omitempty" schema:"label=Environment;desc=Environment variables for this agent;type=keyvalue"`
}

// OpenAIAgentConfig configures the OpenAI API agent.
type OpenAIAgentConfig struct {
	APIKey  string `yaml:"-" json:"api_key,omitempty" schema:"label=API Key;desc=OpenAI API key;sensitive;env=OPENAI_API_KEY;helpUrl=https://platform.openai.com/api-keys"`
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty" schema:"label=Base URL;desc=API base URL (for proxies or Azure OpenAI);default=https://api.openai.com;placeholder=https://api.openai.com"`
	Model   string `yaml:"model,omitempty" json:"model,omitempty" schema:"label=Model;desc=Model name;default=gpt-4o;placeholder=gpt-4o"`
}

// AnthropicAgentConfig configures the Anthropic API agent.
type AnthropicAgentConfig struct {
	APIKey  string `yaml:"-" json:"api_key,omitempty" schema:"label=API Key;desc=Anthropic API key;sensitive;env=ANTHROPIC_API_KEY;helpUrl=https://console.anthropic.com/settings/keys"`
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty" schema:"label=Base URL;desc=API base URL;default=https://api.anthropic.com;placeholder=https://api.anthropic.com"`
	Model   string `yaml:"model,omitempty" json:"model,omitempty" schema:"label=Model;desc=Model name;default=claude-sonnet-4-20250514;placeholder=claude-sonnet-4-20250514"`
}

// OllamaAgentConfig configures the Ollama agent.
type OllamaAgentConfig struct {
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty" schema:"label=Base URL;desc=Ollama server URL;default=http://localhost:11434;placeholder=http://localhost:11434"`
	Model   string `yaml:"model,omitempty" json:"model,omitempty" schema:"label=Model;desc=Model name (must support tool calling);default=llama3.1;placeholder=llama3.1"`
}

// ProviderSettings configures task providers (GitHub, GitLab, etc.).
type ProviderSettings struct {
	Default     string            `yaml:"default,omitempty" json:"default,omitempty" schema:"label=Default Provider;desc=Provider used when none specified;options=github|gitlab|wrike|linear|azuredevops|file"`
	GitHub      GitHubConfig      `yaml:"github,omitempty" json:"github,omitempty"`
	GitLab      GitLabConfig      `yaml:"gitlab,omitempty" json:"gitlab,omitempty"`
	Wrike       WrikeConfig       `yaml:"wrike,omitempty" json:"wrike,omitempty"`
	Linear      LinearConfig      `yaml:"linear,omitempty" json:"linear,omitempty"`
	Jira        JiraConfig        `yaml:"jira,omitempty" json:"jira,omitempty"`
	AzureDevOps AzureDevOpsConfig `yaml:"azuredevops,omitempty" json:"azuredevops,omitempty"`
}

// AzureDevOpsConfig configures the Azure DevOps provider.
type AzureDevOpsConfig struct {
	Token              string            `yaml:"-" json:"token,omitempty" schema:"label=Token;desc=Personal access token (work items, code scopes);sensitive;env=AZURE_DEVOPS_TOKEN;helpUrl=https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate"`
	BaseURL            string            `yaml:"base_url,omitempty" json:"base_url,omitempty" schema:"label=Base URL;desc=Azure DevOps instance URL;default=https://dev.azure.com;placeholder=https://dev.azure.com"`
	Organization       string            `yaml:"organization,omitempty" json:"organization,omitempty" schema:"label=Organization;desc=Azure DevOps organization name;required"`
	Project            string            `yaml:"project,omitempty" json:"project,omitempty" schema:"label=Project;desc=Azure DevOps project name;required"`
	Repository         string            `yaml:"repository,omitempty" json:"repository,omitempty" schema:"label=Repository;desc=Git repository name (defaults to project name)"`
	AllowTicketComment bool              `yaml:"allow_ticket_comment,omitempty" json:"allow_ticket_comment,omitempty" schema:"label=Allow Ticket Comments;desc=Post status comments on work items"`
	StatusSync         bool              `yaml:"status_sync,omitempty" json:"status_sync,omitempty" schema:"label=Status Sync;desc=Update work item state when task state changes"`
	StatusMapping      map[string]string `yaml:"status_mapping,omitempty" json:"status_mapping,omitempty" schema:"label=Status Mapping;desc=Map kvelmo states to Azure DevOps work item states;type=keyvalue"`
}

// GitHubConfig configures the GitHub provider.
type GitHubConfig struct {
	// Token is stored in .env as GITHUB_TOKEN, not in settings.yaml.
	// The yaml:"-" tag prevents yaml serialization.
	// The env=GITHUB_TOKEN tells the system which env var to use.
	Token              string            `yaml:"-" json:"token,omitempty" schema:"label=Token;desc=Personal access token (repo, workflow scopes);sensitive;env=GITHUB_TOKEN;helpUrl=https://github.com/settings/tokens"`
	Owner              string            `yaml:"owner,omitempty" json:"owner,omitempty" schema:"label=Owner;desc=Default repository owner;placeholder=auto-detect from git remote"`
	AllowTicketComment bool              `yaml:"allow_ticket_comment,omitempty" json:"allow_ticket_comment,omitempty" schema:"label=Allow Ticket Comments;desc=Post status comments on GitHub issues"`
	StatusSync         bool              `yaml:"status_sync,omitempty" json:"status_sync,omitempty" schema:"label=Status Sync;desc=Update issue labels when task state changes"`
	StatusMapping      map[string]string `yaml:"status_mapping,omitempty" json:"status_mapping,omitempty" schema:"label=Status Mapping;desc=Map kvelmo states to GitHub labels (e.g. implementing: in-progress);type=keyvalue"`
}

// GitLabConfig configures the GitLab provider.
type GitLabConfig struct {
	Token              string            `yaml:"-" json:"token,omitempty" schema:"label=Token;desc=Personal access token (api scope);sensitive;env=GITLAB_TOKEN;helpUrl=https://gitlab.com/-/user_settings/personal_access_tokens"`
	BaseURL            string            `yaml:"base_url,omitempty" json:"base_url,omitempty" schema:"label=Base URL;desc=GitLab instance URL;default=https://gitlab.com;placeholder=https://gitlab.com"`
	AllowTicketComment bool              `yaml:"allow_ticket_comment,omitempty" json:"allow_ticket_comment,omitempty" schema:"label=Allow Ticket Comments;desc=Post status comments on GitLab issues"`
	StatusSync         bool              `yaml:"status_sync,omitempty" json:"status_sync,omitempty" schema:"label=Status Sync;desc=Update issue/MR status when task state changes"`
	StatusMapping      map[string]string `yaml:"status_mapping,omitempty" json:"status_mapping,omitempty" schema:"label=Status Mapping;desc=Map kvelmo states to GitLab labels or statuses (e.g. implementing: in-progress);type=keyvalue"`
}

// WrikeConfig configures the Wrike provider.
type WrikeConfig struct {
	Token string `yaml:"-" json:"token,omitempty" schema:"label=Token;desc=Wrike API token;sensitive;env=WRIKE_TOKEN;helpUrl=https://www.wrike.com/frontend/apps/index.html#/api"`
	// IncludeParentContext and IncludeSiblingContext have default=true,
	// so we must NOT use omitempty (it drops false values on serialize).
	IncludeParentContext  bool `yaml:"include_parent_context" json:"include_parent_context" schema:"label=Include Parent Context;desc=Fetch parent task and include its context in AI prompts;default=true"`
	IncludeSiblingContext bool `yaml:"include_sibling_context" json:"include_sibling_context" schema:"label=Include Sibling Context;desc=Fetch sibling tasks and include them in AI prompts to avoid duplication;default=true"`
	// AllowTicketComment controls whether status comments are posted to Wrike tasks.
	AllowTicketComment bool `yaml:"allow_ticket_comment,omitempty" json:"allow_ticket_comment,omitempty" schema:"label=Allow Ticket Comments;desc=Post status comments on Wrike tasks"`
}

// LinearConfig configures the Linear provider.
type LinearConfig struct {
	// Token is stored in .env as LINEAR_TOKEN, not in settings.yaml.
	Token string `yaml:"-" json:"token,omitempty" schema:"label=Token;desc=Linear API token;sensitive;env=LINEAR_TOKEN;helpUrl=https://linear.app/settings/api"`
	// Team is the default team key (e.g. "ENG") used when creating tasks or listing.
	Team string `yaml:"team,omitempty" json:"team,omitempty" schema:"label=Default Team;desc=Default team key (e.g. ENG);placeholder=auto-detect"`
	// IncludeParentContext controls whether the parent issue is fetched and
	// included in AI prompts when planning/implementing a Linear task.
	// Has default=true, so we must NOT use omitempty (it drops false values on serialize).
	IncludeParentContext bool `yaml:"include_parent_context" json:"include_parent_context" schema:"label=Include Parent Context;desc=Fetch parent issue and include its context in AI prompts;default=true"`
	// IncludeSiblingContext controls whether sibling tasks (sub-issues of the
	// same parent) are fetched and included in AI prompts. Up to 5 siblings
	// are included.
	// Has default=true, so we must NOT use omitempty (it drops false values on serialize).
	IncludeSiblingContext bool `yaml:"include_sibling_context" json:"include_sibling_context" schema:"label=Include Sibling Context;desc=Fetch sibling issues and include them in AI prompts;default=true"`
	// AllowTicketComment controls whether status comments are posted to Linear issues.
	AllowTicketComment bool              `yaml:"allow_ticket_comment,omitempty" json:"allow_ticket_comment,omitempty" schema:"label=Allow Ticket Comments;desc=Post status comments on Linear issues"`
	StatusSync         bool              `yaml:"status_sync,omitempty" json:"status_sync,omitempty" schema:"label=Status Sync;desc=Update issue status when task state changes"`
	StatusMapping      map[string]string `yaml:"status_mapping,omitempty" json:"status_mapping,omitempty" schema:"label=Status Mapping;desc=Map kvelmo states to Linear statuses;type=keyvalue"`
}

// JiraConfig configures the Jira provider.
type JiraConfig struct {
	Token              string            `yaml:"-" json:"token,omitempty" schema:"label=API Token;desc=Jira API token;sensitive;env=JIRA_TOKEN;helpUrl=https://id.atlassian.com/manage-profile/security/api-tokens"`
	Email              string            `yaml:"email,omitempty" json:"email,omitempty" schema:"label=Email;desc=Jira account email for API authentication"`
	BaseURL            string            `yaml:"base_url,omitempty" json:"base_url,omitempty" schema:"label=Base URL;desc=Jira instance URL;placeholder=https://yoursite.atlassian.net"`
	AllowTicketComment bool              `yaml:"allow_ticket_comment,omitempty" json:"allow_ticket_comment,omitempty" schema:"label=Allow Ticket Comments;desc=Post status comments on Jira issues"`
	StatusSync         bool              `yaml:"status_sync,omitempty" json:"status_sync,omitempty" schema:"label=Status Sync;desc=Update issue status when task state changes"`
	StatusMapping      map[string]string `yaml:"status_mapping,omitempty" json:"status_mapping,omitempty" schema:"label=Status Mapping;desc=Map kvelmo states to Jira transitions;type=keyvalue"`
}

// GitSettings configures git behavior for the workflow.
// Pointer bools allow project-level false to override global-level true.
type GitSettings struct {
	BaseBranch              string      `yaml:"base_branch,omitempty" json:"base_branch,omitempty" schema:"label=Base Branch;desc=Default branch for PRs (auto-detected from git remote if empty);placeholder=auto-detect"`
	BranchPattern           string      `yaml:"branch_pattern,omitempty" json:"branch_pattern,omitempty" schema:"label=Branch Pattern;desc=Pattern for branch names. Variables: {key}, {type}, {slug};default=feature/{key}--{slug}"`
	CommitPrefix            string      `yaml:"commit_prefix,omitempty" json:"commit_prefix,omitempty" schema:"label=Commit Prefix;desc=Pattern for commit messages. Variables: {key};default=[{key}]"`
	CommitPattern           string      `yaml:"commit_pattern,omitempty" json:"commit_pattern,omitempty" schema:"label=Commit Pattern;desc=Regex to validate commit messages. Leave empty to skip validation;placeholder=^(feat|fix|chore)\\(.*\\):.*;advanced"`
	PRTitlePattern          string      `yaml:"pr_title_pattern,omitempty" json:"pr_title_pattern,omitempty" schema:"label=PR Title Pattern;desc=Template for PR titles. Variables: {title}, {key}, {type}, {slug};default=[{key}] {title}"`
	PRRequiredSections      []string    `yaml:"pr_required_sections,omitempty" json:"pr_required_sections,omitempty" schema:"label=PR Required Sections;desc=PR template section keywords that must be filled before submission (e.g. summary, test plan);type=tags;advanced"`
	BranchValidationPattern string      `yaml:"branch_validation_pattern,omitempty" json:"branch_validation_pattern,omitempty" schema:"label=Branch Validation;desc=Regex to validate generated branch names. Leave empty to skip validation;advanced"`
	CreateBranch            *bool       `yaml:"create_branch,omitempty" json:"create_branch,omitempty" schema:"label=Create Branch;desc=Automatically create a branch when starting a task. If the branch already exists, switches to it;default=true"`
	AutoCommit              *bool       `yaml:"auto_commit,omitempty" json:"auto_commit,omitempty" schema:"label=Auto Commit;desc=Automatically commit after implementation;default=true"`
	SignCommits             *bool       `yaml:"sign_commits,omitempty" json:"sign_commits,omitempty" schema:"label=Sign Commits;desc=GPG sign commits;showWhen=git.auto_commit:true"`
	AllowPRComment          *bool       `yaml:"allow_pr_comment,omitempty" json:"allow_pr_comment,omitempty" schema:"label=Allow PR Comments;desc=Post status comments on pull requests after submit"`
	PRCustomSections        []PRSection `yaml:"pr_custom_sections,omitempty" json:"pr_custom_sections,omitempty" schema:"label=Custom PR Sections;desc=Additional sections to include in auto-generated PR body;advanced"`
}

// PRSection defines a custom section to include in auto-generated PR descriptions.
type PRSection struct {
	Title   string `yaml:"title" json:"title" schema:"label=Section Title;required"`
	Content string `yaml:"content,omitempty" json:"content,omitempty" schema:"label=Default Content;desc=Default content or instructions for this section"`
}

// WorkerSettings configures the worker pool.
type WorkerSettings struct {
	Max int `yaml:"max,omitempty" json:"max,omitempty" schema:"label=Max Workers;desc=Maximum concurrent workers;default=3;min=1;max=10"`
}

// StorageSettings configures where specs, plans, reviews, and chat are stored.
type StorageSettings struct {
	SaveInProject  *bool                  `yaml:"save_in_project,omitempty" json:"save_in_project,omitempty" schema:"label=Save in Project;desc=Store specs/plans/chat in .valksor/ instead of home (~/.valksor/kvelmo/);default=false"`
	SpecOutputPath string                 `yaml:"spec_output_path,omitempty" json:"spec_output_path,omitempty" schema:"label=Spec Output Path;desc=Write specs to this repo path. Variables: {key}, {slug}. Example: docs/specs/{key}.md;advanced"`
	PlanOutputPath string                 `yaml:"plan_output_path,omitempty" json:"plan_output_path,omitempty" schema:"label=Plan Output Path;desc=Write plans to this repo path. Variables: {key}, {slug}. Example: docs/plans/{key}.md;advanced"`
	CommitSpecs    *bool                  `yaml:"commit_specs,omitempty" json:"commit_specs,omitempty" schema:"label=Commit Specs to Git;desc=Auto-commit specs and plans to git when written to repo output paths;default=false;advanced"`
	ChangelogPath  string                 `yaml:"changelog_path,omitempty" json:"changelog_path,omitempty" schema:"label=Changelog Path;desc=Path to CHANGELOG.md for auto-generated entries. Empty to disable;default=;placeholder=CHANGELOG.md;advanced"`
	Recording      RecordingSettings      `yaml:"recording,omitempty" json:"recording,omitempty"`
	ActivityLog    ActivityLogSettings    `yaml:"activity_log,omitempty" json:"activity_log,omitempty"`
	MetricsHistory MetricsHistorySettings `yaml:"metrics_history,omitempty" json:"metrics_history,omitempty"`
}

// ActivityLogSettings configures the RPC activity log.
type ActivityLogSettings struct {
	Enabled     bool   `yaml:"enabled,omitempty" json:"enabled,omitempty" schema:"label=Enable Activity Log;desc=Log all RPC calls to JSONL files for debugging and audit;default=false"`
	MaxFiles    int    `yaml:"max_files,omitempty" json:"max_files,omitempty" schema:"label=Max Log Files;desc=Maximum number of daily log files to retain;default=30;min=1;max=365;advanced"`
	ForwardURL  string `yaml:"forward_url,omitempty" json:"forward_url,omitempty" schema:"label=Forward URL;desc=Webhook URL to forward activity entries to (e.g. SIEM endpoint);advanced"`
	ForwardAuth string `yaml:"-" json:"forward_auth,omitempty" schema:"label=Forward Auth;desc=Authorization header value for forwarding;sensitive;env=KVELMO_LOG_FORWARD_AUTH"`
}

// MetricsHistorySettings configures time-series metrics storage.
type MetricsHistorySettings struct {
	Enabled       bool `yaml:"enabled,omitempty" json:"enabled,omitempty" schema:"label=Enable Metrics History;desc=Store periodic metric snapshots for trend analysis;default=false"`
	IntervalMin   int  `yaml:"interval_min,omitempty" json:"interval_min,omitempty" schema:"label=Interval (min);desc=Minutes between metric snapshots;default=5;min=1;max=60;advanced"`
	RetentionDays int  `yaml:"retention_days,omitempty" json:"retention_days,omitempty" schema:"label=Retention (days);desc=Days to keep historical metrics;default=90;min=1;max=365;advanced"`
}

// RecordingSettings configures agent interaction recording.
type RecordingSettings struct {
	Enabled bool   `yaml:"enabled,omitempty" json:"enabled,omitempty" schema:"label=Enable Recording;desc=Record agent interactions to JSONL files for debugging and replay;default=false"`
	Dir     string `yaml:"dir,omitempty" json:"dir,omitempty" schema:"label=Recording Directory;desc=Directory for recording files (default: ~/.valksor/kvelmo/recordings)"`
}

// ExternalReviewMode controls when the external review tool runs in the quality gate.
type ExternalReviewMode string

const (
	ExternalReviewAsk    ExternalReviewMode = "ask"    // Prompt user before running (default)
	ExternalReviewAlways ExternalReviewMode = "always" // Always run without prompting
	ExternalReviewNever  ExternalReviewMode = "never"  // Skip external review entirely
)

// ExternalReviewConfig configures an external CLI review tool in the quality gate.
type ExternalReviewConfig struct {
	Mode    ExternalReviewMode `yaml:"mode,omitempty" json:"mode,omitempty" schema:"label=Mode;desc=When to run external review tool;options=ask|always|never;default=ask"`
	Command string             `yaml:"command,omitempty" json:"command,omitempty" schema:"label=Command;desc=CLI command to run for review (must accept 'review' subcommand);default=coderabbit"`
}

// PolicySettings configures workflow enforcement guardrails.
type PolicySettings struct {
	RequiredPhases       []string         `yaml:"required_phases,omitempty" json:"required_phases,omitempty" schema:"label=Required Phases;desc=Workflow phases that cannot be skipped (e.g. review, simplify);type=tags"`
	SensitivePaths       []string         `yaml:"sensitive_paths,omitempty" json:"sensitive_paths,omitempty" schema:"label=Sensitive Paths;desc=Glob patterns for files requiring mandatory review (e.g. pkg/auth/*);type=tags"`
	MinSpecSections      int              `yaml:"min_spec_sections,omitempty" json:"min_spec_sections,omitempty" schema:"label=Min Specifications;desc=Minimum specification files required before implementation;default=0;min=0;max=10"`
	RequireSecurityScan  bool             `yaml:"require_security_scan,omitempty" json:"require_security_scan,omitempty" schema:"label=Require Security Scan;desc=Block submission when security findings exist;default=false"`
	ApprovalRequired     map[string]bool  `yaml:"approval_required,omitempty" json:"approval_required,omitempty" schema:"label=Approval Required;desc=Transitions requiring explicit human approval (e.g. submit: true);type=keyvalue"`
	ReviewChecklist      []string         `yaml:"review_checklist,omitempty" json:"review_checklist,omitempty" schema:"label=Review Checklist;desc=Items that must be checked before submit completes (e.g. security, performance);type=tags"`
	RequireSignedCommits bool             `yaml:"require_signed_commits,omitempty" json:"require_signed_commits,omitempty" schema:"label=Require Signed Commits;desc=Block workflow if GPG commit signing is not enabled;default=false"`
	DocRequirements      []DocRequirement `yaml:"doc_requirements,omitempty" json:"doc_requirements,omitempty"`
}

// DocRequirement defines a rule: when files matching Trigger change, files matching Requires must also change.
type DocRequirement struct {
	Trigger  string `yaml:"trigger" json:"trigger" schema:"label=Trigger Pattern;desc=Glob pattern for source files;required"`
	Requires string `yaml:"requires" json:"requires" schema:"label=Required Pattern;desc=Glob pattern for documentation files that must also change;required"`
}

// RetrySettings configures automatic retry on agent failure.
type RetrySettings struct {
	MaxAttempts    int `yaml:"max_attempts,omitempty" json:"max_attempts,omitempty" schema:"label=Max Attempts;desc=Maximum retry attempts on agent failure (0 = no retry);default=0;min=0;max=5"`
	BackoffSeconds int `yaml:"backoff_seconds,omitempty" json:"backoff_seconds,omitempty" schema:"label=Backoff (sec);desc=Seconds between retry attempts;default=5;min=1;max=60;advanced"`
}

// CISettings configures CI pipeline status watching after submit.
type CISettings struct {
	WatchEnabled    bool `yaml:"watch_enabled,omitempty" json:"watch_enabled,omitempty" schema:"label=Watch CI;desc=Poll CI status after PR submission;default=false"`
	PollIntervalSec int  `yaml:"poll_interval_sec,omitempty" json:"poll_interval_sec,omitempty" schema:"label=Poll Interval (sec);desc=Seconds between CI status polls;default=30;min=10;max=300;advanced"`
	AutoFix         bool `yaml:"auto_fix,omitempty" json:"auto_fix,omitempty" schema:"label=Auto Fix CI;desc=Automatically attempt to fix CI failures after PR submission. When enabled, spawns an agent to fix failures, commits changes, and pushes to the PR branch;default=false"`
	MaxFixAttempts  int  `yaml:"max_fix_attempts,omitempty" json:"max_fix_attempts,omitempty" schema:"label=Max Fix Attempts;desc=Maximum CI fix attempts before giving up;default=3;min=1;max=10;advanced"`
}

// TransitionHook is a shell command that runs before a workflow transition.
// If Required is true, the hook must succeed (exit 0) for the transition to proceed.
type TransitionHook struct {
	Command     string `yaml:"command" json:"command" schema:"label=Command;desc=Shell command to execute;required"`
	Description string `yaml:"description,omitempty" json:"description,omitempty" schema:"label=Description;desc=Human-readable description of what this hook does"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty" schema:"label=Required;desc=Block the transition if hook fails;default=false"`
}

// HooksSettings maps transition event names to shell commands that run before the transition.
// Keys are prefixed with "pre_" + event name (e.g., "pre_submit", "pre_implement").
type HooksSettings map[string][]TransitionHook

// WorkflowSettings contains per-project workflow options.
// These are intentionally project-scoped and not meaningful at global level.
type WorkflowSettings struct {
	UseWorktreeIsolation  *bool                         `yaml:"use_worktree_isolation,omitempty" json:"use_worktree_isolation,omitempty" schema:"label=Use Worktree Isolation;desc=Create an isolated git worktree for each task, enabling parallel work without conflicts;default=true"`
	AutoAdvance           *bool                         `yaml:"auto_advance,omitempty" json:"auto_advance,omitempty" schema:"label=Auto Advance;desc=Automatically progress through plan, implement, and review phases;default=false"`
	SkipPhases            []string                      `yaml:"skip_phases,omitempty" json:"skip_phases,omitempty" schema:"label=Skip Phases;desc=Phases to skip by default when auto-advancing (simplify, optimize, plan);type=tags"`
	ExternalReview        ExternalReviewConfig          `yaml:"external_review,omitempty" json:"external_review,omitempty" schema:"label=External Review;desc=External CLI review tool integration"`
	Policy                PolicySettings                `yaml:"policy,omitempty" json:"policy,omitempty"`
	Retry                 RetrySettings                 `yaml:"retry,omitempty" json:"retry,omitempty"`
	PhasePolicies         map[string]string             `yaml:"phase_policies,omitempty" json:"phase_policies,omitempty" schema:"label=Phase Policies;desc=Per-phase failure policy overrides: fail, retry, or skip (e.g., simplify: skip)"`
	CI                    CISettings                    `yaml:"ci,omitempty" json:"ci,omitempty"`
	Hooks                 HooksSettings                 `yaml:"hooks,omitempty" json:"hooks,omitempty"`
	HoldTheLine           *bool                         `yaml:"hold_the_line,omitempty" json:"hold_the_line,omitempty" schema:"label=Hold the Line;desc=Only gate on findings in lines changed by the agent, ignoring pre-existing tech debt;default=true"`
	FailureClassification FailureClassificationSettings `yaml:"failure_classification,omitempty" json:"failure_classification,omitempty"`
	AutoFix               AutoFixSettings               `yaml:"auto_fix,omitempty" json:"auto_fix,omitempty"`
	ProgressEstimation    *bool                         `yaml:"progress_estimation,omitempty" json:"progress_estimation,omitempty" schema:"label=Progress Estimation;desc=Show estimated progress and ETA during active phases;default=true"`
	PhaseGuardrails       map[string]GuardrailConfig    `yaml:"phase_guardrails,omitempty" json:"phase_guardrails,omitempty" schema:"label=Phase Guardrails;desc=Pre/post validation checks per phase;type=json;advanced"`
	AdversarialReview     *AdversarialReviewSettings    `yaml:"adversarial_review,omitempty" json:"adversarial_review,omitempty"`
	Forking               *ForkingSettings              `yaml:"forking,omitempty" json:"forking,omitempty"`
}

// AdversarialReviewSettings configures persona-based adversarial code review.
type AdversarialReviewSettings struct {
	Enabled         bool     `yaml:"enabled,omitempty" json:"enabled,omitempty" schema:"label=Adversarial Review;desc=Run persona-based adversarial reviews during review phase;default=false"`
	Personas        []string `yaml:"personas,omitempty" json:"personas,omitempty" schema:"label=Personas;desc=Review personas to use (security, performance, maintainability);type=tags;default=security,performance"`
	Agent           string   `yaml:"agent,omitempty" json:"agent,omitempty" schema:"label=Review Agent;desc=Agent to use for adversarial reviews (empty = default review agent)"`
	BlockOnFindings bool     `yaml:"block_on_findings,omitempty" json:"block_on_findings,omitempty" schema:"label=Block on Findings;desc=Block review phase if adversarial review finds issues;default=false"`
}

// ForkingSettings configures conversation forking for parallel alternatives.
type ForkingSettings struct {
	Enabled  bool `yaml:"enabled,omitempty" json:"enabled,omitempty" schema:"label=Forking;desc=Allow forking tasks into parallel alternative approaches;default=false"`
	MaxForks int  `yaml:"max_forks,omitempty" json:"max_forks,omitempty" schema:"label=Max Forks;desc=Maximum number of parallel forks per task;default=3;min=1;max=10"`
}

// GuardrailConfig defines pre and post guardrails for a workflow phase.
type GuardrailConfig struct {
	Pre  []string `yaml:"pre,omitempty" json:"pre,omitempty"`
	Post []string `yaml:"post,omitempty" json:"post,omitempty"`
}

// FailureClassificationSettings configures failure pattern detection.
type FailureClassificationSettings struct {
	Enabled       bool `yaml:"enabled,omitempty" json:"enabled,omitempty" schema:"label=Failure Classification;desc=Classify quality gate findings as flaky or genuine based on error patterns;default=false"`
	FlakyRetries  int  `yaml:"flaky_retries,omitempty" json:"flaky_retries,omitempty" schema:"label=Flaky Retries;desc=Number of retries for findings classified as flaky;default=1;min=0;max=5"`
	HistoryWindow int  `yaml:"history_window,omitempty" json:"history_window,omitempty" schema:"label=History Window;desc=Number of recent runs to check for flaky patterns;default=10;min=5;max=100;advanced"`
}

// AutoFixSettings configures automatic quality gate fix attempts.
// When enabled, quality gate failures are fed back to the agent for correction
// in a bounded retry loop, similar to the CI auto-fix loop.
type AutoFixSettings struct {
	Enabled     bool     `yaml:"enabled,omitempty" json:"enabled,omitempty" schema:"label=Auto Fix Quality;desc=Automatically feed quality gate errors back to the agent for correction;default=false"`
	MaxAttempts int      `yaml:"max_attempts,omitempty" json:"max_attempts,omitempty" schema:"label=Max Attempts;desc=Maximum auto-fix attempts before giving up;default=3;min=1;max=10"`
	Phases      []string `yaml:"phases,omitempty" json:"phases,omitempty" schema:"label=Phases;desc=Phases that trigger auto-fix on quality gate failure;type=tags;default=implement,simplify,optimize"`
}

// SecuritySettings configures agent security controls.
type SecuritySettings struct {
	CanaryEnabled    bool     `yaml:"canary_enabled,omitempty" json:"canary_enabled,omitempty" schema:"label=Canary Sandboxing;desc=Seed fake credentials in agent HOME to detect unauthorized file access;default=false"`
	RedactionEnabled *bool    `yaml:"redaction_enabled,omitempty" json:"redaction_enabled,omitempty" schema:"label=Secret Redaction;desc=Automatically redact secrets from content sent to AI agents;default=true"`
	RedactionExtra   []string `yaml:"redaction_extra,omitempty" json:"redaction_extra,omitempty" schema:"label=Extra Redaction Patterns;desc=Additional regex patterns to redact;type=tags;advanced"`
}

// WatchdogSettings configures the memory leak watchdog.
type WatchdogSettings struct {
	Enabled     bool `yaml:"enabled,omitempty" json:"enabled,omitempty" schema:"label=Enable Watchdog;desc=Monitor heap growth and trigger graceful shutdown on confirmed memory leak;default=true"`
	IntervalSec int  `yaml:"interval_sec,omitempty" json:"interval_sec,omitempty" schema:"label=Interval (sec);desc=How often to sample heap usage (min 10s — ReadMemStats stops the world briefly);default=30;min=10;max=300;advanced"`
	WindowSize  int  `yaml:"window_size,omitempty" json:"window_size,omitempty" schema:"label=Window Size;desc=Number of consecutive samples required to confirm a leak;default=10;min=5;max=60;advanced"`
	ThresholdMB int  `yaml:"threshold_mb,omitempty" json:"threshold_mb,omitempty" schema:"label=Threshold (MB);desc=Total heap growth over the window before triggering;default=200;min=50;advanced"`
}

// NotifySettings configures webhook notifications.
type NotifySettings struct {
	Enabled   *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty" schema:"label=Enable Notifications;desc=Send webhook notifications on task state changes and failures;default=false"`
	Silent    bool              `yaml:"silent,omitempty" json:"silent,omitempty" schema:"label=Silent Mode;desc=Mute all notifications (webhooks, desktop, terminal bell);default=false"`
	Webhooks  []WebhookEndpoint `yaml:"webhooks,omitempty" json:"webhooks,omitempty" schema:"label=Webhook Endpoints;desc=HTTP endpoints to receive notifications;type=tags"`
	OnFailure bool              `yaml:"on_failure,omitempty" json:"on_failure,omitempty" schema:"label=Always Notify Failures;desc=Send failure notifications regardless of event filter;default=true"`
	Terminal  *bool             `yaml:"terminal,omitempty" json:"terminal,omitempty" schema:"label=Terminal Bell;desc=Ring terminal bell when long-running operations complete or fail;default=true"`
	Desktop   bool              `yaml:"desktop,omitempty" json:"desktop,omitempty" schema:"label=Desktop Notifications;desc=Show native desktop notifications (macOS only);default=false"`
	Events    []string          `yaml:"events,omitempty" json:"events,omitempty" schema:"label=Notification Events;desc=Event types for desktop and terminal notifications (empty = all). Values: planned, implemented, failed, submitted, finished;type=tags"`
}

// WebhookEndpoint configures a single webhook destination.
type WebhookEndpoint struct {
	URL    string   `yaml:"url" json:"url" schema:"label=URL;desc=Webhook endpoint URL;required"`
	Format string   `yaml:"format,omitempty" json:"format,omitempty" schema:"label=Format;desc=Payload format;options=generic|slack;default=generic"`
	Events []string `yaml:"events,omitempty" json:"events,omitempty" schema:"label=Events;desc=Event types to send (empty = all);type=tags"`
}

// Scope represents where settings are stored/loaded from.
type Scope string

const (
	// ScopeGlobal refers to ~/.valksor/kvelmo/kvelmo.yaml.
	ScopeGlobal Scope = "global"
	// ScopeProject refers to .valksor/kvelmo.yaml.
	ScopeProject Scope = "project"
)

// SettingsResponse is returned by the settings.get socket handler.
type SettingsResponse struct {
	Schema    *Schema   `json:"schema"`    // Generated schema for UI rendering
	Effective *Settings `json:"effective"` // Merged global + project settings
	Global    *Settings `json:"global"`    // Global-only values
	Project   *Settings `json:"project"`   // Project-only overrides
}

// SettingsSetParams is the input for the settings.set socket handler.
type SettingsSetParams struct {
	Scope  Scope          `json:"scope"`  // "global" or "project"
	Values map[string]any `json:"values"` // Dot-notation paths to values
}

// Schema types for UI generation

// FieldType represents the type of a schema field for UI rendering.
type FieldType string

const (
	TypeString   FieldType = "string"
	TypeBoolean  FieldType = "boolean"
	TypeNumber   FieldType = "number"
	TypeSelect   FieldType = "select"
	TypeTextarea FieldType = "textarea"
	TypePassword FieldType = "password"
	TypeTags     FieldType = "tags"     // For []string fields
	TypeKeyValue FieldType = "keyvalue" // For map[string]string fields
	TypeList     FieldType = "list"     // For dynamic lists (e.g., custom_agents)
)

// SelectOption represents a choice in a select/dropdown field.
type SelectOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ValidationRules defines validation constraints for a field.
type ValidationRules struct {
	Required       bool   `json:"required,omitempty"`
	Min            *int   `json:"min,omitempty"`
	Max            *int   `json:"max,omitempty"`
	MaxLength      *int   `json:"maxLength,omitempty"`
	Pattern        string `json:"pattern,omitempty"`
	PatternMessage string `json:"patternMessage,omitempty"`
}

// Condition defines when a field should be visible.
type Condition struct {
	Field     string `json:"field"`               // Path to the controlling field
	Equals    any    `json:"equals,omitempty"`    // Show when field equals this value
	NotEquals any    `json:"notEquals,omitempty"` // Show when field does not equal this value
}

// Field represents a single configuration field in the schema.
type Field struct {
	Path        string           `json:"path"`                  // Dot-notation path: "git.commit_prefix"
	Type        FieldType        `json:"type"`                  // UI field type
	Label       string           `json:"label"`                 // Human-readable label
	Description string           `json:"description,omitempty"` // Help text
	Placeholder string           `json:"placeholder,omitempty"` // Input placeholder
	Default     any              `json:"default,omitempty"`     // Default value
	Options     []SelectOption   `json:"options,omitempty"`     // For select/multiselect types
	Multiple    bool             `json:"multiple,omitempty"`    // True for multiselect fields (renders checkboxes)
	ItemSchema  []Field          `json:"itemSchema,omitempty"`  // For list type - schema of each list item
	Validation  *ValidationRules `json:"validation,omitempty"`  // Validation constraints
	Sensitive   bool             `json:"sensitive,omitempty"`   // Mask in UI, protect in API
	EnvVar      string           `json:"envVar,omitempty"`      // Environment variable name for sensitive fields
	HelpURL     string           `json:"helpUrl,omitempty"`     // Link to help page (e.g., token setup)
	ShowWhen    *Condition       `json:"showWhen,omitempty"`    // Conditional visibility
	Advanced    bool             `json:"advanced,omitempty"`    // Hide in simple mode
}

// Section groups related fields together in the UI.
type Section struct {
	ID          string  `json:"id"`                    // Unique section identifier
	Title       string  `json:"title"`                 // Display title
	Description string  `json:"description,omitempty"` // Section description
	Icon        string  `json:"icon,omitempty"`        // Icon name (lucide-react)
	Category    string  `json:"category"`              // "core" | "providers" | "features"
	Fields      []Field `json:"fields"`                // Fields in this section
}

// Schema represents the complete settings schema for UI generation.
type Schema struct {
	Version  string    `json:"version"`  // Schema version for compatibility
	Sections []Section `json:"sections"` // All settings sections
}

// SectionMeta holds metadata for a section that is not defined in struct tags.
type SectionMeta struct {
	Title       string
	Description string
	Icon        string
	Category    string // "core" | "providers" | "features"
}
