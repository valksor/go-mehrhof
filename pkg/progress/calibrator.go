package progress

import (
	"sync"
	"time"
)

// Calibrator computes per-phase average durations from historical PhaseMetrics.
type Calibrator struct {
	mu        sync.Mutex
	durations map[string][]time.Duration // phase name -> historical durations
}

// NewCalibrator creates an empty Calibrator.
func NewCalibrator() *Calibrator {
	return &Calibrator{
		durations: make(map[string][]time.Duration),
	}
}

// Add records a completed phase duration.
func (c *Calibrator) Add(phase string, duration time.Duration) {
	c.mu.Lock()
	c.durations[phase] = append(c.durations[phase], duration)
	c.mu.Unlock()
}

// MeanDuration returns the average duration for a phase.
// Returns 0 if no data available.
func (c *Calibrator) MeanDuration(phase string) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	durations := c.durations[phase]
	if len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}

	return total / time.Duration(len(durations))
}
