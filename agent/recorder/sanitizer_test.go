package recorder

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/valksor/kvelmo/settings"
)

func TestSanitize_KnownTokens(t *testing.T) {
	token := "ghp_abcdefghij1234567890klmnopqrstuvwxyz1234"
	san := NewSanitizer([]string{token})

	input := []byte("Authorization: Bearer " + token + " is active")
	got := string(san.Sanitize(input))

	if got == string(input) {
		t.Fatal("expected token to be masked, but output is unchanged")
	}

	masked := settings.MaskToken(token)
	want := "Authorization: Bearer " + masked + " is active"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSanitize_RegexPatterns(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantTag string // expected [REDACTED:<tag>] substring
	}{
		{
			name:    "AWS access key",
			input:   "key=AKIAIOSFODNN7EXAMPLE",
			wantTag: "[REDACTED:AWS Access Key]",
		},
		{
			name:    "AWS secret key",
			input:   `aws_secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY0"`,
			wantTag: "[REDACTED:AWS Secret Key]",
		},
		{
			name:    "GitHub token",
			input:   "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn",
			wantTag: "[REDACTED:GitHub Token]",
		},
		{
			name:    "Generic API key",
			input:   `api_key = "sk_test_FAKE_00000000000000000000"`,
			wantTag: "[REDACTED:Generic API Key]",
		},
		{
			name:    "Private key header",
			input:   "-----BEGIN RSA PRIVATE KEY-----",
			wantTag: "[REDACTED:Private Key]",
		},
		{
			name:    "Private key full block",
			input:   "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBALRiMLAH\n-----END RSA PRIVATE KEY-----",
			wantTag: "[REDACTED:Private Key]",
		},
		{
			name:    "JWT token",
			input:   "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc123def456",
			wantTag: "[REDACTED:JWT Token]",
		},
	}

	san := NewSanitizer(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(san.Sanitize([]byte(tt.input)))
			if got == tt.input {
				t.Errorf("input was not sanitized: %q", got)
			}
			if !contains(got, tt.wantTag) {
				t.Errorf("expected tag %q in output %q", tt.wantTag, got)
			}
		})
	}
}

func TestSanitize_CleanDataPassthrough(t *testing.T) {
	san := NewSanitizer([]string{"secret123"})

	clean := "This is perfectly clean data with no secrets."
	got := san.SanitizeString(clean)

	if got != clean {
		t.Errorf("clean data was modified: got %q, want %q", got, clean)
	}
}

func TestSanitize_MultipleSecrets(t *testing.T) {
	token1 := "mytoken_abcdefghijklmnop"
	token2 := "anothertoken_1234567890ab"
	san := NewSanitizer([]string{token1, token2})

	input := "first=" + token1 + " second=" + token2
	got := san.SanitizeString(input)

	if contains(got, token1) {
		t.Errorf("token1 not masked in %q", got)
	}
	if contains(got, token2) {
		t.Errorf("token2 not masked in %q", got)
	}
}

func TestSanitize_EmptySanitizer(t *testing.T) {
	san := NewSanitizer(nil)

	input := "no secrets here, just normal text 12345"
	got := san.SanitizeString(input)

	if got != input {
		t.Errorf("empty sanitizer modified input: got %q, want %q", got, input)
	}
}

func TestSanitize_NilReceiver(t *testing.T) {
	var san *Sanitizer

	input := []byte("should pass through")
	got := san.Sanitize(input)

	if string(got) != string(input) {
		t.Errorf("nil sanitizer modified input: got %q", got)
	}
}

func TestSanitizeString(t *testing.T) {
	token := "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn"
	san := NewSanitizer([]string{token})

	got := san.SanitizeString("Bearer " + token)

	if contains(got, token) {
		t.Errorf("token not masked in SanitizeString output: %q", got)
	}
}

func TestCollectSensitiveValues(t *testing.T) {
	s := &settings.Settings{
		Providers: settings.ProviderSettings{
			GitHub: settings.GitHubConfig{Token: "gh-tok"},
			GitLab: settings.GitLabConfig{Token: "gl-tok"},
			Wrike:  settings.WrikeConfig{Token: ""},
			Linear: settings.LinearConfig{Token: "lin-tok"},
		},
	}

	vals := CollectSensitiveValues(s)

	// Should include 3 settings tokens (Wrike is empty) + any env vars.
	found := map[string]bool{}
	for _, v := range vals {
		found[v] = true
	}

	if !found["gh-tok"] {
		t.Error("missing GitHub token")
	}
	if !found["gl-tok"] {
		t.Error("missing GitLab token")
	}
	if !found["lin-tok"] {
		t.Error("missing Linear token")
	}

	// Empty Wrike token must not appear.
	if found[""] {
		t.Error("empty string should not be in collected values")
	}
}

func TestCollectSensitiveValues_EnvVars(t *testing.T) {
	// The sanitizer reads token values from kvelmo's config-dir .env, never the
	// host environment. Write a global .env under a temp KVELMO_HOME.
	t.Setenv("KVELMO_HOME", t.TempDir())
	envPath, err := settings.GlobalEnvPath()
	if err != nil {
		t.Fatalf("GlobalEnvPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("GITHUB_TOKEN=env-gh-tok\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := &settings.Settings{}
	vals := CollectSensitiveValues(s)

	if !slices.Contains(vals, "env-gh-tok") {
		t.Error("expected GITHUB_TOKEN from config .env in collected values")
	}
}

func TestNewSanitizer_IgnoresEmptyTokens(t *testing.T) {
	san := NewSanitizer([]string{"", "", "real-token", ""})

	if len(san.knownTokens) != 1 {
		t.Errorf("expected 1 known token, got %d", len(san.knownTokens))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
