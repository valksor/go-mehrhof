package conductor

import (
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

// specWatcher tracks specification file modification times to detect
// mid-execution changes. Uses polling (no fsnotify dependency) at
// graph node boundaries.
type specWatcher struct {
	files     map[string]time.Time // path → last known mod time
	changed   atomic.Bool
	lastCheck time.Time
}

// newSpecWatcher creates a watcher for the given spec file paths.
// Records initial modification times as baseline.
func newSpecWatcher(paths []string) *specWatcher {
	sw := &specWatcher{
		files:     make(map[string]time.Time, len(paths)),
		lastCheck: time.Now(),
	}
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil {
			sw.files[path] = info.ModTime()
		}
	}
	return sw
}

// Check polls all tracked files for modifications since the last check.
// Returns true if any spec was modified.
func (sw *specWatcher) Check() bool {
	if sw == nil || len(sw.files) == 0 {
		return false
	}

	for path, baseline := range sw.files {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().After(baseline) {
			sw.files[path] = info.ModTime()
			sw.changed.Store(true)
			slog.Info("specification modified during execution", "path", path)
		}
	}

	sw.lastCheck = time.Now()
	return sw.changed.Load()
}

// Changed returns true if any spec modification has been detected.
func (sw *specWatcher) Changed() bool {
	if sw == nil {
		return false
	}
	return sw.changed.Load()
}

// Reset clears the changed flag after the agent has been notified.
func (sw *specWatcher) Reset() {
	if sw != nil {
		sw.changed.Store(false)
	}
}
