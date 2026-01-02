package commands

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/plugin"
	"github.com/valksor/go-mehrhof/internal/storage"
)

var pluginGlobal bool // --global flag for install/remove

var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Manage plugins",
	Long: `Manage provider, agent, and workflow plugins.

Plugins extend mehr with custom integrations:
  - Providers: Jira, YouTrack, Linear, etc.
  - Agents: Codex, custom AI models, etc.
  - Workflows: Custom phases, guards, effects

Plugins must be explicitly enabled in .mehrhof/config.yaml:

  plugins:
    enabled:
      - jira
      - youtrack
    config:
      jira:
        url: "https://company.atlassian.net"`,
}

var pluginsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered plugins",
	Long: `List all discovered plugins from global and project directories.

Plugin locations:
  Global:  ~/.mehrhof/plugins/
  Project: .mehrhof/plugins/

The output shows:
  - NAME: Plugin identifier
  - TYPE: provider, agent, or workflow
  - SCOPE: global or project (project overrides global)
  - ENABLED: Whether the plugin is enabled in config
  - DESCRIPTION: Human-readable description

Examples:
  mehr plugins list`,
	RunE: runPluginsList,
}

var pluginsInstallCmd = &cobra.Command{
	Use:   "install <source>",
	Short: "Install a plugin",
	Long: `Install a plugin from a git repository or local path.

Sources:
  - Git URL: https://github.com/user/mehrhof-jira
  - Local path: ./my-plugin

Flags:
  --global    Install to ~/.mehrhof/plugins/ (default: .mehrhof/plugins/)

Examples:
  mehr plugins install https://github.com/user/mehrhof-jira
  mehr plugins install ./my-plugin --global`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginsInstall,
}

var pluginsRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a plugin",
	Long: `Remove an installed plugin by name.

This deletes the plugin directory. Make sure to also remove
the plugin from 'plugins.enabled' in config.yaml.

Flags:
  --global    Remove from ~/.mehrhof/plugins/ (default: .mehrhof/plugins/)

Examples:
  mehr plugins remove jira
  mehr plugins remove jira --global`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginsRemove,
}

var pluginsValidateCmd = &cobra.Command{
	Use:   "validate [name]",
	Short: "Validate plugin manifest and connectivity",
	Long: `Validate a plugin's manifest and test that it can be loaded.

If no name is provided, validates all discovered plugins.

Examples:
  mehr plugins validate jira
  mehr plugins validate`,
	RunE: runPluginsValidate,
}

var pluginsInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show detailed plugin information",
	Long: `Show detailed information about a specific plugin.

Examples:
  mehr plugins info jira`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginsInfo,
}

func init() {
	rootCmd.AddCommand(pluginsCmd)
	pluginsCmd.AddCommand(pluginsListCmd)
	pluginsCmd.AddCommand(pluginsInstallCmd)
	pluginsCmd.AddCommand(pluginsRemoveCmd)
	pluginsCmd.AddCommand(pluginsValidateCmd)
	pluginsCmd.AddCommand(pluginsInfoCmd)

	pluginsInstallCmd.Flags().BoolVar(&pluginGlobal, "global", false, "Install to global plugins directory")
	pluginsRemoveCmd.Flags().BoolVar(&pluginGlobal, "global", false, "Remove from global plugins directory")
}

func runPluginsList(cmd *cobra.Command, args []string) error {
	discovery, err := getPluginDiscovery()
	if err != nil {
		return err
	}

	manifests, err := discovery.Discover()
	if err != nil {
		return fmt.Errorf("discover plugins: %w", err)
	}

	if len(manifests) == 0 {
		fmt.Println("No plugins discovered.")
		fmt.Println()
		fmt.Printf("Plugin locations:\n")
		fmt.Printf("  Global:  %s\n", discovery.GlobalDir())
		fmt.Printf("  Project: %s\n", discovery.ProjectDir())
		return nil
	}

	// Load workspace config to check enabled status
	ws, _ := storage.OpenWorkspace(".", nil)
	var cfg *storage.WorkspaceConfig
	if ws != nil {
		cfg, _ = ws.LoadConfig()
	}

	enabledSet := make(map[string]bool)
	if cfg != nil {
		for _, name := range cfg.Plugins.Enabled {
			enabledSet[name] = true
		}
	}

	// Sort manifests by name
	slices.SortFunc(manifests, func(a, b *plugin.Manifest) int {
		return cmp.Compare(a.Name, b.Name)
	})

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tTYPE\tSCOPE\tENABLED\tDESCRIPTION")

	for _, m := range manifests {
		enabled := "no"
		if enabledSet[m.Name] {
			enabled = "yes"
		}

		desc := m.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		if desc == "" {
			desc = "-"
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			m.Name,
			m.Type,
			m.Scope,
			enabled,
			desc)
	}

	_ = w.Flush()

	fmt.Println()
	fmt.Println("Enable plugins in .mehrhof/config.yaml under 'plugins.enabled:'")

	return nil
}

func runPluginsInstall(cmd *cobra.Command, args []string) error {
	source := args[0]

	// Determine target directory
	var targetDir string
	if pluginGlobal {
		globalDir, err := plugin.DefaultGlobalDir()
		if err != nil {
			return fmt.Errorf("get global plugins dir: %w", err)
		}
		targetDir = globalDir
	} else {
		targetDir = plugin.DefaultProjectDir(".")
	}

	// Ensure target directory exists
	if err := plugin.EnsureDir(targetDir); err != nil {
		return fmt.Errorf("create plugins directory: %w", err)
	}

	// Check if source is a git URL or local path
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "git@") {
		return installFromGit(source, targetDir)
	}

	return installFromLocal(source, targetDir)
}

func installFromGit(url, targetDir string) error {
	// Extract plugin name from URL
	name := filepath.Base(url)
	name = strings.TrimSuffix(name, ".git")

	// Remove "mehrhof-" prefix if present
	name = strings.TrimPrefix(name, "mehrhof-")

	pluginDir := filepath.Join(targetDir, name)

	// Check if already installed
	if _, err := os.Stat(pluginDir); err == nil {
		return fmt.Errorf("plugin %s already installed at %s", name, pluginDir)
	}

	fmt.Printf("Installing plugin '%s' from %s...\n", name, url)

	// Clone repository
	gitCmd := exec.CommandContext(context.Background(), "git", "clone", "--depth", "1", url, pluginDir)
	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr

	if err := gitCmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Validate the installed plugin
	manifestPath := filepath.Join(pluginDir, plugin.ManifestFileName)
	if _, err := plugin.LoadManifest(manifestPath); err != nil {
		// Clean up on validation failure
		_ = os.RemoveAll(pluginDir)
		return fmt.Errorf("invalid plugin (missing or invalid plugin.yaml): %w", err)
	}

	fmt.Printf("Plugin '%s' installed successfully.\n", name)
	fmt.Printf("Enable it by adding '%s' to plugins.enabled in .mehrhof/config.yaml\n", name)

	return nil
}

func installFromLocal(sourcePath, targetDir string) error {
	// Resolve source path
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}

	// Validate source has a manifest
	sourceManifest := filepath.Join(absSource, plugin.ManifestFileName)
	manifest, err := plugin.LoadManifest(sourceManifest)
	if err != nil {
		return fmt.Errorf("invalid plugin source: %w", err)
	}

	pluginDir := filepath.Join(targetDir, manifest.Name)

	// Check if already installed
	if _, err := os.Stat(pluginDir); err == nil {
		return fmt.Errorf("plugin %s already installed at %s", manifest.Name, pluginDir)
	}

	fmt.Printf("Installing plugin '%s' from %s...\n", manifest.Name, absSource)

	// Copy directory
	if err := copyDir(absSource, pluginDir); err != nil {
		return fmt.Errorf("copy plugin: %w", err)
	}

	fmt.Printf("Plugin '%s' installed successfully.\n", manifest.Name)
	fmt.Printf("Enable it by adding '%s' to plugins.enabled in .mehrhof/config.yaml\n", manifest.Name)

	return nil
}

func runPluginsRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	discovery, err := getPluginDiscovery()
	if err != nil {
		return err
	}

	// Find the plugin
	manifest, err := discovery.DiscoverByName(name)
	if err != nil {
		return fmt.Errorf("find plugin: %w", err)
	}
	if manifest == nil {
		return fmt.Errorf("plugin '%s' not found", name)
	}

	// Check scope matches flag
	if pluginGlobal && manifest.Scope != plugin.ScopeGlobal {
		return fmt.Errorf("plugin '%s' is not in global directory (found in %s)", name, manifest.Scope)
	}
	if !pluginGlobal && manifest.Scope != plugin.ScopeProject {
		return fmt.Errorf("plugin '%s' is not in project directory (found in %s), use --global flag", name, manifest.Scope)
	}

	fmt.Printf("Removing plugin '%s' from %s...\n", name, manifest.Dir)

	if err := os.RemoveAll(manifest.Dir); err != nil {
		return fmt.Errorf("remove plugin directory: %w", err)
	}

	fmt.Printf("Plugin '%s' removed.\n", name)
	fmt.Println("Remember to remove it from plugins.enabled in .mehrhof/config.yaml if present.")

	return nil
}

func runPluginsValidate(cmd *cobra.Command, args []string) error {
	discovery, err := getPluginDiscovery()
	if err != nil {
		return err
	}

	var manifests []*plugin.Manifest

	if len(args) > 0 {
		// Validate specific plugin
		manifest, err := discovery.DiscoverByName(args[0])
		if err != nil {
			return fmt.Errorf("find plugin: %w", err)
		}
		if manifest == nil {
			return fmt.Errorf("plugin '%s' not found", args[0])
		}
		manifests = []*plugin.Manifest{manifest}
	} else {
		// Validate all plugins
		var err error
		manifests, err = discovery.Discover()
		if err != nil {
			return fmt.Errorf("discover plugins: %w", err)
		}
	}

	if len(manifests) == 0 {
		fmt.Println("No plugins to validate.")
		return nil
	}

	loader := plugin.NewLoader()
	ctx := context.Background()
	hasErrors := false

	for _, m := range manifests {
		fmt.Printf("Validating '%s'...\n", m.Name)

		// Check executable exists
		execPath := m.ExecutablePath()
		if execPath != "" {
			if _, err := os.Stat(execPath); err != nil {
				fmt.Printf("  ERROR: Executable not found: %s\n", execPath)
				hasErrors = true
				continue
			}
		}

		// Try to load and initialize
		proc, err := loader.Load(ctx, m)
		if err != nil {
			fmt.Printf("  ERROR: Failed to load: %v\n", err)
			hasErrors = true
			continue
		}

		// Try to call init
		initMethod := "provider.init"
		switch m.Type {
		case plugin.PluginTypeAgent:
			initMethod = "agent.init"
		case plugin.PluginTypeProvider:
			initMethod = "provider.init"
		case plugin.PluginTypeWorkflow:
			initMethod = "workflow.init"
		}

		_, err = proc.Call(ctx, initMethod, &plugin.InitParams{Config: make(map[string]any)})
		if err != nil {
			fmt.Printf("  ERROR: Init failed: %v\n", err)
			_ = proc.Stop()
			hasErrors = true
			continue
		}

		_ = proc.Stop()
		fmt.Printf("  OK\n")
	}

	if hasErrors {
		return fmt.Errorf("some plugins failed validation")
	}

	fmt.Println("\nAll plugins validated successfully.")
	return nil
}

func runPluginsInfo(cmd *cobra.Command, args []string) error {
	name := args[0]

	discovery, err := getPluginDiscovery()
	if err != nil {
		return err
	}

	manifest, err := discovery.DiscoverByName(name)
	if err != nil {
		return fmt.Errorf("find plugin: %w", err)
	}
	if manifest == nil {
		return fmt.Errorf("plugin '%s' not found", name)
	}

	fmt.Printf("Name:        %s\n", manifest.Name)
	fmt.Printf("Type:        %s\n", manifest.Type)
	fmt.Printf("Version:     %s\n", manifest.Version)
	fmt.Printf("Protocol:    %s\n", manifest.Protocol)
	fmt.Printf("Description: %s\n", manifest.Description)
	fmt.Printf("Scope:       %s\n", manifest.Scope)
	fmt.Printf("Directory:   %s\n", manifest.Dir)

	if manifest.Author != "" {
		fmt.Printf("Author:      %s\n", manifest.Author)
	}
	if manifest.Homepage != "" {
		fmt.Printf("Homepage:    %s\n", manifest.Homepage)
	}
	if manifest.Requires != "" {
		fmt.Printf("Requires:    %s\n", manifest.Requires)
	}

	// Type-specific info
	switch manifest.Type {
	case plugin.PluginTypeProvider:
		if manifest.Provider != nil {
			fmt.Println("\nProvider Configuration:")
			fmt.Printf("  Schemes:      %s\n", strings.Join(manifest.Provider.Schemes, ", "))
			fmt.Printf("  Priority:     %d\n", manifest.Provider.Priority)
			fmt.Printf("  Capabilities: %s\n", strings.Join(manifest.Provider.Capabilities, ", "))
		}
	case plugin.PluginTypeAgent:
		if manifest.Agent != nil {
			fmt.Println("\nAgent Configuration:")
			fmt.Printf("  Streaming: %v\n", manifest.Agent.Streaming)
			if len(manifest.Agent.Capabilities) > 0 {
				fmt.Printf("  Capabilities: %s\n", strings.Join(manifest.Agent.Capabilities, ", "))
			}
		}
	case plugin.PluginTypeWorkflow:
		if manifest.Workflow != nil {
			fmt.Println("\nWorkflow Configuration:")
			if len(manifest.Workflow.Phases) > 0 {
				fmt.Println("  Phases:")
				for _, p := range manifest.Workflow.Phases {
					fmt.Printf("    - %s: %s\n", p.Name, p.Description)
				}
			}
			if len(manifest.Workflow.Guards) > 0 {
				fmt.Println("  Guards:")
				for _, g := range manifest.Workflow.Guards {
					fmt.Printf("    - %s: %s\n", g.Name, g.Description)
				}
			}
			if len(manifest.Workflow.Effects) > 0 {
				fmt.Println("  Effects:")
				for _, e := range manifest.Workflow.Effects {
					fmt.Printf("    - %s: %s\n", e.Name, e.Description)
				}
			}
		}
	}

	// Environment variables
	if len(manifest.Env) > 0 {
		fmt.Println("\nExpected Environment Variables:")
		for name, spec := range manifest.Env {
			required := ""
			if spec.Required {
				required = " (required)"
			}
			fmt.Printf("  %s%s\n", name, required)
			fmt.Printf("    %s\n", spec.Description)
		}
	}

	return nil
}

// getPluginDiscovery creates a plugin discovery instance.
func getPluginDiscovery() (*plugin.Discovery, error) {
	globalDir, err := plugin.DefaultGlobalDir()
	if err != nil {
		return nil, fmt.Errorf("get global plugins dir: %w", err)
	}

	// Get current working directory for project plugins
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	projectDir := plugin.DefaultProjectDir(cwd)

	return plugin.NewDiscovery(globalDir, projectDir), nil
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, info.Mode())
	})
}
