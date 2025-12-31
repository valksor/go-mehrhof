package validation

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/valksor/go-mehrhof/internal/storage"
)

// Error codes for workspace validation
const (
	CodeYAMLSyntax          = "YAML_SYNTAX"
	CodeAgentAliasCircular  = "AGENT_ALIAS_CIRCULAR"
	CodeAgentAliasUndefined = "AGENT_ALIAS_UNDEFINED"
	CodeAgentAliasNoExtends = "AGENT_ALIAS_NO_EXTENDS"
	CodeGitPatternInvalid   = "GIT_PATTERN_INVALID"
	CodeGitPatternEmpty     = "GIT_PATTERN_EMPTY"
	CodeEnvVarUnset         = "ENV_VAR_UNSET"
	CodeInvalidEnum         = "INVALID_ENUM"
	CodeInvalidRange        = "INVALID_RANGE"
	CodePluginNotFound      = "PLUGIN_NOT_FOUND"
	CodeInvalidPath         = "INVALID_PATH"
)

// Valid git pattern placeholders
var validGitPlaceholders = []string{"{key}", "{task_id}", "{type}", "{slug}", "{title}"}

// Pattern to match environment variable references like ${VAR_NAME}
var envVarRefPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// validateWorkspaceConfig validates all aspects of a workspace configuration
func validateWorkspaceConfig(cfg *storage.WorkspaceConfig, configPath string, builtInAgents []string, result *Result) {
	validateGitSettings(cfg.Git, configPath, result)
	validateAgentSettings(cfg.Agent, configPath, builtInAgents, cfg.Agents, result)
	validateWorkflowSettings(cfg.Workflow, configPath, result)
	validateStorageSettings(cfg.Storage, configPath, result)
	validateAgentAliases(cfg.Agents, configPath, builtInAgents, result)
	validatePluginsConfig(cfg.Plugins, configPath, result)

	if cfg.GitHub != nil {
		validateGitHubSettings(cfg.GitHub, configPath, result)
	}
}

// validateGitSettings validates git-related configuration
func validateGitSettings(git storage.GitSettings, configPath string, result *Result) {
	// Validate branch pattern
	if git.BranchPattern == "" {
		result.AddWarning(CodeGitPatternEmpty, "Branch pattern is empty", "git.branch_pattern", configPath)
	} else {
		validateGitPattern(git.BranchPattern, "git.branch_pattern", configPath, result)
	}

	// Validate commit prefix
	if git.CommitPrefix != "" {
		validateGitPattern(git.CommitPrefix, "git.commit_prefix", configPath, result)
	}
}

// validateGitPattern checks if a git pattern contains valid placeholders
func validateGitPattern(pattern, path, configPath string, result *Result) {
	// Find all placeholders in the pattern
	placeholderPattern := regexp.MustCompile(`\{[^}]+\}`)
	matches := placeholderPattern.FindAllString(pattern, -1)

	for _, match := range matches {
		if !slices.Contains(validGitPlaceholders, match) {
			result.AddWarningWithSuggestion(
				CodeGitPatternInvalid,
				fmt.Sprintf("Unknown placeholder %q in pattern", match),
				path,
				configPath,
				fmt.Sprintf("Valid placeholders: %s", strings.Join(validGitPlaceholders, ", ")),
			)
		}
	}

	// Check for patterns that might produce invalid branch names
	if strings.Contains(pattern, "..") {
		result.AddWarning(CodeGitPatternInvalid, "Pattern contains '..' which may produce invalid branch names", path, configPath)
	}
	if strings.HasPrefix(pattern, "/") || strings.HasSuffix(pattern, "/") {
		result.AddWarning(CodeGitPatternInvalid, "Pattern should not start or end with '/'", path, configPath)
	}
}

// validateAgentSettings validates agent-related configuration
func validateAgentSettings(agent storage.AgentSettings, configPath string, builtInAgents []string, aliases map[string]storage.AgentAliasConfig, result *Result) {
	// Validate default agent
	if agent.Default != "" {
		isBuiltIn := slices.Contains(builtInAgents, agent.Default)
		_, isAlias := aliases[agent.Default]
		if !isBuiltIn && !isAlias {
			result.AddErrorWithSuggestion(
				CodeInvalidEnum,
				fmt.Sprintf("Unknown default agent %q", agent.Default),
				"agent.default",
				configPath,
				fmt.Sprintf("Available agents: %s", strings.Join(builtInAgents, ", ")),
			)
		}
	}

	// Validate timeout range (0-3600 seconds)
	if agent.Timeout < 0 || agent.Timeout > 3600 {
		result.AddError(CodeInvalidRange, fmt.Sprintf("Timeout %d is out of range (0-3600)", agent.Timeout), "agent.timeout", configPath)
	}

	// Validate max retries range (0-10)
	if agent.MaxRetries < 0 || agent.MaxRetries > 10 {
		result.AddError(CodeInvalidRange, fmt.Sprintf("Max retries %d is out of range (0-10)", agent.MaxRetries), "agent.max_retries", configPath)
	}
}

// validateWorkflowSettings validates workflow-related configuration
func validateWorkflowSettings(workflow storage.WorkflowSettings, configPath string, result *Result) {
	// Validate session retention days (reasonable range: 1-365)
	if workflow.SessionRetentionDays < 0 || workflow.SessionRetentionDays > 365 {
		result.AddWarning(CodeInvalidRange, fmt.Sprintf("Session retention days %d may be unreasonable (expected 1-365)", workflow.SessionRetentionDays), "workflow.session_retention_days", configPath)
	}
}

// validateStorageSettings validates storage-related configuration
func validateStorageSettings(storage storage.StorageSettings, configPath string, result *Result) {
	if storage.WorkDir == "" {
		return // Empty is fine, will use default
	}

	// Check for absolute paths
	if strings.HasPrefix(storage.WorkDir, "/") || strings.HasPrefix(storage.WorkDir, "\\") {
		result.AddError(CodeInvalidPath, "Work directory must be relative to project root, not absolute", "storage.work_dir", configPath)
		return
	}

	// Check for home directory expansion
	if strings.HasPrefix(storage.WorkDir, "~") {
		result.AddError(CodeInvalidPath, "Work directory cannot use home directory (~) expansion", "storage.work_dir", configPath)
		return
	}

	// Check for path traversal attempts
	if strings.Contains(storage.WorkDir, "..") {
		result.AddError(CodeInvalidPath, "Work directory cannot contain '..' (would escape project root)", "storage.work_dir", configPath)
		return
	}

	// Check for invalid characters (basic sanity check)
	// Valid: alphanumeric, hyphen, underscore, dot, forward slash
	validPathPattern := regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
	if !validPathPattern.MatchString(storage.WorkDir) {
		result.AddError(CodeInvalidPath, "Work directory contains invalid characters", "storage.work_dir", configPath)
	}
}

// validateAgentAliases validates agent alias configurations including circular dependency detection
func validateAgentAliases(aliases map[string]storage.AgentAliasConfig, configPath string, builtInAgents []string, result *Result) {
	if len(aliases) == 0 {
		return
	}

	// Track resolved and resolving aliases for circular dependency detection
	resolved := make(map[string]bool)
	resolving := make(map[string]bool)

	var resolve func(name string, chain []string) bool
	resolve = func(name string, chain []string) bool {
		if resolved[name] {
			return true
		}

		if resolving[name] {
			// Circular dependency detected
			chainStr := strings.Join(append(chain, name), " -> ")
			result.AddErrorWithSuggestion(
				CodeAgentAliasCircular,
				fmt.Sprintf("Circular dependency detected: %s", chainStr),
				fmt.Sprintf("agents.%s", name),
				configPath,
				"Remove circular reference in 'extends' field",
			)
			return false
		}

		alias, ok := aliases[name]
		if !ok {
			return true // Not an alias, skip
		}

		// Validate that extends is specified
		if alias.Extends == "" {
			result.AddError(CodeAgentAliasNoExtends, "Alias must specify 'extends' field", fmt.Sprintf("agents.%s.extends", name), configPath)
			return false
		}

		resolving[name] = true
		newChain := append(chain, name)

		// Check if base agent exists
		isBuiltIn := slices.Contains(builtInAgents, alias.Extends)
		_, isAlias := aliases[alias.Extends]

		if !isBuiltIn && !isAlias {
			result.AddErrorWithSuggestion(
				CodeAgentAliasUndefined,
				fmt.Sprintf("Extends unknown agent %q", alias.Extends),
				fmt.Sprintf("agents.%s.extends", name),
				configPath,
				fmt.Sprintf("Available agents: %s", strings.Join(builtInAgents, ", ")),
			)
			resolving[name] = false
			return false
		}

		// If extending another alias, resolve it first
		if isAlias {
			if !resolve(alias.Extends, newChain) {
				resolving[name] = false
				return false
			}
		}

		// Validate environment variable references
		validateEnvVarReferences(alias.Env, fmt.Sprintf("agents.%s.env", name), configPath, result)

		resolved[name] = true
		resolving[name] = false
		return true
	}

	// Resolve all aliases
	for name := range aliases {
		resolve(name, nil)
	}
}

// validateEnvVarReferences checks if environment variable references can be resolved
func validateEnvVarReferences(env map[string]string, basePath, configPath string, result *Result) {
	for key, value := range env {
		matches := envVarRefPattern.FindAllStringSubmatch(value, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				varName := match[1]
				if os.Getenv(varName) == "" {
					result.AddWarningWithSuggestion(
						CodeEnvVarUnset,
						fmt.Sprintf("Environment variable ${%s} is not set", varName),
						fmt.Sprintf("%s.%s", basePath, key),
						configPath,
						fmt.Sprintf("Set %s environment variable", varName),
					)
				}
			}
		}
	}
}

// validatePluginsConfig validates plugin configuration
func validatePluginsConfig(plugins storage.PluginsConfig, configPath string, result *Result) {
	// Note: We can't check if plugins actually exist without plugin discovery,
	// so we only validate the configuration structure here
	for pluginName := range plugins.Config {
		found := false
		for _, enabled := range plugins.Enabled {
			if enabled == pluginName {
				found = true
				break
			}
		}
		if !found {
			result.AddWarning(
				CodePluginNotFound,
				fmt.Sprintf("Plugin %q has config but is not in enabled list", pluginName),
				fmt.Sprintf("plugins.config.%s", pluginName),
				configPath,
			)
		}
	}
}

// validateGitHubSettings validates GitHub provider configuration
func validateGitHubSettings(gh *storage.GitHubSettings, configPath string, result *Result) {
	// Validate branch pattern if specified
	if gh.BranchPattern != "" {
		validateGitPattern(gh.BranchPattern, "github.branch_pattern", configPath, result)
	}

	// Validate commit prefix if specified
	if gh.CommitPrefix != "" {
		validateGitPattern(gh.CommitPrefix, "github.commit_prefix", configPath, result)
	}
}
