package settings

import "maps"

func init() {
	// ── Providers ─────────────────────────────────────────────────────────────
	providerSetters := map[string]setter{
		"providers.default": {
			set: func(cfg *Settings, v any) error { return setString(&cfg.Providers.Default, v, "providers.default") },
			get: func(cfg *Settings) any { return cfg.Providers.Default },
		},

		// GitHub
		"providers.github.token": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.GitHub.Token, v, "providers.github.token")
			},
			get: func(cfg *Settings) any { return cfg.Providers.GitHub.Token },
		},
		"providers.github.owner": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.GitHub.Owner, v, "providers.github.owner")
			},
			get: func(cfg *Settings) any { return cfg.Providers.GitHub.Owner },
		},
		"providers.github.allow_ticket_comment": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.GitHub.AllowTicketComment, v, "providers.github.allow_ticket_comment")
			},
			get: func(cfg *Settings) any { return cfg.Providers.GitHub.AllowTicketComment },
		},
		"providers.github.status_sync": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.GitHub.StatusSync, v, "providers.github.status_sync")
			},
			get: func(cfg *Settings) any { return cfg.Providers.GitHub.StatusSync },
		},
		"providers.github.status_mapping": {
			set: func(cfg *Settings, v any) error {
				return setStatusMapping(&cfg.Providers.GitHub.StatusMapping, v, "providers.github.status_mapping")
			},
			get: func(cfg *Settings) any { return cfg.Providers.GitHub.StatusMapping },
		},

		// GitLab
		"providers.gitlab.token": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.GitLab.Token, v, "providers.gitlab.token")
			},
			get: func(cfg *Settings) any { return cfg.Providers.GitLab.Token },
		},
		"providers.gitlab.base_url": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.GitLab.BaseURL, v, "providers.gitlab.base_url")
			},
			get: func(cfg *Settings) any { return cfg.Providers.GitLab.BaseURL },
		},
		"providers.gitlab.allow_ticket_comment": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.GitLab.AllowTicketComment, v, "providers.gitlab.allow_ticket_comment")
			},
			get: func(cfg *Settings) any { return cfg.Providers.GitLab.AllowTicketComment },
		},
		"providers.gitlab.status_sync": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.GitLab.StatusSync, v, "providers.gitlab.status_sync")
			},
			get: func(cfg *Settings) any { return cfg.Providers.GitLab.StatusSync },
		},
		"providers.gitlab.status_mapping": {
			set: func(cfg *Settings, v any) error {
				return setStatusMapping(&cfg.Providers.GitLab.StatusMapping, v, "providers.gitlab.status_mapping")
			},
			get: func(cfg *Settings) any { return cfg.Providers.GitLab.StatusMapping },
		},

		// Wrike
		"providers.wrike.token": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.Wrike.Token, v, "providers.wrike.token")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Wrike.Token },
		},
		"providers.wrike.include_parent_context": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.Wrike.IncludeParentContext, v, "providers.wrike.include_parent_context")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Wrike.IncludeParentContext },
		},
		"providers.wrike.include_sibling_context": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.Wrike.IncludeSiblingContext, v, "providers.wrike.include_sibling_context")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Wrike.IncludeSiblingContext },
		},
		"providers.wrike.allow_ticket_comment": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.Wrike.AllowTicketComment, v, "providers.wrike.allow_ticket_comment")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Wrike.AllowTicketComment },
		},

		// Linear
		"providers.linear.token": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.Linear.Token, v, "providers.linear.token")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Linear.Token },
		},
		"providers.linear.team": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.Linear.Team, v, "providers.linear.team")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Linear.Team },
		},
		"providers.linear.include_parent_context": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.Linear.IncludeParentContext, v, "providers.linear.include_parent_context")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Linear.IncludeParentContext },
		},
		"providers.linear.include_sibling_context": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.Linear.IncludeSiblingContext, v, "providers.linear.include_sibling_context")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Linear.IncludeSiblingContext },
		},
		"providers.linear.allow_ticket_comment": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.Linear.AllowTicketComment, v, "providers.linear.allow_ticket_comment")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Linear.AllowTicketComment },
		},
		"providers.linear.status_sync": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.Linear.StatusSync, v, "providers.linear.status_sync")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Linear.StatusSync },
		},
		"providers.linear.status_mapping": {
			set: func(cfg *Settings, v any) error {
				return setStatusMapping(&cfg.Providers.Linear.StatusMapping, v, "providers.linear.status_mapping")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Linear.StatusMapping },
		},

		// Jira
		"providers.jira.token": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.Jira.Token, v, "providers.jira.token")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Jira.Token },
		},
		"providers.jira.email": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.Jira.Email, v, "providers.jira.email")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Jira.Email },
		},
		"providers.jira.base_url": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.Jira.BaseURL, v, "providers.jira.base_url")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Jira.BaseURL },
		},
		"providers.jira.allow_ticket_comment": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.Jira.AllowTicketComment, v, "providers.jira.allow_ticket_comment")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Jira.AllowTicketComment },
		},
		"providers.jira.status_sync": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.Jira.StatusSync, v, "providers.jira.status_sync")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Jira.StatusSync },
		},
		"providers.jira.status_mapping": {
			set: func(cfg *Settings, v any) error {
				return setStatusMapping(&cfg.Providers.Jira.StatusMapping, v, "providers.jira.status_mapping")
			},
			get: func(cfg *Settings) any { return cfg.Providers.Jira.StatusMapping },
		},

		// Azure DevOps
		"providers.azuredevops.token": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.AzureDevOps.Token, v, "providers.azuredevops.token")
			},
			get: func(cfg *Settings) any { return cfg.Providers.AzureDevOps.Token },
		},
		"providers.azuredevops.base_url": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.AzureDevOps.BaseURL, v, "providers.azuredevops.base_url")
			},
			get: func(cfg *Settings) any { return cfg.Providers.AzureDevOps.BaseURL },
		},
		"providers.azuredevops.organization": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.AzureDevOps.Organization, v, "providers.azuredevops.organization")
			},
			get: func(cfg *Settings) any { return cfg.Providers.AzureDevOps.Organization },
		},
		"providers.azuredevops.project": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.AzureDevOps.Project, v, "providers.azuredevops.project")
			},
			get: func(cfg *Settings) any { return cfg.Providers.AzureDevOps.Project },
		},
		"providers.azuredevops.repository": {
			set: func(cfg *Settings, v any) error {
				return setString(&cfg.Providers.AzureDevOps.Repository, v, "providers.azuredevops.repository")
			},
			get: func(cfg *Settings) any { return cfg.Providers.AzureDevOps.Repository },
		},
		"providers.azuredevops.allow_ticket_comment": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.AzureDevOps.AllowTicketComment, v, "providers.azuredevops.allow_ticket_comment")
			},
			get: func(cfg *Settings) any { return cfg.Providers.AzureDevOps.AllowTicketComment },
		},
		"providers.azuredevops.status_sync": {
			set: func(cfg *Settings, v any) error {
				return setBool(&cfg.Providers.AzureDevOps.StatusSync, v, "providers.azuredevops.status_sync")
			},
			get: func(cfg *Settings) any { return cfg.Providers.AzureDevOps.StatusSync },
		},
		"providers.azuredevops.status_mapping": {
			set: func(cfg *Settings, v any) error {
				return setStatusMapping(&cfg.Providers.AzureDevOps.StatusMapping, v, "providers.azuredevops.status_mapping")
			},
			get: func(cfg *Settings) any { return cfg.Providers.AzureDevOps.StatusMapping },
		},
	}

	maps.Copy(setterMap, providerSetters)
}
