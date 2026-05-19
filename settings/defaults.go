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
			// Default to claude-mcp. Background, in three parts:
			//
			// 1. The legacy "claude" adapter was dual-mode (WebSocket Agent
			//    SDK + CLI fallback). It was split into two single-mode
			//    adapters after claude CLI 2.1.121 made the SDK WebSocket
			//    transport nonfunctional:
			//      - "claude"     → agent/claude/    (plain `claude --print`)
			//      - "claude-sdk" → agent/claudesdk/ (WebSocket --sdk-url)
			// 2. The "claude-sdk" path is broken on the official Anthropic
			//    CLI since 2.1.121: --sdk-url is restricted to Anthropic's
			//    own backend and the spawned process exits with: "host
			//    '127.0.0.1' is not an approved Anthropic endpoint. This
			//    flag is reserved for Remote Control worker processes
			//    connecting to Anthropic's backend." Whether the
			//    restriction will be lifted is unknown — it is a separate
			//    issue from the 2026-06-15 billing change. The adapter is
			//    still registered because some proxy setups (Claude Code
			//    Router etc.) accept --sdk-url and may still need this path.
			// 3. The "claude" (plain CLI) path works today and is what
			//    custom agents like `glm extends: claude` actually use via
			//    their proxy. Its billing classification is `claude -p`, so
			//    the 2026-06-15 Agent SDK credit-pool split applies even
			//    though the SDK transport is not used.
			//
			// claude-mcp wins as the default because it (a) works on the
			// current claude CLI and (b) preserves Max-subscription billing
			// after the 2026-06-15 Agent SDK credit-pool split. It spawns
			// claude in interactive TUI mode under a PTY with --mcp-config.
			Default: "claude-mcp",
			Allowed: []string{"claude", "claude-sdk", "claude-mcp", "codex"},
		},
		Providers: ProviderSettings{
			Default: providerDefault,
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

// Section IDs and category names. Several literals repeat across this file
// (and in schema.go), so they live as constants for goconst hygiene and to
// give each call site a clear semantic name. `sectionCustomAgents` is
// already defined in schema.go.
const (
	sectionAgent     = "agent"
	sectionProviders = "providers"
	sectionGit       = "git"

	categoryCore      = "core"
	categoryProviders = "providers"

	// providerDefault is the name of the default task provider. Mirrors
	// provider.NameGitHub but kept untyped here to avoid importing the
	// provider package solely for a string constant.
	providerDefault = "github"
)

// SectionRegistry maps section IDs to their metadata.
// This metadata is used when generating the schema for UI rendering.
var SectionRegistry = map[string]SectionMeta{
	sectionAgent: {
		Title:       "Agent",
		Description: "AI agent configuration",
		Icon:        "bot",
		Category:    categoryCore,
	},
	sectionProviders: {
		Title:       "Providers",
		Description: "Task provider integrations",
		Icon:        "plug",
		Category:    categoryProviders,
	},
	"providers.github": {
		Title:       "GitHub",
		Description: "GitHub integration settings",
		Icon:        "github",
		Category:    categoryProviders,
	},
	"providers.gitlab": {
		Title:       "GitLab",
		Description: "GitLab integration settings",
		Icon:        "gitlab",
		Category:    categoryProviders,
	},
	"providers.wrike": {
		Title:       "Wrike",
		Description: "Wrike integration settings",
		Icon:        "briefcase",
		Category:    categoryProviders,
	},
	"providers.linear": {
		Title:       "Linear",
		Description: "Linear integration settings",
		Icon:        "linear",
		Category:    categoryProviders,
	},
	sectionGit: {
		Title:       "Git",
		Description: "Version control settings",
		Icon:        "git-branch",
		Category:    categoryCore,
	},
	"workers": {
		Title:       "Workers",
		Description: "Worker pool configuration",
		Icon:        "users",
		Category:    categoryCore,
	},
	"watchdog": {
		Title:       "Watchdog",
		Description: "Memory leak detection",
		Icon:        "activity",
		Category:    categoryCore,
	},
	sectionCustomAgents: {
		Title:       "Custom Agents",
		Description: "User-defined agent configurations",
		Icon:        "wand",
		Category:    categoryCore,
	},
	"workflow": {
		Title:       "Workflow",
		Description: "Per-project workflow options",
		Icon:        "git-fork",
		Category:    categoryCore,
	},
}
