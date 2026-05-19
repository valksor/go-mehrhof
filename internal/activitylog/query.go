package activitylog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// QueryOptions controls filtering when reading back activity log entries.
type QueryOptions struct {
	Since         time.Duration // Return entries from now-Since to now.
	MethodPattern string        // Pipe-separated contains match (e.g. "start|plan|implement").
	ErrorsOnly    bool          // Only return entries with a non-empty Error field.
	Limit         int           // Maximum entries to return; 0 means unlimited.
}

// Query reads activity log entries matching the given filters. It scans
// relevant daily files in reverse chronological order and returns entries
// newest-first.
func (l *Log) Query(opts QueryOptions) ([]Entry, error) {
	now := l.nowFunc()
	var cutoff time.Time
	if opts.Since > 0 {
		cutoff = now.Add(-opts.Since)
	}

	logFiles, err := l.logFilesSorted()
	if err != nil {
		return nil, err
	}

	var results []Entry

	// Iterate files in reverse (newest first) for efficiency.
	for _, v := range slices.Backward(logFiles) {
		name := v

		// Quick date-based skip: if Since is set and the file's day is entirely
		// before the cutoff, we can stop scanning older files.
		if opts.Since > 0 {
			day := dayFromFileName(name)
			fileDate, parseErr := time.Parse(dateFormat, day)
			if parseErr == nil {
				// The file covers the entire day, so its latest possible entry is
				// end-of-day. If that's before cutoff, skip this and all older files.
				endOfDay := fileDate.Add(24*time.Hour - time.Nanosecond)
				if endOfDay.Before(cutoff) {
					break
				}
			}
		}

		entries, readErr := l.readFile(filepath.Join(l.dir, name))
		if readErr != nil {
			return nil, readErr
		}

		for _, v := range slices.Backward(entries) {
			e := v

			if opts.Since > 0 && e.Timestamp.Before(cutoff) {
				continue
			}
			if opts.ErrorsOnly && e.Error == "" {
				continue
			}
			if !matchesMethodPattern(e.Method, opts.MethodPattern) {
				continue
			}

			results = append(results, e)

			if opts.Limit > 0 && len(results) >= opts.Limit {
				return results, nil
			}
		}
	}

	return results, nil
}

// ListByTaskTrace returns all entries matching a task trace ID, ordered by
// timestamp (oldest first). This scans all log files and filters by the
// TaskTraceID field to reconstruct the full causality chain for a task lifecycle.
func (l *Log) ListByTaskTrace(traceID string) ([]Entry, error) {
	logFiles, err := l.logFilesSorted()
	if err != nil {
		return nil, err
	}

	var results []Entry

	for _, name := range logFiles {
		entries, readErr := l.readFile(filepath.Join(l.dir, name))
		if readErr != nil {
			return nil, readErr
		}

		for _, e := range entries {
			if e.TaskTraceID == traceID {
				results = append(results, e)
			}
		}
	}

	return results, nil
}

// ListByMethod returns entries matching the exact method name, ordered newest
// first. If limit <= 0, all matching entries are returned.
func (l *Log) ListByMethod(method string, limit int) ([]Entry, error) {
	logFiles, err := l.logFilesSorted()
	if err != nil {
		return nil, err
	}

	var results []Entry

	for _, v := range slices.Backward(logFiles) {
		entries, readErr := l.readFile(filepath.Join(l.dir, v))
		if readErr != nil {
			return nil, readErr
		}

		for _, v := range slices.Backward(entries) {
			if v.Method == method {
				results = append(results, v)

				if limit > 0 && len(results) >= limit {
					return results, nil
				}
			}
		}
	}

	return results, nil
}

// readFile reads all entries from a single JSONL file.
func (l *Log) readFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("open activity log file %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // Skip malformed lines.
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return entries, fmt.Errorf("scan activity log file %s: %w", path, err)
	}

	return entries, nil
}
