package timeline

import (
	"context"
	"testing"
	"time"

	"github.com/valksor/kvelmo/pkg/activitylog"
)

func newTestLog(t *testing.T) *activitylog.Log {
	t.Helper()
	dir := t.TempDir()
	l, err := activitylog.New(dir, 5)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return l
}

func writeAndClose(t *testing.T, l *activitylog.Log, entries []activitylog.Entry) {
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

func TestTaskTimeline_ReturnsMatchingEntries(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()

	entries := []activitylog.Entry{
		{Timestamp: now.Add(-3 * time.Minute), Method: "task.start", TaskTraceID: "trace-1", TaskID: "task-1"},
		{Timestamp: now.Add(-2 * time.Minute), Method: "task.plan", TaskTraceID: "trace-1", TaskID: "task-1"},
		{Timestamp: now.Add(-1 * time.Minute), Method: "task.start", TaskTraceID: "trace-2", TaskID: "task-2"},
		{Timestamp: now, Method: "task.implement", TaskTraceID: "trace-1", TaskID: "task-1"},
	}
	writeAndClose(t, l, entries)

	svc := New(l)
	activities, err := svc.TaskTimeline("trace-1", 0)
	if err != nil {
		t.Fatalf("TaskTimeline: %v", err)
	}
	if len(activities) != 3 {
		t.Fatalf("expected 3 activities, got %d", len(activities))
	}

	// Should be oldest-first
	if activities[0].Type != "task.start" {
		t.Errorf("first activity type = %q, want task.start", activities[0].Type)
	}
	if activities[2].Type != "task.implement" {
		t.Errorf("last activity type = %q, want task.implement", activities[2].Type)
	}

	// All should have trace-1
	for _, a := range activities {
		if a.TraceID != "trace-1" {
			t.Errorf("activity trace_id = %q, want trace-1", a.TraceID)
		}
	}
}

func TestTaskTimeline_RespectsLimit(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()

	entries := []activitylog.Entry{
		{Timestamp: now.Add(-3 * time.Minute), Method: "task.start", TaskTraceID: "trace-1"},
		{Timestamp: now.Add(-2 * time.Minute), Method: "task.plan", TaskTraceID: "trace-1"},
		{Timestamp: now.Add(-1 * time.Minute), Method: "task.implement", TaskTraceID: "trace-1"},
		{Timestamp: now, Method: "task.submit", TaskTraceID: "trace-1"},
	}
	writeAndClose(t, l, entries)

	svc := New(l)
	activities, err := svc.TaskTimeline("trace-1", 2)
	if err != nil {
		t.Fatalf("TaskTimeline: %v", err)
	}
	if len(activities) != 2 {
		t.Fatalf("expected 2 activities with limit, got %d", len(activities))
	}
}

func TestDescribe_TemplateFormatting(t *testing.T) {
	tests := []struct {
		name     string
		entry    activitylog.Entry
		wantDesc string
	}{
		{
			name:     "task.start with task ID",
			entry:    activitylog.Entry{Method: "task.start", TaskID: "my-task"},
			wantDesc: "Task loaded: my-task",
		},
		{
			name:     "task.plan no placeholder",
			entry:    activitylog.Entry{Method: "task.plan"},
			wantDesc: "Agent started planning",
		},
		{
			name:     "task.implement no placeholder",
			entry:    activitylog.Entry{Method: "task.implement"},
			wantDesc: "Agent started implementing",
		},
		{
			name:     "task.submit no placeholder",
			entry:    activitylog.Entry{Method: "task.submit"},
			wantDesc: "PR submitted",
		},
		{
			name:     "agent.error with error message",
			entry:    activitylog.Entry{Method: "agent.error", Error: "connection timeout"},
			wantDesc: "Agent error: connection timeout",
		},
		{
			name:     "unknown method",
			entry:    activitylog.Entry{Method: "custom.action"},
			wantDesc: "custom.action",
		},
		{
			name:     "unknown method with error",
			entry:    activitylog.Entry{Method: "custom.action", Error: "failed"},
			wantDesc: "custom.action (error: failed)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describe(tt.entry)
			if got != tt.wantDesc {
				t.Errorf("describe() = %q, want %q", got, tt.wantDesc)
			}
		})
	}
}

func TestProjectSummary_CountsByType(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()

	entries := []activitylog.Entry{
		{Timestamp: now.Add(-5 * time.Minute), Method: "task.start"},
		{Timestamp: now.Add(-4 * time.Minute), Method: "task.plan"},
		{Timestamp: now.Add(-3 * time.Minute), Method: "task.implement"},
		{Timestamp: now.Add(-2 * time.Minute), Method: "task.plan"},
		{Timestamp: now.Add(-1 * time.Minute), Method: "task.start"},
	}
	writeAndClose(t, l, entries)

	svc := New(l)
	counts, err := svc.ProjectSummary(now.Add(-10 * time.Minute))
	if err != nil {
		t.Fatalf("ProjectSummary: %v", err)
	}

	if counts["task.start"] != 2 {
		t.Errorf("task.start count = %d, want 2", counts["task.start"])
	}
	if counts["task.plan"] != 2 {
		t.Errorf("task.plan count = %d, want 2", counts["task.plan"])
	}
	if counts["task.implement"] != 1 {
		t.Errorf("task.implement count = %d, want 1", counts["task.implement"])
	}
}

func TestTaskTimeline_EmptyLog(t *testing.T) {
	l := newTestLog(t)
	writeAndClose(t, l, nil)

	svc := New(l)
	activities, err := svc.TaskTimeline("nonexistent-trace", 0)
	if err != nil {
		t.Fatalf("TaskTimeline: %v", err)
	}
	if len(activities) != 0 {
		t.Errorf("expected 0 activities, got %d", len(activities))
	}
}

func TestProjectSummary_EmptyLog(t *testing.T) {
	l := newTestLog(t)
	writeAndClose(t, l, nil)

	svc := New(l)
	counts, err := svc.ProjectSummary(time.Now().Add(-1 * time.Hour))
	if err != nil {
		t.Fatalf("ProjectSummary: %v", err)
	}
	if len(counts) != 0 {
		t.Errorf("expected empty counts, got %d entries", len(counts))
	}
}

func TestPhaseFromMethod(t *testing.T) {
	tests := []struct {
		method string
		want   string
	}{
		{"task.plan", "plan"},
		{"task.implement", "implement"},
		{"agent.connect", "connect"},
		{"start", "start"},
	}

	for _, tt := range tests {
		got := phaseFromMethod(tt.method)
		if got != tt.want {
			t.Errorf("phaseFromMethod(%q) = %q, want %q", tt.method, got, tt.want)
		}
	}
}
