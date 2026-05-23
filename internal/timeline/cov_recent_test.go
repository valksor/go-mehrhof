package timeline

import (
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/activitylog"
)

func TestRecentActivity_NewestFirst(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()

	entries := []activitylog.Entry{
		{Timestamp: now.Add(-3 * time.Minute), Method: "task.start", TaskID: "t1"},
		{Timestamp: now.Add(-2 * time.Minute), Method: "task.plan", TaskID: "t1"},
		{Timestamp: now.Add(-1 * time.Minute), Method: "task.implement", TaskID: "t1"},
	}
	writeAndClose(t, l, entries)

	svc := New(l)
	activities, err := svc.RecentActivity(activitylog.QueryOptions{})
	if err != nil {
		t.Fatalf("RecentActivity: %v", err)
	}
	if len(activities) != 3 {
		t.Fatalf("expected 3 activities, got %d", len(activities))
	}

	// RecentActivity reverses the newest-first Query output, yielding oldest-first.
	if activities[0].Type != "task.start" {
		t.Errorf("first activity type = %q, want task.start", activities[0].Type)
	}
	if activities[2].Type != "task.implement" {
		t.Errorf("last activity type = %q, want task.implement", activities[2].Type)
	}
}

func TestRecentActivity_HonorsQueryFilters(t *testing.T) {
	l := newTestLog(t)
	now := time.Now()

	entries := []activitylog.Entry{
		{Timestamp: now.Add(-3 * time.Minute), Method: "task.start"},
		{Timestamp: now.Add(-2 * time.Minute), Method: "agent.error", Error: "boom"},
		{Timestamp: now.Add(-1 * time.Minute), Method: "task.plan"},
	}
	writeAndClose(t, l, entries)

	svc := New(l)

	t.Run("errors only", func(t *testing.T) {
		activities, err := svc.RecentActivity(activitylog.QueryOptions{ErrorsOnly: true})
		if err != nil {
			t.Fatalf("RecentActivity: %v", err)
		}
		if len(activities) != 1 {
			t.Fatalf("expected 1 error activity, got %d", len(activities))
		}
		if activities[0].Error != "boom" {
			t.Errorf("error = %q, want boom", activities[0].Error)
		}
		if activities[0].Description != "Agent error: boom" {
			t.Errorf("description = %q, want %q", activities[0].Description, "Agent error: boom")
		}
	})

	t.Run("limit", func(t *testing.T) {
		activities, err := svc.RecentActivity(activitylog.QueryOptions{Limit: 1})
		if err != nil {
			t.Fatalf("RecentActivity: %v", err)
		}
		if len(activities) != 1 {
			t.Fatalf("expected 1 activity with limit, got %d", len(activities))
		}
	})
}

func TestRecentActivity_EmptyLog(t *testing.T) {
	l := newTestLog(t)
	writeAndClose(t, l, nil)

	svc := New(l)
	activities, err := svc.RecentActivity(activitylog.QueryOptions{})
	if err != nil {
		t.Fatalf("RecentActivity: %v", err)
	}
	if len(activities) != 0 {
		t.Errorf("expected 0 activities, got %d", len(activities))
	}
}
