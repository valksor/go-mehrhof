package activitylog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// recordAndDrain starts the writer, records the given entries, then stops and
// waits for the writer to flush before returning.
func recordAndDrain(t *testing.T, l *Log, entries []Entry) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Start(ctx)
		close(done)
	}()

	for _, e := range entries {
		l.Record(e)
	}

	cancel()
	<-done
	l.Close()
}

func TestNew_DefaultMaxFiles(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir, 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if l.maxFiles != defaultMaxFiles {
		t.Errorf("maxFiles = %d, want %d", l.maxFiles, defaultMaxFiles)
	}
	if l.dir != dir {
		t.Errorf("dir = %q, want %q", l.dir, dir)
	}
}

func TestNew_NegativeMaxFiles(t *testing.T) {
	l, err := New(t.TempDir(), -5)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if l.maxFiles != defaultMaxFiles {
		t.Errorf("maxFiles = %d, want %d", l.maxFiles, defaultMaxFiles)
	}
}

func TestNew_CreatesNestedDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "activity")
	if _, err := New(dir, 5); err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("expected dir to be created: %v", err)
	}
}

func TestSetForwarder_ForwardsOnRecord(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	l := newTestLog(t)
	l.SetForwarder(NewForwarder(srv.URL, ""))
	if l.forwarder == nil {
		t.Fatal("SetForwarder did not set forwarder")
	}

	// Keep the context alive while async forward goroutines complete. Cancelling
	// the writer context would also cancel in-flight forward requests, so we wait
	// for the forwards to land before stopping the writer.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Start(ctx)
		close(done)
	}()

	l.Record(Entry{Timestamp: time.Now(), Method: "task.start"})
	l.Record(Entry{Timestamp: time.Now(), Method: "task.plan"})

	deadline := time.After(2 * time.Second)
	for hits.Load() < 2 {
		select {
		case <-deadline:
			cancel()
			<-done
			l.Close()
			t.Fatalf("forwarder hit endpoint %d times, want 2", hits.Load())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	cancel()
	<-done
	l.Close()
}

func TestListByTaskTrace(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()
	recordAndDrain(t, l, []Entry{
		{Timestamp: now.Add(-3 * time.Minute), Method: "task.start", TaskTraceID: "trace-a"},
		{Timestamp: now.Add(-2 * time.Minute), Method: "task.plan", TaskTraceID: "trace-b"},
		{Timestamp: now.Add(-1 * time.Minute), Method: "task.implement", TaskTraceID: "trace-a"},
		{Timestamp: now, Method: "task.submit", TaskTraceID: "trace-a"},
	})

	t.Run("matching trace ordered oldest-first", func(t *testing.T) {
		got, err := l.ListByTaskTrace("trace-a")
		if err != nil {
			t.Fatalf("ListByTaskTrace: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(got))
		}
		if got[0].Method != "task.start" {
			t.Errorf("first method = %q, want task.start", got[0].Method)
		}
		if got[2].Method != "task.submit" {
			t.Errorf("last method = %q, want task.submit", got[2].Method)
		}
	})

	t.Run("unknown trace returns empty", func(t *testing.T) {
		got, err := l.ListByTaskTrace("trace-none")
		if err != nil {
			t.Fatalf("ListByTaskTrace: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected 0 entries, got %d", len(got))
		}
	})
}

func TestListByMethod(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()
	recordAndDrain(t, l, []Entry{
		{Timestamp: now.Add(-4 * time.Minute), Method: "task.plan", DurationMs: 1},
		{Timestamp: now.Add(-3 * time.Minute), Method: "task.implement", DurationMs: 2},
		{Timestamp: now.Add(-2 * time.Minute), Method: "task.plan", DurationMs: 3},
		{Timestamp: now.Add(-1 * time.Minute), Method: "task.plan", DurationMs: 4},
	})

	t.Run("returns matching newest-first", func(t *testing.T) {
		got, err := l.ListByMethod("task.plan", 0)
		if err != nil {
			t.Fatalf("ListByMethod: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(got))
		}
		// Newest first => DurationMs 4 should come first.
		if got[0].DurationMs != 4 {
			t.Errorf("first DurationMs = %d, want 4", got[0].DurationMs)
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		got, err := l.ListByMethod("task.plan", 2)
		if err != nil {
			t.Fatalf("ListByMethod: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(got))
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		got, err := l.ListByMethod("task.nonexistent", 0)
		if err != nil {
			t.Fatalf("ListByMethod: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected 0 entries, got %d", len(got))
		}
	})
}

func TestReadFile_SkipsMalformedLines(t *testing.T) {
	l := newTestLog(t)
	path := filepath.Join(l.dir, filePrefix+"2026-01-02"+fileExt)
	// Valid entry, malformed JSON, empty line, valid entry.
	content := `{"method":"task.start","timestamp":"2026-01-02T10:00:00Z"}
not-json-at-all

{"method":"task.plan","timestamp":"2026-01-02T11:00:00Z"}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	entries, err := l.readFile(path)
	if err != nil {
		t.Fatalf("readFile: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 valid entries (malformed/empty skipped), got %d", len(entries))
	}
	if entries[0].Method != "task.start" || entries[1].Method != "task.plan" {
		t.Errorf("unexpected methods: %q, %q", entries[0].Method, entries[1].Method)
	}
}

func TestReadFile_MissingReturnsNil(t *testing.T) {
	l := newTestLog(t)
	entries, err := l.readFile(filepath.Join(l.dir, "does-not-exist.jsonl"))
	if err != nil {
		t.Fatalf("readFile on missing file should not error: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries, got %v", entries)
	}
}

func TestCleanup_RemovesOldestFilesBeyondMax(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir, 2) // keep only 2 files
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Pre-seed three older daily files so cleanup must prune them.
	for _, day := range []string{"2026-01-01", "2026-01-02", "2026-01-03"} {
		p := filepath.Join(dir, filePrefix+day+fileExt)
		if err := os.WriteFile(p, []byte("{}\n"), 0o600); err != nil {
			t.Fatalf("seed file: %v", err)
		}
	}
	// A non-matching file must be left untouched.
	other := filepath.Join(dir, "unrelated.txt")
	if err := os.WriteFile(other, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed unrelated: %v", err)
	}

	// Writing an entry on a new day triggers rotation + cleanup.
	recordAndDrain(t, l, []Entry{
		{Timestamp: time.Date(2026, 1, 4, 12, 0, 0, 0, time.UTC), Method: "task.start"},
	})

	files, err := l.logFilesSorted()
	if err != nil {
		t.Fatalf("logFilesSorted: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 log files after cleanup, got %d: %v", len(files), files)
	}
	// The newest day's file must survive.
	if files[len(files)-1] != filePrefix+"2026-01-04"+fileExt {
		t.Errorf("newest file = %q, want 2026-01-04", files[len(files)-1])
	}
	// The unrelated file must remain.
	if _, err := os.Stat(other); err != nil {
		t.Errorf("unrelated file removed: %v", err)
	}
}

func TestForward_MarshalFailureDoesNotPanic(t *testing.T) {
	// An Entry contains no un-marshalable fields, so exercise the request-create
	// failure path instead: a control char in the URL makes NewRequest fail.
	f := NewForwarder("http://\x7f-bad-url", "")
	// Should return quietly without panicking.
	f.Forward(context.Background(), testEntry())
}
