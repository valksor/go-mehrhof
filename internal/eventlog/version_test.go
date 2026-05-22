package eventlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadAll_LegacyEntryHasZeroVersion(t *testing.T) {
	dir := t.TempDir()
	// A pre-versioning entry has no "v" field; it must still read back cleanly
	// with Version 0 (the additive forward-read policy).
	line := `{"timestamp":"2026-01-01T00:00:00Z","type":"task_loaded","message":"x"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Version != 0 {
		t.Errorf("legacy entry Version = %d, want 0", entries[0].Version)
	}
	if entries[0].Type != EventTaskLoaded {
		t.Errorf("Type = %q, want %q", entries[0].Type, EventTaskLoaded)
	}
}

func TestAppendStampsVersion(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = l.Close() }()

	if err := l.Append(Entry{Type: EventTaskLoaded, Message: "x"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	entries, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Version != CurrentEventLogVersion {
		t.Errorf("Version = %d, want %d", entries[0].Version, CurrentEventLogVersion)
	}
}

func TestAppendPreservesExplicitVersion(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = l.Close() }()

	// An entry that already carries a version (e.g. replayed from an older log)
	// is not overwritten.
	if err := l.Append(Entry{Version: 1, Type: EventTaskLoaded}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	entries, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 1 || entries[0].Version != 1 {
		t.Fatalf("entries = %+v, want one entry with Version 1", entries)
	}
}
