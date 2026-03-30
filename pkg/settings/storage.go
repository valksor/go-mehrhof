package settings

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"

	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/paths"
	"gopkg.in/yaml.v3"
)

// GlobalPath returns the path to the global settings file.
// Uses paths.Paths().BaseDir() for test isolation support.
func GlobalPath() (string, error) {
	return filepath.Join(paths.BaseDir(), meta.ConfigFile), nil
}

// GlobalDirPath returns the path to the global settings directory.
// Uses paths.Paths().BaseDir() for test isolation support.
func GlobalDirPath() (string, error) {
	return paths.BaseDir(), nil
}

// ProjectPath returns the path to the project settings file.
// projectRoot should be the root directory of the project.
func ProjectPath(projectRoot string) string {
	return filepath.Join(projectRoot, meta.OrgDir, meta.ConfigFile)
}

// ProjectDirPath returns the path to the project settings directory.
func ProjectDirPath(projectRoot string) string {
	return filepath.Join(projectRoot, meta.OrgDir)
}

// Load loads settings from the specified path.
// Returns nil if the file doesn't exist (not an error).
func Load(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // Documented behavior: nil means file not found
		}

		return nil, fmt.Errorf("read settings: %w", err)
	}

	var s Settings
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}

	return &s, nil
}

// Save saves settings to the specified path.
// Creates parent directories if they don't exist.
func Save(path string, s *Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	return nil
}

// LoadGlobal loads settings from the global path.
func LoadGlobal() (*Settings, error) {
	path, err := GlobalPath()
	if err != nil {
		return nil, err
	}

	return Load(path)
}

// LoadProject loads settings from the project path.
func LoadProject(projectRoot string) (*Settings, error) {
	return Load(ProjectPath(projectRoot))
}

// SaveGlobal saves settings to the global path.
func SaveGlobal(s *Settings) error {
	path, err := GlobalPath()
	if err != nil {
		return err
	}

	return Save(path, s)
}

// SaveProject saves settings to the project path.
func SaveProject(projectRoot string, s *Settings) error {
	return Save(ProjectPath(projectRoot), s)
}

// FindNearestProjectConfig walks up from startDir looking for .valksor/kvelmo.yaml files.
// Returns all config paths found, ordered from deepest (most specific) to shallowest (least specific).
// Stops at stopDir (typically the git repo root). Returns nil if no configs found.
func FindNearestProjectConfig(startDir, stopDir string) []string {
	var paths []string
	dir := startDir
	stopDir = filepath.Clean(stopDir)

	for {
		candidate := ProjectPath(dir)
		if _, err := os.Stat(candidate); err == nil {
			paths = append(paths, candidate)
		}

		if filepath.Clean(dir) == stopDir {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	// Reverse so shallowest is first (gets overridden by deeper configs)
	for i, j := 0, len(paths)-1; i < j; i, j = i+1, j-1 {
		paths[i], paths[j] = paths[j], paths[i]
	}

	return paths
}

// LoadEffective loads and merges global and project settings.
// Project settings override global settings.
// Also loads and injects environment variables from .env files.
func LoadEffective(projectRoot string) (*Settings, *Settings, *Settings, error) {
	// Load .env files into an in-memory map (project overrides global)
	envMap, err := LoadEnvMap(projectRoot)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load env: %w", err)
	}

	// Load global settings
	global, err := LoadGlobal()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load global: %w", err)
	}

	// Load project settings
	project, err := LoadProject(projectRoot)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load project: %w", err)
	}

	// Start with defaults
	effective := DefaultSettings()

	// Determine which preset to use (project overrides global)
	presetName := ""
	if global != nil && global.Preset != "" {
		presetName = global.Preset
	}
	if project != nil && project.Preset != "" {
		presetName = project.Preset
	}

	// Apply preset defaults (before user settings, so user values take precedence)
	if presetName != "" {
		if preset := ApplyPreset(presetName); preset != nil {
			Merge(effective, preset)
		}
	}

	// Merge global (if exists)
	if global != nil {
		Merge(effective, global)
	}

	// Merge project (if exists, takes precedence)
	if project != nil {
		Merge(effective, project)
	}

	// Inject environment variables from .env files into sensitive fields
	InjectEnvVars(effective, envMap)

	// Apply KVELMO_* env var overrides (highest priority)
	applyEnvOverrides(effective)

	return effective, global, project, nil
}

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
	switch path {
	// Agent
	case KeyAgentDefault:
		if v, ok := value.(string); ok {
			s.Agent.Default = v

			return nil
		}

		return errors.New("agent.default must be a string")
	case "agent.allowed":
		if v, ok := value.([]string); ok {
			s.Agent.Allowed = v

			return nil
		}
		// Handle []any from JSON unmarshaling
		if arr, ok := value.([]any); ok {
			strs := make([]string, len(arr))
			for i, item := range arr {
				if str, ok := item.(string); ok {
					strs[i] = str
				} else {
					return fmt.Errorf("agent.allowed[%d] must be a string", i)
				}
			}
			s.Agent.Allowed = strs

			return nil
		}

		return errors.New("agent.allowed must be a string array")

	// Providers
	case "providers.default":
		if v, ok := value.(string); ok {
			s.Providers.Default = v

			return nil
		}

		return errors.New("providers.default must be a string")
	case "providers.github.token":
		if v, ok := value.(string); ok {
			s.Providers.GitHub.Token = v

			return nil
		}

		return errors.New("providers.github.token must be a string")
	case "providers.github.owner":
		if v, ok := value.(string); ok {
			s.Providers.GitHub.Owner = v

			return nil
		}

		return errors.New("providers.github.owner must be a string")
	case "providers.github.allow_ticket_comment":
		if v, ok := value.(bool); ok {
			s.Providers.GitHub.AllowTicketComment = v

			return nil
		}

		return errors.New("providers.github.allow_ticket_comment must be a boolean")
	case "providers.gitlab.token":
		if v, ok := value.(string); ok {
			s.Providers.GitLab.Token = v

			return nil
		}

		return errors.New("providers.gitlab.token must be a string")
	case "providers.gitlab.base_url":
		if v, ok := value.(string); ok {
			s.Providers.GitLab.BaseURL = v

			return nil
		}

		return errors.New("providers.gitlab.base_url must be a string")
	case "providers.wrike.token":
		if v, ok := value.(string); ok {
			s.Providers.Wrike.Token = v

			return nil
		}

		return errors.New("providers.wrike.token must be a string")
	case "providers.wrike.include_parent_context":
		if v, ok := value.(bool); ok {
			s.Providers.Wrike.IncludeParentContext = v

			return nil
		}

		return errors.New("providers.wrike.include_parent_context must be a boolean")
	case "providers.wrike.include_sibling_context":
		if v, ok := value.(bool); ok {
			s.Providers.Wrike.IncludeSiblingContext = v

			return nil
		}

		return errors.New("providers.wrike.include_sibling_context must be a boolean")
	case "providers.gitlab.allow_ticket_comment":
		if v, ok := value.(bool); ok {
			s.Providers.GitLab.AllowTicketComment = v

			return nil
		}

		return errors.New("providers.gitlab.allow_ticket_comment must be a boolean")
	case "providers.gitlab.status_sync":
		if v, ok := value.(bool); ok {
			s.Providers.GitLab.StatusSync = v

			return nil
		}

		return errors.New("providers.gitlab.status_sync must be a boolean")
	case "providers.gitlab.status_mapping":
		return setStatusMapping(&s.Providers.GitLab.StatusMapping, value, "providers.gitlab.status_mapping")
	case "providers.wrike.allow_ticket_comment":
		if v, ok := value.(bool); ok {
			s.Providers.Wrike.AllowTicketComment = v

			return nil
		}

		return errors.New("providers.wrike.allow_ticket_comment must be a boolean")
	case "providers.linear.token":
		if v, ok := value.(string); ok {
			s.Providers.Linear.Token = v

			return nil
		}

		return errors.New("providers.linear.token must be a string")
	case "providers.linear.team":
		if v, ok := value.(string); ok {
			s.Providers.Linear.Team = v

			return nil
		}

		return errors.New("providers.linear.team must be a string")
	case "providers.linear.include_parent_context":
		if v, ok := value.(bool); ok {
			s.Providers.Linear.IncludeParentContext = v

			return nil
		}

		return errors.New("providers.linear.include_parent_context must be a boolean")
	case "providers.linear.include_sibling_context":
		if v, ok := value.(bool); ok {
			s.Providers.Linear.IncludeSiblingContext = v

			return nil
		}

		return errors.New("providers.linear.include_sibling_context must be a boolean")
	case "providers.linear.allow_ticket_comment":
		if v, ok := value.(bool); ok {
			s.Providers.Linear.AllowTicketComment = v

			return nil
		}

		return errors.New("providers.linear.allow_ticket_comment must be a boolean")

	// Jira
	case "providers.jira.token":
		if v, ok := value.(string); ok {
			s.Providers.Jira.Token = v

			return nil
		}

		return errors.New("providers.jira.token must be a string")
	case "providers.jira.email":
		if v, ok := value.(string); ok {
			s.Providers.Jira.Email = v

			return nil
		}

		return errors.New("providers.jira.email must be a string")
	case "providers.jira.base_url":
		if v, ok := value.(string); ok {
			s.Providers.Jira.BaseURL = v

			return nil
		}

		return errors.New("providers.jira.base_url must be a string")
	case "providers.jira.allow_ticket_comment":
		if v, ok := value.(bool); ok {
			s.Providers.Jira.AllowTicketComment = v

			return nil
		}

		return errors.New("providers.jira.allow_ticket_comment must be a boolean")

	// Status sync settings
	case "providers.github.status_sync":
		if v, ok := value.(bool); ok {
			s.Providers.GitHub.StatusSync = v

			return nil
		}

		return errors.New("providers.github.status_sync must be a boolean")
	case "providers.github.status_mapping":
		return setStatusMapping(&s.Providers.GitHub.StatusMapping, value, "providers.github.status_mapping")
	case "providers.linear.status_sync":
		if v, ok := value.(bool); ok {
			s.Providers.Linear.StatusSync = v

			return nil
		}

		return errors.New("providers.linear.status_sync must be a boolean")
	case "providers.linear.status_mapping":
		return setStatusMapping(&s.Providers.Linear.StatusMapping, value, "providers.linear.status_mapping")
	case "providers.jira.status_sync":
		if v, ok := value.(bool); ok {
			s.Providers.Jira.StatusSync = v

			return nil
		}

		return errors.New("providers.jira.status_sync must be a boolean")
	case "providers.jira.status_mapping":
		return setStatusMapping(&s.Providers.Jira.StatusMapping, value, "providers.jira.status_mapping")

	// Azure DevOps
	case "providers.azuredevops.token":
		if v, ok := value.(string); ok {
			s.Providers.AzureDevOps.Token = v

			return nil
		}

		return errors.New("providers.azuredevops.token must be a string")
	case "providers.azuredevops.base_url":
		if v, ok := value.(string); ok {
			s.Providers.AzureDevOps.BaseURL = v

			return nil
		}

		return errors.New("providers.azuredevops.base_url must be a string")
	case "providers.azuredevops.organization":
		if v, ok := value.(string); ok {
			s.Providers.AzureDevOps.Organization = v

			return nil
		}

		return errors.New("providers.azuredevops.organization must be a string")
	case "providers.azuredevops.project":
		if v, ok := value.(string); ok {
			s.Providers.AzureDevOps.Project = v

			return nil
		}

		return errors.New("providers.azuredevops.project must be a string")
	case "providers.azuredevops.repository":
		if v, ok := value.(string); ok {
			s.Providers.AzureDevOps.Repository = v

			return nil
		}

		return errors.New("providers.azuredevops.repository must be a string")
	case "providers.azuredevops.allow_ticket_comment":
		if v, ok := value.(bool); ok {
			s.Providers.AzureDevOps.AllowTicketComment = v

			return nil
		}

		return errors.New("providers.azuredevops.allow_ticket_comment must be a boolean")
	case "providers.azuredevops.status_sync":
		if v, ok := value.(bool); ok {
			s.Providers.AzureDevOps.StatusSync = v

			return nil
		}

		return errors.New("providers.azuredevops.status_sync must be a boolean")
	case "providers.azuredevops.status_mapping":
		return setStatusMapping(&s.Providers.AzureDevOps.StatusMapping, value, "providers.azuredevops.status_mapping")

	// Git
	case "git.base_branch":
		if v, ok := value.(string); ok {
			s.Git.BaseBranch = v

			return nil
		}

		return errors.New("git.base_branch must be a string")
	case "git.branch_pattern":
		if v, ok := value.(string); ok {
			s.Git.BranchPattern = v

			return nil
		}

		return errors.New("git.branch_pattern must be a string")
	case "git.commit_prefix":
		if v, ok := value.(string); ok {
			s.Git.CommitPrefix = v

			return nil
		}

		return errors.New("git.commit_prefix must be a string")
	case "git.commit_pattern":
		if v, ok := value.(string); ok {
			s.Git.CommitPattern = v

			return nil
		}

		return errors.New("git.commit_pattern must be a string")
	case "git.pr_title_pattern":
		if v, ok := value.(string); ok {
			s.Git.PRTitlePattern = v

			return nil
		}

		return errors.New("git.pr_title_pattern must be a string")
	case "git.branch_validation_pattern":
		if v, ok := value.(string); ok {
			s.Git.BranchValidationPattern = v

			return nil
		}

		return errors.New("git.branch_validation_pattern must be a string")
	case "git.create_branch":
		if v, ok := value.(bool); ok {
			s.Git.CreateBranch = &v

			return nil
		}

		return errors.New("git.create_branch must be a boolean")
	case "git.auto_commit":
		if v, ok := value.(bool); ok {
			s.Git.AutoCommit = &v

			return nil
		}

		return errors.New("git.auto_commit must be a boolean")
	case "git.sign_commits":
		if v, ok := value.(bool); ok {
			s.Git.SignCommits = &v

			return nil
		}

		return errors.New("git.sign_commits must be a boolean")
	case "git.allow_pr_comment":
		if v, ok := value.(bool); ok {
			s.Git.AllowPRComment = &v

			return nil
		}

		return errors.New("git.allow_pr_comment must be a boolean")

	// Workers
	case "workers.max":
		switch v := value.(type) {
		case int:
			s.Workers.Max = v

			return nil
		case float64:
			s.Workers.Max = int(v)

			return nil
		}

		return errors.New("workers.max must be a number")

	// Storage
	case "storage.save_in_project":
		if v, ok := value.(bool); ok {
			s.Storage.SaveInProject = &v

			return nil
		}

		return errors.New("storage.save_in_project must be a boolean")
	case "storage.spec_output_path":
		if v, ok := value.(string); ok {
			s.Storage.SpecOutputPath = v

			return nil
		}

		return errors.New("storage.spec_output_path must be a string")
	case "storage.plan_output_path":
		if v, ok := value.(string); ok {
			s.Storage.PlanOutputPath = v

			return nil
		}

		return errors.New("storage.plan_output_path must be a string")
	case "storage.commit_specs":
		if v, ok := value.(bool); ok {
			s.Storage.CommitSpecs = &v

			return nil
		}

		return errors.New("storage.commit_specs must be a boolean")
	case "storage.changelog_path":
		if v, ok := value.(string); ok {
			s.Storage.ChangelogPath = v

			return nil
		}

		return errors.New("storage.changelog_path must be a string")

	// Workflow
	case "workflow.use_worktree_isolation":
		if v, ok := value.(bool); ok {
			s.Workflow.UseWorktreeIsolation = &v

			return nil
		}

		return errors.New("workflow.use_worktree_isolation must be a boolean")

	case "workflow.external_review.mode":
		if v, ok := value.(string); ok {
			mode := ExternalReviewMode(v)
			switch mode {
			case ExternalReviewAsk, ExternalReviewAlways, ExternalReviewNever:
				s.Workflow.ExternalReview.Mode = mode

				return nil
			default:
				return errors.New("workflow.external_review.mode must be one of: ask, always, never")
			}
		}

		return errors.New("workflow.external_review.mode must be a string")
	case "workflow.external_review.command":
		if v, ok := value.(string); ok {
			s.Workflow.ExternalReview.Command = v

			return nil
		}

		return errors.New("workflow.external_review.command must be a string")

	// Policy - Approval Required
	case "workflow.policy.approval_required":
		if v, ok := value.(map[string]any); ok {
			m := make(map[string]bool)
			for k, val := range v {
				if b, ok := val.(bool); ok {
					m[k] = b
				} else {
					return fmt.Errorf("workflow.policy.approval_required.%s must be a boolean", k)
				}
			}
			s.Workflow.Policy.ApprovalRequired = m

			return nil
		}

		return errors.New("workflow.policy.approval_required must be a map of event->boolean")

	// Policy - Review Checklist
	case "workflow.policy.review_checklist":
		if v, ok := value.([]string); ok {
			s.Workflow.Policy.ReviewChecklist = v

			return nil
		}
		if arr, ok := value.([]any); ok {
			strs := make([]string, len(arr))
			for i, item := range arr {
				if str, ok := item.(string); ok {
					strs[i] = str
				} else {
					return fmt.Errorf("workflow.policy.review_checklist[%d] must be a string", i)
				}
			}
			s.Workflow.Policy.ReviewChecklist = strs

			return nil
		}

		return errors.New("workflow.policy.review_checklist must be a string array")

	// UI
	case "ui.onboarding_dismissed":
		if v, ok := value.(bool); ok {
			s.UI.OnboardingDismissed = v

			return nil
		}

		return errors.New("ui.onboarding_dismissed must be a boolean")

	// Custom Agents
	case "custom_agents":
		// Expect a map of agent name -> agent config
		if v, ok := value.(map[string]any); ok {
			agents := make(map[string]CustomAgent)
			for name, agentData := range v {
				agentMap, ok := agentData.(map[string]any)
				if !ok {
					return fmt.Errorf("custom_agents.%s must be an object", name)
				}
				agent := CustomAgent{}
				if extends, ok := agentMap["extends"].(string); ok {
					agent.Extends = extends
				}
				if desc, ok := agentMap["description"].(string); ok {
					agent.Description = desc
				}
				if argsAny, ok := agentMap["args"].([]any); ok {
					args := make([]string, 0, len(argsAny))
					for _, a := range argsAny {
						if s, ok := a.(string); ok {
							args = append(args, s)
						}
					}
					agent.Args = args
				}
				if envMap, ok := agentMap["env"].(map[string]any); ok {
					env := make(map[string]string)
					for k, v := range envMap {
						if s, ok := v.(string); ok {
							env[k] = s
						}
					}
					agent.Env = env
				}
				agents[name] = agent
			}
			s.CustomAgents = agents

			return nil
		}

		return errors.New("custom_agents must be an object")

	default:
		return fmt.Errorf("unknown path: %s", path)
	}
}

// GetValue gets a value at a dot-notation path from the settings.
func GetValue(s *Settings, path string) (any, error) {
	switch path {
	// Agent
	case KeyAgentDefault:
		return s.Agent.Default, nil
	case "agent.allowed":
		return s.Agent.Allowed, nil

	// Providers
	case "providers.default":
		return s.Providers.Default, nil
	case "providers.github.token":
		return s.Providers.GitHub.Token, nil
	case "providers.github.owner":
		return s.Providers.GitHub.Owner, nil
	case "providers.github.allow_ticket_comment":
		return s.Providers.GitHub.AllowTicketComment, nil
	case "providers.gitlab.token":
		return s.Providers.GitLab.Token, nil
	case "providers.gitlab.base_url":
		return s.Providers.GitLab.BaseURL, nil
	case "providers.wrike.token":
		return s.Providers.Wrike.Token, nil
	case "providers.wrike.include_parent_context":
		return s.Providers.Wrike.IncludeParentContext, nil
	case "providers.wrike.include_sibling_context":
		return s.Providers.Wrike.IncludeSiblingContext, nil
	case "providers.gitlab.allow_ticket_comment":
		return s.Providers.GitLab.AllowTicketComment, nil
	case "providers.gitlab.status_sync":
		return s.Providers.GitLab.StatusSync, nil
	case "providers.gitlab.status_mapping":
		return s.Providers.GitLab.StatusMapping, nil
	case "providers.wrike.allow_ticket_comment":
		return s.Providers.Wrike.AllowTicketComment, nil
	case "providers.linear.token":
		return s.Providers.Linear.Token, nil
	case "providers.linear.team":
		return s.Providers.Linear.Team, nil
	case "providers.linear.include_parent_context":
		return s.Providers.Linear.IncludeParentContext, nil
	case "providers.linear.include_sibling_context":
		return s.Providers.Linear.IncludeSiblingContext, nil
	case "providers.linear.allow_ticket_comment":
		return s.Providers.Linear.AllowTicketComment, nil

	// Jira
	case "providers.jira.token":
		return s.Providers.Jira.Token, nil
	case "providers.jira.email":
		return s.Providers.Jira.Email, nil
	case "providers.jira.base_url":
		return s.Providers.Jira.BaseURL, nil
	case "providers.jira.allow_ticket_comment":
		return s.Providers.Jira.AllowTicketComment, nil

	// Status sync
	case "providers.github.status_sync":
		return s.Providers.GitHub.StatusSync, nil
	case "providers.github.status_mapping":
		return s.Providers.GitHub.StatusMapping, nil
	case "providers.linear.status_sync":
		return s.Providers.Linear.StatusSync, nil
	case "providers.linear.status_mapping":
		return s.Providers.Linear.StatusMapping, nil
	case "providers.jira.status_sync":
		return s.Providers.Jira.StatusSync, nil
	case "providers.jira.status_mapping":
		return s.Providers.Jira.StatusMapping, nil

	// Azure DevOps
	case "providers.azuredevops.token":
		return s.Providers.AzureDevOps.Token, nil
	case "providers.azuredevops.base_url":
		return s.Providers.AzureDevOps.BaseURL, nil
	case "providers.azuredevops.organization":
		return s.Providers.AzureDevOps.Organization, nil
	case "providers.azuredevops.project":
		return s.Providers.AzureDevOps.Project, nil
	case "providers.azuredevops.repository":
		return s.Providers.AzureDevOps.Repository, nil
	case "providers.azuredevops.allow_ticket_comment":
		return s.Providers.AzureDevOps.AllowTicketComment, nil
	case "providers.azuredevops.status_sync":
		return s.Providers.AzureDevOps.StatusSync, nil
	case "providers.azuredevops.status_mapping":
		return s.Providers.AzureDevOps.StatusMapping, nil

	// Git
	case "git.base_branch":
		return s.Git.BaseBranch, nil
	case "git.branch_pattern":
		return s.Git.BranchPattern, nil
	case "git.commit_prefix":
		return s.Git.CommitPrefix, nil
	case "git.commit_pattern":
		return s.Git.CommitPattern, nil
	case "git.pr_title_pattern":
		return s.Git.PRTitlePattern, nil
	case "git.branch_validation_pattern":
		return s.Git.BranchValidationPattern, nil
	case "git.create_branch":
		return BoolValue(s.Git.CreateBranch, true), nil
	case "git.auto_commit":
		return BoolValue(s.Git.AutoCommit, true), nil
	case "git.sign_commits":
		return BoolValue(s.Git.SignCommits, false), nil
	case "git.allow_pr_comment":
		return BoolValue(s.Git.AllowPRComment, false), nil

	// Workers
	case "workers.max":
		return s.Workers.Max, nil

	// Storage
	case "storage.save_in_project":
		return BoolValue(s.Storage.SaveInProject, false), nil
	case "storage.spec_output_path":
		return s.Storage.SpecOutputPath, nil
	case "storage.plan_output_path":
		return s.Storage.PlanOutputPath, nil
	case "storage.commit_specs":
		return BoolValue(s.Storage.CommitSpecs, false), nil
	case "storage.changelog_path":
		return s.Storage.ChangelogPath, nil

	// Workflow
	case "workflow.use_worktree_isolation":
		return BoolValue(s.Workflow.UseWorktreeIsolation, true), nil
	case "workflow.external_review.mode":
		if s.Workflow.ExternalReview.Mode == "" {
			return string(ExternalReviewAsk), nil
		}

		return string(s.Workflow.ExternalReview.Mode), nil
	case "workflow.external_review.command":
		if s.Workflow.ExternalReview.Command == "" {
			return "coderabbit", nil
		}

		return s.Workflow.ExternalReview.Command, nil

	// Policy
	case "workflow.policy.approval_required":
		return s.Workflow.Policy.ApprovalRequired, nil
	case "workflow.policy.review_checklist":
		return s.Workflow.Policy.ReviewChecklist, nil

	// UI
	case "ui.onboarding_dismissed":
		return s.UI.OnboardingDismissed, nil

	// Custom Agents
	case "custom_agents":
		return s.CustomAgents, nil

	default:
		return nil, fmt.Errorf("unknown path: %s", path)
	}
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

		// Convert value to the appropriate type and apply
		switch ov.path {
		// String fields
		case KeyAgentDefault, "providers.default", "providers.github.owner",
			"providers.gitlab.base_url", "providers.linear.team",
			"git.base_branch", "git.branch_pattern", "git.commit_prefix",
			"git.commit_pattern", "git.pr_title_pattern", "git.branch_validation_pattern",
			"storage.spec_output_path", "storage.plan_output_path", "storage.changelog_path":
			_ = SetValue(s, ov.path, val)

		// Boolean fields
		case "git.create_branch", "git.auto_commit", "git.sign_commits",
			"git.allow_pr_comment", "storage.save_in_project", "storage.commit_specs",
			"workflow.use_worktree_isolation":
			if b, err := strconv.ParseBool(val); err == nil {
				_ = SetValue(s, ov.path, b)
			}

		// Integer fields
		case "workers.max":
			if n, err := strconv.Atoi(val); err == nil {
				_ = SetValue(s, ov.path, n)
			}

		// String fields (enum + freeform)
		case "workflow.external_review.mode", "workflow.external_review.command":
			_ = SetValue(s, ov.path, val)
		}
	}
}
