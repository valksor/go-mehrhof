package eventlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndReadAll(t *testing.T) {
	dir := t.TempDir()

	log, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	entries := []Entry{
		{Type: EventTaskLoaded, Phase: "load", Message: "Task loaded"},
		{Type: EventPhaseStarted, Phase: "plan", Message: "Planning started"},
		{Type: EventPhaseCompleted, Phase: "plan", Message: "Planning done"},
		{Type: EventPhaseStarted, Phase: "implement", Message: "Implementing"},
	}

	for _, e := range entries {
		if err := log.Append(e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	_ = log.Close()

	read, err := ReadAll(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(read) != len(entries) {
		t.Fatalf("got %d entries, want %d", len(read), len(entries))
	}

	for i, e := range read {
		if e.Type != entries[i].Type {
			t.Errorf("entry %d: type = %q, want %q", i, e.Type, entries[i].Type)
		}
		if e.Phase != entries[i].Phase {
			t.Errorf("entry %d: phase = %q, want %q", i, e.Phase, entries[i].Phase)
		}
	}
}

func TestQuery(t *testing.T) {
	dir := t.TempDir()

	log, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if err := log.Append(Entry{Timestamp: now.Add(-2 * time.Hour), Type: EventPhaseStarted, Phase: "plan"}); err != nil {
		t.Fatal(err)
	}
	if err := log.Append(Entry{Timestamp: now.Add(-1 * time.Hour), Type: EventPhaseCompleted, Phase: "plan"}); err != nil {
		t.Fatal(err)
	}
	if err := log.Append(Entry{Timestamp: now, Type: EventPhaseStarted, Phase: "implement"}); err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}

	// Query by type.
	results, err := Query(dir, QueryOptions{Type: EventPhaseStarted})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("type filter: got %d, want 2", len(results))
	}

	// Query by phase.
	results, err = Query(dir, QueryOptions{Phase: "plan"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("phase filter: got %d, want 2", len(results))
	}
}

func TestReadAllEmpty(t *testing.T) {
	dir := t.TempDir()

	entries, err := ReadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries for empty dir, want 0", len(entries))
	}
}

func TestNewCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "path")

	log, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = log.Close()

	if _, err := os.Stat(filepath.Join(dir, "events.jsonl")); err != nil {
		t.Errorf("events.jsonl not created: %v", err)
	}
}
