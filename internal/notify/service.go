package notify

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/valksor/kvelmo/paths"
)

// Notification represents a single notification to deliver.
type Notification struct {
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	ProjectID string         `json:"project_id,omitempty"`
	TaskID    string         `json:"task_id,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// EventCallback is called for real-time delivery (e.g., WebSocket).
type EventCallback func(notification Notification)

// Service manages notification creation and delivery.
// It combines the type registry, user preferences, webhook delivery, and
// real-time event callbacks into a single coordinator.
type Service struct {
	prefs    *PreferenceStore
	webhooks *Notifier // existing webhook notifier (may be nil)
	callback EventCallback
	mu       sync.RWMutex
}

// DefaultPreferencesPath returns the default path for the preferences JSON file.
func DefaultPreferencesPath() string {
	return paths.BaseDir() + "/notify-prefs.json"
}

// NewService creates a notification service with preference storage at the given path.
func NewService(prefsPath string) *Service {
	return &Service{
		prefs: NewPreferenceStore(prefsPath),
	}
}

// SetEventCallback sets the real-time delivery callback.
func (s *Service) SetEventCallback(fn EventCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.callback = fn
}

// SetWebhookNotifier configures the underlying webhook notifier for delivery.
func (s *Service) SetWebhookNotifier(n *Notifier) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.webhooks = n
}

// PreferenceStore returns the underlying preference store for direct access.
func (s *Service) PreferenceStore() *PreferenceStore {
	return s.prefs
}

// Send creates and delivers a notification. It checks preferences before
// dispatching to webhooks and the event callback.
func (s *Service) Send(ctx context.Context, n Notification) error {
	if n.Timestamp.IsZero() {
		n.Timestamp = time.Now()
	}

	// Check if this notification type is enabled for the project.
	if !s.prefs.GetPreference(n.Type, n.ProjectID) {
		slog.Debug("notify: notification suppressed by preference",
			"type", n.Type,
			"project_id", n.ProjectID,
		)

		return nil
	}

	s.mu.RLock()
	cb := s.callback
	wh := s.webhooks
	s.mu.RUnlock()

	// Deliver to real-time callback.
	if cb != nil {
		cb(n)
	}

	// Deliver to webhooks.
	if wh != nil {
		wh.Send(Payload{
			Event:       n.Type,
			Timestamp:   n.Timestamp,
			TaskID:      n.TaskID,
			TaskTitle:   n.Title,
			Message:     n.Body,
			ProjectPath: n.ProjectID,
		})
	}

	return nil
}

// Notify implements the conductor.Notifier interface.
// It maps conductor event data into a Notification and delivers it.
func (s *Service) Notify(ctx context.Context, eventType string, data map[string]any) error {
	n := Notification{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}

	if v, ok := data["title"].(string); ok {
		n.Title = v
	}

	if v, ok := data["body"].(string); ok {
		n.Body = v
	}

	if v, ok := data["project_id"].(string); ok {
		n.ProjectID = v
	}

	if v, ok := data["task_id"].(string); ok {
		n.TaskID = v
	}

	return s.Send(ctx, n)
}
