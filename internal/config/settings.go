package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings holds user preferences that persist between sessions
type Settings struct {
	// Preferred agent (overrides config default)
	PreferredAgent string `json:"preferred_agent,omitempty"`

	// Default target branch for merges
	TargetBranch string `json:"target_branch,omitempty"`

	// Last used provider
	LastProvider string `json:"last_provider,omitempty"`

	// Recent task IDs (for quick access)
	RecentTasks []string `json:"recent_tasks,omitempty"`
}

// SettingsPath returns the path to the settings file
func SettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mehrhof", "settings.json")
}

// LoadSettings reads settings from disk
func LoadSettings() (*Settings, error) {
	path := SettingsPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Settings{}, nil // Return empty settings if file doesn't exist
		}
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// Save writes settings to disk
func (s *Settings) Save() error {
	path := SettingsPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// AddRecentTask adds a task to recent list (max 10, most recent first)
func (s *Settings) AddRecentTask(taskID string) {
	// Remove if already present
	filtered := make([]string, 0, len(s.RecentTasks))
	for _, t := range s.RecentTasks {
		if t != taskID {
			filtered = append(filtered, t)
		}
	}

	// Add to front
	s.RecentTasks = append([]string{taskID}, filtered...)

	// Trim to max 10
	if len(s.RecentTasks) > 10 {
		s.RecentTasks = s.RecentTasks[:10]
	}
}
