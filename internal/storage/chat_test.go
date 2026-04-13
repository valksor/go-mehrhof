package storage

import (
	"testing"
)

func newTestChatStore(t *testing.T) *ChatStore {
	t.Helper()

	return NewChatStore(newTestStore(t))
}

func TestSaveLoadMessage(t *testing.T) {
	cs := newTestChatStore(t)

	msg := ChatMessage{
		ID:      "msg-1",
		Role:    "user",
		Content: "Hello, world!",
	}

	if err := cs.SaveMessage("task-1", msg); err != nil {
		t.Fatalf("SaveMessage() error = %v", err)
	}

	history, err := cs.LoadHistory("task-1")
	if err != nil {
		t.Fatalf("LoadHistory() error = %v", err)
	}

	if history.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want task-1", history.TaskID)
	}
	if len(history.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(history.Messages))
	}
	if history.Messages[0].Content != "Hello, world!" {
		t.Errorf("Content = %q, want Hello, world!", history.Messages[0].Content)
	}
}

func TestSaveMessage_AutoTimestamp(t *testing.T) {
	cs := newTestChatStore(t)

	msg := ChatMessage{ID: "msg-1", Role: "user", Content: "test"}
	if err := cs.SaveMessage("task-1", msg); err != nil {
		t.Fatal(err)
	}

	history, err := cs.LoadHistory("task-1")
	if err != nil {
		t.Fatal(err)
	}

	if history.Messages[0].Timestamp == "" {
		t.Error("Timestamp not auto-set when empty")
	}
}

func TestSaveMessage_PreservesTimestamp(t *testing.T) {
	cs := newTestChatStore(t)

	msg := ChatMessage{
		ID:        "msg-1",
		Role:      "user",
		Content:   "test",
		Timestamp: "2024-01-01T00:00:00Z",
	}
	if err := cs.SaveMessage("task-1", msg); err != nil {
		t.Fatal(err)
	}

	history, _ := cs.LoadHistory("task-1")
	if history.Messages[0].Timestamp != "2024-01-01T00:00:00Z" {
		t.Errorf("Timestamp = %q, want 2024-01-01T00:00:00Z", history.Messages[0].Timestamp)
	}
}

func TestLoadHistory_Missing(t *testing.T) {
	cs := newTestChatStore(t)

	history, err := cs.LoadHistory("nonexistent-task")
	if err != nil {
		t.Fatalf("LoadHistory() missing error = %v, want nil", err)
	}
	if history == nil {
		t.Fatal("LoadHistory() = nil, want empty history")
	}
	if len(history.Messages) != 0 {
		t.Errorf("Messages len = %d, want 0", len(history.Messages))
	}
	if history.TaskID != "nonexistent-task" {
		t.Errorf("TaskID = %q, want nonexistent-task", history.TaskID)
	}
}

func TestSaveMultipleMessages(t *testing.T) {
	cs := newTestChatStore(t)

	messages := []ChatMessage{
		{ID: "1", Role: "user", Content: "First"},
		{ID: "2", Role: "assistant", Content: "Second"},
		{ID: "3", Role: "user", Content: "Third"},
	}

	for _, msg := range messages {
		if err := cs.SaveMessage("task-1", msg); err != nil {
			t.Fatalf("SaveMessage(%q) error = %v", msg.ID, err)
		}
	}

	history, err := cs.LoadHistory("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Messages) != 3 {
		t.Errorf("Messages len = %d, want 3", len(history.Messages))
	}
}

func TestClearHistory(t *testing.T) {
	cs := newTestChatStore(t)

	for i, role := range []string{"user", "assistant"} {
		msg := ChatMessage{ID: string(rune('1' + i)), Role: role, Content: "msg"}
		if err := cs.SaveMessage("task-1", msg); err != nil {
			t.Fatal(err)
		}
	}

	if err := cs.ClearHistory("task-1"); err != nil {
		t.Fatalf("ClearHistory() error = %v", err)
	}

	history, err := cs.LoadHistory("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Messages) != 0 {
		t.Errorf("Messages len after clear = %d, want 0", len(history.Messages))
	}
}

func TestDeleteHistory(t *testing.T) {
	cs := newTestChatStore(t)

	if err := cs.SaveMessage("task-1", ChatMessage{ID: "1", Role: "user", Content: "hi"}); err != nil {
		t.Fatal(err)
	}

	if err := cs.DeleteHistory("task-1"); err != nil {
		t.Fatalf("DeleteHistory() error = %v", err)
	}

	// After delete, loading returns empty (not error)
	history, err := cs.LoadHistory("task-1")
	if err != nil {
		t.Fatalf("LoadHistory() after delete error = %v", err)
	}
	if len(history.Messages) != 0 {
		t.Errorf("Messages len after delete = %d, want 0", len(history.Messages))
	}
}

func TestDeleteHistory_NonExistent(t *testing.T) {
	cs := newTestChatStore(t)

	if err := cs.DeleteHistory("never-saved"); err != nil {
		t.Errorf("DeleteHistory() non-existent error = %v, want nil", err)
	}
}

func TestMessageCount(t *testing.T) {
	cs := newTestChatStore(t)

	n, err := cs.MessageCount("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("MessageCount() empty = %d, want 0", n)
	}

	for i := range 3 {
		msg := ChatMessage{ID: string(rune('a' + i)), Role: "user", Content: "msg"}
		if err := cs.SaveMessage("task-1", msg); err != nil {
			t.Fatal(err)
		}
	}

	n, err = cs.MessageCount("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("MessageCount() = %d, want 3", n)
	}
}

func TestGetLastMessage_Empty(t *testing.T) {
	cs := newTestChatStore(t)

	msg, err := cs.GetLastMessage("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Errorf("GetLastMessage() empty = %v, want nil", msg)
	}
}

func TestGetLastMessage_ReturnsLast(t *testing.T) {
	cs := newTestChatStore(t)

	messages := []ChatMessage{
		{ID: "1", Role: "user", Content: "First"},
		{ID: "2", Role: "assistant", Content: "Last"},
	}
	for _, msg := range messages {
		if err := cs.SaveMessage("task-1", msg); err != nil {
			t.Fatal(err)
		}
	}

	got, err := cs.GetLastMessage("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("GetLastMessage() = nil, want last message")
	}
	if got.Content != "Last" {
		t.Errorf("Content = %q, want Last", got.Content)
	}
}

func TestGetMessagesByRole(t *testing.T) {
	cs := newTestChatStore(t)

	messages := []ChatMessage{
		{ID: "1", Role: "user", Content: "u1"},
		{ID: "2", Role: "assistant", Content: "a1"},
		{ID: "3", Role: "user", Content: "u2"},
		{ID: "4", Role: "system", Content: "s1"},
	}
	for _, msg := range messages {
		if err := cs.SaveMessage("task-1", msg); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		role      string
		wantCount int
	}{
		{"user", 2},
		{"assistant", 1},
		{"system", 1},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			msgs, err := cs.GetMessagesByRole("task-1", tt.role)
			if err != nil {
				t.Fatal(err)
			}
			if len(msgs) != tt.wantCount {
				t.Errorf("GetMessagesByRole(%q) len = %d, want %d", tt.role, len(msgs), tt.wantCount)
			}
		})
	}
}
