package settings

// ProviderSettings configures task providers (GitHub, GitLab, etc.).
type ProviderSettings struct {
	Default     string            `yaml:"default,omitempty" json:"default,omitempty" schema:"label=Default Provider;desc=Provider used when none specified;options=github|gitlab|wrike|linear|azuredevops|file"`
	GitHub      GitHubConfig      `yaml:"github,omitempty" json:"github,omitzero"`
	GitLab      GitLabConfig      `yaml:"gitlab,omitempty" json:"gitlab,omitzero"`
	Wrike       WrikeConfig       `yaml:"wrike,omitempty" json:"wrike,omitzero"`
	Linear      LinearConfig      `yaml:"linear,omitempty" json:"linear,omitzero"`
	Jira        JiraConfig        `yaml:"jira,omitempty" json:"jira,omitzero"`
	AzureDevOps AzureDevOpsConfig `yaml:"azuredevops,omitempty" json:"azuredevops,omitzero"`
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
