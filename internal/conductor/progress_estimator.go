package conductor

import (
	"sync"
	"time"
)

// ProgressEstimate represents the current progress estimation.
type ProgressEstimate struct {
	Percent    float64 `json:"percent"`     // 0-100
	ETASeconds int     `json:"eta_seconds"` // -1 = unknown
	Signals    int     `json:"signals"`     // tool_use events counted
	Calibrated bool    `json:"calibrated"`  // true if historical data available
}

// ProgressEstimator tracks agent activity signals and computes progress.
type ProgressEstimator struct {
	mu           sync.Mutex
	startTime    time.Time
	signals      int
	meanDuration time.Duration // from calibrator, 0 = uncalibrated
}

// NewProgressEstimator creates a ProgressEstimator. If meanDuration > 0, ETA is calibrated.
func NewProgressEstimator(meanDuration time.Duration) *ProgressEstimator {
	return &ProgressEstimator{
		startTime:    time.Now(),
		meanDuration: meanDuration,
	}
}

// Signal records an agent activity event (tool_use, tool_result, etc.).
func (e *ProgressEstimator) Signal() {
	e.mu.Lock()
	e.signals++
	e.mu.Unlock()
}

// Get returns the current progress estimate.
// Uses elapsed/mean ratio when calibrated, or signal-count heuristic when not.
func (e *ProgressEstimator) Get() ProgressEstimate {
	e.mu.Lock()
	defer e.mu.Unlock()

	calibrated := e.meanDuration > 0
	signals := e.signals

	if calibrated {
		elapsed := time.Since(e.startTime)
		ratio := float64(elapsed) / float64(e.meanDuration)
		percent := ratio * 100
		if percent > 95 {
			percent = 95
		}
		if percent < 0 {
			percent = 0
		}

		remaining := e.meanDuration - elapsed
		etaSeconds := max(int(remaining.Seconds()), 0)

		return ProgressEstimate{
			Percent:    percent,
			ETASeconds: etaSeconds,
			Signals:    signals,
			Calibrated: true,
		}
	}

	// Uncalibrated: each signal = ~2% progress, capped at 90%.
	percent := float64(signals) * 2
	if percent > 90 {
		percent = 90
	}

	return ProgressEstimate{
		Percent:    percent,
		ETASeconds: -1,
		Signals:    signals,
		Calibrated: false,
	}
}

// Reset clears the estimator for a new phase.
func (e *ProgressEstimator) Reset(meanDuration time.Duration) {
	e.mu.Lock()
	e.startTime = time.Now()
	e.signals = 0
	e.meanDuration = meanDuration
	e.mu.Unlock()
}

// ProgressCalibrator computes per-phase average durations from historical PhaseMetrics.
type ProgressCalibrator struct {
	mu        sync.Mutex
	durations map[string][]time.Duration // phase name -> historical durations
}

// NewProgressCalibrator creates an empty ProgressCalibrator.
func NewProgressCalibrator() *ProgressCalibrator {
	return &ProgressCalibrator{
		durations: make(map[string][]time.Duration),
	}
}

// Add records a completed phase duration.
func (c *ProgressCalibrator) Add(phase string, duration time.Duration) {
	c.mu.Lock()
	c.durations[phase] = append(c.durations[phase], duration)
	c.mu.Unlock()
}

// MeanDuration returns the average duration for a phase.
// Returns 0 if no data available.
func (c *ProgressCalibrator) MeanDuration(phase string) time.Duration {
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
