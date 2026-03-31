package settings

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// setBool sets a plain bool field from a value that must be bool.
func setBool(field *bool, value any, path string) error {
	if v, ok := value.(bool); ok {
		*field = v

		return nil
	}

	return fmt.Errorf("%s must be a boolean", path)
}

// setBoolPtr sets a *bool field from a value that must be bool.
func setBoolPtr(field **bool, value any, path string) error {
	if v, ok := value.(bool); ok {
		*field = &v

		return nil
	}

	return fmt.Errorf("%s must be a boolean", path)
}

// setString sets a string field from a value that must be string.
func setString(field *string, value any, path string) error {
	if v, ok := value.(string); ok {
		*field = v

		return nil
	}

	return fmt.Errorf("%s must be a string", path)
}

// setInt sets an int field from a value that must be int or float64 (JSON numbers).
func setInt(field *int, value any, path string) error {
	switch v := value.(type) {
	case int:
		*field = v

		return nil
	case float64:
		if v < math.MinInt || v > math.MaxInt {
			return fmt.Errorf("%s value out of range", path)
		}
		if v != float64(int(v)) {
			return fmt.Errorf("%s must be a whole number", path)
		}
		*field = int(v)

		return nil
	}

	return fmt.Errorf("%s must be an integer", path)
}

// setStringSlice sets a []string field from a value that is []string or []any.
func setStringSlice(field *[]string, value any, path string) error {
	if v, ok := value.([]string); ok {
		*field = v

		return nil
	}
	// Handle []any from JSON unmarshaling
	if arr, ok := value.([]any); ok {
		strs := make([]string, len(arr))
		for i, item := range arr {
			if str, ok := item.(string); ok {
				strs[i] = str
			} else {
				return fmt.Errorf("%s[%d] must be a string", path, i)
			}
		}
		*field = strs

		return nil
	}

	return fmt.Errorf("%s must be a string array", path)
}

// setter holds a set function and a get function for a single settings path.
type setter struct {
	set func(cfg *Settings, value any) error
	get func(cfg *Settings) any
}

// setterMap maps dot-notation paths to their setter/getter implementations.
//
//nolint:gochecknoglobals // Package-level registry, equivalent to the switch it replaces.
var setterMap = map[string]setter{
	// ── Agent ──────────────────────────────────────────────────────────────────
	KeyAgentDefault: {
		set: func(cfg *Settings, v any) error { return setString(&cfg.Agent.Default, v, KeyAgentDefault) },
		get: func(cfg *Settings) any { return cfg.Agent.Default },
	},
	"agent.allowed": {
		set: func(cfg *Settings, v any) error { return setStringSlice(&cfg.Agent.Allowed, v, "agent.allowed") },
		get: func(cfg *Settings) any { return cfg.Agent.Allowed },
	},

	// ── Providers ─────────────────────────────────────────────────────────────
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

	// ── Git ───────────────────────────────────────────────────────────────────
	"git.base_branch": {
		set: func(cfg *Settings, v any) error { return setString(&cfg.Git.BaseBranch, v, "git.base_branch") },
		get: func(cfg *Settings) any { return cfg.Git.BaseBranch },
	},
	"git.branch_pattern": {
		set: func(cfg *Settings, v any) error { return setString(&cfg.Git.BranchPattern, v, "git.branch_pattern") },
		get: func(cfg *Settings) any { return cfg.Git.BranchPattern },
	},
	"git.commit_prefix": {
		set: func(cfg *Settings, v any) error { return setString(&cfg.Git.CommitPrefix, v, "git.commit_prefix") },
		get: func(cfg *Settings) any { return cfg.Git.CommitPrefix },
	},
	"git.commit_pattern": {
		set: func(cfg *Settings, v any) error { return setString(&cfg.Git.CommitPattern, v, "git.commit_pattern") },
		get: func(cfg *Settings) any { return cfg.Git.CommitPattern },
	},
	"git.pr_title_pattern": {
		set: func(cfg *Settings, v any) error { return setString(&cfg.Git.PRTitlePattern, v, "git.pr_title_pattern") },
		get: func(cfg *Settings) any { return cfg.Git.PRTitlePattern },
	},
	"git.branch_validation_pattern": {
		set: func(cfg *Settings, v any) error {
			return setString(&cfg.Git.BranchValidationPattern, v, "git.branch_validation_pattern")
		},
		get: func(cfg *Settings) any { return cfg.Git.BranchValidationPattern },
	},
	"git.create_branch": {
		set: func(cfg *Settings, v any) error { return setBoolPtr(&cfg.Git.CreateBranch, v, "git.create_branch") },
		get: func(cfg *Settings) any { return BoolValue(cfg.Git.CreateBranch, true) },
	},
	"git.auto_commit": {
		set: func(cfg *Settings, v any) error { return setBoolPtr(&cfg.Git.AutoCommit, v, "git.auto_commit") },
		get: func(cfg *Settings) any { return BoolValue(cfg.Git.AutoCommit, true) },
	},
	"git.sign_commits": {
		set: func(cfg *Settings, v any) error { return setBoolPtr(&cfg.Git.SignCommits, v, "git.sign_commits") },
		get: func(cfg *Settings) any { return BoolValue(cfg.Git.SignCommits, false) },
	},
	"git.allow_pr_comment": {
		set: func(cfg *Settings, v any) error {
			return setBoolPtr(&cfg.Git.AllowPRComment, v, "git.allow_pr_comment")
		},
		get: func(cfg *Settings) any { return BoolValue(cfg.Git.AllowPRComment, false) },
	},

	// ── Workers ───────────────────────────────────────────────────────────────
	"workers.max": {
		set: func(cfg *Settings, v any) error { return setInt(&cfg.Workers.Max, v, "workers.max") },
		get: func(cfg *Settings) any { return cfg.Workers.Max },
	},

	// ── Storage ───────────────────────────────────────────────────────────────
	"storage.save_in_project": {
		set: func(cfg *Settings, v any) error {
			return setBoolPtr(&cfg.Storage.SaveInProject, v, "storage.save_in_project")
		},
		get: func(cfg *Settings) any { return BoolValue(cfg.Storage.SaveInProject, false) },
	},
	"storage.spec_output_path": {
		set: func(cfg *Settings, v any) error {
			return setString(&cfg.Storage.SpecOutputPath, v, "storage.spec_output_path")
		},
		get: func(cfg *Settings) any { return cfg.Storage.SpecOutputPath },
	},
	"storage.plan_output_path": {
		set: func(cfg *Settings, v any) error {
			return setString(&cfg.Storage.PlanOutputPath, v, "storage.plan_output_path")
		},
		get: func(cfg *Settings) any { return cfg.Storage.PlanOutputPath },
	},
	"storage.commit_specs": {
		set: func(cfg *Settings, v any) error {
			return setBoolPtr(&cfg.Storage.CommitSpecs, v, "storage.commit_specs")
		},
		get: func(cfg *Settings) any { return BoolValue(cfg.Storage.CommitSpecs, false) },
	},
	"storage.changelog_path": {
		set: func(cfg *Settings, v any) error {
			return setString(&cfg.Storage.ChangelogPath, v, "storage.changelog_path")
		},
		get: func(cfg *Settings) any { return cfg.Storage.ChangelogPath },
	},

	// ── Workflow ───────────────────────────────────────────────────────────────
	"workflow.use_worktree_isolation": {
		set: func(cfg *Settings, v any) error {
			return setBoolPtr(&cfg.Workflow.UseWorktreeIsolation, v, "workflow.use_worktree_isolation")
		},
		get: func(cfg *Settings) any { return BoolValue(cfg.Workflow.UseWorktreeIsolation, true) },
	},
	"workflow.external_review.mode": {
		set: func(cfg *Settings, v any) error {
			s, ok := v.(string)
			if !ok {
				return errors.New("workflow.external_review.mode must be a string")
			}
			mode := ExternalReviewMode(s)
			switch mode {
			case ExternalReviewAsk, ExternalReviewAlways, ExternalReviewNever:
				cfg.Workflow.ExternalReview.Mode = mode

				return nil
			default:
				return errors.New("workflow.external_review.mode must be one of: ask, always, never")
			}
		},
		get: func(cfg *Settings) any {
			if cfg.Workflow.ExternalReview.Mode == "" {
				return string(ExternalReviewAsk)
			}

			return string(cfg.Workflow.ExternalReview.Mode)
		},
	},
	"workflow.external_review.command": {
		set: func(cfg *Settings, v any) error {
			s, ok := v.(string)
			if !ok {
				return errors.New("workflow.external_review.command must be a string")
			}
			if strings.ContainsAny(s, "/\\") {
				return fmt.Errorf("workflow.external_review.command must be a plain name, not a path: %q", s)
			}
			cfg.Workflow.ExternalReview.Command = s

			return nil
		},
		get: func(cfg *Settings) any {
			if cfg.Workflow.ExternalReview.Command == "" {
				return "coderabbit"
			}

			return cfg.Workflow.ExternalReview.Command
		},
	},

	// ── Policy ────────────────────────────────────────────────────────────────
	"workflow.policy.approval_required": {
		set: func(cfg *Settings, v any) error {
			m, ok := v.(map[string]any)
			if !ok {
				return errors.New("workflow.policy.approval_required must be a map of event->boolean")
			}
			result := make(map[string]bool)
			for k, val := range m {
				if b, ok := val.(bool); ok {
					result[k] = b
				} else {
					return fmt.Errorf("workflow.policy.approval_required.%s must be a boolean", k)
				}
			}
			cfg.Workflow.Policy.ApprovalRequired = result

			return nil
		},
		get: func(cfg *Settings) any { return cfg.Workflow.Policy.ApprovalRequired },
	},
	"workflow.policy.review_checklist": {
		set: func(cfg *Settings, v any) error {
			return setStringSlice(&cfg.Workflow.Policy.ReviewChecklist, v, "workflow.policy.review_checklist")
		},
		get: func(cfg *Settings) any { return cfg.Workflow.Policy.ReviewChecklist },
	},

	// ── UI ─────────────────────────────────────────────────────────────────────
	"ui.onboarding_dismissed": {
		set: func(cfg *Settings, v any) error {
			return setBool(&cfg.UI.OnboardingDismissed, v, "ui.onboarding_dismissed")
		},
		get: func(cfg *Settings) any { return cfg.UI.OnboardingDismissed },
	},

	// ── Custom Agents ─────────────────────────────────────────────────────────
	"custom_agents": {
		set: func(cfg *Settings, v any) error {
			m, ok := v.(map[string]any)
			if !ok {
				return errors.New("custom_agents must be an object")
			}
			agents := make(map[string]CustomAgent)
			for name, agentData := range m {
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
					for i, a := range argsAny {
						s, ok := a.(string)
						if !ok {
							return fmt.Errorf("custom_agents.%s.args[%d] must be a string", name, i)
						}
						args = append(args, s)
					}
					agent.Args = args
				}
				if envAny, ok := agentMap["env"].(map[string]any); ok {
					env := make(map[string]string)
					for k, val := range envAny {
						s, ok := val.(string)
						if !ok {
							return fmt.Errorf("custom_agents.%s.env.%s must be a string", name, k)
						}
						env[k] = s
					}
					agent.Env = env
				}
				agents[name] = agent
			}
			cfg.CustomAgents = agents

			return nil
		},
		get: func(cfg *Settings) any { return cfg.CustomAgents },
	},
}
