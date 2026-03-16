package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/paths"
	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/socket"
)

var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage " + meta.Name + " configuration",
	Long:  "View and modify " + meta.Name + " configuration settings.",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current effective configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Try socket first; fall back to reading files directly.
		if data, err := configShowViaSocket(); err == nil {
			fmt.Println(string(data))

			return nil
		}

		return configShowOffline()
	},
}

// configShowViaSocket fetches effective config from the running server.
func configShowViaSocket() ([]byte, error) {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	resp, err := client.Call(ctx, "settings.get", nil)
	if err != nil {
		return nil, fmt.Errorf("settings.get: %w", err)
	}

	var result struct {
		Effective json.RawMessage `json:"effective"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return json.MarshalIndent(result.Effective, "", "  ")
}

// configShowOffline reads config files directly and prints the effective config.
func configShowOffline() error {
	effective, err := loadEffectiveOffline()
	if err != nil {
		return err
	}

	pretty, err := json.MarshalIndent(effective, "", "  ")
	if err != nil {
		return fmt.Errorf("format: %w", err)
	}
	fmt.Println(string(pretty))

	return nil
}

// loadEffectiveOffline loads effective settings by reading YAML files directly.
func loadEffectiveOffline() (*settings.Settings, error) {
	effective := settings.DefaultSettings()

	global, err := settings.LoadGlobal()
	if err != nil {
		return nil, fmt.Errorf("load global config: %w", err)
	}
	if global != nil {
		settings.Merge(effective, global)
	}

	// Try to load project config from current directory.
	cwd, err := os.Getwd()
	if err == nil {
		project, projErr := settings.LoadProject(cwd)
		if projErr == nil && project != nil {
			settings.Merge(effective, project)
		}
	}

	return effective, nil
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show global configuration file path",
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot determine home directory:", err)

			return
		}
		fmt.Println(filepath.Join(home, meta.GlobalDir, meta.ConfigFile))
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <path>",
	Short: "Get a configuration value (e.g. workers.max, git.branch_pattern)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		// Try socket first; fall back to reading files directly.
		if value, err := configGetViaSocket(path); err == nil {
			printConfigValue(value)

			return nil
		}

		return configGetOffline(path)
	},
}

// configGetViaSocket fetches a config value from the running server.
func configGetViaSocket(path string) (any, error) {
	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	resp, err := client.Call(ctx, "settings.get", nil)
	if err != nil {
		return nil, fmt.Errorf("settings.get: %w", err)
	}

	var result struct {
		Effective map[string]any `json:"effective"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return nestedGet(result.Effective, path)
}

// configGetOffline reads config files directly and extracts the requested value.
func configGetOffline(path string) error {
	effective, err := loadEffectiveOffline()
	if err != nil {
		return err
	}

	// Convert to map[string]any for dot-notation lookup.
	data, err := json.Marshal(effective)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	value, err := nestedGet(m, path)
	if err != nil {
		return err
	}

	printConfigValue(value)

	return nil
}

// printConfigValue prints a config value in a human-friendly format.
func printConfigValue(value any) {
	switch v := value.(type) {
	case string:
		fmt.Println(v)
	case bool:
		fmt.Println(v)
	default:
		data, err := json.Marshal(value)
		if err != nil {
			fmt.Println(value)

			return
		}
		fmt.Println(string(data))
	}
}

var configSetCmd = &cobra.Command{
	Use:   "set <path> <value>",
	Short: "Set a global configuration value (e.g. workers.max 5, git.auto_commit true)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, raw := args[0], args[1]

		client, ctx, cancel, err := globalSocketClient()
		if err != nil {
			return err
		}
		defer func() { _ = client.Close() }()
		defer cancel()

		// Coerce the value: try JSON decode first (handles numbers and booleans),
		// then fall back to treating the raw string as a plain string value.
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			value = raw
		}

		params := map[string]any{
			"scope":  "global",
			"values": map[string]any{path: value},
		}
		if _, err := client.Call(ctx, "settings.set", params); err != nil {
			return fmt.Errorf("settings.set: %w", err)
		}

		fmt.Printf("Set %s = %s\n", path, raw)

		return nil
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize global configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, cancel, err := globalSocketClient()
		if err != nil {
			return err
		}
		defer func() { _ = client.Close() }()
		defer cancel()

		values := map[string]any{}

		// Apply preset defaults if requested.
		if configInitPreset != "" {
			preset := settings.ApplyPreset(configInitPreset)
			if preset == nil {
				return fmt.Errorf("unknown preset %q (available: %v)", configInitPreset, settings.PresetNames)
			}

			// Convert preset to map[string]any via JSON round-trip.
			data, err := json.Marshal(preset)
			if err != nil {
				return fmt.Errorf("marshal preset: %w", err)
			}
			if err := json.Unmarshal(data, &values); err != nil {
				return fmt.Errorf("unmarshal preset: %w", err)
			}

			// Record which preset was used.
			values["preset"] = configInitPreset
		}

		params := map[string]any{
			"scope":  "global",
			"values": values,
		}
		if _, err := client.Call(ctx, "settings.set", params); err != nil {
			return fmt.Errorf("settings.set: %w", err)
		}

		home, _ := os.UserHomeDir()
		fmt.Printf("Configuration initialized at %s\n", filepath.Join(home, meta.GlobalDir, meta.ConfigFile))
		if configInitPreset != "" {
			fmt.Printf("Applied preset: %s\n", configInitPreset)
		}

		return nil
	},
}

var (
	configEditGlobal  bool
	configEditProject bool
	configInitPreset  string
)

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open config file in editor",
	RunE:  runConfigEdit,
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		return errors.New("$EDITOR not set")
	}

	var configPath string
	if configEditProject {
		// Project config: .valksor/kvelmo.yaml in current directory.
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		configPath = settings.ProjectPath(cwd)
	} else {
		// Default to global: ~/.valksor/kvelmo/kvelmo.yaml.
		configPath = paths.ConfigPath()
	}

	// Create file if it doesn't exist.
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte("# "+meta.Name+" configuration\n"), 0o644); err != nil {
			return fmt.Errorf("create config file: %w", err)
		}
	}

	// Resolve editor to an absolute path to satisfy security linters.
	editorPath, err := exec.LookPath(editor)
	if err != nil {
		return fmt.Errorf("editor %q not found in PATH: %w", editor, err)
	}

	// Open editor.
	editorCmd := exec.CommandContext(cmd.Context(), editorPath, configPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	return editorCmd.Run()
}

// globalSocketClient connects to the global socket and returns a client, context, and cancel func.
func globalSocketClient() (*socket.Client, context.Context, context.CancelFunc, error) {
	gPath := socket.GlobalSocketPath()
	if !socket.SocketExists(gPath) {
		return nil, nil, nil, errors.New(meta.Name + " server not running\nRun '" + meta.Name + " serve' first")
	}
	client, err := socket.NewClient(gPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("connect to server: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	return client, ctx, cancel, nil
}

// nestedGet extracts a value from a nested map using a dot-notation path.
func nestedGet(m map[string]any, path string) (any, error) {
	parts := strings.SplitN(path, ".", 2)
	val, ok := m[parts[0]]
	if !ok {
		return nil, fmt.Errorf("unknown configuration path: %s", path)
	}
	if len(parts) == 1 {
		return val, nil
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unknown configuration path: %s", path)
	}

	return nestedGet(nested, parts[1])
}

func init() {
	configEditCmd.Flags().BoolVar(&configEditGlobal, "global", false, "Edit global config")
	configEditCmd.Flags().BoolVar(&configEditProject, "project", false, "Edit project config")

	configInitCmd.Flags().StringVar(&configInitPreset, "preset", "", "Apply a configuration preset (e.g. compliance)")

	ConfigCmd.AddCommand(configShowCmd)
	ConfigCmd.AddCommand(configPathCmd)
	ConfigCmd.AddCommand(configInitCmd)
	ConfigCmd.AddCommand(configSetCmd)
	ConfigCmd.AddCommand(configGetCmd)
	ConfigCmd.AddCommand(configEditCmd)
	ConfigCmd.AddCommand(configValidateCmd)
	ConfigCmd.AddCommand(configDiffCmd)
}

var configDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show differences between global and project configuration",
	RunE:  runConfigDiff,
}

func runConfigDiff(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	project, loadErr := settings.LoadProject(cwd)
	if loadErr != nil {
		fmt.Println("No project config found")

		return fmt.Errorf("load project config: %w", loadErr)
	}
	if project == nil {
		fmt.Println("No project config found")

		return nil
	}

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project config: %w", err)
	}

	fmt.Println("Project overrides:")
	fmt.Println(string(data))

	return nil
}
