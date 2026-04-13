package activitylog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestLog(t *testing.T) *Log {
	t.Helper()
	dir := t.TempDir()
	l, err := New(dir, 5)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return l
}

func TestLog_RecordAndQuery(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()
	l.nowFunc = func() time.Time { return now }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Start(ctx)
		close(done)
	}()

	for i := range 5 {
		l.Record(Entry{
			Timestamp:     now.Add(time.Duration(i) * time.Second),
			Method:        "test.method",
			CorrelationID: "corr-1",
			DurationMs:    int64(10 + i),
			ParamsSize:    100 + i,
		})
	}

	// Give the writer goroutine time to flush.
	cancel()
	<-done
	l.Close()

	entries, err := l.Query(QueryOptions{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	// Results are newest-first.
	if entries[0].DurationMs != 14 {
		t.Errorf("expected newest entry DurationMs=14, got %d", entries[0].DurationMs)
	}
	if entries[0].Method != "test.method" {
		t.Errorf("expected method test.method, got %s", entries[0].Method)
	}
	if entries[0].CorrelationID != "corr-1" {
		t.Errorf("expected correlation_id corr-1, got %s", entries[0].CorrelationID)
	}
}

func TestLog_QueryWithFilters(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()
	l.nowFunc = func() time.Time { return now }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Start(ctx)
		close(done)
	}()

	entries := []Entry{
		{Timestamp: now.Add(-2 * time.Hour), Method: "task.start", DurationMs: 10, ParamsSize: 50},
		{Timestamp: now.Add(-1 * time.Hour), Method: "task.plan", DurationMs: 20, ParamsSize: 60},
		{Timestamp: now.Add(-30 * time.Minute), Method: "task.implement", DurationMs: 30, Error: "timeout", ParamsSize: 70},
		{Timestamp: now.Add(-10 * time.Minute), Method: "task.review", DurationMs: 40, ParamsSize: 80},
		{Timestamp: now.Add(-5 * time.Minute), Method: "task.submit", DurationMs: 50, Error: "conflict", ParamsSize: 90},
	}
	for _, e := range entries {
		l.Record(e)
	}

	cancel()
	<-done
	l.Close()

	t.Run("Since", func(t *testing.T) {
		results, err := l.Query(QueryOptions{Since: 45 * time.Minute})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 entries within 45min, got %d", len(results))
		}
	})

	t.Run("MethodPattern", func(t *testing.T) {
		results, err := l.Query(QueryOptions{MethodPattern: "start|plan|implement"})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 entries matching pattern, got %d", len(results))
		}
	})

	t.Run("ErrorsOnly", func(t *testing.T) {
		results, err := l.Query(QueryOptions{ErrorsOnly: true})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 error entries, got %d", len(results))
		}
	})

	t.Run("Limit", func(t *testing.T) {
		results, err := l.Query(QueryOptions{Limit: 2})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 entries with limit, got %d", len(results))
		}
	})
}

func TestLog_NonBlocking(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir, 5)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Do NOT start the writer — channel will fill up.
	// Fill the channel.
	for range channelCapacity {
		l.Record(Entry{
			Timestamp: time.Now(),
			Method:    "fill",
		})
	}

	// This must not block even though the channel is full.
	blocked := make(chan struct{})
	go func() {
		l.Record(Entry{
			Timestamp: time.Now(),
			Method:    "overflow",
		})
		close(blocked)
	}()

	select {
	case <-blocked:
		// Success: Record returned without blocking.
	case <-time.After(1 * time.Second):
		t.Fatal("Record blocked on full channel")
	}
}

func TestLog_FileRotation(t *testing.T) {
	l := newTestLog(t)

	day1 := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Start(ctx)
		close(done)
	}()

	// Write entry for day 1.
	l.Record(Entry{
		Timestamp:  day1,
		Method:     "day1.method",
		DurationMs: 10,
		ParamsSize: 50,
	})

	// Give writer time to process.
	time.Sleep(50 * time.Millisecond)

	// Write entry for day 2 — should trigger file rotation.
	l.Record(Entry{
		Timestamp:  day2,
		Method:     "day2.method",
		DurationMs: 20,
		ParamsSize: 60,
	})

	cancel()
	<-done
	l.Close()

	// Verify two files were created.
	file1 := filepath.Join(l.dir, "activity-2026-03-14.jsonl")
	file2 := filepath.Join(l.dir, "activity-2026-03-15.jsonl")

	if _, err := os.Stat(file1); err != nil {
		t.Errorf("expected file for day 1: %v", err)
	}
	if _, err := os.Stat(file2); err != nil {
		t.Errorf("expected file for day 2: %v", err)
	}

	// Verify entries are in the correct files.
	l.nowFunc = func() time.Time { return day2.Add(time.Hour) }
	entries, err := l.Query(QueryOptions{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries total, got %d", len(entries))
	}
	if entries[0].Method != "day2.method" {
		t.Errorf("expected newest entry method=day2.method, got %s", entries[0].Method)
	}
	if entries[1].Method != "day1.method" {
		t.Errorf("expected oldest entry method=day1.method, got %s", entries[1].Method)
	}
}

func TestEntryNewFields(t *testing.T) {
	entry := Entry{
		Timestamp:  time.Now(),
		Method:     "task.start",
		UserID:     "testuser",
		TaskID:     "task-123",
		AgentModel: "claude",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Entry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.UserID != "testuser" {
		t.Errorf("UserID = %q, want %q", decoded.UserID, "testuser")
	}
	if decoded.TaskID != "task-123" {
		t.Errorf("TaskID = %q, want %q", decoded.TaskID, "task-123")
	}
	if decoded.AgentModel != "claude" {
		t.Errorf("AgentModel = %q, want %q", decoded.AgentModel, "claude")
	}
}

func TestEntryFilesAffected_Serialization(t *testing.T) {
	entry := Entry{
		Timestamp:     time.Now(),
		Method:        "submit",
		FilesAffected: []string{"pkg/auth/handler.go", "pkg/auth/handler_test.go"},
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Entry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.FilesAffected) != 2 {
		t.Fatalf("FilesAffected length = %d, want 2", len(decoded.FilesAffected))
	}
	if decoded.FilesAffected[0] != "pkg/auth/handler.go" {
		t.Errorf("FilesAffected[0] = %q, want pkg/auth/handler.go", decoded.FilesAffected[0])
	}
}

func TestEntryFilesAffected_OmittedWhenEmpty(t *testing.T) {
	entry := Entry{
		Timestamp: time.Now(),
		Method:    "ping",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// FilesAffected should be omitted from JSON when nil
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, exists := raw["files_affected"]; exists {
		t.Error("files_affected should be omitted when empty")
	}
}

func TestEntryFilesAffected_RoundTrip(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Start(ctx)
		close(done)
	}()

	l.Record(Entry{
		Timestamp:     now,
		Method:        "implement",
		DurationMs:    500,
		FilesAffected: []string{"main.go", "util.go"},
	})

	cancel()
	<-done
	l.Close()

	entries, err := l.Query(QueryOptions{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if len(entries[0].FilesAffected) != 2 {
		t.Errorf("FilesAffected length = %d, want 2", len(entries[0].FilesAffected))
	}
}
