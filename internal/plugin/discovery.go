package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	_maps "maps"
	_slices "slices"
)

const (
	// ManifestFileName is the expected name of the plugin manifest file.
	ManifestFileName = "plugin.yaml"

	// ScopeGlobal indicates a plugin from the global plugins directory.
	ScopeGlobal = "global"

	// ScopeProject indicates a plugin from the project plugins directory.
	ScopeProject = "project"
)

// Discovery handles finding plugins in configured directories.
type Discovery struct {
	globalDir  string
	projectDir string
}

// NewDiscovery creates a new plugin discovery instance.
// globalDir is typically ~/.mehrhof/plugins/
// projectDir is typically .mehrhof/plugins/ (relative to workspace root)
func NewDiscovery(globalDir, projectDir string) *Discovery {
	return &Discovery{
		globalDir:  globalDir,
		projectDir: projectDir,
	}
}

// DefaultGlobalDir returns the default global plugins directory.
func DefaultGlobalDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".mehrhof", "plugins"), nil
}

// DefaultProjectDir returns the default project plugins directory.
func DefaultProjectDir(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, ".mehrhof", "plugins")
}

// Discover finds all plugins in configured directories.
// Project plugins with the same name override global plugins.
func (d *Discovery) Discover() ([]*Manifest, error) {
	plugins := make(map[string]*Manifest)

	// 1. Scan global plugins first (lower priority)
	if d.globalDir != "" {
		globalPlugins, err := d.scanDir(d.globalDir, ScopeGlobal)
		if err != nil {
			// Global dir might not exist, that's OK
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("scan global plugins: %w", err)
			}
		}
		for _, p := range globalPlugins {
			plugins[p.Name] = p
		}
	}

	// 2. Scan project plugins (override global)
	if d.projectDir != "" {
		projectPlugins, err := d.scanDir(d.projectDir, ScopeProject)
		if err != nil {
			// Project dir might not exist, that's OK
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("scan project plugins: %w", err)
			}
		}
		for _, p := range projectPlugins {
			plugins[p.Name] = p
		}
	}

	// Convert map to slice and clip excess capacity
	result := _slices.Collect(_maps.Values(plugins))
	return _slices.Clip(result), nil
}

// DiscoverByType finds all plugins of a specific type.
func (d *Discovery) DiscoverByType(pluginType PluginType) ([]*Manifest, error) {
	all, err := d.Discover()
	if err != nil {
		return nil, err
	}

	// Pre-allocate with len(all) as upper bound, then clip
	result := make([]*Manifest, 0, len(all))
	for _, p := range all {
		if p.Type == pluginType {
			result = append(result, p)
		}
	}
	return _slices.Clip(result), nil
}

// DiscoverByName finds a specific plugin by name.
// Returns nil if not found.
func (d *Discovery) DiscoverByName(name string) (*Manifest, error) {
	all, err := d.Discover()
	if err != nil {
		return nil, err
	}

	for _, p := range all {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, nil
}

// scanDir scans a directory for plugin manifests.
func (d *Discovery) scanDir(dir string, scope string) ([]*Manifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var manifests []*Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(dir, entry.Name(), ManifestFileName)
		manifest, err := LoadManifest(manifestPath)
		if err != nil {
			// Skip invalid plugins but log the error
			// In production, this could use a proper logger
			continue
		}

		manifest.Scope = scope
		manifests = append(manifests, manifest)
	}

	return manifests, nil
}

// GlobalDir returns the configured global plugins directory.
func (d *Discovery) GlobalDir() string {
	return d.globalDir
}

// ProjectDir returns the configured project plugins directory.
func (d *Discovery) ProjectDir() string {
	return d.projectDir
}

// EnsureDir creates the plugins directory if it doesn't exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// PluginDir returns the directory for a specific plugin.
func PluginDir(baseDir, pluginName string) string {
	return filepath.Join(baseDir, pluginName)
}

// InstallPath returns the path where a plugin should be installed.
func InstallPath(baseDir, pluginName string) string {
	return filepath.Join(baseDir, pluginName, ManifestFileName)
}
