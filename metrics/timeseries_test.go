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

func TestNewTimeSeriesStoreDefaults(t *testing.T) {
	m := New()

	// Empty dir, zero interval, zero retention → all fall back to defaults.
	store := NewTimeSeriesStore(m, "", 0, 0)
	if store.dir == "" {
		t.Error("default dir should be non-empty when dir arg is blank")
	}
	if store.interval != 5*time.Minute {
		t.Errorf("default interval = %v, want 5m", store.interval)
	}
	if store.retentionDays != 90 {
		t.Errorf("default retentionDays = %d, want 90", store.retentionDays)
	}

	// Negative inputs also fall back.
	store2 := NewTimeSeriesStore(m, "/tmp/x", -1*time.Second, -5)
	if store2.dir != "/tmp/x" {
		t.Errorf("dir = %q, want /tmp/x", store2.dir)
	}
	if store2.interval != 5*time.Minute {
		t.Errorf("negative interval did not fall back, got %v", store2.interval)
	}
	if store2.retentionDays != 90 {
		t.Errorf("negative retention did not fall back, got %d", store2.retentionDays)
	}
}

func TestTimeSeriesStore_CleanupSkipsUnknownFiles(t *testing.T) {
	dir := t.TempDir()
	m := New()
	store := NewTimeSeriesStore(m, dir, time.Minute, 7)

	// Mix valid timeseries files with junk that doesn't match the metrics-*.jsonl pattern.
	junkFiles := []string{
		"random.txt",
		"metrics-not-a-date.jsonl",
		"metrics-2026-13-99.jsonl", // invalid date
		"metrics.jsonl",            // missing date entirely
	}
	for _, name := range junkFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("noise\n"), 0o640); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	// Sub-directory should also be skipped.
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o750); err != nil {
		t.Fatalf("seed subdir: %v", err)
	}

	store.cleanup()

	// All junk files should remain — cleanup only touches files it recognises.
	for _, name := range junkFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("cleanup removed unrelated file %q: %v", name, err)
		}
	}
}

func TestTimeSeriesStore_CleanupMissingDir(t *testing.T) {
	m := New()
	// Pointing at a non-existent dir should be a silent no-op.
	store := NewTimeSeriesStore(m, filepath.Join(t.TempDir(), "does-not-exist"), time.Minute, 7)

	// Must not panic.
	store.cleanup()
}

func TestTimeSeriesStore_QueryMissingFiles(t *testing.T) {
	dir := t.TempDir()
	m := New()
	store := NewTimeSeriesStore(m, dir, time.Minute, 7)

	// Query a range whose files don't exist — should return empty, no error.
	results, err := store.Query(time.Now().AddDate(0, 0, -2), time.Now())
	if err != nil {
		t.Fatalf("Query with missing files should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty dir, got %d", len(results))
	}
}

func TestTimeSeriesStore_ReadFileSkipsMalformed(t *testing.T) {
	dir := t.TempDir()
	m := New()
	store := NewTimeSeriesStore(m, dir, time.Minute, 7)

	day := time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)
	path := filepath.Join(dir, store.filenameForDay(day))

	// File with a mix of valid + malformed + blank lines.
	content := `{"timestamp":"2026-05-19T01:00:00Z","jobs_submitted":1}
not valid json
{"timestamp":"2026-05-19T02:00:00Z","jobs_submitted":2}

{still invalid

{"timestamp":"2026-05-19T03:00:00Z","jobs_submitted":3}
`
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatalf("seed: %v", err)
	}

	results, err := store.Query(day, day.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 valid entries (malformed skipped), got %d", len(results))
	}
}

func TestTimeSeriesStore_ParseDayFromFilename(t *testing.T) {
	ts := &TimeSeriesStore{}
	cases := []struct {
		name string
		want bool
	}{
		{"metrics-2026-05-19.jsonl", true},
		{"metrics-2026-13-01.jsonl", false}, // invalid month
		{"prefix-2026-05-19.jsonl", false},
		{"metrics-2026-05-19.txt", false},
		{"metrics-not-a-date.jsonl", false},
		{"metrics-.jsonl", false},
	}
	for _, c := range cases {
		_, ok := ts.parseDayFromFilename(c.name)
		if ok != c.want {
			t.Errorf("parseDayFromFilename(%q) ok=%v, want %v", c.name, ok, c.want)
		}
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
