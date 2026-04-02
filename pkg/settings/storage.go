package settings

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/paths"
	"gopkg.in/yaml.v3"
)

// GlobalPath returns the path to the global settings file.
// Uses paths.Paths().BaseDir() for test isolation support.
func GlobalPath() (string, error) {
	return filepath.Join(paths.BaseDir(), meta.ConfigFile), nil
}

// GlobalDirPath returns the path to the global settings directory.
// Uses paths.Paths().BaseDir() for test isolation support.
func GlobalDirPath() (string, error) {
	return paths.BaseDir(), nil
}

// ProjectPath returns the path to the project settings file.
// projectRoot should be the root directory of the project.
func ProjectPath(projectRoot string) string {
	return filepath.Join(projectRoot, meta.OrgDir, meta.ConfigFile)
}

// ProjectDirPath returns the path to the project settings directory.
func ProjectDirPath(projectRoot string) string {
	return filepath.Join(projectRoot, meta.OrgDir)
}

// Load loads settings from the specified path.
// Returns nil if the file doesn't exist (not an error).
func Load(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // Documented behavior: nil means file not found
		}

		return nil, fmt.Errorf("read settings: %w", err)
	}

	var s Settings
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}

	return &s, nil
}

// Save saves settings to the specified path.
// Creates parent directories if they don't exist.
func Save(path string, s *Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	return nil
}

// LoadGlobal loads settings from the global path.
func LoadGlobal() (*Settings, error) {
	path, err := GlobalPath()
	if err != nil {
		return nil, err
	}

	return Load(path)
}

// LoadProject loads settings from the project path.
func LoadProject(projectRoot string) (*Settings, error) {
	return Load(ProjectPath(projectRoot))
}

// SaveGlobal saves settings to the global path.
func SaveGlobal(s *Settings) error {
	path, err := GlobalPath()
	if err != nil {
		return err
	}

	return Save(path, s)
}

// SaveProject saves settings to the project path.
func SaveProject(projectRoot string, s *Settings) error {
	return Save(ProjectPath(projectRoot), s)
}

// FindNearestProjectConfig walks up from startDir looking for .valksor/kvelmo.yaml files.
// Returns all config paths found, ordered from deepest (most specific) to shallowest (least specific).
// Stops at stopDir (typically the git repo root). Returns nil if no configs found.
func FindNearestProjectConfig(startDir, stopDir string) []string {
	var paths []string
	dir := startDir
	stopDir = filepath.Clean(stopDir)

	for {
		candidate := ProjectPath(dir)
		if _, err := os.Stat(candidate); err == nil {
			paths = append(paths, candidate)
		}

		if filepath.Clean(dir) == stopDir {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	// Reverse so shallowest is first (gets overridden by deeper configs)
	for i, j := 0, len(paths)-1; i < j; i, j = i+1, j-1 {
		paths[i], paths[j] = paths[j], paths[i]
	}

	return paths
}

// LoadEffective loads and merges global and project settings.
// Project settings override global settings.
// Also loads and injects environment variables from .env files.
func LoadEffective(projectRoot string) (*Settings, *Settings, *Settings, error) {
	// Load .env files into an in-memory map (project overrides global)
	envMap, err := LoadEnvMap(projectRoot)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load env: %w", err)
	}

	// Load global settings
	global, err := LoadGlobal()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load global: %w", err)
	}

	// Load project settings
	project, err := LoadProject(projectRoot)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load project: %w", err)
	}

	// Start with defaults
	effective := DefaultSettings()

	// Determine which preset to use (project overrides global)
	presetName := ""
	if global != nil && global.Preset != "" {
		presetName = global.Preset
	}
	if project != nil && project.Preset != "" {
		presetName = project.Preset
	}

	// Apply preset defaults (before user settings, so user values take precedence)
	if presetName != "" {
		if preset := ApplyPreset(presetName); preset != nil {
			Merge(effective, preset)
		}
	}

	// Merge global (if exists)
	if global != nil {
		Merge(effective, global)
	}

	// Merge project (if exists, takes precedence)
	if project != nil {
		Merge(effective, project)
	}

	// Inject environment variables from .env files into sensitive fields
	InjectEnvVars(effective, envMap)

	// Apply KVELMO_* env var overrides (highest priority)
	applyEnvOverrides(effective)

	return effective, global, project, nil
}
