package notify

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestService_SendChecksPreferences(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(filepath.Join(dir, "prefs.json"))

	var received []Notification
	var mu sync.Mutex

	svc.SetEventCallback(func(n Notification) {
		mu.Lock()
		received = append(received, n)
		mu.Unlock()
	})

	// Send an enabled notification.
	err := svc.Send(context.Background(), Notification{
		Type:  "task.phase_complete", // default: true
		Title: "Phase done",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	mu.Lock()
	if len(received) != 1 {
		t.Fatalf("expected 1 delivered notification, got %d", len(received))
	}
	mu.Unlock()
}

func TestService_SendSkipsDisabledNotifications(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(filepath.Join(dir, "prefs.json"))

	var received []Notification
	var mu sync.Mutex

	svc.SetEventCallback(func(n Notification) {
		mu.Lock()
		received = append(received, n)
		mu.Unlock()
	})

	// Disable the type.
	err := svc.PreferenceStore().SetPreference(Preference{
		Type:    "task.phase_complete",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	err = svc.Send(context.Background(), Notification{
		Type:  "task.phase_complete",
		Title: "Phase done",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	mu.Lock()
	if len(received) != 0 {
		t.Errorf("expected 0 delivered notifications (disabled), got %d", len(received))
	}
	mu.Unlock()
}

func TestService_EventCallbackCalledForEnabled(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(filepath.Join(dir, "prefs.json"))

	var callbackCalled atomic.Bool
	var received Notification
	var mu sync.Mutex

	svc.SetEventCallback(func(n Notification) {
		mu.Lock()
		received = n
		mu.Unlock()
		callbackCalled.Store(true)
	})

	err := svc.Send(context.Background(), Notification{
		Type:  "task.failed",
		Title: "Build broke",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if !callbackCalled.Load() {
		t.Error("expected event callback to be called")
	}

	mu.Lock()
	if received.Type != "task.failed" {
		t.Errorf("expected type=task.failed, got %s", received.Type)
	}
	if received.Title != "Build broke" {
		t.Errorf("expected title=Build broke, got %s", received.Title)
	}
	mu.Unlock()
}

func TestService_Notify_ImplementsConductorNotifier(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(filepath.Join(dir, "prefs.json"))

	var received []Notification
	var mu sync.Mutex

	svc.SetEventCallback(func(n Notification) {
		mu.Lock()
		received = append(received, n)
		mu.Unlock()
	})

	err := svc.Notify(context.Background(), "task.review_needed", map[string]any{
		"title":      "Review time",
		"body":       "PR is ready",
		"project_id": "proj-abc",
		"task_id":    "task-42",
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(received))
	}

	n := received[0]
	if n.Type != "task.review_needed" {
		t.Errorf("expected type=task.review_needed, got %s", n.Type)
	}
	if n.Title != "Review time" {
		t.Errorf("expected title=Review time, got %s", n.Title)
	}
	if n.Body != "PR is ready" {
		t.Errorf("expected body=PR is ready, got %s", n.Body)
	}
	if n.ProjectID != "proj-abc" {
		t.Errorf("expected project_id=proj-abc, got %s", n.ProjectID)
	}
	if n.TaskID != "task-42" {
		t.Errorf("expected task_id=task-42, got %s", n.TaskID)
	}
	if n.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestService_Notify_SkipsWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(filepath.Join(dir, "prefs.json"))

	// Disable the type.
	err := svc.PreferenceStore().SetPreference(Preference{
		Type:    "task.review_needed",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	var callbackCalled atomic.Bool
	svc.SetEventCallback(func(_ Notification) {
		callbackCalled.Store(true)
	})

	err = svc.Notify(context.Background(), "task.review_needed", map[string]any{
		"title": "Should not arrive",
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if callbackCalled.Load() {
		t.Error("expected callback NOT to be called for disabled type")
	}
}

func TestService_SendSetsTimestamp(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(filepath.Join(dir, "prefs.json"))

	var received Notification
	var mu sync.Mutex

	svc.SetEventCallback(func(n Notification) {
		mu.Lock()
		received = n
		mu.Unlock()
	})

	err := svc.Send(context.Background(), Notification{
		Type:  "task.failed",
		Title: "test",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	mu.Lock()
	if received.Timestamp.IsZero() {
		t.Error("expected Send to set timestamp when zero")
	}
	mu.Unlock()
}

func TestService_NoCallbackNoPanic(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(filepath.Join(dir, "prefs.json"))

	// No callback set, no webhooks — should not panic.
	err := svc.Send(context.Background(), Notification{
		Type:  "task.failed",
		Title: "test",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
}
