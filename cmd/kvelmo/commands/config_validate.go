package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/internal/cli"
	"github.com/valksor/kvelmo/settings"
)

var configValidateJSON bool

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration and check dependencies",
	Long: `Validate checks that kvelmo configuration is correct and dependencies are available.

It verifies:
  - Settings files are valid YAML
  - Required fields have valid values
  - Git is installed
  - At least one AI agent CLI is available`,
	RunE: runConfigValidate,
}

func init() {
	configValidateCmd.Flags().BoolVar(&configValidateJSON, "json", false, "Output raw JSON")
}

const (
	statusError   = "error"
	statusOK      = "ok"
	statusWarning = "warning"
)

type validateCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

type validateResult struct {
	Valid  bool            `json:"valid"`
	Checks []validateCheck `json:"checks"`
}

func runConfigValidate(_ *cobra.Command, _ []string) error {
	result := validateResult{Valid: true}

	// Check settings can be loaded.
	effective, settingsErr := loadEffectiveOffline()
	if settingsErr != nil {
		result.Valid = false
		result.Checks = append(result.Checks, validateCheck{
			Name:   "Settings",
			Status: statusError,
			Detail: settingsErr.Error(),
			Fix:    "Run 'kvelmo config init' or fix YAML syntax in config file",
		})
	} else {
		result.Checks = append(result.Checks, validateCheck{
			Name:   "Settings",
			Status: statusOK,
			Detail: "valid",
		})
	}

	// Run preflight checks (git, agent CLIs).
	preflight := agent.RunPreflight()
	for _, c := range preflight.Checks {
		status := statusOK
		switch c.Status {
		case agent.CheckPassed:
			// ok
		case agent.CheckFailed:
			status = statusError
			result.Valid = false
		case agent.CheckWarning:
			status = statusWarning
		}
		result.Checks = append(result.Checks, validateCheck{
			Name:   c.Name,
			Status: status,
			Detail: c.Detail,
			Fix:    c.Fix,
		})
	}

	// Check provider tokens (informational, not required for validity).
	providerChecks := []struct {
		name   string
		envVar string
	}{
		{"GitHub", "GITHUB_TOKEN"},
		{"GitLab", "GITLAB_TOKEN"},
		{"Linear", "LINEAR_TOKEN"},
		{"Wrike", "WRIKE_TOKEN"},
	}

	for _, p := range providerChecks {
		if token := detectExistingToken(p.envVar, settings.ScopeGlobal, ""); token != nil {
			result.Checks = append(result.Checks, validateCheck{
				Name:   p.name,
				Status: statusOK,
				Detail: "token configured",
			})
		} else {
			result.Checks = append(result.Checks, validateCheck{
				Name:   p.name,
				Status: statusWarning,
				Detail: "not configured",
				Fix:    fmt.Sprintf("Set %s or run 'kvelmo provider login %s'", p.envVar, p.name),
			})
		}
	}

	// Check agent default is valid if settings loaded.
	if effective != nil && effective.Agent.Default != "" {
		allowed := []string{"claude", "codex", "openai", "anthropic", "ollama"}
		valid := slices.Contains(allowed, effective.Agent.Default)
		// Also allow custom agents.
		if !valid {
			if _, ok := effective.CustomAgents[effective.Agent.Default]; ok {
				valid = true
			}
		}
		if !valid {
			result.Valid = false
			result.Checks = append(result.Checks, validateCheck{
				Name:   "agent.default",
				Status: statusError,
				Detail: fmt.Sprintf("unknown agent %q", effective.Agent.Default),
				Fix:    "Set agent.default to 'claude', 'codex', 'openai', 'anthropic', or 'ollama'",
			})
		}
	}

	if configValidateJSON {
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		fmt.Println(string(out))

		return nil
	}

	if cli.Quiet {
		if !result.Valid {
			os.Exit(1)
		}

		return nil
	}

	// Formatted output.
	fmt.Println()
	fmt.Println("  Configuration Validation")
	fmt.Println("  ─────────────────────────────────────")
	fmt.Println()

	for _, c := range result.Checks {
		symbol := "✓"
		switch c.Status {
		case statusError:
			symbol = "✗"
		case statusWarning:
			symbol = "⚠"
		}

		detail := ""
		if c.Detail != "" {
			detail = " (" + c.Detail + ")"
		}

		displayName := c.Name
		switch c.Name {
		case "git":
			displayName = "Git"
		case "claude":
			displayName = "Claude CLI"
		case "claude-auth":
			displayName = "Claude Auth"
		case "codex":
			displayName = "Codex CLI"
		case "openai":
			displayName = "OpenAI API"
		case "anthropic":
			displayName = "Anthropic API"
		case "ollama":
			displayName = "Ollama"
		}

		fmt.Printf("  %-14s %s %s%s\n", displayName+":", symbol, c.Status, detail)
	}

	fmt.Println()

	if result.Valid {
		fmt.Println("  ✓ Configuration is valid!")
	} else {
		fmt.Println("  ✗ Configuration has errors:")
		for _, c := range result.Checks {
			if c.Status == statusError && c.Fix != "" {
				fmt.Printf("    • %s\n", c.Fix)
			}
		}
	}
	fmt.Println()

	return nil
}
