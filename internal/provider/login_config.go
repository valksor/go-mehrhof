package provider

// LoginConfig holds configuration for a provider's login command.
type LoginConfig struct {
	Name        string `json:"name"`
	EnvVar      string `json:"env_var"`
	HelpURL     string `json:"help_url"`
	HelpSteps   string `json:"help_steps"`
	Scopes      string `json:"scopes"`
	TokenPrefix string `json:"token_prefix"`
}

// LoginConfigs maps provider names to their login configuration.
var LoginConfigs = map[string]LoginConfig{
	NameGitHub: {
		Name:        "GitHub",
		EnvVar:      "GITHUB_TOKEN",
		HelpURL:     "https://github.com/settings/tokens",
		HelpSteps:   "Settings -> Developer settings -> Personal access tokens -> Tokens (classic)",
		Scopes:      "repo, read:user (or Fine-grained with repository access)",
		TokenPrefix: "ghp_",
	},
	NameGitLab: {
		Name:        "GitLab",
		EnvVar:      "GITLAB_TOKEN",
		HelpURL:     "https://gitlab.com/-/user_settings/personal_access_tokens",
		HelpSteps:   "Preferences -> Access Tokens -> Add new token",
		Scopes:      "api, read_user, read_repository",
		TokenPrefix: "glpat-",
	},
	NameLinear: {
		Name:        "Linear",
		EnvVar:      "LINEAR_TOKEN",
		HelpURL:     "https://linear.app/settings/api",
		HelpSteps:   "Settings -> API -> Personal API keys -> Create key",
		Scopes:      "Workspace access",
		TokenPrefix: "lin_api_",
	},
	NameWrike: {
		Name:        "Wrike",
		EnvVar:      "WRIKE_TOKEN",
		HelpURL:     "https://www.wrike.com/frontend/apps/index.html#/api",
		HelpSteps:   "Apps & Integrations -> API -> Permanent access tokens",
		Scopes:      "Default (read/write access)",
		TokenPrefix: "",
	},
	NameJira: {
		Name:        "Jira",
		EnvVar:      "JIRA_TOKEN",
		HelpURL:     "https://id.atlassian.com/manage-profile/security/api-tokens",
		HelpSteps:   "Account Settings -> Security -> API tokens -> Create API token",
		Scopes:      "Read/write project access",
		TokenPrefix: "",
	},
	NameAzureDevOps: {
		Name:        "Azure DevOps",
		EnvVar:      "AZURE_DEVOPS_TOKEN",
		HelpURL:     "https://dev.azure.com/_usersSettings/tokens",
		HelpSteps:   "User Settings -> Personal Access Tokens -> New Token",
		Scopes:      "Work Items (Read & Write), Code (Read & Write)",
		TokenPrefix: "",
	},
}
