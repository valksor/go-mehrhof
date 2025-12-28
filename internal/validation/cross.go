package validation

import (
	"fmt"
	"slices"

	"github.com/valksor/go-mehrhof/internal/config"
	"github.com/valksor/go-mehrhof/internal/storage"
)

// Error codes for cross-config validation
const (
	CodeAgentMismatch  = "AGENT_MISMATCH"
	CodeConfigConflict = "CONFIG_CONFLICT"
)

// validateCrossConfig performs cross-validation between app and workspace configs
func validateCrossConfig(app *config.Config, ws *storage.WorkspaceConfig, builtInAgents []string, result *Result) {
	validateAgentConsistency(app, ws, builtInAgents, result)
	validateGitConsistency(app, ws, result)
}

// validateAgentConsistency ensures agent settings are consistent between configs
func validateAgentConsistency(app *config.Config, ws *storage.WorkspaceConfig, builtInAgents []string, result *Result) {
	// Check if workspace default agent is valid considering both built-in and aliases
	if ws.Agent.Default != "" {
		isBuiltIn := slices.Contains(builtInAgents, ws.Agent.Default)
		_, isAlias := ws.Agents[ws.Agent.Default]

		if !isBuiltIn && !isAlias {
			// If the agent is also not the app default, it's a problem
			if ws.Agent.Default != app.Agent.Default {
				result.AddWarningWithSuggestion(
					CodeAgentMismatch,
					fmt.Sprintf("Workspace default agent %q is not a built-in agent or defined alias", ws.Agent.Default),
					"agent.default",
					".mehrhof/config.yaml",
					fmt.Sprintf("Define %q as an alias or use a built-in agent: %v", ws.Agent.Default, builtInAgents),
				)
			}
		}
	}
}

// validateGitConsistency checks for potentially conflicting git settings
func validateGitConsistency(app *config.Config, ws *storage.WorkspaceConfig, result *Result) {
	// Warn if both app and workspace have different non-empty branch patterns
	if app.Git.BranchPattern != "" && ws.Git.BranchPattern != "" {
		if app.Git.BranchPattern != ws.Git.BranchPattern {
			result.AddInfo(
				CodeConfigConflict,
				fmt.Sprintf("Different branch patterns: app=%q, workspace=%q (workspace takes precedence)", app.Git.BranchPattern, ws.Git.BranchPattern),
				"git.branch_pattern",
				"(both)",
			)
		}
	}

	// Warn if both app and workspace have different non-empty commit prefixes
	if app.Git.CommitPrefix != "" && ws.Git.CommitPrefix != "" {
		if app.Git.CommitPrefix != ws.Git.CommitPrefix {
			result.AddInfo(
				CodeConfigConflict,
				fmt.Sprintf("Different commit prefixes: app=%q, workspace=%q (workspace takes precedence)", app.Git.CommitPrefix, ws.Git.CommitPrefix),
				"git.commit_prefix",
				"(both)",
			)
		}
	}
}
