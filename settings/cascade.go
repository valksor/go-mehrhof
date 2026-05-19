package settings

import (
	"maps"
	"slices"
	"sync"
)

// Exported setting keys. Constants for keys that are referenced from
// multiple files in the package; literal keys used in only one place
// remain inline.
const (
	KeyAgentDefault                 = "agent.default"
	KeyGitBranchPattern             = "git.branch_pattern"
	KeyGitCommitPrefix              = "git.commit_prefix"
	KeyGitAutoCommit                = "git.auto_commit"
	KeyGitCreateBranch              = "git.create_branch"
	KeyWorkersMax                   = "workers.max"
	KeyStorageSaveInProject         = "storage.save_in_project"
	KeyWorkflowUseWorktreeIsolation = "workflow.use_worktree_isolation"
)

// DataType values used in SettingDefinition.DataType.
const (
	dataTypeString = "string"
	dataTypeBool   = "bool"
	dataTypeInt    = "int"
)

// bothScopes is shared by every entry in DefaultDefinitions whose value
// can be set at both global and project scope. Centralizing it avoids
// repeating the []string{"global", "project"} literal on each row.
// MUST NOT be mutated — the slice header is shared across all entries that
// reference it, so any append/index-write would corrupt every other entry.
var bothScopes = []string{string(ScopeGlobal), string(ScopeProject)}

// ResolvedSetting holds a resolved value with its source.
type ResolvedSetting struct {
	Key    string `json:"key"`
	Value  any    `json:"value"`
	Source string `json:"source"` // "global", "project", "default"
}

// SettingDefinition describes a configurable setting.
type SettingDefinition struct {
	Key          string   `json:"key"`
	DisplayName  string   `json:"display_name"`
	Description  string   `json:"description"`
	DataType     string   `json:"data_type"` // "string", "int", "bool", "duration"
	DefaultValue any      `json:"default_value"`
	Scopes       []string `json:"scopes"`   // ["global", "project"]
	Category     string   `json:"category"` // "agent", "quality", "workflow", "notify"
}

// DefaultDefinitions contains the known overridable settings with their metadata.
var DefaultDefinitions = []SettingDefinition{
	{Key: KeyAgentDefault, DisplayName: "Default Agent", Description: "Agent used when none specified", DataType: dataTypeString, DefaultValue: "claude", Scopes: bothScopes, Category: "agent"},
	{Key: "agent.strategy", DisplayName: "Agent Strategy", Description: "Agent reasoning strategy", DataType: dataTypeString, DefaultValue: "direct", Scopes: bothScopes, Category: "agent"},
	{Key: "workflow.auto_advance", DisplayName: "Auto Advance", Description: "Automatically progress through phases", DataType: dataTypeBool, DefaultValue: false, Scopes: bothScopes, Category: "workflow"},
	{Key: KeyWorkflowUseWorktreeIsolation, DisplayName: "Worktree Isolation", Description: "Create isolated git worktree for each task", DataType: dataTypeBool, DefaultValue: true, Scopes: bothScopes, Category: "workflow"},
	{Key: KeyGitBranchPattern, DisplayName: "Branch Pattern", Description: "Pattern for branch names", DataType: dataTypeString, DefaultValue: "feature/{key}--{slug}", Scopes: bothScopes, Category: "git"},
	{Key: KeyGitCommitPrefix, DisplayName: "Commit Prefix", Description: "Pattern for commit messages", DataType: dataTypeString, DefaultValue: "[{key}]", Scopes: bothScopes, Category: "git"},
	{Key: KeyGitAutoCommit, DisplayName: "Auto Commit", Description: "Automatically commit after implementation", DataType: dataTypeBool, DefaultValue: true, Scopes: bothScopes, Category: "git"},
	{Key: KeyGitCreateBranch, DisplayName: "Create Branch", Description: "Automatically create a branch when starting a task", DataType: dataTypeBool, DefaultValue: true, Scopes: bothScopes, Category: "git"},
	{Key: KeyWorkersMax, DisplayName: "Max Workers", Description: "Maximum concurrent workers", DataType: dataTypeInt, DefaultValue: 3, Scopes: bothScopes, Category: "workers"},
	{Key: "notify.enabled", DisplayName: "Notifications Enabled", Description: "Send webhook notifications on state changes", DataType: dataTypeBool, DefaultValue: false, Scopes: bothScopes, Category: "notify"},
	{Key: "notify.terminal", DisplayName: "Terminal Bell", Description: "Ring terminal bell on completion or failure", DataType: dataTypeBool, DefaultValue: true, Scopes: bothScopes, Category: "notify"},
	{Key: KeyStorageSaveInProject, DisplayName: "Save in Project", Description: "Store specs/plans/chat in project directory", DataType: dataTypeBool, DefaultValue: false, Scopes: bothScopes, Category: "storage"},
	{Key: "workflow.hold_the_line", DisplayName: "Hold the Line", Description: "Only gate on findings in changed lines", DataType: dataTypeBool, DefaultValue: true, Scopes: bothScopes, Category: "workflow"},
	{Key: "agent.token_budget", DisplayName: "Token Budget", Description: "Max tokens per agent execution (0 = unlimited)", DataType: dataTypeInt, DefaultValue: 0, Scopes: bothScopes, Category: "agent"},
	{Key: "agent.task_token_budget", DisplayName: "Task Token Budget", Description: "Max tokens per task across all phases (0 = unlimited)", DataType: dataTypeInt, DefaultValue: 0, Scopes: bothScopes, Category: "agent"},
}

// CascadeResolver resolves settings with project -> global -> default cascade.
type CascadeResolver struct {
	definitions map[string]SettingDefinition
	mu          sync.RWMutex
}

// NewCascadeResolver creates a resolver pre-loaded with DefaultDefinitions.
func NewCascadeResolver() *CascadeResolver {
	r := &CascadeResolver{
		definitions: make(map[string]SettingDefinition, len(DefaultDefinitions)),
	}
	r.RegisterDefinitions(DefaultDefinitions...)

	return r
}

// RegisterDefinitions adds setting definitions to the resolver.
func (r *CascadeResolver) RegisterDefinitions(defs ...SettingDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, d := range defs {
		r.definitions[d.Key] = d
	}
}

// GetResolved returns the effective value for a key, checking project first, then global, then default.
// If projectSettings is nil, only global and default are considered.
// If both are nil, only the default from the definition is returned.
func (r *CascadeResolver) GetResolved(key string, projectSettings, globalSettings *Settings) ResolvedSetting {
	r.mu.RLock()
	def, known := r.definitions[key]
	r.mu.RUnlock()

	if !known {
		return ResolvedSetting{}
	}

	return r.resolve(key, def, projectSettings, globalSettings)
}

// GetAllResolved returns all registered definitions with their resolved values and sources.
func (r *CascadeResolver) GetAllResolved(projectSettings, globalSettings *Settings) []ResolvedSetting {
	r.mu.RLock()
	keys := slices.Sorted(maps.Keys(r.definitions))
	r.mu.RUnlock()

	results := make([]ResolvedSetting, 0, len(keys))
	for _, key := range keys {
		results = append(results, r.GetResolved(key, projectSettings, globalSettings))
	}

	return results
}

// InvalidateCache is a no-op retained for API compatibility.
// The resolver no longer caches results — each call resolves fresh.
func (r *CascadeResolver) InvalidateCache() {}

// resolve performs the actual cascade lookup: project -> global -> default.
func (r *CascadeResolver) resolve(key string, def SettingDefinition, projectSettings, globalSettings *Settings) ResolvedSetting {
	scopeProject := string(ScopeProject)
	scopeGlobal := string(ScopeGlobal)

	// Try project scope first.
	if projectSettings != nil && slices.Contains(def.Scopes, scopeProject) {
		if val, ok := getSettingFromConfig(key, projectSettings); ok {
			return ResolvedSetting{Key: key, Value: val, Source: scopeProject}
		}
	}

	// Try global scope.
	if globalSettings != nil && slices.Contains(def.Scopes, scopeGlobal) {
		if val, ok := getSettingFromConfig(key, globalSettings); ok {
			return ResolvedSetting{Key: key, Value: val, Source: scopeGlobal}
		}
	}

	// Fall back to default from definition.
	return ResolvedSetting{Key: key, Value: def.DefaultValue, Source: "default"}
}

// getSettingFromConfig extracts a setting value from a Settings struct by key path.
// Returns (value, true) if the field is explicitly set (non-zero), or (nil, false) otherwise.
func getSettingFromConfig(key string, s *Settings) (any, bool) {
	switch key {
	case KeyAgentDefault:
		if s.Agent.Default != "" {
			return s.Agent.Default, true
		}
	case "agent.strategy":
		if s.Agent.Strategy != "" {
			return s.Agent.Strategy, true
		}
	case "workflow.auto_advance":
		if s.Workflow.AutoAdvance != nil {
			return *s.Workflow.AutoAdvance, true
		}
	case KeyWorkflowUseWorktreeIsolation:
		if s.Workflow.UseWorktreeIsolation != nil {
			return *s.Workflow.UseWorktreeIsolation, true
		}
	case "workflow.hold_the_line":
		if s.Workflow.HoldTheLine != nil {
			return *s.Workflow.HoldTheLine, true
		}
	case KeyGitBranchPattern:
		if s.Git.BranchPattern != "" {
			return s.Git.BranchPattern, true
		}
	case KeyGitCommitPrefix:
		if s.Git.CommitPrefix != "" {
			return s.Git.CommitPrefix, true
		}
	case KeyGitAutoCommit:
		if s.Git.AutoCommit != nil {
			return *s.Git.AutoCommit, true
		}
	case KeyGitCreateBranch:
		if s.Git.CreateBranch != nil {
			return *s.Git.CreateBranch, true
		}
	case KeyWorkersMax:
		if s.Workers.Max > 0 {
			return s.Workers.Max, true
		}
	case "notify.enabled":
		if s.Notify.Enabled != nil {
			return *s.Notify.Enabled, true
		}
	case "notify.terminal":
		if s.Notify.Terminal != nil {
			return *s.Notify.Terminal, true
		}
	case KeyStorageSaveInProject:
		if s.Storage.SaveInProject != nil {
			return *s.Storage.SaveInProject, true
		}
	}

	return nil, false
}
