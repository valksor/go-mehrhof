// Package watchdog monitors heap growth rate and triggers if memory climbs
// monotonically without the GC ever reclaiming it — the signature of a real
// leak vs. a legitimate spike.
//
//	[100MB, 102MB, 105MB, 110MB, 118MB, 130MB] → slope never dips → LEAK
//	[100MB, 300MB, 105MB, 102MB]               → spike then GC drops → NORMAL
package watchdog

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

// Config controls watchdog behaviour.
type Config struct {
	// Interval is how often to sample heap usage.
	// Shorter = more reactive but runtime.ReadMemStats stops the world briefly;
	// intervals below 10s are not recommended in production.
	Interval time.Duration

	// WindowSize is the number of consecutive samples to keep.
	// Detection fires only when the full window is populated.
	WindowSize int

	// ThresholdMB is the minimum total heap growth (first → last sample)
	// required before the monotonic check is evaluated.
	ThresholdMB float64

	// NoiseMB is the maximum heap dip allowed between consecutive samples
	// before the window is considered healthy (GC is working).
	// A value of 5 MB tolerates minor allocation variance without false positives.
	NoiseMB float64
}

// DefaultConfig returns production-safe defaults:
// sample every 30 s over a 10-sample (5 min) window, trigger at 200 MB growth.
func DefaultConfig() Config {
	return Config{
		Interval:    30 * time.Second,
		WindowSize:  10,
		ThresholdMB: 200,
		NoiseMB:     5,
	}
}

// Start launches the watchdog in a background goroutine.
// The returned channel is closed when the goroutine exits (context cancelled or
// leak detected). Callers that do not need to wait on teardown may discard it.
func Start(ctx context.Context, cfg Config) <-chan struct{} {
	done := make(chan struct{})
	samples := make([]float64, 0, cfg.WindowSize)
	ticker := time.NewTicker(cfg.Interval)

	go func() {
		defer close(done)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				heapMB := float64(ms.HeapInuse) / 1024 / 1024

				samples = append(samples, heapMB)
				if len(samples) > cfg.WindowSize {
					samples = samples[1:]
				}

				if len(samples) == cfg.WindowSize && isMonotonicallyGrowing(samples, cfg.ThresholdMB, cfg.NoiseMB) {
					profilePath := fmt.Sprintf("/tmp/kvelmo-leak-%d.pprof", time.Now().Unix())
					if f, err := os.Create(profilePath); err == nil {
						runtime.GC()
						if werr := pprof.WriteHeapProfile(f); werr != nil {
							slog.Warn("leak: failed to write heap profile", "error", werr)
						}
						_ = f.Close()
					}
					slog.Error(
						"LEAK DETECTED: heap grew monotonically — shutting down",
						"growth_mb", samples[len(samples)-1]-samples[0],
						"samples", cfg.WindowSize,
						"window_seconds", cfg.Interval.Seconds()*float64(cfg.WindowSize),
						"heap_profile", profilePath,
					)
					os.Exit(1)
				}
			}
		}
	}()

	return done
}

// isMonotonicallyGrowing returns true when:
//  1. Total growth (first → last) exceeds thresholdMB, AND
//  2. No consecutive pair dips by more than noiseMB (GC never meaningfully reclaimed)
func isMonotonicallyGrowing(samples []float64, thresholdMB, noiseMB float64) bool {
	if samples[len(samples)-1]-samples[0] < thresholdMB {
		return false
	}
	for i := 1; i < len(samples); i++ {
		if samples[i] < samples[i-1]-noiseMB {
			return false
		}
	}

	return true
}
