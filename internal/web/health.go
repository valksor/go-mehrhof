package web

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/valksor/kvelmo/metrics"
)

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write(mustJSON(map[string]string{"status": "ok"}))
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.globalSocketPath == "" {
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write(mustJSON(map[string]string{"status": "ok"}))

		return
	}

	// Check global socket connectivity
	dialer := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := dialer.DialContext(r.Context(), "unix", s.globalSocketPath)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)

		_, _ = w.Write(mustJSON(map[string]string{"status": "not_ready", "reason": "global socket unavailable"}))

		return
	}
	_ = conn.Close()

	w.WriteHeader(http.StatusOK)

	_, _ = w.Write(mustJSON(map[string]string{"status": "ready"}))
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	snap := metrics.Global().Snapshot()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(metrics.RenderPrometheus(snap)))
}

func mustJSON(v map[string]string) []byte {
	data, _ := json.Marshal(v) //nolint:errchkjson // map[string]string cannot fail to marshal

	return data
}
