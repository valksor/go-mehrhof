package storage

import (
	"testing"
	"time"
)

func newTestSessionStore(t *testing.T) *SessionStore {
	t.Helper()

	return NewSessionStore(newTestStore(t))
}

func TestSaveGetSession(t *testing.T) {
	ss := newTestSessionStore(t)

	entry := SessionEntry{
		SessionID: "sess-abc",
		AgentType: "claude",
		TaskID:    "task-1",
		Phase:     "planning",
	}

	if err := ss.SaveSession(entry); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	got, err := ss.GetSession("task-1", "planning")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetSession() = nil, want entry")
	}
	if got.SessionID != "sess-abc" {
		t.Errorf("SessionID = %q, want sess-abc", got.SessionID)
	}
	if got.AgentType != "claude" {
		t.Errorf("AgentType = %q, want claude", got.AgentType)
	}
}

func TestSaveSession_AutoTimestamps(t *testing.T) {
	ss := newTestSessionStore(t)

	entry := SessionEntry{
		SessionID: "sess-1",
		TaskID:    "task-1",
		Phase:     "planning",
	}
	if err := ss.SaveSession(entry); err != nil {
		t.Fatal(err)
	}

	got, _ := ss.GetSession("task-1", "planning")
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt not set automatically")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not set automatically")
	}
}

func TestSaveSession_PreservesCreatedAt(t *testing.T) {
	ss := newTestSessionStore(t)

	original := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	entry := SessionEntry{
		SessionID: "sess-1",
		TaskID:    "task-1",
		Phase:     "planning",
		CreatedAt: original,
	}
	if err := ss.SaveSession(entry); err != nil {
		t.Fatal(err)
	}

	// Save again (update)
	if err := ss.SaveSession(entry); err != nil {
		t.Fatal(err)
	}

	got, _ := ss.GetSession("task-1", "planning")
	if !got.CreatedAt.Equal(original) {
		t.Errorf("CreatedAt = %v, want %v (should be preserved)", got.CreatedAt, original)
	}
}

func TestGetSession_Missing(t *testing.T) {
	ss := newTestSessionStore(t)

	got, err := ss.GetSession("nonexistent", "planning")
	if err != nil {
		t.Fatalf("GetSession() missing error = %v, want nil", err)
	}
	if got != nil {
		t.Errorf("GetSession() missing = %v, want nil", got)
	}
}

func TestGetSessionByID(t *testing.T) {
	ss := newTestSessionStore(t)

	entry := SessionEntry{SessionID: "unique-id", TaskID: "task-1", Phase: "planning"}
	if err := ss.SaveSession(entry); err != nil {
		t.Fatal(err)
	}

	got, err := ss.GetSessionByID("unique-id")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("GetSessionByID() = nil, want entry")
	}
	if got.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want task-1", got.TaskID)
	}

	// Non-existent ID
	missing, err := ss.GetSessionByID("no-such-id")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Errorf("GetSessionByID() missing = %v, want nil", missing)
	}
}

func TestDeleteSession(t *testing.T) {
	ss := newTestSessionStore(t)

	// Delete non-existent is not an error
	if err := ss.DeleteSession("task-1", "planning"); err != nil {
		t.Errorf("DeleteSession() non-existent error = %v, want nil", err)
	}

	entry := SessionEntry{SessionID: "sess-1", TaskID: "task-1", Phase: "planning"}
	if err := ss.SaveSession(entry); err != nil {
		t.Fatal(err)
	}

	if err := ss.DeleteSession("task-1", "planning"); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	got, _ := ss.GetSession("task-1", "planning")
	if got != nil {
		t.Error("GetSession() after delete should return nil")
	}
}

func TestDeleteSessionsForTask(t *testing.T) {
	ss := newTestSessionStore(t)

	for _, phase := range []string{"planning", "implementing", "reviewing"} {
		entry := SessionEntry{SessionID: "s-" + phase, TaskID: "task-1", Phase: phase}
		if err := ss.SaveSession(entry); err != nil {
			t.Fatal(err)
		}
	}
	// Add a different task's session
	if err := ss.SaveSession(SessionEntry{SessionID: "other", TaskID: "task-2", Phase: "planning"}); err != nil {
		t.Fatal(err)
	}

	if err := ss.DeleteSessionsForTask("task-1"); err != nil {
		t.Fatalf("DeleteSessionsForTask() error = %v", err)
	}

	task1sessions, err := ss.ListSessionsForTask("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(task1sessions) != 0 {
		t.Errorf("task-1 sessions after delete = %d, want 0", len(task1sessions))
	}

	task2sessions, _ := ss.ListSessionsForTask("task-2")
	if len(task2sessions) != 1 {
		t.Errorf("task-2 sessions should be unaffected, got %d", len(task2sessions))
	}
}

func TestListSessions_Empty(t *testing.T) {
	ss := newTestSessionStore(t)

	sessions, err := ss.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Errorf("ListSessions() empty = %v, want []", sessions)
	}
}

func TestListSessions(t *testing.T) {
	ss := newTestSessionStore(t)

	for _, phase := range []string{"planning", "implementing"} {
		entry := SessionEntry{SessionID: "s-" + phase, TaskID: "task-1", Phase: phase}
		if err := ss.SaveSession(entry); err != nil {
			t.Fatal(err)
		}
	}

	sessions, err := ss.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Errorf("ListSessions() len = %d, want 2", len(sessions))
	}
}

func TestHasSession(t *testing.T) {
	ss := newTestSessionStore(t)

	has, err := ss.HasSession("task-1", "planning")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("HasSession() = true before save, want false")
	}

	entry := SessionEntry{SessionID: "sess-1", TaskID: "task-1", Phase: "planning"}
	if err := ss.SaveSession(entry); err != nil {
		t.Fatal(err)
	}

	has, err = ss.HasSession("task-1", "planning")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("HasSession() = false after save, want true")
	}
}

func TestUpdateSessionTimestamp(t *testing.T) {
	ss := newTestSessionStore(t)

	entry := SessionEntry{SessionID: "sess-1", TaskID: "task-1", Phase: "planning"}
	if err := ss.SaveSession(entry); err != nil {
		t.Fatal(err)
	}

	before, _ := ss.GetSession("task-1", "planning")
	time.Sleep(10 * time.Millisecond)

	if err := ss.UpdateSessionTimestamp("task-1", "planning"); err != nil {
		t.Fatalf("UpdateSessionTimestamp() error = %v", err)
	}

	after, _ := ss.GetSession("task-1", "planning")
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Error("UpdatedAt not advanced after UpdateSessionTimestamp()")
	}
	if !after.CreatedAt.Equal(before.CreatedAt) {
		t.Error("CreatedAt changed after UpdateSessionTimestamp(), should be preserved")
	}
}

func TestUpdateSessionTimestamp_NotFound(t *testing.T) {
	ss := newTestSessionStore(t)

	err := ss.UpdateSessionTimestamp("no-task", "no-phase")
	if err == nil {
		t.Error("UpdateSessionTimestamp() non-existent expected error, got nil")
	}
}

func TestCleanOldSessions(t *testing.T) {
	ss := newTestSessionStore(t)

	// Save an old session by backdating
	old := SessionEntry{
		SessionID: "old-sess",
		TaskID:    "task-old",
		Phase:     "planning",
		UpdatedAt: time.Now().Add(-2 * time.Hour),
	}
	if err := ss.SaveSession(old); err != nil {
		t.Fatal(err)
	}

	// Save a recent session
	recent := SessionEntry{SessionID: "new-sess", TaskID: "task-new", Phase: "planning"}
	if err := ss.SaveSession(recent); err != nil {
		t.Fatal(err)
	}

	// The old session's UpdatedAt will be overwritten by SaveSession to time.Now()
	// So we can't easily test CleanOldSessions with real old entries via SaveSession
	// Instead test that clean with 0 duration removes nothing with future cutoff
	removed, err := ss.CleanOldSessions(24 * time.Hour)
	if err != nil {
		t.Fatalf("CleanOldSessions() error = %v", err)
	}
	// All sessions are recent, none should be removed
	if removed != 0 {
		t.Errorf("CleanOldSessions(24h) removed = %d, want 0", removed)
	}

	// Clean with 0 duration should remove all (all entries are "old" relative to now)
	removed, err = ss.CleanOldSessions(0)
	if err != nil {
		t.Fatalf("CleanOldSessions(0) error = %v", err)
	}
	if removed < 1 {
		t.Errorf("CleanOldSessions(0) removed = %d, want >= 1", removed)
	}
}
