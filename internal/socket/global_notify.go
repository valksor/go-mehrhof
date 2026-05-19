package socket

import (
	"context"
	"time"

	"github.com/valksor/kvelmo/internal/notify"
)

// notifier is set via SetNotifier to enable webhook notifications.
var notifier *notify.Notifier

// SetNotifier sets the global notifier for webhook dispatch.
func SetNotifier(n *notify.Notifier) {
	notifier = n
}

// GetNotifier returns the global notifier (may be nil if not configured).
func GetNotifier() *notify.Notifier {
	return notifier
}

// handleNotifyTest sends a test notification to all configured webhooks.
func (g *GlobalSocket) handleNotifyTest(_ context.Context, req *Request) (*Response, error) {
	if notifier == nil {
		return NewResultResponse(req.ID, map[string]any{
			"sent":     0,
			keyMessage: "notifications not enabled (set notify.enabled: true in config)",
		})
	}

	notifier.Send(notify.Payload{
		Event:       "test",
		Timestamp:   time.Now(),
		TaskID:      "test",
		TaskTitle:   "Test Notification",
		State:       string(StateNone),
		Message:     "This is a test notification from kvelmo",
		ProjectPath: "test",
	})

	return NewResultResponse(req.ID, map[string]any{
		"sent":     1,
		keyMessage: "test notification queued",
	})
}
