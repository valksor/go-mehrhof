package metrics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTimeSeriesStore_StartAndQuery(t *testing.T) {
	dir := t.TempDir()
	m := New()
	m.RecordJobSubmitted()
	m.RecordJobSubmitted()
	m.RecordJobCompleted()

	store := NewTimeSeriesStore(m, dir, 10*time.Millisecond, 90)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		store.Start(ctx)
		close(done)
	}()

	// Wait for a few snapshots to be written.
	time.Sleep(80 * time.Millisecond)
	cancel()
	<-done

	from := time.Now().Add(-1 * time.Minute)
	results, err := store.Query(from, time.Time{})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected at least 2 snapshots, got %d", len(results))
	}

	// Verify snapshot content matches what we recorded.
	snap := results[0]
	if snap.JobsSubmitted != 2 {
		t.Errorf("expected JobsSubmitted=2, got %d", snap.JobsSubmitted)
	}
	if snap.JobsCompleted != 1 {
		t.Errorf("expected JobsCompleted=1, got %d", snap.JobsCompleted)
	}

	// Verify entries are sorted by time.
	for i := 1; i < len(results); i++ {
		if results[i].Timestamp.Before(results[i-1].Timestamp) {
			t.Errorf("results not sorted: entry %d (%v) before entry %d (%v)",
				i, results[i].Timestamp, i-1, results[i-1].Timestamp)
		}
	}
}

func TestTimeSeriesStore_Retention(t *testing.T) {
	dir := t.TempDir()
	m := New()
	store := NewTimeSeriesStore(m, dir, time.Minute, 7)

	// Create files: one recent, one old (30 days ago), one very old (100 days ago).
	recent := time.Now().UTC().Truncate(24 * time.Hour)
	old := recent.AddDate(0, 0, -30)
	veryOld := recent.AddDate(0, 0, -100)

	for _, day := range []time.Time{recent, old, veryOld} {
		name := store.filenameForDay(day)
		snap := TimedSnapshot{Snapshot: m.Snapshot(), Timestamp: day}
		data, err := json.Marshal(snap)
		if err != nil {
			t.Fatalf("marshal snapshot: %v", err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o640); err != nil {
			t.Fatalf("write test file: %v", err)
		}
	}

	// Verify all three files exist.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Fatalf("expected 3 files before cleanup, got %d", len(entries))
	}

	store.cleanup()

	entries, _ = os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file after cleanup, got %d", len(entries))
	}

	// The remaining file should be the recent one.
	expectedName := store.filenameForDay(recent)
	if entries[0].Name() != expectedName {
		t.Errorf("expected remaining file %s, got %s", expectedName, entries[0].Name())
	}
}

func TestTimeSeriesStore_QueryTimeRange(t *testing.T) {
	dir := t.TempDir()
	m := New()
	store := NewTimeSeriesStore(m, dir, time.Minute, 90)

	// Write entries with known timestamps spanning two days.
	baseDay := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	timestamps := []time.Time{
		baseDay.Add(10 * time.Hour),
		baseDay.Add(14 * time.Hour),
		baseDay.Add(18 * time.Hour),
		baseDay.Add(26 * time.Hour), // next day: 2026-03-15 02:00
		baseDay.Add(30 * time.Hour), // next day: 2026-03-15 06:00
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	for _, ts := range timestamps {
		snap := TimedSnapshot{
			Snapshot:  m.Snapshot(),
			Timestamp: ts,
		}
		data, err := json.Marshal(snap)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		data = append(data, '\n')

		day := ts.Truncate(24 * time.Hour)
		filename := store.filenameForDay(day)
		path := filepath.Join(dir, filename)

		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if _, err := f.Write(data); err != nil {
			t.Fatalf("write: %v", err)
		}
		_ = f.Close()
	}

	// Query a sub-range: 14:00 on day 1 through 02:00 on day 2 (inclusive).
	from := baseDay.Add(14 * time.Hour)
	to := baseDay.Add(26 * time.Hour)

	results, err := store.Query(from, to)
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 entries in range, got %d", len(results))
	}

	if !results[0].Timestamp.Equal(timestamps[1]) {
		t.Errorf("first result: expected %v, got %v", timestamps[1], results[0].Timestamp)
	}
	if !results[1].Timestamp.Equal(timestamps[2]) {
		t.Errorf("second result: expected %v, got %v", timestamps[2], results[1].Timestamp)
	}
	if !results[2].Timestamp.Equal(timestamps[3]) {
		t.Errorf("third result: expected %v, got %v", timestamps[3], results[2].Timestamp)
	}

	// Query with zero 'to' should return everything from 'from' onward.
	results, err = store.Query(from, time.Time{})
	if err != nil {
		t.Fatalf("Query (zero to) error: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 entries from 14:00 onward, got %d", len(results))
	}
}
