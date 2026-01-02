package commands

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/valksor/go-mehrhof/internal/storage"
)

// providerLoginConfig holds configuration for a provider's login command.
type providerLoginConfig struct {
	Name        string // Display name (e.g., "GitHub")
	EnvVar      string // Environment variable name (e.g., "GITHUB_TOKEN")
	ConfigField string // Field path in WorkspaceConfig (e.g., "GitHub.Token")
	HelpURL     string // URL for getting a token
	TokenPrefix string // Optional token prefix for validation (e.g., "ghp_")
}

// providerLoginConfigs maps provider names to their login configuration.
var providerLoginConfigs = map[string]providerLoginConfig{
	"github": {
		Name:        "GitHub",
		EnvVar:      "GITHUB_TOKEN",
		ConfigField: "GitHub.Token",
		HelpURL:     "https://github.com/settings/tokens",
		TokenPrefix: "ghp_",
	},
	"gitlab": {
		Name:        "GitLab",
		EnvVar:      "GITLAB_TOKEN",
		ConfigField: "GitLab.Token",
		HelpURL:     "https://gitlab.com/-/user_settings/personal_access_tokens",
		TokenPrefix: "glpat-",
	},
	"notion": {
		Name:        "Notion",
		EnvVar:      "NOTION_TOKEN",
		ConfigField: "Notion.Token",
		HelpURL:     "https://www.notion.so/my-integrations",
		TokenPrefix: "secret_",
	},
	"jira": {
		Name:        "Jira",
		EnvVar:      "JIRA_TOKEN",
		ConfigField: "Jira.Token",
		HelpURL:     "https://id.atlassian.com/manage-profile/security/api-tokens",
		TokenPrefix: "",
	},
	"linear": {
		Name:        "Linear",
		EnvVar:      "LINEAR_API_KEY",
		ConfigField: "Linear.Token",
		HelpURL:     "https://linear.app/settings/api",
		TokenPrefix: "lin_api_",
	},
	"wrike": {
		Name:        "Wrike",
		EnvVar:      "WRIKE_TOKEN",
		ConfigField: "Wrike.Token",
		HelpURL:     "https://www.wrike.com/workspace.htm",
		TokenPrefix: "",
	},
	"youtrack": {
		Name:        "YouTrack",
		EnvVar:      "YOUTRACK_TOKEN",
		ConfigField: "YouTrack.Token",
		HelpURL:     "https://www.jetbrains.com/help/youtrack/manage-user-token.html",
		TokenPrefix: "",
	},
}

// getProviderLoginConfig returns the login config for a provider, or nil if not found.
func getProviderLoginConfig(name string) *providerLoginConfig {
	// Normalize aliases
	normalized := normalizeProviderName(name)
	if cfg, ok := providerLoginConfigs[normalized]; ok {
		return &cfg
	}

	return nil
}

// normalizeProviderName converts provider aliases to canonical names.
func normalizeProviderName(name string) string {
	switch strings.ToLower(name) {
	case "gh", "git":
		return "github"
	case "gl":
		return "gitlab"
	case "nt":
		return "notion"
	case "yt":
		return "youtrack"
	default:
		return strings.ToLower(name)
	}
}

// tokenSource represents where a token value was found.
type tokenSource struct {
	Source string // Description of where token was found
	Value  string // The token value (possibly masked)
}

// detectExistingToken checks if a token is already configured.
// Returns (source, value) or ("", "") if not found.
func detectExistingToken(cfg providerLoginConfig, ws *storage.Workspace) *tokenSource {
	// 1. Check system environment variable
	if val := os.Getenv(cfg.EnvVar); val != "" {
		return &tokenSource{Source: cfg.EnvVar + " environment variable", Value: maskToken(val)}
	}

	// 2. Check .env file
	envVars, err := ws.LoadEnv()
	if err == nil {
		if val, ok := envVars[cfg.EnvVar]; ok && val != "" {
			return &tokenSource{Source: ".mehrhof/.env file", Value: maskToken(val)}
		}
	}

	// 3. Check config.yaml
	workspaceCfg, err := ws.LoadConfig()
	if err == nil {
		val := getConfigToken(workspaceCfg, cfg.ConfigField)
		if val != "" {
			return &tokenSource{Source: "config.yaml", Value: maskToken(val)}
		}
	}

	return nil
}

// getConfigToken retrieves a token from WorkspaceConfig using field path (e.g., "GitHub.Token").
func getConfigToken(cfg *storage.WorkspaceConfig, fieldPath string) string {
	parts := strings.Split(fieldPath, ".")
	if len(parts) != 2 {
		return ""
	}

	switch parts[0] {
	case "GitHub":
		if cfg.GitHub != nil && parts[1] == "Token" {
			return cfg.GitHub.Token
		}
	case "GitLab":
		if cfg.GitLab != nil && parts[1] == "Token" {
			return cfg.GitLab.Token
		}
	case "Notion":
		if cfg.Notion != nil && parts[1] == "Token" {
			return cfg.Notion.Token
		}
	case "Jira":
		if cfg.Jira != nil && parts[1] == "Token" {
			return cfg.Jira.Token
		}
	case "Linear":
		if cfg.Linear != nil && parts[1] == "Token" {
			return cfg.Linear.Token
		}
	case "Wrike":
		if cfg.Wrike != nil && parts[1] == "Token" {
			return cfg.Wrike.Token
		}
	case "YouTrack":
		if cfg.YouTrack != nil && parts[1] == "Token" {
			return cfg.YouTrack.Token
		}
	}

	return ""
}

// maskToken returns a masked version of a token for display.
func maskToken(token string) string {
	if len(token) <= 8 {
		return "*******"
	}

	return token[:4] + "..." + token[len(token)-4:]
}

// confirmOverride asks the user if they want to replace an existing token.
func confirmOverride(cmd *cobra.Command, source, maskedValue string) (bool, error) {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Token already configured via %s: %s\n", source, maskedValue)
	_, _ = fmt.Fprintf(out, "Override? [y/N]: ")

	reader := bufio.NewReader(cmd.InOrStdin())
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))

	return response == "y" || response == "yes", nil
}

// promptForToken interactively prompts the user for a token.
func promptForToken(cmd *cobra.Command, cfg providerLoginConfig) (string, error) {
	out := cmd.OutOrStdout()
	in := bufio.NewReader(cmd.InOrStdin())

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "Enter your %s API token\n", cfg.Name)
	_, _ = fmt.Fprintf(out, "Get a token at: %s\n", cfg.HelpURL)
	_, _ = fmt.Fprintln(out, "Token will be saved to .mehrhof/.env")
	_, _ = fmt.Fprint(out, "Leave empty to cancel: ")

	token, err := in.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("cancelled")
	}

	// Optional: validate token prefix
	if cfg.TokenPrefix != "" && !strings.HasPrefix(token, cfg.TokenPrefix) {
		_, _ = fmt.Fprintf(out, "Warning: Token doesn't start with expected prefix '%s'\n", cfg.TokenPrefix)
	}

	return token, nil
}

// writeTokenToEnv writes a token to the .env file, creating or updating it.
func writeTokenToEnv(envPath, key, value string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Read existing content
	var existingLines []string
	if data, err := os.ReadFile(envPath); err == nil {
		existingLines = strings.Split(string(data), "\n")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read .env: %w", err)
	}

	// Find and replace or append
	found := false
	var result strings.Builder

	for _, line := range existingLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+"=") {
			result.WriteString(key + "=" + value + "\n")
			found = true
		} else if trimmed != "" {
			result.WriteString(line + "\n")
		}
	}

	if !found {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(key + "=" + value + "\n")
	}

	// Atomic write: write to temp file then rename
	content := result.String()
	tmpPath := envPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}

	if err := os.Rename(tmpPath, envPath); err != nil {
		// Clean up temp file on error
		_ = os.Remove(tmpPath)

		return fmt.Errorf("rename .env: %w", err)
	}

	return nil
}

// runProviderLogin executes the login flow for a provider.
func runProviderLogin(providerName string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfg := getProviderLoginConfig(providerName)
		if cfg == nil {
			return fmt.Errorf("unknown provider: %s", providerName)
		}

		// Get working directory
		root, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		ws, err := storage.OpenWorkspace(root, nil)
		if err != nil {
			return fmt.Errorf("open workspace: %w", err)
		}

		// Check for existing token
		existing := detectExistingToken(*cfg, ws)

		if existing != nil {
			override, err := confirmOverride(cmd, existing.Source, existing.Value)
			if err != nil {
				return err
			}
			if !override {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")

				return nil
			}
		}

		// Prompt for token
		token, err := promptForToken(cmd, *cfg)
		if err != nil && err.Error() == "cancelled" {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")

			return nil
		}
		if err != nil {
			return err
		}

		// Write to .env
		envPath := ws.EnvPath()
		if err := writeTokenToEnv(envPath, cfg.EnvVar, token); err != nil {
			return fmt.Errorf("write token to .env: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nToken saved to %s\n", envPath)

		return nil
	}
}

// Create provider commands.
func createProviderCommands() []*cobra.Command {
	var commands []*cobra.Command

	for name := range providerLoginConfigs {
		providerName := name // Capture loop variable
		cmd := &cobra.Command{
			Use:    providerName,
			Short:  providerName + " provider commands",
			Hidden: true,
		}

		loginCmd := &cobra.Command{
			Use:    "login",
			Short:  "Authenticate with " + providerName,
			RunE:   runProviderLogin(providerName),
			Hidden: false,
		}

		cmd.AddCommand(loginCmd)
		commands = append(commands, cmd)
	}

	return commands
}

func init() {
	// Register all provider commands
	for _, cmd := range createProviderCommands() {
		rootCmd.AddCommand(cmd)
	}
}
