package settings

import (
	"fmt"
	"log/slog"
	"maps"
	"os"
	"strconv"
)

// Merge merges src into dst. Non-zero values in src override dst.
// This is a shallow merge for top-level fields, but preserves
// existing values in dst that are not set in src.
func Merge(dst, src *Settings) {
	if src == nil {
		return
	}

	// Agent settings
	if src.Agent.Default != "" {
		dst.Agent.Default = src.Agent.Default
	}
	if len(src.Agent.Allowed) > 0 {
		dst.Agent.Allowed = src.Agent.Allowed
	}

	// Provider settings
	if src.Providers.Default != "" {
		dst.Providers.Default = src.Providers.Default
	}
	mergeGitHubConfig(&dst.Providers.GitHub, &src.Providers.GitHub)
	mergeGitLabConfig(&dst.Providers.GitLab, &src.Providers.GitLab)
	mergeWrikeConfig(&dst.Providers.Wrike, &src.Providers.Wrike)
	mergeLinearConfig(&dst.Providers.Linear, &src.Providers.Linear)
	mergeJiraConfig(&dst.Providers.Jira, &src.Providers.Jira)
	mergeAzureDevOpsConfig(&dst.Providers.AzureDevOps, &src.Providers.AzureDevOps)

	// Git settings
	if src.Git.BaseBranch != "" {
		dst.Git.BaseBranch = src.Git.BaseBranch
	}
	if src.Git.BranchPattern != "" {
		dst.Git.BranchPattern = src.Git.BranchPattern
	}
	if src.Git.CommitPrefix != "" {
		dst.Git.CommitPrefix = src.Git.CommitPrefix
	}
	if src.Git.CommitPattern != "" {
		dst.Git.CommitPattern = src.Git.CommitPattern
	}
	if src.Git.PRTitlePattern != "" {
		dst.Git.PRTitlePattern = src.Git.PRTitlePattern
	}
	if len(src.Git.PRRequiredSections) > 0 {
		dst.Git.PRRequiredSections = src.Git.PRRequiredSections
	}
	if src.Git.BranchValidationPattern != "" {
		dst.Git.BranchValidationPattern = src.Git.BranchValidationPattern
	}
	// Pointer bools: non-nil means explicitly set (allows false to override true)
	if src.Git.CreateBranch != nil {
		dst.Git.CreateBranch = src.Git.CreateBranch
	}
	if src.Git.AutoCommit != nil {
		dst.Git.AutoCommit = src.Git.AutoCommit
	}
	if src.Git.SignCommits != nil {
		dst.Git.SignCommits = src.Git.SignCommits
	}
	if src.Git.AllowPRComment != nil {
		dst.Git.AllowPRComment = src.Git.AllowPRComment
	}

	// Workers settings
	if src.Workers.Max > 0 {
		dst.Workers.Max = src.Workers.Max
	}

	// Storage settings
	if src.Storage.SaveInProject != nil {
		dst.Storage.SaveInProject = src.Storage.SaveInProject
	}
	if src.Storage.SpecOutputPath != "" {
		dst.Storage.SpecOutputPath = src.Storage.SpecOutputPath
	}
	if src.Storage.PlanOutputPath != "" {
		dst.Storage.PlanOutputPath = src.Storage.PlanOutputPath
	}
	if src.Storage.CommitSpecs != nil {
		dst.Storage.CommitSpecs = src.Storage.CommitSpecs
	}
	if src.Storage.ChangelogPath != "" {
		dst.Storage.ChangelogPath = src.Storage.ChangelogPath
	}

	// Workflow settings
	if src.Workflow.UseWorktreeIsolation != nil {
		dst.Workflow.UseWorktreeIsolation = src.Workflow.UseWorktreeIsolation
	}
	if src.Workflow.ExternalReview.Mode != "" {
		dst.Workflow.ExternalReview.Mode = src.Workflow.ExternalReview.Mode
	}
	if src.Workflow.ExternalReview.Command != "" {
		dst.Workflow.ExternalReview.Command = src.Workflow.ExternalReview.Command
	}
	if src.Workflow.HoldTheLine != nil {
		dst.Workflow.HoldTheLine = src.Workflow.HoldTheLine
	}

	// Policy settings
	if len(src.Workflow.Policy.RequiredPhases) > 0 {
		dst.Workflow.Policy.RequiredPhases = src.Workflow.Policy.RequiredPhases
	}
	if len(src.Workflow.Policy.SensitivePaths) > 0 {
		dst.Workflow.Policy.SensitivePaths = src.Workflow.Policy.SensitivePaths
	}
	if src.Workflow.Policy.MinSpecSections > 0 {
		dst.Workflow.Policy.MinSpecSections = src.Workflow.Policy.MinSpecSections
	}
	if src.Workflow.Policy.RequireSecurityScan {
		dst.Workflow.Policy.RequireSecurityScan = true
	}
	if len(src.Workflow.Policy.ApprovalRequired) > 0 {
		if dst.Workflow.Policy.ApprovalRequired == nil {
			dst.Workflow.Policy.ApprovalRequired = make(map[string]bool)
		}
		maps.Copy(dst.Workflow.Policy.ApprovalRequired, src.Workflow.Policy.ApprovalRequired)
	}
	if len(src.Workflow.Policy.ReviewChecklist) > 0 {
		dst.Workflow.Policy.ReviewChecklist = src.Workflow.Policy.ReviewChecklist
	}
	if len(src.Workflow.Policy.DocRequirements) > 0 {
		dst.Workflow.Policy.DocRequirements = src.Workflow.Policy.DocRequirements
	}

	// Phase guardrails
	if len(src.Workflow.PhaseGuardrails) > 0 {
		if dst.Workflow.PhaseGuardrails == nil {
			dst.Workflow.PhaseGuardrails = make(map[string]GuardrailConfig)
		}
		maps.Copy(dst.Workflow.PhaseGuardrails, src.Workflow.PhaseGuardrails)
	}

	// Workflow hooks
	if len(src.Workflow.Hooks) > 0 {
		if dst.Workflow.Hooks == nil {
			dst.Workflow.Hooks = make(HooksSettings)
		}
		maps.Copy(dst.Workflow.Hooks, src.Workflow.Hooks)
	}

	// Preset
	if src.Preset != "" {
		dst.Preset = src.Preset
	}

	// UI settings
	if src.UI.OnboardingDismissed {
		dst.UI.OnboardingDismissed = true
	}

	// Custom agents - merge by key
	if len(src.CustomAgents) > 0 {
		if dst.CustomAgents == nil {
			dst.CustomAgents = make(map[string]CustomAgent)
		}
		maps.Copy(dst.CustomAgents, src.CustomAgents)
	}
}

func mergeGitHubConfig(dst, src *GitHubConfig) {
	if src.Token != "" {
		dst.Token = src.Token
	}
	if src.Owner != "" {
		dst.Owner = src.Owner
	}
	if src.AllowTicketComment {
		dst.AllowTicketComment = true
	}
	if src.StatusSync {
		dst.StatusSync = true
	}
	if len(src.StatusMapping) > 0 {
		if dst.StatusMapping == nil {
			dst.StatusMapping = make(map[string]string)
		}
		maps.Copy(dst.StatusMapping, src.StatusMapping)
	}
}

func mergeGitLabConfig(dst, src *GitLabConfig) {
	if src.Token != "" {
		dst.Token = src.Token
	}
	if src.BaseURL != "" {
		dst.BaseURL = src.BaseURL
	}
	if src.AllowTicketComment {
		dst.AllowTicketComment = true
	}
	if src.StatusSync {
		dst.StatusSync = true
	}
	if len(src.StatusMapping) > 0 {
		if dst.StatusMapping == nil {
			dst.StatusMapping = make(map[string]string)
		}
		maps.Copy(dst.StatusMapping, src.StatusMapping)
	}
}

func mergeWrikeConfig(dst, src *WrikeConfig) {
	if src.Token != "" {
		dst.Token = src.Token
	}
	// Boolean fields: only override dst when src explicitly sets them to false
	// (the zero value). We use a yaml-aware approach — if src was loaded from
	// YAML and a bool field is present, it overrides. Since we can't distinguish
	// "not set" from false for plain booleans, we follow the pattern used by the
	// rest of the Merge function: only set when src is true (opt-in fields that
	// are on by default stay on unless explicitly turned off at the project level).
	//
	// For fields that default to true, we propagate a false override only if the
	// src Settings actually came from a file (non-nil). The caller is responsible
	// for passing a non-nil src only when the file was loaded.
	if src.IncludeParentContext {
		dst.IncludeParentContext = true
	}
	if src.IncludeSiblingContext {
		dst.IncludeSiblingContext = true
	}
	if src.AllowTicketComment {
		dst.AllowTicketComment = true
	}
}

func mergeLinearConfig(dst, src *LinearConfig) {
	if src.Token != "" {
		dst.Token = src.Token
	}
	if src.Team != "" {
		dst.Team = src.Team
	}
	if src.IncludeParentContext {
		dst.IncludeParentContext = true
	}
	if src.IncludeSiblingContext {
		dst.IncludeSiblingContext = true
	}
	if src.AllowTicketComment {
		dst.AllowTicketComment = true
	}
	if src.StatusSync {
		dst.StatusSync = true
	}
	if len(src.StatusMapping) > 0 {
		if dst.StatusMapping == nil {
			dst.StatusMapping = make(map[string]string)
		}
		maps.Copy(dst.StatusMapping, src.StatusMapping)
	}
}

func mergeJiraConfig(dst, src *JiraConfig) {
	if src.Token != "" {
		dst.Token = src.Token
	}
	if src.Email != "" {
		dst.Email = src.Email
	}
	if src.BaseURL != "" {
		dst.BaseURL = src.BaseURL
	}
	if src.AllowTicketComment {
		dst.AllowTicketComment = true
	}
	if src.StatusSync {
		dst.StatusSync = true
	}
	if len(src.StatusMapping) > 0 {
		if dst.StatusMapping == nil {
			dst.StatusMapping = make(map[string]string)
		}
		maps.Copy(dst.StatusMapping, src.StatusMapping)
	}
}

func mergeAzureDevOpsConfig(dst, src *AzureDevOpsConfig) {
	if src.Token != "" {
		dst.Token = src.Token
	}
	if src.BaseURL != "" {
		dst.BaseURL = src.BaseURL
	}
	if src.Organization != "" {
		dst.Organization = src.Organization
	}
	if src.Project != "" {
		dst.Project = src.Project
	}
	if src.Repository != "" {
		dst.Repository = src.Repository
	}
	if src.AllowTicketComment {
		dst.AllowTicketComment = true
	}
	if src.StatusSync {
		dst.StatusSync = true
	}
	if len(src.StatusMapping) > 0 {
		if dst.StatusMapping == nil {
			dst.StatusMapping = make(map[string]string)
		}
		maps.Copy(dst.StatusMapping, src.StatusMapping)
	}
}

// setStatusMapping is a helper for SetValue that converts a map[string]any to map[string]string.
func setStatusMapping(dst *map[string]string, value any, path string) error {
	if v, ok := value.(map[string]string); ok {
		*dst = v

		return nil
	}
	if v, ok := value.(map[string]any); ok {
		m := make(map[string]string, len(v))
		for k, val := range v {
			if s, ok := val.(string); ok {
				m[k] = s
			} else {
				return fmt.Errorf("%s.%s must be a string", path, k)
			}
		}
		*dst = m

		return nil
	}

	return fmt.Errorf("%s must be a map of string->string", path)
}

// SetValue sets a value at a dot-notation path in the settings.
// Returns an error if the path is invalid.
func SetValue(s *Settings, path string, value any) error {
	entry, ok := setterMap[path]
	if !ok {
		return fmt.Errorf("unknown path: %s", path)
	}

	return entry.set(s, value)
}

// GetValue gets a value at a dot-notation path from the settings.
func GetValue(s *Settings, path string) (any, error) {
	entry, ok := setterMap[path]
	if !ok {
		return nil, fmt.Errorf("unknown path: %s", path)
	}

	return entry.get(s), nil
}

// SensitivePaths maps setting paths to their corresponding environment variable names.
// These paths should be stored in .env files rather than settings.json.
//

var SensitivePaths = map[string]string{
	"providers.github.token":      "GITHUB_TOKEN",
	"providers.gitlab.token":      "GITLAB_TOKEN",
	"providers.wrike.token":       "WRIKE_TOKEN",
	"providers.linear.token":      "LINEAR_TOKEN",
	"providers.jira.token":        "JIRA_TOKEN",
	"providers.azuredevops.token": "AZURE_DEVOPS_TOKEN",
	"agent.anthropic.api_key":     "ANTHROPIC_API_KEY",
	"agent.openai.api_key":        "OPENAI_API_KEY",
}

// IsSensitivePath returns true if the path should be stored in .env.
func IsSensitivePath(path string) bool {
	_, ok := SensitivePaths[path]

	return ok
}

// GetEnvVarForPath returns the environment variable name for a sensitive path.
func GetEnvVarForPath(path string) string {
	return SensitivePaths[path]
}

// envOverrides maps KVELMO_* environment variable suffixes to their settings paths.
// The env var name is "KVELMO_" + the key (e.g. KVELMO_AGENT_DEFAULT → agent.default).
var envOverrides = []struct {
	envSuffix string // suffix after KVELMO_ prefix
	path      string // dot-notation settings path
}{
	// Agent
	{"AGENT_DEFAULT", KeyAgentDefault},

	// Providers
	{"PROVIDERS_DEFAULT", "providers.default"},
	{"PROVIDERS_GITHUB_OWNER", "providers.github.owner"},
	{"PROVIDERS_GITLAB_BASE_URL", "providers.gitlab.base_url"},
	{"PROVIDERS_LINEAR_TEAM", "providers.linear.team"},

	// Git
	{"GIT_BASE_BRANCH", "git.base_branch"},
	{"GIT_BRANCH_PATTERN", "git.branch_pattern"},
	{"GIT_COMMIT_PREFIX", "git.commit_prefix"},
	{"GIT_COMMIT_PATTERN", "git.commit_pattern"},
	{"GIT_PR_TITLE_PATTERN", "git.pr_title_pattern"},
	{"GIT_BRANCH_VALIDATION_PATTERN", "git.branch_validation_pattern"},
	{"GIT_CREATE_BRANCH", "git.create_branch"},
	{"GIT_AUTO_COMMIT", "git.auto_commit"},
	{"GIT_SIGN_COMMITS", "git.sign_commits"},
	{"GIT_ALLOW_PR_COMMENT", "git.allow_pr_comment"},

	// Workers
	{"WORKERS_MAX", "workers.max"},

	// Storage
	{"STORAGE_SAVE_IN_PROJECT", "storage.save_in_project"},
	{"STORAGE_SPEC_OUTPUT_PATH", "storage.spec_output_path"},
	{"STORAGE_PLAN_OUTPUT_PATH", "storage.plan_output_path"},
	{"STORAGE_COMMIT_SPECS", "storage.commit_specs"},
	{"STORAGE_CHANGELOG_PATH", "storage.changelog_path"},

	// Workflow
	{"WORKFLOW_USE_WORKTREE_ISOLATION", "workflow.use_worktree_isolation"},
	{"WORKFLOW_EXTERNAL_REVIEW_MODE", "workflow.external_review.mode"},
	{"WORKFLOW_EXTERNAL_REVIEW_COMMAND", "workflow.external_review.command"},
}

// applyEnvOverrides checks for KVELMO_* environment variables and applies them
// as overrides to the settings. This gives env vars the highest priority.
func applyEnvOverrides(s *Settings) {
	for _, ov := range envOverrides {
		val, ok := os.LookupEnv("KVELMO_" + ov.envSuffix)
		if !ok {
			continue
		}

		// Convert value to the appropriate type and apply.
		// Log errors instead of silently discarding them so misconfigured
		// env vars are visible — especially for security-sensitive fields
		// like workflow.external_review.command.
		var setErr error
		switch ov.path {
		// String fields
		case KeyAgentDefault, "providers.default", "providers.github.owner",
			"providers.gitlab.base_url", "providers.linear.team",
			"git.base_branch", "git.branch_pattern", "git.commit_prefix",
			"git.commit_pattern", "git.pr_title_pattern", "git.branch_validation_pattern",
			"storage.spec_output_path", "storage.plan_output_path", "storage.changelog_path":
			setErr = SetValue(s, ov.path, val)

		// Boolean fields
		case "git.create_branch", "git.auto_commit", "git.sign_commits",
			"git.allow_pr_comment", "storage.save_in_project", "storage.commit_specs",
			"workflow.use_worktree_isolation":
			if b, err := strconv.ParseBool(val); err == nil {
				setErr = SetValue(s, ov.path, b)
			}

		// Integer fields
		case "workers.max":
			if n, err := strconv.Atoi(val); err == nil {
				setErr = SetValue(s, ov.path, n)
			}

		// String fields (enum + freeform)
		case "workflow.external_review.mode", "workflow.external_review.command":
			setErr = SetValue(s, ov.path, val)
		}

		if setErr != nil {
			slog.Warn("env override rejected", "env", "KVELMO_"+ov.envSuffix, "path", ov.path, "error", setErr)
		}
	}
}
