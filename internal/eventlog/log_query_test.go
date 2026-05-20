package eventlog

import (
	"testing"
)

func TestLog_Query(t *testing.T) {
	dir := t.TempDir()

	log, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, e := range []Entry{
		{Type: EventPhaseStarted, Phase: "plan", Message: "1"},
		{Type: EventPhaseCompleted, Phase: "plan", Message: "2"},
		{Type: EventPhaseStarted, Phase: "implement", Message: "3"},
		{Type: EventPhaseStarted, Phase: "review", Message: "4"},
	} {
		if err := log.Append(e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	_ = log.Close()

	starts, err := log.Query(EventPhaseStarted, 0)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(starts) != 3 {
		t.Errorf("Query unlimited starts = %d, want 3", len(starts))
	}

	limited, err := log.Query(EventPhaseStarted, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 2 {
		t.Errorf("Query limit=2 returned %d entries", len(limited))
	}
	// Last two entries should be the implement and review starts.
	if limited[1].Phase != "review" {
		t.Errorf("expected review as last entry, got %q", limited[1].Phase)
	}
}

func TestLog_Query_NoMatches(t *testing.T) {
	dir := t.TempDir()
	log, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Append(Entry{Type: EventPhaseStarted}); err != nil {
		t.Fatal(err)
	}
	_ = log.Close()

	results, err := log.Query(EventTaskLoaded, 5)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestLog_Query_EmptyDir(t *testing.T) {
	// Querying a fresh log returns empty without error.
	dir := t.TempDir()
	log, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = log.Close() }()

	results, err := log.Query(EventPhaseStarted, 10)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}
