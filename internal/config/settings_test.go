package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsAddRecentTask(t *testing.T) {
	s := &Settings{}

	// Add first task
	s.AddRecentTask("task1")
	if len(s.RecentTasks) != 1 {
		t.Errorf("RecentTasks length = %d, want 1", len(s.RecentTasks))
	}
	if s.RecentTasks[0] != "task1" {
		t.Errorf("RecentTasks[0] = %q, want %q", s.RecentTasks[0], "task1")
	}

	// Add second task
	s.AddRecentTask("task2")
	if len(s.RecentTasks) != 2 {
		t.Errorf("RecentTasks length = %d, want 2", len(s.RecentTasks))
	}
	if s.RecentTasks[0] != "task2" {
		t.Errorf("RecentTasks[0] = %q, want %q (most recent first)", s.RecentTasks[0], "task2")
	}

	// Add duplicate task (should move to front)
	s.AddRecentTask("task1")
	if len(s.RecentTasks) != 2 {
		t.Errorf("RecentTasks length = %d, want 2 (no duplicates)", len(s.RecentTasks))
	}
	if s.RecentTasks[0] != "task1" {
		t.Errorf("RecentTasks[0] = %q, want %q (moved to front)", s.RecentTasks[0], "task1")
	}
}

func TestSettingsAddRecentTaskMaxLimit(t *testing.T) {
	s := &Settings{}

	// Add 12 tasks
	for i := 1; i <= 12; i++ {
		s.AddRecentTask("task" + string(rune('0'+i)))
	}

	// Should be limited to 10
	if len(s.RecentTasks) != 10 {
		t.Errorf("RecentTasks length = %d, want 10 (max limit)", len(s.RecentTasks))
	}
}

func TestSettingsSaveAndLoad(t *testing.T) {
	// Create a temporary home directory
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Setenv(HOME): %v", err)
	}
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Logf("restore Setenv(HOME): %v", err)
		}
	}()

	s := &Settings{
		PreferredAgent: "claude",
		TargetBranch:   "main",
		LastProvider:   "file",
		RecentTasks:    []string{"task1", "task2"},
	}

	if err := s.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}

	if loaded.PreferredAgent != s.PreferredAgent {
		t.Errorf("PreferredAgent = %q, want %q", loaded.PreferredAgent, s.PreferredAgent)
	}
	if loaded.TargetBranch != s.TargetBranch {
		t.Errorf("TargetBranch = %q, want %q", loaded.TargetBranch, s.TargetBranch)
	}
	if loaded.LastProvider != s.LastProvider {
		t.Errorf("LastProvider = %q, want %q", loaded.LastProvider, s.LastProvider)
	}
	if len(loaded.RecentTasks) != len(s.RecentTasks) {
		t.Errorf("RecentTasks length = %d, want %d", len(loaded.RecentTasks), len(s.RecentTasks))
	}
}

func TestLoadSettingsNonExistent(t *testing.T) {
	// Create a temporary home directory with no settings file
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Setenv(HOME): %v", err)
	}
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Logf("restore Setenv(HOME): %v", err)
		}
	}()

	s, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}

	// Should return empty settings
	if s.PreferredAgent != "" {
		t.Errorf("PreferredAgent = %q, want empty", s.PreferredAgent)
	}
	if len(s.RecentTasks) != 0 {
		t.Errorf("RecentTasks length = %d, want 0", len(s.RecentTasks))
	}
}

func TestSettingsPath(t *testing.T) {
	path := SettingsPath()

	if path == "" {
		t.Error("SettingsPath returned empty string")
	}

	// Should end with settings.json
	if filepath.Base(path) != "settings.json" {
		t.Errorf("SettingsPath base = %q, want %q", filepath.Base(path), "settings.json")
	}

	// Should be in .mehrhof directory
	if filepath.Base(filepath.Dir(path)) != ".mehrhof" {
		t.Errorf("SettingsPath parent = %q, want %q", filepath.Base(filepath.Dir(path)), ".mehrhof")
	}
}
