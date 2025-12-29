package commands

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List and manage task providers",
	Long: `List available task providers and show their configuration requirements.

Providers are sources for tasks - files, directories, GitHub issues, Jira tickets, etc.`,
}

var providersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available providers",
	Long:  `List all available task providers with their schemes and descriptions.`,
	RunE:  runProvidersList,
}

var providersInfoCmd = &cobra.Command{
	Use:   "info <provider>",
	Short: "Show provider information",
	Long:  `Show detailed information about a specific provider including setup requirements.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runProvidersInfo,
}

func init() {
	rootCmd.AddCommand(providersCmd)
	providersCmd.AddCommand(providersListCmd)
	providersCmd.AddCommand(providersInfoCmd)
}

func runProvidersList(cmd *cobra.Command, args []string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SCHEME\tPROVIDER\tDESCRIPTION")
	_, _ = fmt.Fprintln(w, "------\t--------\t-----------")

	providers := []struct {
		scheme      string
		name        string
		description string
	}{
		{"file", "File", "Single markdown file"},
		{"dir", "Directory", "Directory with README.md"},
		{"github", "GitHub", "GitHub issues and pull requests"},
		{"gitlab", "GitLab", "GitLab issues and merge requests"},
		{"jira", "Jira", "Atlassian Jira tickets"},
		{"linear", "Linear", "Linear issues"},
		{"notion", "Notion", "Notion pages and databases"},
		{"wrike", "Wrike", "Wrike tasks"},
		{"youtrack", "YouTrack", "JetBrains YouTrack issues"},
	}

	for _, p := range providers {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", p.scheme, p.name, p.description)
	}

	_ = w.Flush()

	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  mehr start <scheme>:<reference>  # Use provider with scheme")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  mehr start file:task.md")
	fmt.Println("  mehr start dir:./tasks/")
	fmt.Println("  mehr start github:owner/repo#123")
	fmt.Println("  mehr start jira:PROJECT-123")

	return nil
}

func runProvidersInfo(cmd *cobra.Command, args []string) error {
	providerName := strings.ToLower(args[0])

	info := getProviderInfo(providerName)
	if info == nil {
		fmt.Printf("Unknown provider: %s\n\n", providerName)
		fmt.Println("Run 'mehr providers list' to see available providers.")
		return nil
	}

	fmt.Printf("Provider: %s\n\n", info.Name)
	fmt.Printf("Scheme: %s\n\n", info.Scheme)
	fmt.Printf("Description:\n  %s\n\n", info.Description)

	if len(info.Setup) > 0 {
		fmt.Println("Setup:")
		for _, step := range info.Setup {
			fmt.Printf("  %s\n", step)
		}
		fmt.Println()
	}

	if len(info.EnvVars) > 0 {
		fmt.Println("Required environment variables:")
		for _, env := range info.EnvVars {
			fmt.Printf("  %s\n", env)
		}
		fmt.Println()
	}

	if len(info.Config) > 0 {
		fmt.Println("Configuration (in .mehrhof/config.yaml):")
		for _, cfg := range info.Config {
			fmt.Printf("  %s\n", cfg)
		}
		fmt.Println()
	}

	fmt.Printf("Usage:\n  %s\n\n", info.Usage)

	return nil
}

type providerInfo struct {
	Name        string
	Scheme      string
	Description string
	Setup       []string
	EnvVars     []string
	Config      []string
	Usage       string
}

func getProviderInfo(name string) *providerInfo {
	switch name {
	case "file", "f":
		return &providerInfo{
			Name:        "File Provider",
			Scheme:      "file",
			Description: "Load tasks from individual markdown files",
			Usage:       "mehr start file:path/to/task.md",
		}

	case "dir", "directory", "d":
		return &providerInfo{
			Name:        "Directory Provider",
			Scheme:      "dir",
			Description: "Load tasks from directories (looks for README.md)",
			Usage:       "mehr start dir:./tasks/",
		}

	case "github", "gh", "git":
		return &providerInfo{
			Name:        "GitHub Provider",
			Scheme:      "github",
			Description: "Load tasks from GitHub issues and PRs",
			EnvVars:     []string{"GITHUB_TOKEN"},
			Config: []string{
				"github:",
				"  token: \"${GITHUB_TOKEN}\"",
				"  owner: \"your-org\"",
				"  repo: \"your-repo\"",
			},
			Usage: "mehr start github:owner/repo#123",
		}

	case "jira":
		return &providerInfo{
			Name:        "Jira Provider",
			Scheme:      "jira",
			Description: "Load tasks from Atlassian Jira",
			EnvVars:     []string{"JIRA_TOKEN"},
			Config: []string{
				"jira:",
				"  url: \"https://your-domain.atlassian.net\"",
				"  token: \"${JIRA_TOKEN}\"",
			},
			Usage: "mehr start jira:PROJECT-123",
		}

	case "linear":
		return &providerInfo{
			Name:        "Linear Provider",
			Scheme:      "linear",
			Description: "Load tasks from Linear",
			EnvVars:     []string{"LINEAR_API_KEY"},
			Config: []string{
				"linear:",
				"  api_key: \"${LINEAR_API_KEY}\"",
			},
			Usage: "mehr start linear:ABC-123",
		}

	case "notion":
		return &providerInfo{
			Name:        "Notion Provider",
			Scheme:      "notion",
			Description: "Load tasks from Notion pages and databases",
			EnvVars:     []string{"NOTION_TOKEN"},
			Config: []string{
				"notion:",
				"  token: \"${NOTION_TOKEN}\"",
			},
			Usage: "mehr start notion:a1b2c3d4e5f678901234567890abcdef",
		}

	case "youtrack", "yt":
		return &providerInfo{
			Name:        "YouTrack Provider",
			Scheme:      "youtrack",
			Description: "Load tasks from JetBrains YouTrack",
			EnvVars:     []string{"YOUTRACK_TOKEN"},
			Config: []string{
				"youtrack:",
				"  host: \"https://your-domain.youtrack.cloud\"",
				"  token: \"${YOUTRACK_TOKEN}\"",
			},
			Usage: "mehr start youtrack:PROJECT-123",
		}

	case "wrike":
		return &providerInfo{
			Name:        "Wrike Provider",
			Scheme:      "wrike",
			Description: "Load tasks from Wrike",
			EnvVars:     []string{"WRIKE_TOKEN"},
			Config: []string{
				"wrike:",
				"  token: \"${WRIKE_TOKEN}\"",
			},
			Usage: "mehr start wrike:TASK-123",
		}

	default:
		return nil
	}
}
