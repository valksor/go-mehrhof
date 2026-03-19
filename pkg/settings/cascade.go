package settings

import (
	"slices"
	"sort"
	"sync"
)

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
	DataType     string   `json:"data_type"`    // "string", "int", "bool", "duration"
	DefaultValue any      `json:"default_value"`
	Scopes       []string `json:"scopes"`    // ["global", "project"]
	Category     string   `json:"category"`  // "agent", "quality", "workflow", "notify"
}

// DefaultDefinitions contains the known overridable settings with their metadata.
var DefaultDefinitions = []SettingDefinition{
	{Key: "agent.default", DisplayName: "Default Agent", Description: "Agent used when none specified", DataType: "string", DefaultValue: "claude", Scopes: []string{"global", "project"}, Category: "agent"},
	{Key: "agent.strategy", DisplayName: "Agent Strategy", Description: "Agent reasoning strategy", DataType: "string", DefaultValue: "direct", Scopes: []string{"global", "project"}, Category: "agent"},
	{Key: "workflow.auto_advance", DisplayName: "Auto Advance", Description: "Automatically progress through phases", DataType: "bool", DefaultValue: false, Scopes: []string{"global", "project"}, Category: "workflow"},
	{Key: "workflow.use_worktree_isolation", DisplayName: "Worktree Isolation", Description: "Create isolated git worktree for each task", DataType: "bool", DefaultValue: true, Scopes: []string{"global", "project"}, Category: "workflow"},
	{Key: "git.branch_pattern", DisplayName: "Branch Pattern", Description: "Pattern for branch names", DataType: "string", DefaultValue: "feature/{key}--{slug}", Scopes: []string{"global", "project"}, Category: "git"},
	{Key: "git.commit_prefix", DisplayName: "Commit Prefix", Description: "Pattern for commit messages", DataType: "string", DefaultValue: "[{key}]", Scopes: []string{"global", "project"}, Category: "git"},
	{Key: "git.auto_commit", DisplayName: "Auto Commit", Description: "Automatically commit after implementation", DataType: "bool", DefaultValue: true, Scopes: []string{"global", "project"}, Category: "git"},
	{Key: "git.create_branch", DisplayName: "Create Branch", Description: "Automatically create a branch when starting a task", DataType: "bool", DefaultValue: true, Scopes: []string{"global", "project"}, Category: "git"},
	{Key: "workers.max", DisplayName: "Max Workers", Description: "Maximum concurrent workers", DataType: "int", DefaultValue: 3, Scopes: []string{"global", "project"}, Category: "workers"},
	{Key: "notify.enabled", DisplayName: "Notifications Enabled", Description: "Send webhook notifications on state changes", DataType: "bool", DefaultValue: false, Scopes: []string{"global", "project"}, Category: "notify"},
	{Key: "notify.terminal", DisplayName: "Terminal Bell", Description: "Ring terminal bell on completion or failure", DataType: "bool", DefaultValue: true, Scopes: []string{"global", "project"}, Category: "notify"},
	{Key: "storage.save_in_project", DisplayName: "Save in Project", Description: "Store specs/plans/chat in project directory", DataType: "bool", DefaultValue: false, Scopes: []string{"global", "project"}, Category: "storage"},
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
	keys := make([]string, 0, len(r.definitions))
	for k := range r.definitions {
		keys = append(keys, k)
	}
	r.mu.RUnlock()

	sort.Strings(keys)

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
	// Try project scope first.
	if projectSettings != nil && slices.Contains(def.Scopes, "project") {
		if val, ok := getSettingFromConfig(key, projectSettings); ok {
			return ResolvedSetting{Key: key, Value: val, Source: "project"}
		}
	}

	// Try global scope.
	if globalSettings != nil && slices.Contains(def.Scopes, "global") {
		if val, ok := getSettingFromConfig(key, globalSettings); ok {
			return ResolvedSetting{Key: key, Value: val, Source: "global"}
		}
	}

	// Fall back to default from definition.
	return ResolvedSetting{Key: key, Value: def.DefaultValue, Source: "default"}
}

// getSettingFromConfig extracts a setting value from a Settings struct by key path.
// Returns (value, true) if the field is explicitly set (non-zero), or (nil, false) otherwise.
func getSettingFromConfig(key string, s *Settings) (any, bool) {
	switch key {
	case "agent.default":
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
	case "workflow.use_worktree_isolation":
		if s.Workflow.UseWorktreeIsolation != nil {
			return *s.Workflow.UseWorktreeIsolation, true
		}
	case "git.branch_pattern":
		if s.Git.BranchPattern != "" {
			return s.Git.BranchPattern, true
		}
	case "git.commit_prefix":
		if s.Git.CommitPrefix != "" {
			return s.Git.CommitPrefix, true
		}
	case "git.auto_commit":
		if s.Git.AutoCommit != nil {
			return *s.Git.AutoCommit, true
		}
	case "git.create_branch":
		if s.Git.CreateBranch != nil {
			return *s.Git.CreateBranch, true
		}
	case "workers.max":
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
	case "storage.save_in_project":
		if s.Storage.SaveInProject != nil {
			return *s.Storage.SaveInProject, true
		}
	}

	return nil, false
}

