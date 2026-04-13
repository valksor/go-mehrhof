package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"
)

const (
	channelCapacity = 100
	httpTimeout     = 10 * time.Second
	maxRetries      = 3
)

// retryBackoffs defines the delay before each retry attempt.
var retryBackoffs = [maxRetries]time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

// Notifier dispatches webhook notifications asynchronously via a buffered channel.
type Notifier struct {
	ch        chan Payload
	endpoints []WebhookEndpoint
	client    *http.Client
	onFailure bool
}

// New creates a Notifier. When onFailure is true, error events are always
// dispatched regardless of per-endpoint event filters.
func New(endpoints []WebhookEndpoint, onFailure bool) *Notifier {
	for i := range endpoints {
		if endpoints[i].Format == "" {
			endpoints[i].Format = FormatGeneric
		}
	}

	return &Notifier{
		ch:        make(chan Payload, channelCapacity),
		endpoints: endpoints,
		client:    &http.Client{Timeout: httpTimeout},
		onFailure: onFailure,
	}
}

// Send enqueues a payload for async dispatch. It never blocks; if the channel
// is full the payload is dropped with a warning log.
func (n *Notifier) Send(payload Payload) {
	select {
	case n.ch <- payload:
	default:
		slog.Warn("notify: channel full, dropping payload",
			"event", payload.Event,
			"task_id", payload.TaskID,
		)
	}
}

// Start reads payloads from the channel and dispatches them to matching
// endpoints. It blocks until ctx is cancelled or the channel is closed.
func (n *Notifier) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case p, ok := <-n.ch:
			if !ok {
				return
			}
			n.dispatch(ctx, p)
		}
	}
}

// Close closes the channel and drains any remaining payloads.
func (n *Notifier) Close() {
	close(n.ch)

	for p := range n.ch {
		n.dispatch(context.Background(), p)
	}
}

// dispatch sends a payload to every endpoint whose filter matches.
func (n *Notifier) dispatch(ctx context.Context, p Payload) {
	for _, ep := range n.endpoints {
		if !n.matchesFilter(ep, p) {
			continue
		}
		if err := n.sendToEndpoint(ctx, ep, p); err != nil {
			slog.Error("notify: failed to send webhook",
				"url", ep.URL,
				"event", p.Event,
				"error", err,
			)
		}
	}
}

// matchesFilter returns true when the payload should be sent to the endpoint.
func (n *Notifier) matchesFilter(ep WebhookEndpoint, p Payload) bool {
	// onFailure override: always send error events.
	if n.onFailure && p.Event == "error" {
		return true
	}
	// Empty filter means all events.
	if len(ep.Events) == 0 {
		return true
	}

	return slices.Contains(ep.Events, p.Event)
}

// sendToEndpoint formats the payload and POSTs it to the endpoint with retries.
func (n *Notifier) sendToEndpoint(ctx context.Context, endpoint WebhookEndpoint, payload Payload) error {
	var body []byte
	var err error

	switch endpoint.Format {
	case FormatSlack:
		slackPayload := FormatSlackPayload(payload)
		body, err = json.Marshal(slackPayload)
		if err != nil {
			return fmt.Errorf("marshal slack payload: %w", err)
		}
	case FormatGeneric:
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
	}

	var lastErr error
	for attempt := range maxRetries {
		if attempt > 0 {
			time.Sleep(retryBackoffs[attempt])
		}

		lastErr = n.post(ctx, endpoint.URL, body)
		if lastErr == nil {
			return nil
		}

		slog.Warn("notify: retry webhook",
			"url", endpoint.URL,
			"attempt", attempt+1,
			"error", lastErr,
		)
	}

	return fmt.Errorf("all %d attempts failed: %w", maxRetries, lastErr)
}

// post sends a single HTTP POST with JSON content type and drains the response body.
func (n *Notifier) post(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return nil
}
