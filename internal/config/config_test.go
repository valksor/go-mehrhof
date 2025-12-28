package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDefault(t *testing.T) {
	cfg := NewDefault()

	if cfg == nil {
		t.Fatal("NewDefault returned nil")
	}

	// Test Agent defaults
	if cfg.Agent.Default != "claude" {
		t.Errorf("Agent.Default = %q, want %q", cfg.Agent.Default, "claude")
	}
	if cfg.Agent.Timeout != 300 {
		t.Errorf("Agent.Timeout = %d, want 300", cfg.Agent.Timeout)
	}
	if cfg.Agent.MaxRetries != 3 {
		t.Errorf("Agent.MaxRetries = %d, want 3", cfg.Agent.MaxRetries)
	}

	// Test Claude defaults
	if cfg.Agent.Claude.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Agent.Claude.Model = %q, want %q", cfg.Agent.Claude.Model, "claude-sonnet-4-20250514")
	}
	if cfg.Agent.Claude.MaxTokens != 8192 {
		t.Errorf("Agent.Claude.MaxTokens = %d, want 8192", cfg.Agent.Claude.MaxTokens)
	}
	if cfg.Agent.Claude.Temperature != 0.7 {
		t.Errorf("Agent.Claude.Temperature = %f, want 0.7", cfg.Agent.Claude.Temperature)
	}

	// Test Storage defaults
	if cfg.Storage.Root != ".mehrhof" {
		t.Errorf("Storage.Root = %q, want %q", cfg.Storage.Root, ".mehrhof")
	}
	if cfg.Storage.MaxBlueprints != 100 {
		t.Errorf("Storage.MaxBlueprints = %d, want 100", cfg.Storage.MaxBlueprints)
	}
	if cfg.Storage.SessionRetentionDays != 30 {
		t.Errorf("Storage.SessionRetentionDays = %d, want 30", cfg.Storage.SessionRetentionDays)
	}

	// Test Git defaults
	if cfg.Git.AutoCommit != true {
		t.Errorf("Git.AutoCommit = %v, want true", cfg.Git.AutoCommit)
	}
	if cfg.Git.BranchPattern != "task/{task_id}" {
		t.Errorf("Git.BranchPattern = %q, want %q", cfg.Git.BranchPattern, "task/{task_id}")
	}

	// Test Providers defaults
	if cfg.Providers.File.BasePath != "." {
		t.Errorf("Providers.File.BasePath = %q, want %q", cfg.Providers.File.BasePath, ".")
	}
	if cfg.Providers.Directory.BasePath != "." {
		t.Errorf("Providers.Directory.BasePath = %q, want %q", cfg.Providers.Directory.BasePath, ".")
	}

	// Test UI defaults
	if cfg.UI.Color != true {
		t.Errorf("UI.Color = %v, want true", cfg.UI.Color)
	}
	if cfg.UI.Format != "text" {
		t.Errorf("UI.Format = %q, want %q", cfg.UI.Format, "text")
	}
	if cfg.UI.Progress != "spinner" {
		t.Errorf("UI.Progress = %q, want %q", cfg.UI.Progress, "spinner")
	}
}

func TestDefaultConfigPaths(t *testing.T) {
	paths := DefaultConfigPaths()

	if len(paths) != 3 {
		t.Errorf("DefaultConfigPaths returned %d paths, want 3", len(paths))
	}

	// Check that paths include expected files
	hasEnv := false
	hasEnvLocal := false
	for _, p := range paths {
		if filepath.Base(p) == ".env" {
			hasEnv = true
		}
		if filepath.Base(p) == ".env.local" {
			hasEnvLocal = true
		}
	}

	if !hasEnv {
		t.Error("DefaultConfigPaths does not include .env")
	}
	if !hasEnvLocal {
		t.Error("DefaultConfigPaths does not include .env.local")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "valid claude agent",
			modify: func(c *Config) {
				c.Agent.Default = "claude"
			},
			wantErr: false,
		},
		{
			name: "invalid agent",
			modify: func(c *Config) {
				c.Agent.Default = "invalid"
			},
			wantErr: true,
		},
		{
			name: "valid text format",
			modify: func(c *Config) {
				c.UI.Format = "text"
			},
			wantErr: false,
		},
		{
			name: "valid json format",
			modify: func(c *Config) {
				c.UI.Format = "json"
			},
			wantErr: false,
		},
		{
			name: "invalid format",
			modify: func(c *Config) {
				c.UI.Format = "xml"
			},
			wantErr: true,
		},
		{
			name: "valid spinner progress",
			modify: func(c *Config) {
				c.UI.Progress = "spinner"
			},
			wantErr: false,
		},
		{
			name: "valid dots progress",
			modify: func(c *Config) {
				c.UI.Progress = "dots"
			},
			wantErr: false,
		},
		{
			name: "valid none progress",
			modify: func(c *Config) {
				c.UI.Progress = "none"
			},
			wantErr: false,
		},
		{
			name: "invalid progress",
			modify: func(c *Config) {
				c.UI.Progress = "bar"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewDefault()
			tt.modify(cfg)

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Create a temporary directory for test
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(tmpDir): %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Logf("restore Chdir(%q): %v", oldWd, err)
		}
	}()

	ctx := context.Background()

	// Test loading with no config files (should use defaults)
	cfg, err := Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Agent.Default != "claude" {
		t.Errorf("Agent.Default = %q, want %q", cfg.Agent.Default, "claude")
	}
}

func TestLoadWithEnvFile(t *testing.T) {
	// Create a temporary directory for test
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(tmpDir): %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Logf("restore Chdir(%q): %v", oldWd, err)
		}
	}()

	// Create .env file with custom values
	envContent := "MEHR_UI_FORMAT=json\nMEHR_STORAGE_ROOT=custom-task-dir\n"
	if err := os.WriteFile(".env", []byte(envContent), 0o644); err != nil {
		t.Fatalf("WriteFile(.env): %v", err)
	}

	ctx := context.Background()
	cfg, err := Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.UI.Format != "json" {
		t.Errorf("UI.Format = %q, want %q", cfg.UI.Format, "json")
	}
	if cfg.Storage.Root != "custom-task-dir" {
		t.Errorf("Storage.Root = %q, want %q", cfg.Storage.Root, "custom-task-dir")
	}
}

func TestLoadWithInvalidConfig(t *testing.T) {
	// Create a temporary directory for test
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(tmpDir): %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Logf("restore Chdir(%q): %v", oldWd, err)
		}
	}()

	// Create .env file with invalid agent
	envContent := `MEHR_AGENT_DEFAULT=invalid_agent
	`
	if err := os.WriteFile(".env", []byte(envContent), 0o644); err != nil {
		t.Fatalf("WriteFile(.env): %v", err)
	}

	ctx := context.Background()
	_, loadErr := Load(ctx)
	if loadErr == nil {
		t.Error("Load should fail with invalid agent")
	}
}

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
