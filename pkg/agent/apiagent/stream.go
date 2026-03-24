package apiagent

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
)

// SSEEvent represents a Server-Sent Event.
type SSEEvent struct {
	Event string // Event type (empty if not specified)
	Data  string // Event data payload
}

// ParseSSE reads SSE events from a reader and emits them on a channel.
// The channel is closed when the stream ends or context is canceled.
func ParseSSE(ctx context.Context, r io.Reader) <-chan SSEEvent {
	ch := make(chan SSEEvent, 16)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(r)
		// Increase buffer size for large payloads
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var event SSEEvent
		var dataLines []string

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()

			if line == "" {
				// Empty line = end of event
				if len(dataLines) > 0 {
					event.Data = strings.Join(dataLines, "\n")
					ch <- event
					event = SSEEvent{}
					dataLines = dataLines[:0]
				}

				continue
			}

			if strings.HasPrefix(line, ":") {
				// Comment line, skip
				continue
			}

			if strings.HasPrefix(line, "event: ") || strings.HasPrefix(line, "event:") {
				event.Event = strings.TrimPrefix(line, "event:")
				event.Event = strings.TrimSpace(event.Event)
			} else if strings.HasPrefix(line, "data: ") || strings.HasPrefix(line, "data:") {
				data := strings.TrimPrefix(line, "data:")
				data = strings.TrimPrefix(data, " ")
				dataLines = append(dataLines, data)
			}
		}

		if err := scanner.Err(); err != nil {
			slog.Debug("SSE stream read error", "error", err)
		}

		// Flush any remaining event
		if len(dataLines) > 0 {
			event.Data = strings.Join(dataLines, "\n")
			ch <- event
		}
	}()

	return ch
}

// ParseNDJSON reads newline-delimited JSON objects from a reader.
// The channel is closed when the stream ends or context is canceled.
func ParseNDJSON(ctx context.Context, r io.Reader) <-chan json.RawMessage {
	ch := make(chan json.RawMessage, 16)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			ch <- json.RawMessage(line)
		}

		if err := scanner.Err(); err != nil {
			slog.Debug("NDJSON stream read error", "error", err)
		}
	}()

	return ch
}
