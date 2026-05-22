package storage

import (
	"os"
	"testing"
)

func TestSaveTaskStateStampsVersion(t *testing.T) {
	store := newTestStore(t)
	ts := &TaskState{ID: "T-1", State: "loaded", Title: "x"}

	if err := store.SaveTaskState(ts); err != nil {
		t.Fatalf("SaveTaskState: %v", err)
	}
	if ts.FormatVersion != CurrentTaskStateVersion {
		t.Errorf("FormatVersion after save = %d, want %d", ts.FormatVersion, CurrentTaskStateVersion)
	}

	loaded, err := store.LoadTaskState("T-1")
	if err != nil {
		t.Fatalf("LoadTaskState: %v", err)
	}
	if loaded.FormatVersion != CurrentTaskStateVersion {
		t.Errorf("loaded FormatVersion = %d, want %d", loaded.FormatVersion, CurrentTaskStateVersion)
	}
}

func TestLoadTaskStateRejectsNewer(t *testing.T) {
	store := newTestStore(t)
	if err := EnsureDir(store.WorkDir("T-2")); err != nil {
		t.Fatal(err)
	}
	body := "format_version: 9999\nstate: loaded\nid: T-2\ntitle: x\n"
	if err := os.WriteFile(store.TaskStateFile("T-2"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := store.LoadTaskState("T-2"); err == nil {
		t.Fatal("LoadTaskState() expected error for state newer than supported version")
	}
}

func TestLoadTaskStateLegacy(t *testing.T) {
	store := newTestStore(t)
	if err := EnsureDir(store.WorkDir("T-3")); err != nil {
		t.Fatal(err)
	}
	// Pre-versioning state has no format_version key.
	body := "state: loaded\nid: T-3\ntitle: legacy\n"
	if err := os.WriteFile(store.TaskStateFile("T-3"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadTaskState("T-3")
	if err != nil {
		t.Fatalf("LoadTaskState: %v", err)
	}
	if loaded.FormatVersion != CurrentTaskStateVersion {
		t.Errorf("legacy load FormatVersion = %d, want %d", loaded.FormatVersion, CurrentTaskStateVersion)
	}
	if loaded.Title != "legacy" {
		t.Errorf("Title = %q, want legacy (value must survive)", loaded.Title)
	}
}
