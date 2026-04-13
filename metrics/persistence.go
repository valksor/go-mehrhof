package metrics

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/valksor/kvelmo/paths"
)

// Persister periodically saves metrics snapshots to disk and restores them on startup.
type Persister struct {
	metrics  *Metrics
	path     string
	interval time.Duration
}

// NewPersister creates a metrics persister.
// If path is empty, defaults to ~/.valksor/kvelmo/metrics.json.
// If interval is 0, defaults to 60 seconds.
func NewPersister(m *Metrics, path string, interval time.Duration) *Persister {
	if path == "" {
		path = filepath.Join(paths.BaseDir(), "metrics.json")
	}
	if interval <= 0 {
		interval = 60 * time.Second
	}

	return &Persister{
		metrics:  m,
		path:     path,
		interval: interval,
	}
}

// Load restores metrics from disk. Errors are logged but not returned
// (best-effort restore).
func (p *Persister) Load() {
	data, err := os.ReadFile(p.path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Debug("metrics restore: could not read file", "path", p.path, "error", err)
		}

		return
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		slog.Warn("metrics restore: corrupt file", "path", p.path, "error", err)

		return
	}

	p.metrics.RestoreFrom(snap)
	slog.Debug("metrics restored from disk", "path", p.path)
}

// Start runs the periodic save loop. Blocks until ctx is cancelled.
func (p *Persister) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Final save on shutdown
			p.save()

			return
		case <-ticker.C:
			p.save()
		}
	}
}

func (p *Persister) save() {
	snap := p.metrics.Snapshot()
	data, err := json.Marshal(snap)
	if err != nil {
		slog.Warn("metrics persist: marshal error", "error", err)

		return
	}

	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		slog.Warn("metrics persist: mkdir error", "path", dir, "error", err)

		return
	}

	if err := os.WriteFile(p.path, data, 0o640); err != nil {
		slog.Warn("metrics persist: write error", "path", p.path, "error", err)
	}
}
