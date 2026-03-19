package notify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreferenceStore_DefaultsToTypeDefault(t *testing.T) {
	dir := t.TempDir()
	store := NewPreferenceStore(filepath.Join(dir, "prefs.json"))

	// TypePhaseComplete has Default: true
	if !store.GetPreference("task.phase_complete", "") {
		t.Error("expected task.phase_complete to default to true")
	}

	// TypeGateFailed has Default: false
	if store.GetPreference("quality.gate_failed", "") {
		t.Error("expected quality.gate_failed to default to false")
	}
}

func TestPreferenceStore_UnknownTypeDefaultsEnabled(t *testing.T) {
	dir := t.TempDir()
	store := NewPreferenceStore(filepath.Join(dir, "prefs.json"))

	if !store.GetPreference("unknown.type", "") {
		t.Error("expected unknown type to default to enabled")
	}
}

func TestPreferenceStore_GlobalOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	store := NewPreferenceStore(filepath.Join(dir, "prefs.json"))

	// TypePhaseComplete defaults to true; disable it globally.
	err := store.SetPreference(Preference{
		Type:    "task.phase_complete",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	if store.GetPreference("task.phase_complete", "") {
		t.Error("expected global preference to override type default")
	}
}

func TestPreferenceStore_ProjectOverridesGlobal(t *testing.T) {
	dir := t.TempDir()
	store := NewPreferenceStore(filepath.Join(dir, "prefs.json"))

	// Disable globally.
	err := store.SetPreference(Preference{
		Type:    "task.failed",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("SetPreference global: %v", err)
	}

	// Enable for a specific project.
	err = store.SetPreference(Preference{
		Type:      "task.failed",
		ProjectID: "proj-1",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("SetPreference project: %v", err)
	}

	// Project-specific should be true.
	if !store.GetPreference("task.failed", "proj-1") {
		t.Error("expected project preference to override global")
	}

	// Other projects should get the global preference.
	if store.GetPreference("task.failed", "proj-2") {
		t.Error("expected other projects to use global preference (disabled)")
	}

	// No project should get the global preference.
	if store.GetPreference("task.failed", "") {
		t.Error("expected global preference to be disabled")
	}
}

func TestPreferenceStore_SetPreferencePersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prefs.json")

	store1 := NewPreferenceStore(path)

	err := store1.SetPreference(Preference{
		Type:    "task.submitted",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected preferences file to exist on disk")
	}

	// Create a new store from the same file.
	store2 := NewPreferenceStore(path)

	if store2.GetPreference("task.submitted", "") {
		t.Error("expected persisted preference to be loaded by new store")
	}
}

func TestPreferenceStore_SetPreferenceUpdatesExisting(t *testing.T) {
	dir := t.TempDir()
	store := NewPreferenceStore(filepath.Join(dir, "prefs.json"))

	err := store.SetPreference(Preference{
		Type:    "task.failed",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("SetPreference first: %v", err)
	}

	err = store.SetPreference(Preference{
		Type:    "task.failed",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("SetPreference second: %v", err)
	}

	prefs := store.ListPreferences()
	count := 0
	for _, p := range prefs {
		if p.Type == "task.failed" && p.ProjectID == "" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 preference for task.failed, got %d", count)
	}

	if !store.GetPreference("task.failed", "") {
		t.Error("expected updated preference to be true")
	}
}

func TestPreferenceStore_ListPreferences(t *testing.T) {
	dir := t.TempDir()
	store := NewPreferenceStore(filepath.Join(dir, "prefs.json"))

	prefs := store.ListPreferences()
	if len(prefs) != 0 {
		t.Fatalf("expected 0 preferences initially, got %d", len(prefs))
	}

	_ = store.SetPreference(Preference{Type: "task.failed", Enabled: false})
	_ = store.SetPreference(Preference{Type: "task.submitted", ProjectID: "p1", Enabled: true})

	prefs = store.ListPreferences()
	if len(prefs) != 2 {
		t.Fatalf("expected 2 preferences, got %d", len(prefs))
	}
}

func TestPreferenceStore_ListPreferencesReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	store := NewPreferenceStore(filepath.Join(dir, "prefs.json"))

	_ = store.SetPreference(Preference{Type: "task.failed", Enabled: false})

	prefs := store.ListPreferences()
	prefs[0].Enabled = true

	if store.GetPreference("task.failed", "") {
		t.Error("ListPreferences should return a copy, mutation should not affect store")
	}
}

func TestPreferenceStore_LoadNonExistentFile(t *testing.T) {
	dir := t.TempDir()
	store := NewPreferenceStore(filepath.Join(dir, "nonexistent", "prefs.json"))

	// Should not panic; should start with empty preferences.
	prefs := store.ListPreferences()
	if len(prefs) != 0 {
		t.Fatalf("expected 0 preferences, got %d", len(prefs))
	}
}
