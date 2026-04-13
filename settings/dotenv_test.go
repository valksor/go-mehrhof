package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeEnvFile writes an .env file to the given directory.
func writeEnvFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, ".env")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	return path
}

func TestLoadEnvMap_Missing(t *testing.T) {
	root := t.TempDir()
	// Neither global nor project .env exists — should not error
	env, err := LoadEnvMap(root)
	if err != nil {
		t.Fatalf("LoadEnvMap() missing files error = %v, want nil", err)
	}
	if env == nil {
		t.Error("LoadEnvMap() returned nil, want empty map")
	}
}

func TestLoadEnvMap_Parsing(t *testing.T) {
	dir := t.TempDir()
	content := `# This is a comment
PLAIN_KEY=plain_value
QUOTED_DOUBLE="double quoted"
QUOTED_SINGLE='single quoted'

EMPTY_LINE_ABOVE=ok
`
	// Write to the project .env location
	projectDir := ProjectDirPath(dir)
	writeEnvFile(t, projectDir, content)

	env, err := LoadEnvMap(dir)
	if err != nil {
		t.Fatalf("LoadEnvMap() error = %v", err)
	}

	if env.Get("PLAIN_KEY") != "plain_value" {
		t.Errorf("PLAIN_KEY = %q, want plain_value", env.Get("PLAIN_KEY"))
	}
	if env.Get("QUOTED_DOUBLE") != "double quoted" {
		t.Errorf("QUOTED_DOUBLE = %q, want double quoted", env.Get("QUOTED_DOUBLE"))
	}
	if env.Get("QUOTED_SINGLE") != "single quoted" {
		t.Errorf("QUOTED_SINGLE = %q, want single quoted", env.Get("QUOTED_SINGLE"))
	}
	if env.Get("EMPTY_LINE_ABOVE") != "ok" {
		t.Errorf("EMPTY_LINE_ABOVE = %q, want ok", env.Get("EMPTY_LINE_ABOVE"))
	}
}

func TestSaveEnvVar_CreateAndAppend(t *testing.T) {
	root := t.TempDir()

	if err := SaveEnvVar(ScopeProject, root, "MY_TOKEN", "abc123"); err != nil {
		t.Fatalf("SaveEnvVar() error = %v", err)
	}

	// File should exist
	envPath := ProjectEnvPath(root)
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("env file not created: %v", err)
	}
	if !strings.Contains(string(data), "MY_TOKEN=abc123") {
		t.Errorf("env file = %q, want MY_TOKEN=abc123", string(data))
	}
}

func TestSaveEnvVar_UpdateExisting(t *testing.T) {
	root := t.TempDir()

	// Write initial value
	if err := SaveEnvVar(ScopeProject, root, "MY_TOKEN", "initial"); err != nil {
		t.Fatal(err)
	}

	// Update value
	if err := SaveEnvVar(ScopeProject, root, "MY_TOKEN", "updated"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(ProjectEnvPath(root))
	content := string(data)

	// Should contain updated value, not initial
	if strings.Contains(content, "initial") {
		t.Error("SaveEnvVar() did not replace old value")
	}
	if !strings.Contains(content, "MY_TOKEN=updated") {
		t.Errorf("env file = %q, want MY_TOKEN=updated", content)
	}
	// Key should appear only once
	count := strings.Count(content, "MY_TOKEN=")
	if count != 1 {
		t.Errorf("MY_TOKEN appears %d times, want 1", count)
	}
}

func TestSaveEnvVar_PreservesOtherKeys(t *testing.T) {
	root := t.TempDir()

	if err := SaveEnvVar(ScopeProject, root, "KEY_A", "val_a"); err != nil {
		t.Fatal(err)
	}
	if err := SaveEnvVar(ScopeProject, root, "KEY_B", "val_b"); err != nil {
		t.Fatal(err)
	}
	if err := SaveEnvVar(ScopeProject, root, "KEY_A", "val_a_updated"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(ProjectEnvPath(root))
	content := string(data)

	if !strings.Contains(content, "KEY_A=val_a_updated") {
		t.Errorf("KEY_A not updated in %q", content)
	}
	if !strings.Contains(content, "KEY_B=val_b") {
		t.Errorf("KEY_B missing from %q", content)
	}
}

func TestInjectEnvVars(t *testing.T) {
	env := EnvMap{
		"GITHUB_TOKEN": "gh_test_token",
		"GITLAB_TOKEN": "gl_test_token",
		"WRIKE_TOKEN":  "wk_test_token",
		"LINEAR_TOKEN": "ln_test_token",
	}

	s := DefaultSettings()
	InjectEnvVars(s, env)

	if s.Providers.GitHub.Token != "gh_test_token" {
		t.Errorf("GitHub.Token = %q, want gh_test_token", s.Providers.GitHub.Token)
	}
	if s.Providers.GitLab.Token != "gl_test_token" {
		t.Errorf("GitLab.Token = %q, want gl_test_token", s.Providers.GitLab.Token)
	}
	if s.Providers.Wrike.Token != "wk_test_token" {
		t.Errorf("Wrike.Token = %q, want wk_test_token", s.Providers.Wrike.Token)
	}
	if s.Providers.Linear.Token != "ln_test_token" {
		t.Errorf("Linear.Token = %q, want ln_test_token", s.Providers.Linear.Token)
	}
}

func TestInjectEnvVars_EmptyTokenNotInjected(t *testing.T) {
	// Empty EnvMap should not override existing tokens
	env := EnvMap{}

	s := DefaultSettings()
	s.Providers.GitHub.Token = "existing"
	InjectEnvVars(s, env)

	if s.Providers.GitHub.Token != "existing" {
		t.Errorf("GitHub.Token = %q, empty env should not override existing token", s.Providers.GitHub.Token)
	}
}

func TestInjectEnvVars_NilEnvMap(t *testing.T) {
	// nil EnvMap should work without panicking
	s := DefaultSettings()
	s.Providers.GitHub.Token = "existing"
	InjectEnvVars(s, nil)

	if s.Providers.GitHub.Token != "existing" {
		t.Errorf("GitHub.Token = %q, nil env should not override existing token", s.Providers.GitHub.Token)
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		token string
		want  string
	}{
		{"", ""},
		{"short", "***"},
		{"12345678", "1234***5678"},
		{"abcdefghij", "abcd***ghij"},
		{"ghp_abcdefghij1234567890", "ghp_***7890"},
	}

	for _, tt := range tests {
		got := MaskToken(tt.token)
		if got != tt.want {
			t.Errorf("MaskToken(%q) = %q, want %q", tt.token, got, tt.want)
		}
	}
}

func TestIsMaskedToken(t *testing.T) {
	tests := []struct {
		token string
		want  bool
	}{
		{"ghp_***7890", true},
		{"abcd***efgh", true},
		{"plaintoken", false},
		{"", false},
		{"nostarstar", false},
	}

	for _, tt := range tests {
		got := IsMaskedToken(tt.token)
		if got != tt.want {
			t.Errorf("IsMaskedToken(%q) = %v, want %v", tt.token, got, tt.want)
		}
	}
}

func TestMaskSettings(t *testing.T) {
	s := &Settings{
		Providers: ProviderSettings{
			GitHub: GitHubConfig{Token: "ghp_abcdefghij12345"},
			GitLab: GitLabConfig{Token: "glpat_abcdefghij12345"},
			Wrike:  WrikeConfig{Token: "wk_abcdefghij12345"},
		},
	}

	masked := MaskSettings(s)

	if masked == nil {
		t.Fatal("MaskSettings() = nil")
	}
	if masked.Providers.GitHub.Token == s.Providers.GitHub.Token {
		t.Error("GitHub token not masked")
	}
	if !IsMaskedToken(masked.Providers.GitHub.Token) {
		t.Errorf("GitHub token = %q, should be masked", masked.Providers.GitHub.Token)
	}
	if !IsMaskedToken(masked.Providers.GitLab.Token) {
		t.Errorf("GitLab token = %q, should be masked", masked.Providers.GitLab.Token)
	}
	if !IsMaskedToken(masked.Providers.Wrike.Token) {
		t.Errorf("Wrike token = %q, should be masked", masked.Providers.Wrike.Token)
	}

	// Original should be unchanged
	if s.Providers.GitHub.Token != "ghp_abcdefghij12345" {
		t.Error("MaskSettings() modified original settings")
	}
}

func TestMaskSettings_Nil(t *testing.T) {
	if got := MaskSettings(nil); got != nil {
		t.Errorf("MaskSettings(nil) = %v, want nil", got)
	}
}

func TestProjectEnvPath(t *testing.T) {
	root := t.TempDir()
	path := ProjectEnvPath(root)
	if path == "" {
		t.Error("ProjectEnvPath() returned empty")
	}
	if !strings.HasSuffix(path, ".env") {
		t.Errorf("ProjectEnvPath() = %q, want suffix .env", path)
	}
	if !strings.HasPrefix(path, root) {
		t.Errorf("ProjectEnvPath() = %q, want prefix %q", path, root)
	}
}

func TestEnvMapGet_NilMap(t *testing.T) {
	var env EnvMap
	if got := env.Get("ANY_KEY"); got != "" {
		t.Errorf("nil EnvMap.Get() = %q, want empty string", got)
	}
}

func TestEnvMapGet_MissingKey(t *testing.T) {
	env := EnvMap{"SOME_KEY": "value"}
	if got := env.Get("OTHER_KEY"); got != "" {
		t.Errorf("EnvMap.Get(missing) = %q, want empty string", got)
	}
}
