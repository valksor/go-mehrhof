package settings

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// writeGlobalEnv seeds a global .env under a temp KVELMO_HOME, isolating the
// test from the real config dir.
func writeGlobalEnv(t *testing.T, content string) {
	t.Helper()
	t.Setenv("KVELMO_HOME", t.TempDir())
	envPath, err := GlobalEnvPath()
	if err != nil {
		t.Fatalf("GlobalEnvPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestProcessEnv_KeepsBaseDropsHostSecrets(t *testing.T) {
	writeGlobalEnv(t, "FROM_DOTENV=base\nOVERRIDE_ME=fromfile\n")
	// An allowlisted host var must pass through; a non-allowlisted one must not.
	t.Setenv("TERM", "xterm-e2e")
	t.Setenv("KVELMO_PROCESSENV_HOST_ONLY", "leaked")

	env := ProcessEnv("", map[string]string{"OVERRIDE_ME": "fromoverride", "EXTRA": "x"})

	if !slices.Contains(env, "TERM=xterm-e2e") {
		t.Errorf("allowlisted host var TERM not passed through: %v", env)
	}
	if slices.Contains(env, "KVELMO_PROCESSENV_HOST_ONLY=leaked") {
		t.Errorf("non-allowlisted host var leaked into result: %v", env)
	}
	if !slices.Contains(env, "FROM_DOTENV=base") {
		t.Errorf("config .env var missing from result: %v", env)
	}
	if !slices.Contains(env, "OVERRIDE_ME=fromoverride") {
		t.Errorf("override should win over the .env value: %v", env)
	}
	if !slices.Contains(env, "EXTRA=x") {
		t.Errorf("override-only var missing from result: %v", env)
	}
	if !slices.IsSorted(env) {
		t.Errorf("ProcessEnv result is not sorted: %v", env)
	}
}

func TestProcessEnv_ConfigOverridesHostBase(t *testing.T) {
	// A config-dir value for an allowlisted key must win over the host value.
	t.Setenv("TERM", "from-host")
	writeGlobalEnv(t, "TERM=from-config\n")

	env := ProcessEnv("", nil)
	if !slices.Contains(env, "TERM=from-config") {
		t.Errorf("config value should override host passthrough: %v", env)
	}
	if slices.Contains(env, "TERM=from-host") {
		t.Errorf("host TERM should have been overridden by config: %v", env)
	}
}

func TestProcessEnv_NoConfig_DropsNonAllowlistedHost(t *testing.T) {
	t.Setenv("KVELMO_HOME", t.TempDir()) // no .env written
	t.Setenv("KVELMO_PROCESSENV_HOST_ONLY2", "leaked")

	env := ProcessEnv("", map[string]string{"ONLY": "this"})
	if !slices.Contains(env, "ONLY=this") {
		t.Errorf("override missing from result: %v", env)
	}
	if slices.Contains(env, "KVELMO_PROCESSENV_HOST_ONLY2=leaked") {
		t.Errorf("non-allowlisted host var leaked into result: %v", env)
	}
}
