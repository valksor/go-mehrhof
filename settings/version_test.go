package settings

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigVersionOf(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]any
		want int
	}{
		{"missing key", map[string]any{"agent": "x"}, 0},
		{"nil value", map[string]any{"version": nil}, 0},
		{"int", map[string]any{"version": 3}, 3},
		{"int64", map[string]any{"version": int64(4)}, 4},
		{"float64", map[string]any{"version": float64(2)}, 2},
		{"non-numeric", map[string]any{"version": "two"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := configVersionOf(tt.raw); got != tt.want {
				t.Errorf("configVersionOf() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMigrateRawTo_Mechanism(t *testing.T) {
	// A two-step chain with real transformations proves the engine renames and
	// adds fields and advances the version, independent of CurrentConfigVersion.
	chain := []ConfigMigration{
		{From: 0, To: 1, Apply: func(raw map[string]any) error {
			if v, ok := raw["old_key"]; ok {
				raw["new_key"] = v
				delete(raw, "old_key")
			}

			return nil
		}},
		{From: 1, To: 2, Apply: func(raw map[string]any) error {
			raw["added"] = true

			return nil
		}},
	}

	raw := map[string]any{"old_key": "value"}
	changed, err := migrateRawTo(raw, chain, 2)
	if err != nil {
		t.Fatalf("migrateRawTo() error = %v", err)
	}
	if !changed {
		t.Fatal("migrateRawTo() changed = false, want true")
	}
	if _, ok := raw["old_key"]; ok {
		t.Error("old_key should have been removed")
	}
	if raw["new_key"] != "value" {
		t.Errorf("new_key = %v, want value", raw["new_key"])
	}
	if raw["added"] != true {
		t.Errorf("added = %v, want true", raw["added"])
	}
	if got := configVersionOf(raw); got != 2 {
		t.Errorf("version = %d, want 2", got)
	}
}

func TestMigrateRawTo_AlreadyCurrent(t *testing.T) {
	raw := map[string]any{"version": 1, "agent": map[string]any{"default": "codex"}}
	changed, err := migrateRawTo(raw, configMigrations, CurrentConfigVersion)
	if err != nil {
		t.Fatalf("migrateRawTo() error = %v", err)
	}
	if changed {
		t.Error("migrateRawTo() changed = true, want false for already-current config")
	}
}

func TestMigrateRawTo_NewerThanSupported(t *testing.T) {
	raw := map[string]any{"version": CurrentConfigVersion + 5}
	if _, err := migrateRawTo(raw, configMigrations, CurrentConfigVersion); err == nil {
		t.Fatal("migrateRawTo() expected error for config newer than supported version")
	}
}

func TestMigrateRawTo_MissingStep(t *testing.T) {
	// Chain skips the 0→1 step entirely; target 1 must error rather than
	// silently bumping the version.
	chain := []ConfigMigration{{From: 1, To: 2, Apply: func(map[string]any) error { return nil }}}
	if _, err := migrateRawTo(map[string]any{}, chain, 1); err == nil {
		t.Fatal("migrateRawTo() expected error for missing migration step")
	}
}

func TestMigrateConfigData_StampsBaseline(t *testing.T) {
	// A pre-versioning config (no version key) is migrated and stamped to v1.
	in := []byte("agent:\n  default: codex\n")
	out, changed, err := MigrateConfigData(in)
	if err != nil {
		t.Fatalf("MigrateConfigData() error = %v", err)
	}
	if !changed {
		t.Fatal("MigrateConfigData() changed = false, want true for unversioned config")
	}

	var raw map[string]any
	if err := yaml.Unmarshal(out, &raw); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if got := configVersionOf(raw); got != CurrentConfigVersion {
		t.Errorf("migrated version = %d, want %d", got, CurrentConfigVersion)
	}
}

func TestMigrateConfigData_Idempotent(t *testing.T) {
	in := []byte("version: 1\nagent:\n  default: codex\n")
	_, changed, err := MigrateConfigData(in)
	if err != nil {
		t.Fatalf("MigrateConfigData() error = %v", err)
	}
	if changed {
		t.Error("MigrateConfigData() changed = true, want false for current config")
	}
}

func TestMigrateConfigData_Empty(t *testing.T) {
	_, changed, err := MigrateConfigData([]byte("\n"))
	if err != nil {
		t.Fatalf("MigrateConfigData() error = %v", err)
	}
	if changed {
		t.Error("MigrateConfigData() changed = true, want false for empty config")
	}
}

func TestLoad_MigratesAndPreservesValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kvelmo.yaml")
	// Old config with no version key.
	if err := os.WriteFile(path, []byte("agent:\n  default: codex\ngit:\n  commit_prefix: \"[X]\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if s == nil {
		t.Fatal("Load() returned nil for existing file")
	}
	if s.Version != CurrentConfigVersion {
		t.Errorf("loaded Version = %d, want %d", s.Version, CurrentConfigVersion)
	}
	if s.Agent.Default != "codex" {
		t.Errorf("Agent.Default = %q, want codex (value must survive migration)", s.Agent.Default)
	}
	if s.Git.CommitPrefix != "[X]" {
		t.Errorf("Git.CommitPrefix = %q, want [X]", s.Git.CommitPrefix)
	}
}

func TestLoad_RejectsNewerConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kvelmo.yaml")
	if err := os.WriteFile(path, []byte("version: 9999\nagent:\n  default: codex\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() expected error for config newer than supported version")
	}
}

func TestMigrateConfigFile_InPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kvelmo.yaml")
	if err := os.WriteFile(path, []byte("agent:\n  default: codex\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	changed, err := MigrateConfigFile(path)
	if err != nil {
		t.Fatalf("MigrateConfigFile() error = %v", err)
	}
	if !changed {
		t.Fatal("MigrateConfigFile() changed = false, want true")
	}

	// File now carries the version stamp and preserves the value.
	s, err := Load(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	if s.Version != CurrentConfigVersion {
		t.Errorf("persisted Version = %d, want %d", s.Version, CurrentConfigVersion)
	}
	if s.Agent.Default != "codex" {
		t.Errorf("Agent.Default = %q, want codex", s.Agent.Default)
	}

	// Permissions preserved at 0o600.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 600", perm)
	}

	// Second run is a no-op.
	changed, err = MigrateConfigFile(path)
	if err != nil {
		t.Fatalf("second MigrateConfigFile() error = %v", err)
	}
	if changed {
		t.Error("second MigrateConfigFile() changed = true, want false (idempotent)")
	}
}

func TestMigrateConfigFile_Missing(t *testing.T) {
	changed, err := MigrateConfigFile(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("MigrateConfigFile() error = %v", err)
	}
	if changed {
		t.Error("MigrateConfigFile() changed = true, want false for missing file")
	}
}
