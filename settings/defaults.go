package settings

// BoolValue safely dereferences a *bool, returning the default if nil.
func BoolValue(p *bool, defaultVal bool) bool {
	if p == nil {
		return defaultVal
	}

	return *p
}

// DefaultSettings returns settings with sensible default values.
// These defaults are used when no settings file exists or when
// a field is not specified in either global or project settings.
func DefaultSettings() *Settings {
	return &Settings{
		Agent: AgentSettings{
			Default: "claude",
			Allowed: []string{"claude", "codex"},
		},
		Providers: ProviderSettings{
			Default: "github",
			GitHub: GitHubConfig{
				AllowTicketComment: false,
			},
			GitLab: GitLabConfig{
				BaseURL:            "https://gitlab.com",
				AllowTicketComment: false,
			},
			Wrike: WrikeConfig{
				// Hierarchy context is enabled by default for Wrike because parent
				// and sibling tasks provide essential scope for AI planning/implementation.
				IncludeParentContext:  true,
				IncludeSiblingContext: true,
				AllowTicketComment:    false,
			},
			Linear: LinearConfig{
				// Hierarchy context is enabled by default for Linear, matching Wrike behavior.
				IncludeParentContext:  true,
				IncludeSiblingContext: true,
				AllowTicketComment:    false,
			},
		},
		Git: GitSettings{
			BranchPattern:  "feature/{key}--{slug}",
			CommitPrefix:   "[{key}]",
			PRTitlePattern: "[{key}] {title}",
			CreateBranch:   new(true),
			AutoCommit:     new(true),
			SignCommits:    new(false),
			AllowPRComment: new(false),
		},
		Workers: WorkerSettings{
			Max: 3,
		},
		Storage: StorageSettings{
			SaveInProject: new(false),
		},
		Watchdog: WatchdogSettings{
			Enabled:     true,
			IntervalSec: 30,
			WindowSize:  10,
			ThresholdMB: 200,
		},
		Workflow: WorkflowSettings{
			UseWorktreeIsolation: new(true),
			ExternalReview: ExternalReviewConfig{
				Mode:    ExternalReviewAsk,
				Command: "coderabbit",
			},
		},
		CustomAgents: make(map[string]CustomAgent),
	}
}

// SectionRegistry maps section IDs to their metadata.
// This metadata is used when generating the schema for UI rendering.
var SectionRegistry = map[string]SectionMeta{
	"agent": {
		Title:       "Agent",
		Description: "AI agent configuration",
		Icon:        "bot",
		Category:    "core",
	},
	"providers": {
		Title:       "Providers",
		Description: "Task provider integrations",
		Icon:        "plug",
		Category:    "providers",
	},
	"providers.github": {
		Title:       "GitHub",
		Description: "GitHub integration settings",
		Icon:        "github",
		Category:    "providers",
	},
	"providers.gitlab": {
		Title:       "GitLab",
		Description: "GitLab integration settings",
		Icon:        "gitlab",
		Category:    "providers",
	},
	"providers.wrike": {
		Title:       "Wrike",
		Description: "Wrike integration settings",
		Icon:        "briefcase",
		Category:    "providers",
	},
	"providers.linear": {
		Title:       "Linear",
		Description: "Linear integration settings",
		Icon:        "linear",
		Category:    "providers",
	},
	"git": {
		Title:       "Git",
		Description: "Version control settings",
		Icon:        "git-branch",
		Category:    "core",
	},
	"workers": {
		Title:       "Workers",
		Description: "Worker pool configuration",
		Icon:        "users",
		Category:    "core",
	},
	"watchdog": {
		Title:       "Watchdog",
		Description: "Memory leak detection",
		Icon:        "activity",
		Category:    "core",
	},
	"custom_agents": {
		Title:       "Custom Agents",
		Description: "User-defined agent configurations",
		Icon:        "wand",
		Category:    "core",
	},
	"workflow": {
		Title:       "Workflow",
		Description: "Per-project workflow options",
		Icon:        "git-fork",
		Category:    "core",
	},
}
