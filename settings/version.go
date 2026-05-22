package settings

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// CurrentConfigVersion is the latest config schema version. Increment it when a
// change to the Settings layout requires transforming existing config files —
// renamed, removed, or re-typed fields. Purely additive fields do NOT need a
// bump, because LoadEffective fills unset fields from DefaultSettings.
//
// When bumping, append a matching ConfigMigration to configMigrations covering
// the new step. A missing step is a hard error (see migrateRawTo), so the
// chain can never silently skip a transformation.
const CurrentConfigVersion = 1

// ConfigMigration transforms a raw decoded config from schema version From to
// version To (always a single step, To == From+1). Migrations operate on the
// generic map produced by yaml.Unmarshal rather than the typed Settings struct,
// so they can read fields that no longer exist on the current type before those
// fields are dropped by struct decoding.
type ConfigMigration struct {
	From  int
	To    int
	Apply func(raw map[string]any) error
}

// configMigrations is the ordered chain of config migrations.
//
// The 0→1 entry is the baseline: pre-versioning configs (those without a
// `version` key) report version 0, and v1 is the first stamped version. No
// field transformation is needed because the v1 Settings layout is a superset
// of every pre-versioning config — unset fields are filled from DefaultSettings
// during LoadEffective. The entry exists so the chain is explicit and stamps
// `version: 1`. Future breaking changes append {From: 1, To: 2, ...} and so on.
var configMigrations = []ConfigMigration{
	{
		From:  0,
		To:    1,
		Apply: func(map[string]any) error { return nil },
	},
}

// configVersionOf reads the schema version from a decoded config map. A missing
// or non-numeric `version` key is treated as version 0 (pre-versioning).
func configVersionOf(raw map[string]any) int {
	v, ok := raw["version"]
	if !ok || v == nil {
		return 0
	}

	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// migrationFor returns the migration that upgrades from the given version.
func migrationFor(chain []ConfigMigration, from int) (ConfigMigration, bool) {
	for _, m := range chain {
		if m.From == from {
			return m, true
		}
	}

	return ConfigMigration{}, false
}

// migrateRawTo applies the migration chain to bring raw up to target, mutating
// raw in place. It returns whether any migration ran. A config newer than
// target is an error (a newer kvelmo wrote it); a missing migration step is an
// error (the developer bumped CurrentConfigVersion without registering one).
func migrateRawTo(raw map[string]any, chain []ConfigMigration, target int) (bool, error) {
	current := configVersionOf(raw)

	if current > target {
		return false, fmt.Errorf("config version %d is newer than supported version %d: upgrade kvelmo", current, target)
	}
	if current == target {
		return false, nil
	}

	for current < target {
		mig, ok := migrationFor(chain, current)
		if !ok {
			return false, fmt.Errorf("no config migration registered for version %d→%d", current, current+1)
		}

		// Each migration must advance exactly one version (To == From+1), so the
		// chain can never silently skip a step or fail to make progress.
		if mig.To != current+1 {
			return false, fmt.Errorf("config migration from %d must advance to %d, got To=%d", current, current+1, mig.To)
		}

		if err := mig.Apply(raw); err != nil {
			return false, fmt.Errorf("config migration %d→%d: %w", mig.From, mig.To, err)
		}

		current = mig.To
	}

	raw["version"] = target

	return true, nil
}

// MigrateConfigData brings raw YAML config bytes up to CurrentConfigVersion. It
// returns the (possibly rewritten) bytes and whether a migration occurred. When
// nothing changed it returns the original bytes unmodified.
func MigrateConfigData(data []byte) ([]byte, bool, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, false, fmt.Errorf("parse config for migration: %w", err)
	}

	// Empty or whitespace-only file: nothing to migrate.
	if raw == nil {
		return data, false, nil
	}

	changed, err := migrateRawTo(raw, configMigrations, CurrentConfigVersion)
	if err != nil {
		return nil, false, err
	}
	if !changed {
		return data, false, nil
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		return nil, false, fmt.Errorf("marshal migrated config: %w", err)
	}

	return out, true, nil
}

// MigrateConfigFile migrates a config file in place, rewriting it in kvelmo's
// canonical field order with the version stamped. It is a no-op (returns false)
// when the file is already current or does not exist. Tokens are never touched
// because they live in .env, not the config YAML.
func MigrateConfigFile(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("read config: %w", err)
	}

	migrated, changed, err := MigrateConfigData(data)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}

	// Decode the migrated bytes into Settings and re-Save so the file is
	// written in canonical struct order (version first) rather than the
	// alphabetical order yaml.Marshal produces for a generic map.
	var s Settings
	if err := yaml.Unmarshal(migrated, &s); err != nil {
		return false, fmt.Errorf("parse migrated config: %w", err)
	}
	if s.Version == 0 {
		s.Version = CurrentConfigVersion
	}

	if err := Save(path, &s); err != nil {
		return false, err
	}

	slog.Info("migrated config in place", "path", path, "version", CurrentConfigVersion)

	return true, nil
}
