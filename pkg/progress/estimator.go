// Package progress provides phase progress estimation with ETA calculation.
//
// It tracks agent activity signals (tool_use, tool_result events) and computes
// progress estimates using either calibrated historical data or signal-count
// heuristics. Calibrated estimates use mean phase duration from past executions;
// uncalibrated estimates use a fixed increment per signal.
package progress

import (
	"sync"
	"time"
)

// Estimate represents the current progress estimation.
type Estimate struct {
	Percent    float64 `json:"percent"`     // 0-100
	ETASeconds int     `json:"eta_seconds"` // -1 = unknown
	Signals    int     `json:"signals"`     // tool_use events counted
	Calibrated bool    `json:"calibrated"`  // true if historical data available
}

// Estimator tracks agent activity signals and computes progress.
type Estimator struct {
	mu           sync.Mutex
	startTime    time.Time
	signals      int
	meanDuration time.Duration // from calibrator, 0 = uncalibrated
}

// NewEstimator creates an Estimator. If meanDuration > 0, ETA is calibrated.
func NewEstimator(meanDuration time.Duration) *Estimator {
	return &Estimator{
		startTime:    time.Now(),
		meanDuration: meanDuration,
	}
}

// Signal records an agent activity event (tool_use, tool_result, etc.).
func (e *Estimator) Signal() {
	e.mu.Lock()
	e.signals++
	e.mu.Unlock()
}

// Get returns the current progress estimate.
// Uses elapsed/mean ratio when calibrated, or signal-count heuristic when not.
func (e *Estimator) Get() Estimate {
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

		return Estimate{
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

	return Estimate{
		Percent:    percent,
		ETASeconds: -1,
		Signals:    signals,
		Calibrated: false,
	}
}

// Reset clears the estimator for a new phase.
func (e *Estimator) Reset(meanDuration time.Duration) {
	e.mu.Lock()
	e.startTime = time.Now()
	e.signals = 0
	e.meanDuration = meanDuration
	e.mu.Unlock()
}
