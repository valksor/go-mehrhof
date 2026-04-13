package ciwatch

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type mockFetcher struct {
	calls  atomic.Int32
	status *Status
}

func (m *mockFetcher) FetchCIStatus(_ context.Context, _ string) (*Status, error) {
	m.calls.Add(1)

	return m.status, nil
}

func TestWatcher_PollsAndNotifies(t *testing.T) {
	fetcher := &mockFetcher{
		status: &Status{
			State:     "pending",
			UpdatedAt: time.Now(),
		},
	}

	w := New(fetcher, "PR-1", 10*time.Millisecond)

	var notified atomic.Int32
	w.OnUpdate(func(_ Status) {
		notified.Add(1)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go w.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	if fetcher.calls.Load() < 2 {
		t.Errorf("expected at least 2 polls, got %d", fetcher.calls.Load())
	}
	if notified.Load() < 1 {
		t.Errorf("expected at least 1 notification, got %d", notified.Load())
	}
}

func TestWatcher_StopsOnSuccess(t *testing.T) {
	fetcher := &mockFetcher{
		status: &Status{
			State:     "success",
			UpdatedAt: time.Now(),
		},
	}

	w := New(fetcher, "PR-2", 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Watcher stopped on terminal state
	case <-time.After(150 * time.Millisecond):
		t.Error("watcher did not stop on success state")
	}
}

func TestWatcher_Status(t *testing.T) {
	w := New(nil, "PR-3", time.Second)
	if w.Status() != nil {
		t.Error("expected nil status before polling")
	}
}
