package metrics

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/valksor/kvelmo/paths"
)

// TimedSnapshot pairs a metrics snapshot with the time it was captured.
type TimedSnapshot struct {
	Snapshot

	Timestamp time.Time `json:"timestamp"`
}

// TimeSeriesStore manages append-only JSONL files of periodic metrics snapshots.
// One file is created per day, named metrics-YYYY-MM-DD.jsonl.
type TimeSeriesStore struct {
	metrics       *Metrics
	dir           string
	interval      time.Duration
	retentionDays int
}

// NewTimeSeriesStore creates a new time-series store.
// If dir is empty, defaults to <BaseDir>/timeseries.
// If interval is <= 0, defaults to 5 minutes.
// If retentionDays is <= 0, defaults to 90 days.
func NewTimeSeriesStore(m *Metrics, dir string, interval time.Duration, retentionDays int) *TimeSeriesStore {
	if dir == "" {
		dir = filepath.Join(paths.BaseDir(), "timeseries")
	}
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if retentionDays <= 0 {
		retentionDays = 90
	}

	return &TimeSeriesStore{
		metrics:       m,
		dir:           dir,
		interval:      interval,
		retentionDays: retentionDays,
	}
}

// Start runs the periodic snapshot loop. It blocks until ctx is cancelled.
// On startup and at each day boundary, old files are cleaned up according to retention policy.
func (ts *TimeSeriesStore) Start(ctx context.Context) {
	ts.cleanup()

	ticker := time.NewTicker(ts.interval)
	defer ticker.Stop()

	lastCleanupDay := time.Now().Truncate(24 * time.Hour)

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			ts.snapshot(t)

			today := t.Truncate(24 * time.Hour)
			if today.After(lastCleanupDay) {
				ts.cleanup()
				lastCleanupDay = today
			}
		}
	}
}

// Query reads snapshots within the given time range [from, to].
// If to is the zero value, it defaults to time.Now().
func (ts *TimeSeriesStore) Query(from, to time.Time) ([]TimedSnapshot, error) {
	if to.IsZero() {
		to = time.Now()
	}

	// Determine which day files to read.
	startDay := from.Truncate(24 * time.Hour)
	endDay := to.Truncate(24 * time.Hour)

	var results []TimedSnapshot

	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		filename := ts.filenameForDay(day)
		path := filepath.Join(ts.dir, filename)

		entries, err := ts.readFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, fmt.Errorf("read timeseries file %s: %w", filename, err)
		}

		for _, entry := range entries {
			if (entry.Timestamp.Equal(from) || entry.Timestamp.After(from)) &&
				(entry.Timestamp.Equal(to) || entry.Timestamp.Before(to)) {
				results = append(results, entry)
			}
		}
	}

	slices.SortFunc(results, func(a, b TimedSnapshot) int {
		return a.Timestamp.Compare(b.Timestamp)
	})

	return results, nil
}

// cleanup removes JSONL files older than the retention period.
func (ts *TimeSeriesStore) cleanup() {
	entries, err := os.ReadDir(ts.dir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Debug("timeseries cleanup: read dir error", "dir", ts.dir, "error", err)
		}

		return
	}

	cutoff := time.Now().AddDate(0, 0, -ts.retentionDays).Truncate(24 * time.Hour)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		day, ok := ts.parseDayFromFilename(entry.Name())
		if !ok {
			continue
		}

		if day.Before(cutoff) {
			path := filepath.Join(ts.dir, entry.Name())
			if err := os.Remove(path); err != nil {
				slog.Warn("timeseries cleanup: remove error", "path", path, "error", err)
			} else {
				slog.Debug("timeseries cleanup: removed old file", "path", path)
			}
		}
	}
}

func (ts *TimeSeriesStore) snapshot(t time.Time) {
	snap := TimedSnapshot{
		Timestamp: t.UTC(),
		Snapshot:  ts.metrics.Snapshot(),
	}

	data, err := json.Marshal(snap)
	if err != nil {
		slog.Warn("timeseries snapshot: marshal error", "error", err)

		return
	}

	if err := os.MkdirAll(ts.dir, 0o750); err != nil {
		slog.Warn("timeseries snapshot: mkdir error", "dir", ts.dir, "error", err)

		return
	}

	day := t.UTC().Truncate(24 * time.Hour)
	path := filepath.Join(ts.dir, ts.filenameForDay(day))

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		slog.Warn("timeseries snapshot: open error", "path", path, "error", err)

		return
	}
	defer func() { _ = f.Close() }()

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		slog.Warn("timeseries snapshot: write error", "path", path, "error", err)
	}
}

func (ts *TimeSeriesStore) readFile(path string) ([]TimedSnapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var entries []TimedSnapshot
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry TimedSnapshot
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			slog.Debug("timeseries read: skip malformed line", "error", err)

			continue
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return entries, fmt.Errorf("scan: %w", err)
	}

	return entries, nil
}

func (ts *TimeSeriesStore) filenameForDay(day time.Time) string {
	return fmt.Sprintf("metrics-%s.jsonl", day.UTC().Format(time.DateOnly))
}

func (ts *TimeSeriesStore) parseDayFromFilename(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, "metrics-") || !strings.HasSuffix(name, ".jsonl") {
		return time.Time{}, false
	}

	dateStr := strings.TrimPrefix(name, "metrics-")
	dateStr = strings.TrimSuffix(dateStr, ".jsonl")

	t, err := time.Parse(time.DateOnly, dateStr)
	if err != nil {
		return time.Time{}, false
	}

	return t, true
}
