package activitylog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Forwarder sends activity log entries to an external webhook endpoint.
// Forwarding is non-blocking and best-effort; failures are logged but never block writes.
type Forwarder struct {
	url    string
	auth   string
	client *http.Client
}

// NewForwarder creates a forwarder that POSTs JSON entries to the given URL.
// If auth is non-empty, it is sent as the Authorization header value.
func NewForwarder(url, auth string) *Forwarder {
	return &Forwarder{
		url:  url,
		auth: auth,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Forward sends an entry to the configured endpoint. It is safe for concurrent use.
func (f *Forwarder) Forward(ctx context.Context, entry Entry) {
	data, err := json.Marshal(entry)
	if err != nil {
		slog.Debug("activity forward: marshal failed", "error", err)

		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.url, bytes.NewReader(data))
	if err != nil {
		slog.Debug("activity forward: create request failed", "error", err)

		return
	}

	req.Header.Set("Content-Type", "application/json")
	if f.auth != "" {
		req.Header.Set("Authorization", f.auth)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		slog.Debug("activity forward: request failed", "error", err)

		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		slog.Debug("activity forward: endpoint returned error", "status", resp.StatusCode, "url", f.url)
	}
}

// ForwardAsync sends an entry in a background goroutine. Always non-blocking.
// The parent ctx is used for cancellation; the actual HTTP request uses its own timeout.
func (f *Forwarder) ForwardAsync(ctx context.Context, entry Entry) {
	go func() {
		fwdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		f.Forward(fwdCtx, entry)
	}()
}

// ValidateURL checks that the forwarder URL is reachable with a simple HEAD request.
func (f *Forwarder) ValidateURL(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, f.url, nil)
	if err != nil {
		return fmt.Errorf("activity forward: invalid URL: %w", err)
	}

	if f.auth != "" {
		req.Header.Set("Authorization", f.auth)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("activity forward: endpoint unreachable: %w", err)
	}
	_ = resp.Body.Close()

	return nil
}
