package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/pkg/provider"
	"github.com/valksor/kvelmo/pkg/settings"
)

func TestReadEnvVar(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")

	content := `# Comment line
GITHUB_TOKEN=ghp_test123
GITLAB_TOKEN="glpat-quoted"
EMPTY_LINE=

WRIKE_TOKEN='single-quoted'
NO_EQUALS
`
	if err := os.WriteFile(envFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"plain value", "GITHUB_TOKEN", "ghp_test123"},
		{"double quoted", "GITLAB_TOKEN", "glpat-quoted"},
		{"single quoted", "WRIKE_TOKEN", "single-quoted"},
		{"empty value", "EMPTY_LINE", ""},
		{"missing key", "NONEXISTENT", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readEnvVar(envFile, tt.key)
			if got != tt.want {
				t.Errorf("readEnvVar(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestReadEnvVar_FileNotFound(t *testing.T) {
	got := readEnvVar("/nonexistent/path/.env", "KEY")
	if got != "" {
		t.Errorf("readEnvVar(nonexistent) = %q, want empty", got)
	}
}

func TestDetectExistingToken_FromEnv(t *testing.T) {
	t.Setenv("TEST_TOKEN_DETECT", "secret-value-123")
	result := detectExistingToken("TEST_TOKEN_DETECT", settings.ScopeGlobal, "")
	if result == nil {
		t.Fatal("expected non-nil tokenSource from env var")
	}
	if result.Source != "TEST_TOKEN_DETECT environment variable" {
		t.Errorf("source = %q, want %q", result.Source, "TEST_TOKEN_DETECT environment variable")
	}
}

func TestDetectExistingToken_NotFound(t *testing.T) {
	// Use a unique env var name that won't be set
	result := detectExistingToken("TRULY_NONEXISTENT_TOKEN_XYZ_9999", settings.ScopeGlobal, "")
	if result != nil {
		t.Errorf("expected nil tokenSource for nonexistent token, got %v", result)
	}
}

func TestPrintTokenHelp(t *testing.T) {
	cfg := provider.LoginConfig{
		Name:        "TestProvider",
		EnvVar:      "TEST_TOKEN",
		HelpURL:     "https://example.com/tokens",
		HelpSteps:   "Settings -> Tokens",
		Scopes:      "read, write",
		TokenPrefix: "tp_",
	}

	var buf strings.Builder
	printTokenHelp(&buf, cfg)

	output := buf.String()
	if !strings.Contains(output, "TestProvider") {
		t.Error("output should contain provider name")
	}
	if !strings.Contains(output, "https://example.com/tokens") {
		t.Error("output should contain help URL")
	}
	if !strings.Contains(output, "tp_") {
		t.Error("output should contain token prefix")
	}
}

func TestPrintTokenHelp_NoOptionalFields(t *testing.T) {
	cfg := provider.LoginConfig{
		Name:    "Minimal",
		EnvVar:  "MIN_TOKEN",
		HelpURL: "https://example.com",
	}

	var buf strings.Builder
	printTokenHelp(&buf, cfg)

	output := buf.String()
	if !strings.Contains(output, "Minimal") {
		t.Error("output should contain provider name")
	}
}

func TestProviderLoginConfigs(t *testing.T) {
	// Verify all expected providers are registered
	for _, name := range []string{"github", "gitlab", "linear", "wrike"} {
		cfg, ok := providerLoginConfigs[name]
		if !ok {
			t.Errorf("missing provider config for %q", name)

			continue
		}
		if cfg.Name == "" {
			t.Errorf("provider %q has empty Name", name)
		}
		if cfg.EnvVar == "" {
			t.Errorf("provider %q has empty EnvVar", name)

			continue
		}
		if cfg.HelpURL == "" {
			t.Errorf("provider %q has empty HelpURL", name)
		}
	}
}
