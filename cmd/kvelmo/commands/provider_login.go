package commands

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/provider"
	"github.com/valksor/kvelmo/pkg/settings"
	"golang.org/x/term"
)

// providerLoginConfigs is the canonical provider login configuration from pkg/provider.
var providerLoginConfigs = provider.LoginConfigs

// tokenSource represents where a token value was found.
type tokenSource struct {
	Source string // Description of where the token was found
	Value  string // The masked token value
}

// detectExistingToken checks if a token is already configured.
func detectExistingToken(envVar string, scope settings.Scope, projectRoot string) *tokenSource {
	// Load env map to check .env files
	root := ""
	if scope == settings.ScopeProject {
		root = projectRoot
	}
	envMap, err := settings.LoadEnvMap(root)
	if err != nil {
		return nil
	}

	if val := envMap.Get(envVar); val != "" {
		// Determine source path for display
		var envPath string
		if scope == settings.ScopeProject && projectRoot != "" {
			envPath = settings.ProjectEnvPath(projectRoot)
		} else {
			envPath, _ = settings.GlobalEnvPath()
		}

		return &tokenSource{
			Source: envPath,
			Value:  settings.MaskToken(val),
		}
	}

	return nil
}

// readToken reads a token from stdin, using secure input when available.
func readToken(prompt string) (string, error) {
	fmt.Print(prompt)

	// Check if stdin is a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Non-interactive: read from stdin
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}

		return strings.TrimSpace(line), nil
	}

	// Interactive: use secure password input
	tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // Move to next line after password entry
	if err != nil {
		return "", fmt.Errorf("read token: %w", err)
	}

	return strings.TrimSpace(string(tokenBytes)), nil
}

// confirmOverride asks the user if they want to replace an existing token.
func confirmOverride(cmd *cobra.Command) bool {
	_, _ = fmt.Fprint(cmd.OutOrStdout(), "Override? [y/N]: ")

	reader := bufio.NewReader(cmd.InOrStdin())
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))

	return response == "y" || response == "yes"
}

// printTokenHelp displays formatted guidance for getting a token.
func printTokenHelp(w io.Writer, cfg provider.LoginConfig) {
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "%s Token Setup\n", cfg.Name)
	_, _ = fmt.Fprintln(w, "--------------------------------------------------")
	_, _ = fmt.Fprintf(w, "Get token: %s\n", cfg.HelpURL)
	if cfg.HelpSteps != "" {
		_, _ = fmt.Fprintf(w, "Steps:     %s\n", cfg.HelpSteps)
	}
	if cfg.Scopes != "" {
		_, _ = fmt.Fprintf(w, "Required:  %s\n", cfg.Scopes)
	}
	if cfg.TokenPrefix != "" {
		_, _ = fmt.Fprintf(w, "Format:    Token starts with '%s'\n", cfg.TokenPrefix)
	}
	_, _ = fmt.Fprintln(w, "--------------------------------------------------")
	_, _ = fmt.Fprintln(w)
}

// runProviderLogin executes the login flow for a provider.
func runProviderLogin(providerName string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		cfg, ok := providerLoginConfigs[providerName]
		if !ok {
			return fmt.Errorf("unknown provider: %s", providerName)
		}

		// Determine scope
		useProject, _ := cmd.Flags().GetBool("project")
		scope := settings.ScopeGlobal
		projectRoot := ""

		if useProject {
			scope = settings.ScopeProject
			var err error
			projectRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}
		}

		// Check for existing token
		existing := detectExistingToken(cfg.EnvVar, scope, projectRoot)
		if existing != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Token already configured via %s: %s\n", existing.Source, existing.Value)
			if !confirmOverride(cmd) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")

				return nil
			}
		}

		// Print help
		printTokenHelp(cmd.OutOrStdout(), cfg)

		// Prompt for token
		token, err := readToken(fmt.Sprintf("Enter your %s API token: ", cfg.Name))
		if err != nil {
			return err
		}

		if token == "" {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")

			return nil
		}

		// Optional: warn about token prefix (informational only)
		if cfg.TokenPrefix != "" && !strings.HasPrefix(token, cfg.TokenPrefix) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Note: Token doesn't start with expected prefix '%s'\n", cfg.TokenPrefix)
		}

		// Validate token with API call
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Validating token...")
		if err := testProviderToken(providerName, token); errors.Is(err, errValidationSkipped) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), " skipped (requires org context)\n")
		} else if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), " ✗\n")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: Token validation failed: %v\n", err)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "The token will be saved but may not work.\n")
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), " ✓\n")
		}

		// Save token
		if err := settings.SaveEnvVar(scope, projectRoot, cfg.EnvVar, token); err != nil {
			return fmt.Errorf("save token: %w", err)
		}

		// Print success
		var envPath string
		if scope == settings.ScopeProject {
			envPath = settings.ProjectEnvPath(projectRoot)
		} else {
			envPath, _ = settings.GlobalEnvPath()
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nToken saved to %s\n", envPath)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Use '%s start <task>' to begin working.\n", meta.Name)

		return nil
	}
}

// createProviderCommand creates a provider command with a login subcommand.
func createProviderCommand(providerName string) *cobra.Command {
	cfg := providerLoginConfigs[providerName]

	providerCmd := &cobra.Command{
		Use:   providerName,
		Short: cfg.Name + " provider commands",
	}

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with " + cfg.Name,
		Long: fmt.Sprintf(`Authenticate with %s by providing an API token.

The token is securely stored in a .env file:
  - Global (default): ~/.valksor/%s/.env
  - Project (--project): .valksor/.env

Get your token at: %s`, cfg.Name, meta.Name, cfg.HelpURL),
		RunE: runProviderLogin(providerName),
	}

	loginCmd.Flags().Bool("project", false, "Save token to project .valksor/.env instead of global")

	providerCmd.AddCommand(loginCmd)

	return providerCmd
}

var errValidationSkipped = errors.New("validation skipped")

// testProviderToken validates a token by making a simple API call.
func testProviderToken(provider, token string) error {
	ctx := context.Background()
	client := &http.Client{Timeout: 10 * time.Second}

	var req *http.Request
	var err error

	switch provider {
	case "github":
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")

	case "gitlab":
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://gitlab.com/api/v4/user", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Private-Token", token)

	case "linear":
		body := []byte(`{"query":"{ viewer { id } }"}`)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, "https://api.linear.app/graphql", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", token)
		req.Header.Set("Content-Type", "application/json")

	case "wrike":
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://www.wrike.com/api/v4/contacts?me=true", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)

	case "jira":
		return errValidationSkipped // Jira Cloud uses Basic auth with email:token; cannot validate token alone

	case "azuredevops":
		return errValidationSkipped // Azure DevOps requires org/project context for validation

	default:
		return nil // Unknown provider, skip validation
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		_, _ = io.Copy(io.Discard, resp.Body) // Drain body for connection reuse

		return fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		_, _ = io.Copy(io.Discard, resp.Body) // Drain body for connection reuse

		return fmt.Errorf("API error (HTTP %d)", resp.StatusCode)
	}

	// For Linear, check GraphQL response for errors
	if provider == "linear" {
		var result struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && len(result.Errors) > 0 {
			return fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
		}
	}

	return nil
}

// Provider commands exported for registration in main.go.
var (
	GitHubCmd      = createProviderCommand("github")
	GitLabCmd      = createProviderCommand("gitlab")
	LinearCmd      = createProviderCommand("linear")
	WrikeCmd       = createProviderCommand("wrike")
	JiraCmd        = createProviderCommand("jira")
	AzureDevOpsCmd = createProviderCommand("azuredevops")
)
