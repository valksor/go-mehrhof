package conductor

import (
	"time"

	"github.com/valksor/kvelmo/pkg/settings"
)

// progressEstimationEnabled returns true if progress estimation is enabled
// in the effective settings. Defaults to true.
func (c *Conductor) progressEstimationEnabled() bool {
	cfg := c.GetEffectiveSettings()
	if cfg == nil {
		return true
	}

	return settings.BoolValue(cfg.Workflow.ProgressEstimation, true)
}

// initProgressEstimator creates or resets the progress estimator for a new phase.
// Must be called with c.mu held.
func (c *Conductor) initProgressEstimator(phase string) {
	if !c.progressEstimationEnabled() {
		c.progressEstimator = nil

		return
	}

	var meanDuration time.Duration
	if c.progressCalibrator != nil {
		meanDuration = c.progressCalibrator.MeanDuration(phase)
	}

	// Also seed from historical PhaseMetrics of the current work unit
	// (covers previous runs of the same phase, e.g., after retry).
	if meanDuration == 0 && c.workUnit != nil && c.workUnit.PhaseMetrics != nil {
		if pm, ok := c.workUnit.PhaseMetrics[phase]; ok && pm.Duration > 0 {
			meanDuration = pm.Duration
		}
	}

	if c.progressEstimator == nil {
		c.progressEstimator = NewProgressEstimator(meanDuration)
	} else {
		c.progressEstimator.Reset(meanDuration)
	}
}

// recordProgressCalibration feeds a completed phase's duration into the calibrator.
// Must be called with c.mu held.
func (c *Conductor) recordProgressCalibration(phase string) {
	if c.progressCalibrator == nil || c.workUnit == nil {
		return
	}

	if pm, ok := c.workUnit.PhaseMetrics[phase]; ok && pm.Duration > 0 {
		c.progressCalibrator.Add(phase, pm.Duration)
	}
}

// SignalProgress records an agent activity event for progress tracking.
func (c *Conductor) SignalProgress() {
	c.mu.RLock()
	est := c.progressEstimator
	c.mu.RUnlock()

	if est != nil {
		est.Signal()
	}
}

// GetProgressEstimate returns the current progress estimate, or nil if
// no phase is active or estimation is disabled.
func (c *Conductor) GetProgressEstimate() *ProgressEstimate {
	c.mu.RLock()
	est := c.progressEstimator
	c.mu.RUnlock()

	if est == nil {
		return nil
	}

	e := est.Get()

	return &e
}
