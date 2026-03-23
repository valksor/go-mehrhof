// Package ciwatch monitors CI pipeline status for submitted PRs.
package ciwatch

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Status represents the overall CI status for a PR.
type Status struct {
	State     string    `json:"state"` // "pending", "success", "failure", "unknown"
	Checks    []Check   `json:"checks"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Check represents a single CI check/pipeline.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pending", "success", "failure", "skipped"
	URL    string `json:"url,omitempty"`
}

// StatusFetcher retrieves CI status from a provider.
type StatusFetcher interface {
	FetchCIStatus(ctx context.Context, prID string) (*Status, error)
}

// LogsFetcher retrieves CI failure logs from a provider.
// Providers that support log extraction should implement this interface
// in addition to StatusFetcher.
type LogsFetcher interface {
	FetchCILogs(ctx context.Context, prID string) (string, error)
}

// FailedChecksSummary builds a human-readable summary of failed checks from a Status.
// Used as fallback when the provider does not implement LogsFetcher.
func FailedChecksSummary(s *Status) string {
	if s == nil {
		return ""
	}

	var parts []string
	for _, check := range s.Checks {
		if check.Status == "failure" {
			entry := "- " + check.Name + " (FAILED)"
			if check.URL != "" {
				entry += " " + check.URL
			}
			parts = append(parts, entry)
		}
	}

	if len(parts) == 0 {
		return "CI failed but no individual check failures were detected."
	}

	return "Failed CI checks:\n" + strings.Join(parts, "\n")
}

// Watcher polls CI status for a single PR and emits updates.
type Watcher struct {
	fetcher  StatusFetcher
	prID     string
	interval time.Duration

	mu     sync.RWMutex
	status *Status

	listeners []func(Status)
}

// New creates a CI watcher for a specific PR.
func New(fetcher StatusFetcher, prID string, interval time.Duration) *Watcher {
	if interval <= 0 {
		interval = 30 * time.Second
	}

	return &Watcher{
		fetcher:  fetcher,
		prID:     prID,
		interval: interval,
	}
}

// OnUpdate registers a callback for status changes.
func (w *Watcher) OnUpdate(fn func(Status)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.listeners = append(w.listeners, fn)
}

// Status returns the latest known CI status.
func (w *Watcher) Status() *Status {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.status
}

// Start polls CI status until ctx is cancelled or a terminal state is reached.
func (w *Watcher) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Initial fetch
	w.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll(ctx)

			w.mu.RLock()
			state := ""
			if w.status != nil {
				state = w.status.State
			}
			w.mu.RUnlock()

			// Stop polling on terminal states
			if state == "success" || state == "failure" {
				return
			}
		}
	}
}

func (w *Watcher) poll(ctx context.Context) {
	status, err := w.fetcher.FetchCIStatus(ctx, w.prID)
	if err != nil {
		slog.Debug("ciwatch: poll failed", "pr", w.prID, "error", err)

		return
	}

	w.mu.Lock()
	prev := w.status
	w.status = status
	listeners := make([]func(Status), len(w.listeners))
	copy(listeners, w.listeners)
	w.mu.Unlock()

	// Notify if state changed
	if prev == nil || prev.State != status.State {
		for _, fn := range listeners {
			fn(*status)
		}
	}
}
