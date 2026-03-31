package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"
)

// --- Streaming Handler ---

func (w *WorktreeSocket) handleStreamSubscribe(ctx context.Context, req *Request, conn net.Conn) (*Response, error) {
	var params struct {
		LastSeq uint64 `json:"last_seq,omitempty"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	subID := fmt.Sprintf("sub-%d", time.Now().UnixNano())
	ch := make(chan []byte, 100)

	// Subscribe before snapshotting the buffer so no events are missed between
	// replay and live delivery.
	w.streamsMu.Lock()
	w.streams[subID] = ch
	w.streamsMu.Unlock()

	// Replay missed events if the client provides a last known sequence number.
	if params.LastSeq > 0 {
		w.replayMu.Lock()
		// Copy ring buffer in chronological order (oldest → newest).
		snapshot := make([][]byte, replayBufSize)
		for i := range replayBufSize {
			snapshot[i] = w.replayBuf[(w.replayHead+i)%replayBufSize]
		}
		w.replayMu.Unlock()

		var seqCheck struct {
			Seq uint64 `json:"seq"`
		}
		for _, entry := range snapshot {
			if entry == nil {
				continue
			}
			if err := json.Unmarshal(entry, &seqCheck); err != nil {
				continue
			}
			if seqCheck.Seq > params.LastSeq {
				if _, err := conn.Write(entry); err != nil {
					w.streamsMu.Lock()
					delete(w.streams, subID)
					w.streamsMu.Unlock()
					close(ch)

					return nil, fmt.Errorf("replay: %w", err)
				}
			}
		}
	}

	// Drain the subscription channel and write events to the connection.
	// A 30s heartbeat detects closed connections when events are infrequent.
	go func() {
		defer func() {
			w.streamsMu.Lock()
			delete(w.streams, subID)
			w.streamsMu.Unlock()
		}()
		// Heartbeats are keepalive signals, intentionally without seq numbers.
		// They are not part of the ordered event stream and not buffered for replay.
		heartbeat := []byte("{\"type\":\"heartbeat\"}\n")
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return
				}
				if _, err := conn.Write(event); err != nil {
					return
				}
			case <-ticker.C:
				if _, err := conn.Write(heartbeat); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return NewResultResponse(req.ID, map[string]any{
		"subscription_id": subID,
		"status":          "subscribed",
	})
}

// emitEvent broadcasts an event to all stream subscribers.
func (w *WorktreeSocket) emitEvent(eventType string, data any) {
	event := map[string]any{
		"type":      eventType,
		"data":      data,
		"timestamp": time.Now(),
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal event", "type", eventType, "error", err)

		return
	}

	enriched := w.injectSeqAndBuffer(eventData)

	w.streamsMu.RLock()
	for _, ch := range w.streams {
		select {
		case ch <- enriched:
		default:
			slog.Warn("worktree event channel full, dropping event", "type", eventType)
		}
	}
	w.streamsMu.RUnlock()
}

// --- Progress Estimation ---

func (w *WorktreeSocket) handleProgressGet(_ context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	estimate := w.conductor.GetProgressEstimate()
	if estimate == nil {
		return NewResultResponse(req.ID, map[string]any{
			"active": false,
		})
	}

	return NewResultResponse(req.ID, map[string]any{
		"active":      true,
		"percent":     estimate.Percent,
		"eta_seconds": estimate.ETASeconds,
		"signals":     estimate.Signals,
		"calibrated":  estimate.Calibrated,
	})
}
