// Package eventlog provides an append-only event log for orchestration state.
// Events are stored as JSONL files per task, enabling deterministic replay
// and debugging of the complete task lifecycle.
package eventlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CurrentEventLogVersion is the schema version stamped on each event entry.
// Bump it when the Entry shape changes incompatibly. Because a log file may
// hold entries written by several kvelmo versions across a task's life, the
// version is per-entry rather than per-file, and readers tolerate older
// entries (JSON ignores fields they predate).
const CurrentEventLogVersion = 1

// EventType identifies the kind of orchestration event.
type EventType string

const (
	EventPhaseStarted      EventType = "phase_started"
	EventPhaseCompleted    EventType = "phase_completed"
	EventPhaseFailed       EventType = "phase_failed"
	EventCheckpointCreated EventType = "checkpoint_created"
	EventFindingDetected   EventType = "finding_detected"
	EventRouterDecision    EventType = "router_decision"
	EventSpecChanged       EventType = "spec_changed"
	EventGuardrailChecked  EventType = "guardrail_checked"
	EventTaskLoaded        EventType = "task_loaded"
	EventTaskFinished      EventType = "task_finished"
	EventRiskEvaluated     EventType = "risk_evaluated"
)

// Entry is a single event in the log.
type Entry struct {
	// Version is the event-log schema version (CurrentEventLogVersion). Stamped
	// on Append; a missing or zero value identifies a pre-versioning entry.
	Version   int            `json:"v,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Type      EventType      `json:"type"`
	Phase     string         `json:"phase,omitempty"`
	Message   string         `json:"message,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

// Log is an append-only event log backed by a JSONL file.
type Log struct {
	mu   sync.Mutex
	file *os.File
	dir  string
}

// New creates or opens an event log in the given directory.
// Events are stored in {dir}/events.jsonl.
func New(dir string) (*Log, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create event log dir: %w", err)
	}

	path := filepath.Join(dir, "events.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open event log: %w", err)
	}

	return &Log{file: f, dir: dir}, nil
}

// Append writes an event to the log.
func (l *Log) Append(entry Entry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.Version == 0 {
		entry.Version = CurrentEventLogVersion
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write event: %w", err)
	}

	return nil
}

// Query reads all entries matching the given event type, returning the last limit entries.
func (l *Log) Query(eventType EventType, limit int) ([]Entry, error) {
	all, err := ReadAll(l.dir)
	if err != nil {
		return nil, err
	}

	var results []Entry
	for _, e := range all {
		if e.Type == eventType {
			results = append(results, e)
		}
	}

	// Return only the last `limit` entries.
	if limit > 0 && len(results) > limit {
		results = results[len(results)-limit:]
	}

	return results, nil
}

// Close closes the log file.
func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}

	return nil
}

// ReadAll reads all events from the log file.
func ReadAll(dir string) ([]Entry, error) {
	path := filepath.Join(dir, "events.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("open event log: %w", err)
	}
	defer func() { _ = f.Close() }()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			slog.Debug("skipping malformed event log entry", "error", err)

			continue
		}

		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// Query returns events matching the given filters.
func Query(dir string, opts QueryOptions) ([]Entry, error) {
	all, err := ReadAll(dir)
	if err != nil {
		return nil, err
	}

	var results []Entry
	for _, e := range all {
		if opts.Type != "" && e.Type != opts.Type {
			continue
		}

		if opts.Phase != "" && e.Phase != opts.Phase {
			continue
		}

		if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
			continue
		}

		if !opts.Until.IsZero() && e.Timestamp.After(opts.Until) {
			continue
		}

		results = append(results, e)
	}

	return results, nil
}

// QueryOptions filters events in a query.
type QueryOptions struct {
	Type  EventType
	Phase string
	Since time.Time
	Until time.Time
}
