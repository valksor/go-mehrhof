package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/go-mehrhof/internal/storage"
)

// TestProviderRegistry ensures all expected providers are registered
func TestProviderRegistry(t *testing.T) {
	expectedProviders := []string{
		"github", "gitlab", "notion", "jira", "linear", "wrike", "youtrack",
	}

	for _, provider := range expectedProviders {
		cfg := getProviderLoginConfig(provider)
		if cfg == nil {
			t.Errorf("Provider %q not found in registry", provider)
			continue
		}

		// Validate required fields
		if cfg.Name == "" {
			t.Errorf("Provider %q: Name is empty", provider)
		}
		if cfg.EnvVar == "" {
			t.Errorf("Provider %q: EnvVar is empty", provider)
		}
		if cfg.ConfigField == "" {
			t.Errorf("Provider %q: ConfigField is empty", provider)
		}
		if cfg.HelpURL == "" {
			t.Errorf("Provider %q: HelpURL is empty", provider)
		}
	}
}

// TestNormalizeProviderName tests provider name normalization
func TestNormalizeProviderName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"github", "github"},
		{"GitHub", "github"},
		{"GITHUB", "github"},
		{"gh", "github"},
		{"gitlab", "gitlab"},
		{"gl", "gitlab"},
		{"notion", "notion"},
		{"nt", "notion"},
		{"jira", "jira"},
		{"linear", "linear"},
		{"wrike", "wrike"},
		{"youtrack", "youtrack"},
		{"yt", "youtrack"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeProviderName(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeProviderName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestMaskToken tests token masking
func TestMaskToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "long token",
			token:    "ghp_1234567890abcdefghij",
			expected: "ghp_...ghij",
		},
		{
			name:     "short token",
			token:    "abc123",
			expected: "*******",
		},
		{
			name:     "exactly 8 chars",
			token:    "12345678",
			expected: "*******",
		},
		{
			name:     "empty token",
			token:    "",
			expected: "*******",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskToken(tt.token)
			if got != tt.expected {
				t.Errorf("maskToken(%q) = %q, want %q", tt.token, got, tt.expected)
			}
		})
	}
}

// TestWriteTokenToEnv tests writing tokens to .env file
func TestWriteTokenToEnv(t *testing.T) {
	t.Run("new file", func(t *testing.T) {
		tmpDir := t.TempDir()
		envPath := filepath.Join(tmpDir, ".env")

		err := writeTokenToEnv(envPath, "GITHUB_TOKEN", "ghp_test123")
		if err != nil {
			t.Fatalf("writeTokenToEnv failed: %v", err)
		}

		// Verify file was created
		data, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("read .env: %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "GITHUB_TOKEN=ghp_test123") {
			t.Errorf("Expected token not found in .env: %s", content)
		}

		// Verify file permissions
		info, err := os.Stat(envPath)
		if err != nil {
			t.Fatalf("stat .env: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("File permissions = %o, want %o", info.Mode().Perm(), 0o600)
		}
	})

	t.Run("append to existing", func(t *testing.T) {
		tmpDir := t.TempDir()
		envPath := filepath.Join(tmpDir, ".env")

		// Create initial file
		err := os.WriteFile(envPath, []byte("EXISTING_VAR=value\n"), 0o600)
		if err != nil {
			t.Fatalf("create initial .env: %v", err)
		}

		err = writeTokenToEnv(envPath, "GITHUB_TOKEN", "ghp_test123")
		if err != nil {
			t.Fatalf("writeTokenToEnv failed: %v", err)
		}

		data, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("read .env: %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "EXISTING_VAR=value") {
			t.Errorf("Existing var lost: %s", content)
		}
		if !strings.Contains(content, "GITHUB_TOKEN=ghp_test123") {
			t.Errorf("New token not found: %s", content)
		}
	})

	t.Run("replace existing token", func(t *testing.T) {
		tmpDir := t.TempDir()
		envPath := filepath.Join(tmpDir, ".env")

		// Create initial file with token
		err := os.WriteFile(envPath, []byte("GITHUB_TOKEN=old_token\n"), 0o600)
		if err != nil {
			t.Fatalf("create initial .env: %v", err)
		}

		err = writeTokenToEnv(envPath, "GITHUB_TOKEN", "new_token")
		if err != nil {
			t.Fatalf("writeTokenToEnv failed: %v", err)
		}

		data, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("read .env: %v", err)
		}

		content := string(data)
		if strings.Contains(content, "old_token") {
			t.Errorf("Old token still present: %s", content)
		}
		if !strings.Contains(content, "GITHUB_TOKEN=new_token") {
			t.Errorf("New token not found: %s", content)
		}
	})
}

// TestGetConfigToken tests extracting tokens from WorkspaceConfig
func TestGetConfigToken(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		setup     func(*storage.WorkspaceConfig)
		want      string
	}{
		{
			name:      "github token",
			fieldPath: "GitHub.Token",
			setup: func(cfg *storage.WorkspaceConfig) {
				cfg.GitHub = &storage.GitHubSettings{Token: "ghp_test"}
			},
			want: "ghp_test",
		},
		{
			name:      "gitlab token",
			fieldPath: "GitLab.Token",
			setup: func(cfg *storage.WorkspaceConfig) {
				cfg.GitLab = &storage.GitLabSettings{Token: "glpat_test"}
			},
			want: "glpat_test",
		},
		{
			name:      "notion token",
			fieldPath: "Notion.Token",
			setup: func(cfg *storage.WorkspaceConfig) {
				cfg.Notion = &storage.NotionSettings{Token: "secret_test"}
			},
			want: "secret_test",
		},
		{
			name:      "jira token",
			fieldPath: "Jira.Token",
			setup: func(cfg *storage.WorkspaceConfig) {
				cfg.Jira = &storage.JiraSettings{Token: "jira_test"}
			},
			want: "jira_test",
		},
		{
			name:      "linear token",
			fieldPath: "Linear.Token",
			setup: func(cfg *storage.WorkspaceConfig) {
				cfg.Linear = &storage.LinearSettings{Token: "lin_api_test"}
			},
			want: "lin_api_test",
		},
		{
			name:      "wrike token",
			fieldPath: "Wrike.Token",
			setup: func(cfg *storage.WorkspaceConfig) {
				cfg.Wrike = &storage.WrikeSettings{Token: "wrike_test"}
			},
			want: "wrike_test",
		},
		{
			name:      "youtrack token",
			fieldPath: "YouTrack.Token",
			setup: func(cfg *storage.WorkspaceConfig) {
				cfg.YouTrack = &storage.YouTrackSettings{Token: "yt_test"}
			},
			want: "yt_test",
		},
		{
			name:      "nil provider config",
			fieldPath: "GitHub.Token",
			setup:     func(cfg *storage.WorkspaceConfig) {},
			want:      "",
		},
		{
			name:      "invalid field path",
			fieldPath: "Invalid.Field",
			setup:     func(cfg *storage.WorkspaceConfig) {},
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := storage.NewDefaultWorkspaceConfig()
			tt.setup(cfg)

			got := getConfigToken(cfg, tt.fieldPath)
			if got != tt.want {
				t.Errorf("getConfigToken(%q) = %q, want %q", tt.fieldPath, got, tt.want)
			}
		})
	}
}

// TestLoadEnv tests loading .env file contents
func TestLoadEnv(t *testing.T) {
	t.Run("non-existent file", func(t *testing.T) {
		tmpDir := t.TempDir()
		ws, err := storage.OpenWorkspace(tmpDir)
		if err != nil {
			t.Fatalf("OpenWorkspace: %v", err)
		}

		env, err := ws.LoadEnv()
		if err != nil {
			t.Fatalf("LoadEnv failed: %v", err)
		}
		if len(env) != 0 {
			t.Errorf("LoadEnv returned %d vars, want 0", len(env))
		}
	})

	t.Run("parse env file", func(t *testing.T) {
		tmpDir := t.TempDir()
		ws, err := storage.OpenWorkspace(tmpDir)
		if err != nil {
			t.Fatalf("OpenWorkspace: %v", err)
		}

		// Ensure directory exists
		if err := ws.EnsureInitialized(); err != nil {
			t.Fatalf("EnsureInitialized: %v", err)
		}

		envPath := ws.EnvPath()
		content := `# Comment
VAR1=value1
VAR2=value2

  VAR3  =  value3
`
		if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
			t.Fatalf("write .env: %v", err)
		}

		env, err := ws.LoadEnv()
		if err != nil {
			t.Fatalf("LoadEnv failed: %v", err)
		}

		if env["VAR1"] != "value1" {
			t.Errorf("VAR1 = %q, want %q", env["VAR1"], "value1")
		}
		if env["VAR2"] != "value2" {
			t.Errorf("VAR2 = %q, want %q", env["VAR2"], "value2")
		}
		if env["VAR3"] != "value3" {
			t.Errorf("VAR3 = %q, want %q", env["VAR3"], "value3")
		}
	})
}
