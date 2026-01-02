package config

import (
	"os"
	"path/filepath"
	"testing"
)

// testUnsetenv is a helper to unset env vars in tests, handling errors appropriately.
func testUnsetenv(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Logf("warning: failed to unset %s: %v", key, err)
	}
}

// testSetenv is a helper to set env vars in tests.
func testSetenv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set %s: %v", key, err)
	}
}

func TestLoadDotEnv_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	err := LoadDotEnv(tmpDir)
	if err != nil {
		t.Errorf("expected nil error for missing .env, got: %v", err)
	}
}

func TestLoadDotEnv_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	mehrhofDir := filepath.Join(tmpDir, MehrhofDir)
	if err := os.MkdirAll(mehrhofDir, 0o755); err != nil {
		t.Fatal(err)
	}

	envContent := `TEST_DOTENV_VAR_ONE=value1
TEST_DOTENV_VAR_TWO=value2`
	if err := os.WriteFile(filepath.Join(mehrhofDir, EnvFileName), []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Clear any existing test vars
	testUnsetenv(t, "TEST_DOTENV_VAR_ONE")
	testUnsetenv(t, "TEST_DOTENV_VAR_TWO")
	defer func() {
		testUnsetenv(t, "TEST_DOTENV_VAR_ONE")
		testUnsetenv(t, "TEST_DOTENV_VAR_TWO")
	}()

	err := LoadDotEnv(tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got := os.Getenv("TEST_DOTENV_VAR_ONE"); got != "value1" {
		t.Errorf("TEST_DOTENV_VAR_ONE = %q, want %q", got, "value1")
	}
	if got := os.Getenv("TEST_DOTENV_VAR_TWO"); got != "value2" {
		t.Errorf("TEST_DOTENV_VAR_TWO = %q, want %q", got, "value2")
	}
}

func TestLoadDotEnv_SystemEnvTakesPriority(t *testing.T) {
	tmpDir := t.TempDir()
	mehrhofDir := filepath.Join(tmpDir, MehrhofDir)
	if err := os.MkdirAll(mehrhofDir, 0o755); err != nil {
		t.Fatal(err)
	}

	envContent := `TEST_DOTENV_PRIORITY_VAR=from_file`
	if err := os.WriteFile(filepath.Join(mehrhofDir, EnvFileName), []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set system env var FIRST (before loading .env)
	testSetenv(t, "TEST_DOTENV_PRIORITY_VAR", "from_system")
	defer testUnsetenv(t, "TEST_DOTENV_PRIORITY_VAR")

	err := LoadDotEnv(tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// System var should take priority - godotenv.Load doesn't override existing vars
	if got := os.Getenv("TEST_DOTENV_PRIORITY_VAR"); got != "from_system" {
		t.Errorf("TEST_DOTENV_PRIORITY_VAR = %q, want %q (system env should take priority)", got, "from_system")
	}
}

func TestLoadDotEnv_QuotedValues(t *testing.T) {
	tmpDir := t.TempDir()
	mehrhofDir := filepath.Join(tmpDir, MehrhofDir)
	if err := os.MkdirAll(mehrhofDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Test that godotenv handles quoted values correctly
	envContent := `TEST_DOTENV_QUOTED="value with spaces"
TEST_DOTENV_SINGLE='single quoted'`
	if err := os.WriteFile(filepath.Join(mehrhofDir, EnvFileName), []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	testUnsetenv(t, "TEST_DOTENV_QUOTED")
	testUnsetenv(t, "TEST_DOTENV_SINGLE")
	defer func() {
		testUnsetenv(t, "TEST_DOTENV_QUOTED")
		testUnsetenv(t, "TEST_DOTENV_SINGLE")
	}()

	err := LoadDotEnv(tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got := os.Getenv("TEST_DOTENV_QUOTED"); got != "value with spaces" {
		t.Errorf("TEST_DOTENV_QUOTED = %q, want %q", got, "value with spaces")
	}
	if got := os.Getenv("TEST_DOTENV_SINGLE"); got != "single quoted" {
		t.Errorf("TEST_DOTENV_SINGLE = %q, want %q", got, "single quoted")
	}
}

func TestLoadDotEnv_Comments(t *testing.T) {
	tmpDir := t.TempDir()
	mehrhofDir := filepath.Join(tmpDir, MehrhofDir)
	if err := os.MkdirAll(mehrhofDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Test that comments are properly ignored
	envContent := `# This is a comment
TEST_DOTENV_COMMENTED=real_value
# Another comment`
	if err := os.WriteFile(filepath.Join(mehrhofDir, EnvFileName), []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	testUnsetenv(t, "TEST_DOTENV_COMMENTED")
	defer testUnsetenv(t, "TEST_DOTENV_COMMENTED")

	err := LoadDotEnv(tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got := os.Getenv("TEST_DOTENV_COMMENTED"); got != "real_value" {
		t.Errorf("TEST_DOTENV_COMMENTED = %q, want %q", got, "real_value")
	}
}

func TestLoadDotEnvFromCwd(t *testing.T) {
	// Create a temporary directory with .mehrhof/.env
	tmpDir := t.TempDir()
	mehrhofDir := filepath.Join(tmpDir, MehrhofDir)
	if err := os.MkdirAll(mehrhofDir, 0o755); err != nil {
		t.Fatal(err)
	}

	envContent := `TEST_DOTENV_CWD=from_cwd`
	if err := os.WriteFile(filepath.Join(mehrhofDir, EnvFileName), []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save current working directory
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldCwd); err != nil {
			t.Logf("restore Chdir: %v", err)
		}
	}()

	testUnsetenv(t, "TEST_DOTENV_CWD")
	defer testUnsetenv(t, "TEST_DOTENV_CWD")

	err = LoadDotEnvFromCwd()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got := os.Getenv("TEST_DOTENV_CWD"); got != "from_cwd" {
		t.Errorf("TEST_DOTENV_CWD = %q, want %q", got, "from_cwd")
	}
}

func TestLoadDotEnvFromCwd_NoEnvFile(t *testing.T) {
	// Create a temporary directory without .env
	tmpDir := t.TempDir()

	// Save current working directory
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldCwd); err != nil {
			t.Logf("restore Chdir: %v", err)
		}
	}()

	// Should not error when .env doesn't exist
	err = LoadDotEnvFromCwd()
	if err != nil {
		t.Errorf("expected nil error when .env doesn't exist, got: %v", err)
	}
}
