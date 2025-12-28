package validation

import (
	"fmt"
	"slices"

	"github.com/valksor/go-mehrhof/internal/config"
)

// Error codes for app configuration validation
const (
	CodeEnvSyntax    = "ENV_SYNTAX"
	CodeEnvLoadError = "ENV_LOAD_ERROR"
)

// Valid enum values for app config
var (
	validAgentDefaults   = []string{"claude"}
	validUIFormats       = []string{"text", "json"}
	validProgressStyles  = []string{"spinner", "dots", "none"}
	validProviderDefault = []string{"file", "directory", "github", ""}
)

// validateAppConfig validates all aspects of application configuration
func validateAppConfig(cfg *config.Config, result *Result) {
	validateAppAgentConfig(cfg.Agent, result)
	validateAppStorageConfig(cfg.Storage, result)
	validateAppGitConfig(cfg.Git, result)
	validateAppProvidersConfig(cfg.Providers, result)
	validateAppUIConfig(cfg.UI, result)
}

// validateAppAgentConfig validates agent-related app configuration
func validateAppAgentConfig(agent config.AgentConfig, result *Result) {
	// Validate default agent
	if !slices.Contains(validAgentDefaults, agent.Default) {
		result.AddErrorWithSuggestion(
			CodeInvalidEnum,
			fmt.Sprintf("Invalid agent default %q", agent.Default),
			"agent.default",
			".env",
			fmt.Sprintf("Valid values: %v", validAgentDefaults),
		)
	}

	// Validate timeout range (0-3600 seconds)
	if agent.Timeout < 0 || agent.Timeout > 3600 {
		result.AddError(CodeInvalidRange, fmt.Sprintf("Agent timeout %d is out of range (0-3600)", agent.Timeout), "agent.timeout", ".env")
	}

	// Validate max retries range (0-10)
	if agent.MaxRetries < 0 || agent.MaxRetries > 10 {
		result.AddError(CodeInvalidRange, fmt.Sprintf("Agent max retries %d is out of range (0-10)", agent.MaxRetries), "agent.maxretries", ".env")
	}

	// Validate Claude config
	validateClaudeConfig(agent.Claude, result)
}

// validateClaudeConfig validates Claude-specific configuration
func validateClaudeConfig(claude config.ClaudeConfig, result *Result) {
	// Validate max tokens (reasonable range: 1-100000)
	if claude.MaxTokens < 1 || claude.MaxTokens > 200000 {
		result.AddWarning(CodeInvalidRange, fmt.Sprintf("Claude max tokens %d may be unreasonable (expected 1-200000)", claude.MaxTokens), "agent.claude.maxtokens", ".env")
	}

	// Validate temperature (0.0-2.0)
	if claude.Temperature < 0.0 || claude.Temperature > 2.0 {
		result.AddError(CodeInvalidRange, fmt.Sprintf("Claude temperature %.2f is out of range (0.0-2.0)", claude.Temperature), "agent.claude.temperature", ".env")
	}
}

// validateAppStorageConfig validates storage-related app configuration
func validateAppStorageConfig(storage config.StorageConfig, result *Result) {
	// Validate max blueprints (reasonable range: 1-10000)
	if storage.MaxBlueprints < 1 || storage.MaxBlueprints > 10000 {
		result.AddWarning(CodeInvalidRange, fmt.Sprintf("Max blueprints %d may be unreasonable (expected 1-10000)", storage.MaxBlueprints), "storage.maxblueprints", ".env")
	}

	// Validate session retention days (reasonable range: 1-365)
	if storage.SessionRetentionDays < 0 || storage.SessionRetentionDays > 365 {
		result.AddWarning(CodeInvalidRange, fmt.Sprintf("Session retention days %d may be unreasonable (expected 0-365)", storage.SessionRetentionDays), "storage.sessionretentiondays", ".env")
	}
}

// validateAppGitConfig validates git-related app configuration
func validateAppGitConfig(git config.GitConfig, result *Result) {
	// Validate branch pattern
	if git.BranchPattern != "" {
		validateGitPattern(git.BranchPattern, "git.branchpattern", ".env", result)
	}

	// Validate commit prefix
	if git.CommitPrefix != "" {
		validateGitPattern(git.CommitPrefix, "git.commitprefix", ".env", result)
	}
}

// validateAppProvidersConfig validates provider-related app configuration
func validateAppProvidersConfig(providers config.ProvidersConfig, result *Result) {
	// Validate default provider
	if providers.Default != "" && !slices.Contains(validProviderDefault, providers.Default) {
		result.AddWarningWithSuggestion(
			CodeInvalidEnum,
			fmt.Sprintf("Unknown default provider %q", providers.Default),
			"providers.default",
			".env",
			fmt.Sprintf("Valid values: %v", validProviderDefault),
		)
	}
}

// validateAppUIConfig validates UI-related app configuration
func validateAppUIConfig(ui config.UIConfig, result *Result) {
	// Validate format
	if !slices.Contains(validUIFormats, ui.Format) {
		result.AddErrorWithSuggestion(
			CodeInvalidEnum,
			fmt.Sprintf("Invalid UI format %q", ui.Format),
			"ui.format",
			".env",
			fmt.Sprintf("Valid values: %v", validUIFormats),
		)
	}

	// Validate progress style
	if !slices.Contains(validProgressStyles, ui.Progress) {
		result.AddErrorWithSuggestion(
			CodeInvalidEnum,
			fmt.Sprintf("Invalid progress style %q", ui.Progress),
			"ui.progress",
			".env",
			fmt.Sprintf("Valid values: %v", validProgressStyles),
		)
	}
}
