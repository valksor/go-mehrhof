package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Preference represents a user's notification preference for a specific type and project.
type Preference struct {
	Type      string `json:"type"`
	ProjectID string `json:"project_id,omitempty"` // empty = global default
	Enabled   bool   `json:"enabled"`
}

// PreferenceStore manages notification preferences backed by a JSON file.
type PreferenceStore struct {
	path  string
	mu    sync.RWMutex
	prefs []Preference
}

// NewPreferenceStore creates a store backed by a JSON file.
// It loads existing preferences from disk if the file exists.
func NewPreferenceStore(path string) *PreferenceStore {
	s := &PreferenceStore{path: path}
	_ = s.load()

	return s
}

// GetPreference returns the effective preference for a type and project.
// Cascade: project-specific -> global -> type default.
func (s *PreferenceStore) GetPreference(notifType string, projectID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Look for project-specific preference first.
	if projectID != "" {
		for _, p := range s.prefs {
			if p.Type == notifType && p.ProjectID == projectID {
				return p.Enabled
			}
		}
	}

	// Look for global preference (empty project ID).
	for _, p := range s.prefs {
		if p.Type == notifType && p.ProjectID == "" {
			return p.Enabled
		}
	}

	// Fall back to type default.
	if t, ok := LookupType(notifType); ok {
		return t.Default
	}

	// Unknown type: default to enabled.
	return true
}

// SetPreference sets a preference. If a matching preference already exists
// (same type and project), it is updated in place.
func (s *PreferenceStore) SetPreference(pref Preference) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update existing preference if present.
	for i, p := range s.prefs {
		if p.Type == pref.Type && p.ProjectID == pref.ProjectID {
			s.prefs[i] = pref

			return s.save()
		}
	}

	s.prefs = append(s.prefs, pref)

	return s.save()
}

// ListPreferences returns all preferences.
func (s *PreferenceStore) ListPreferences() []Preference {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Preference, len(s.prefs))
	copy(out, s.prefs)

	return out
}

// load reads preferences from disk.
func (s *PreferenceStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("read preferences: %w", err)
	}

	if err := json.Unmarshal(data, &s.prefs); err != nil {
		return fmt.Errorf("unmarshal preferences: %w", err)
	}

	return nil
}

// save writes preferences to disk atomically via write-to-temp-then-rename.
func (s *PreferenceStore) save() error {
	data, err := json.MarshalIndent(s.prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal preferences: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create preferences directory: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write preferences temp file: %w", err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename preferences file: %w", err)
	}

	return nil
}
