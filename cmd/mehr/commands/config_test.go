//go:build !testbinary
// +build !testbinary

package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/go-mehrhof/internal/storage"
)

// Note: TestConfigCommand_Structure is in common_test.go

func TestConfigCommand_LongDescription(t *testing.T) {
	if configCmd.Long == "" {
		t.Error("Long description is empty")
	}
}

func TestConfigCommand_ShortDescription(t *testing.T) {
	expected := "Manage configuration"
	if configCmd.Short != expected {
		t.Errorf("Short = %q, want %q", configCmd.Short, expected)
	}
}

func TestConfigValidateCommand_Properties(t *testing.T) {
	// Check validate subcommand is properly configured
	if configValidateCmd.Use != "validate" {
		t.Errorf("Use = %q, want %q", configValidateCmd.Use, "validate")
	}

	if configValidateCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if configValidateCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if configValidateCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestConfigValidateCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		defaultValue string
	}{
		{
			name:         "strict flag",
			flagName:     "strict",
			defaultValue: "false",
		},
		{
			name:         "format flag",
			flagName:     "format",
			defaultValue: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := configValidateCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)

				return
			}

			// Check default value
			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}
		})
	}
}

func TestConfigValidateCommand_ShortDescription(t *testing.T) {
	expected := "Validate configuration files"
	if configValidateCmd.Short != expected {
		t.Errorf("Short = %q, want %q", configValidateCmd.Short, expected)
	}
}

func TestConfigValidateCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"Validate workspace configuration",
		"config.yaml",
		"YAML syntax",
		"circular dependencies",
	}

	for _, substr := range contains {
		if !containsString(configValidateCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestConfigValidateCommand_ExamplesContains(t *testing.T) {
	examples := []string{
		"mehr config validate",
		"--strict",
		"--format json",
	}

	for _, example := range examples {
		if !containsString(configValidateCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestConfigCommand_RegisteredInRoot(t *testing.T) {
	// Verify configCmd is a subcommand of rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "config" {
			found = true

			break
		}
	}
	if !found {
		t.Error("config command not registered in root command")
	}
}

func TestConfigValidateCommand_RegisteredInConfig(t *testing.T) {
	// Verify configValidateCmd is a subcommand of configCmd
	found := false
	for _, cmd := range configCmd.Commands() {
		if cmd.Use == "validate" {
			found = true

			break
		}
	}
	if !found {
		t.Error("validate subcommand not registered in config command")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Unit tests for config.go utility functions
// ─────────────────────────────────────────────────────────────────────────────

// TestDetectProjectType tests the detectProjectType function.
func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name         string
		setupFile    string
		fileContent  string
		expectedType string
	}{
		{
			name:         "go project - go.mod",
			setupFile:    "go.mod",
			fileContent:  "module example.com\n\ngo 1.21\n",
			expectedType: "go",
		},
		{
			name:         "node project - package.json",
			setupFile:    "package.json",
			fileContent:  `{"name": "test", "version": "1.0.0"}`,
			expectedType: "node",
		},
		{
			name:         "python project - pyproject.toml",
			setupFile:    "pyproject.toml",
			fileContent:  `[project]\nname = "test"`,
			expectedType: "python",
		},
		{
			name:         "python project - requirements.txt",
			setupFile:    "requirements.txt",
			fileContent:  `requests==2.28.0`,
			expectedType: "python",
		},
		{
			name:         "python project - setup.py",
			setupFile:    "setup.py",
			fileContent:  `from setuptools import setup\nsetup()`,
			expectedType: "python",
		},
		{
			name:         "python project - setup.cfg",
			setupFile:    "setup.cfg",
			fileContent:  `[metadata]\nname = test`,
			expectedType: "python",
		},
		{
			name:         "php project - composer.json",
			setupFile:    "composer.json",
			fileContent:  `{"name": "test/project"}`,
			expectedType: "php",
		},
		{
			name:         "no project markers",
			setupFile:    "",
			fileContent:  "",
			expectedType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.setupFile != "" {
				fullPath := filepath.Join(tmpDir, tt.setupFile)
				if err := os.WriteFile(fullPath, []byte(tt.fileContent), 0o644); err != nil {
					t.Fatalf("write file: %v", err)
				}
			}

			result := detectProjectType(tmpDir)
			if result != tt.expectedType {
				t.Errorf("detectProjectType() = %q, want %q", result, tt.expectedType)
			}
		})
	}
}

// TestApplyProjectCustomizations tests the applyProjectCustomizations function.
func TestApplyProjectCustomizations(t *testing.T) {
	tests := []struct {
		name        string
		projectType string
		checkConfig func(t *testing.T, cfg *storage.WorkspaceConfig)
	}{
		{
			name:        "go project",
			projectType: "go",
			checkConfig: func(t *testing.T, cfg *storage.WorkspaceConfig) {
				t.Helper()
				if cfg.Agent.Default != "claude" {
					t.Errorf("Agent.Default = %q, want 'claude'", cfg.Agent.Default)
				}
				if cfg.Git.CommitPrefix != "[{key}]" {
					t.Errorf("Git.CommitPrefix = %q, want '[{key}]'", cfg.Git.CommitPrefix)
				}
				if cfg.Git.BranchPattern != "{type}/{key}--{slug}" {
					t.Errorf("Git.BranchPattern = %q, want '{type}/{key}--{slug}'", cfg.Git.BranchPattern)
				}
			},
		},
		{
			name:        "node project",
			projectType: "node",
			checkConfig: func(t *testing.T, cfg *storage.WorkspaceConfig) {
				t.Helper()
				if cfg.Agent.Default != "claude" {
					t.Errorf("Agent.Default = %q, want 'claude'", cfg.Agent.Default)
				}
				if cfg.Git.CommitPrefix != "feat({key}):" {
					t.Errorf("Git.CommitPrefix = %q, want 'feat({key}):'", cfg.Git.CommitPrefix)
				}
			},
		},
		{
			name:        "python project",
			projectType: "python",
			checkConfig: func(t *testing.T, cfg *storage.WorkspaceConfig) {
				t.Helper()
				if cfg.Agent.Default != "claude" {
					t.Errorf("Agent.Default = %q, want 'claude'", cfg.Agent.Default)
				}
				if cfg.Git.CommitPrefix != "[{key}]" {
					t.Errorf("Git.CommitPrefix = %q, want '[{key}]'", cfg.Git.CommitPrefix)
				}
			},
		},
		{
			name:        "php project",
			projectType: "php",
			checkConfig: func(t *testing.T, cfg *storage.WorkspaceConfig) {
				t.Helper()
				if cfg.Agent.Default != "claude" {
					t.Errorf("Agent.Default = %q, want 'claude'", cfg.Agent.Default)
				}
				if cfg.Git.CommitPrefix != "[{key}]" {
					t.Errorf("Git.CommitPrefix = %q, want '[{key}]'", cfg.Git.CommitPrefix)
				}
			},
		},
		{
			name:        "unknown project type",
			projectType: "unknown",
			checkConfig: func(t *testing.T, cfg *storage.WorkspaceConfig) {
				t.Helper()
				// Unknown types should not modify config
				// Just verify it doesn't crash
				if cfg == nil {
					t.Error("config should not be nil")
				}
			},
		},
		{
			name:        "empty project type",
			projectType: "",
			checkConfig: func(t *testing.T, cfg *storage.WorkspaceConfig) {
				t.Helper()
				// Empty type should not modify config
				if cfg == nil {
					t.Error("config should not be nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := storage.NewDefaultWorkspaceConfig()
			applyProjectCustomizations(cfg, tt.projectType)
			tt.checkConfig(t, cfg)
		})
	}
}
