package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/metrics"
)

// handleAPIState returns the full machine state as JSON for external aggregation.
// Enables team leads to build dashboards by polling multiple kvelmo instances.
func (s *Server) handleAPIState(w http.ResponseWriter, r *http.Request) {
	state := map[string]any{
		"metrics":   metrics.Global().Snapshot(),
		"timestamp": time.Now().UTC(),
	}

	// Query global socket for projects and tasks
	globalPath := socket.GlobalSocketPath()
	if socket.SocketExists(globalPath) {
		client, err := socket.NewClient(globalPath, socket.WithTimeout(3*time.Second))
		if err == nil {
			defer func() { _ = client.Close() }()

			if resp, err := client.Call(r.Context(), "projects.list", nil); err == nil && resp.Error == nil {
				state["projects"] = resp.Result
			}
			if resp, err := client.Call(r.Context(), "tasks.list", nil); err == nil && resp.Error == nil {
				state["tasks"] = resp.Result
			}
			if resp, err := client.Call(r.Context(), "workers.stats", nil); err == nil && resp.Error == nil {
				state["workers"] = resp.Result
			}
			if resp, err := client.Call(r.Context(), "system.health", nil); err == nil && resp.Error == nil {
				state["health"] = resp.Result
			}
		} else {
			slog.Debug("api/state: connect to global socket failed", "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(state); err != nil {
		slog.Debug("api/state: encode error", "error", err)
	}
}

// handleAPITasks returns active and archived tasks as JSON.
func (s *Server) handleAPITasks(w http.ResponseWriter, r *http.Request) {
	result := map[string]any{
		"timestamp": time.Now().UTC(),
	}

	globalPath := socket.GlobalSocketPath()
	if socket.SocketExists(globalPath) {
		client, err := socket.NewClient(globalPath, socket.WithTimeout(3*time.Second))
		if err == nil {
			defer func() { _ = client.Close() }()

			if resp, err := client.Call(r.Context(), "tasks.list", nil); err == nil && resp.Error == nil {
				result["active"] = resp.Result
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		slog.Debug("api/tasks: encode error", "error", err)
	}
}
