package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/pkg/paths"
	"github.com/valksor/kvelmo/pkg/provider"
	"github.com/valksor/kvelmo/pkg/settings"
)

func TestDetectExistingToken_FromEnvFile(t *testing.T) {
	// Set up a temp base dir with a .env file containing the token
	tmpDir := t.TempDir()
	paths.SetPaths(paths.NewPathResolver(tmpDir))
	t.Cleanup(paths.ResetForTesting)

	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("TEST_TOKEN_DETECT=secret-value-123\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	result := detectExistingToken("TEST_TOKEN_DETECT", settings.ScopeGlobal, "")
	if result == nil {
		t.Fatal("expected non-nil tokenSource from .env file")
	}
	if !strings.Contains(result.Source, ".env") && !strings.Contains(result.Source, tmpDir) {
		t.Errorf("source = %q, want path containing .env or tmpDir", result.Source)
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
