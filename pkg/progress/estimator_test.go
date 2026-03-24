package progress

import (
	"testing"
	"time"
)

func TestEstimator_CalibratedProgress(t *testing.T) {
	tests := []struct {
		name         string
		meanDuration time.Duration
		elapsed      time.Duration
		wantMinPct   float64
		wantMaxPct   float64
	}{
		{
			name:         "25% elapsed",
			meanDuration: 4 * time.Second,
			elapsed:      1 * time.Second,
			wantMinPct:   20,
			wantMaxPct:   35,
		},
		{
			name:         "50% elapsed",
			meanDuration: 4 * time.Second,
			elapsed:      2 * time.Second,
			wantMinPct:   45,
			wantMaxPct:   60,
		},
		{
			name:         "75% elapsed",
			meanDuration: 4 * time.Second,
			elapsed:      3 * time.Second,
			wantMinPct:   70,
			wantMaxPct:   85,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEstimator(tt.meanDuration)
			e.mu.Lock()
			e.startTime = time.Now().Add(-tt.elapsed)
			e.mu.Unlock()

			est := e.Get()
			if !est.Calibrated {
				t.Error("expected calibrated = true")
			}
			if est.Percent < tt.wantMinPct || est.Percent > tt.wantMaxPct {
				t.Errorf("percent = %.1f, want [%.1f, %.1f]", est.Percent, tt.wantMinPct, tt.wantMaxPct)
			}
		})
	}
}

func TestEstimator_UncalibratedProgress(t *testing.T) {
	tests := []struct {
		name       string
		signals    int
		wantMinPct float64
		wantMaxPct float64
	}{
		{
			name:       "no signals",
			signals:    0,
			wantMinPct: 0,
			wantMaxPct: 0,
		},
		{
			name:       "5 signals = ~10%",
			signals:    5,
			wantMinPct: 10,
			wantMaxPct: 10,
		},
		{
			name:       "25 signals = ~50%",
			signals:    25,
			wantMinPct: 50,
			wantMaxPct: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEstimator(0) // uncalibrated
			for range tt.signals {
				e.Signal()
			}

			est := e.Get()
			if est.Calibrated {
				t.Error("expected calibrated = false")
			}
			if est.ETASeconds != -1 {
				t.Errorf("eta = %d, want -1 for uncalibrated", est.ETASeconds)
			}
			if est.Percent < tt.wantMinPct || est.Percent > tt.wantMaxPct {
				t.Errorf("percent = %.1f, want [%.1f, %.1f]", est.Percent, tt.wantMinPct, tt.wantMaxPct)
			}
		})
	}
}

func TestEstimator_CappedAt95(t *testing.T) {
	tests := []struct {
		name         string
		meanDuration time.Duration
		elapsed      time.Duration
	}{
		{
			name:         "calibrated at 200% elapsed",
			meanDuration: 1 * time.Second,
			elapsed:      2 * time.Second,
		},
		{
			name:         "calibrated at 500% elapsed",
			meanDuration: 1 * time.Second,
			elapsed:      5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEstimator(tt.meanDuration)
			e.mu.Lock()
			e.startTime = time.Now().Add(-tt.elapsed)
			e.mu.Unlock()

			est := e.Get()
			if est.Percent > 95 {
				t.Errorf("percent = %.1f, want <= 95", est.Percent)
			}
		})
	}

	// Uncalibrated cap at 90
	t.Run("uncalibrated capped at 90", func(t *testing.T) {
		e := NewEstimator(0)
		for range 100 {
			e.Signal()
		}
		est := e.Get()
		if est.Percent > 90 {
			t.Errorf("percent = %.1f, want <= 90 for uncalibrated", est.Percent)
		}
	})
}

func TestEstimator_ETADecreases(t *testing.T) {
	meanDuration := 10 * time.Second
	e := NewEstimator(meanDuration)

	// At start, ETA should be close to full duration.
	e.mu.Lock()
	e.startTime = time.Now().Add(-1 * time.Second)
	e.mu.Unlock()
	est1 := e.Get()

	// After more time, ETA should be lower.
	e.mu.Lock()
	e.startTime = time.Now().Add(-5 * time.Second)
	e.mu.Unlock()
	est2 := e.Get()

	if est2.ETASeconds >= est1.ETASeconds {
		t.Errorf("ETA should decrease: first=%d, second=%d", est1.ETASeconds, est2.ETASeconds)
	}
}

func TestEstimator_Reset(t *testing.T) {
	e := NewEstimator(5 * time.Second)
	for range 10 {
		e.Signal()
	}

	// Set start time in the past to accumulate progress.
	e.mu.Lock()
	e.startTime = time.Now().Add(-3 * time.Second)
	e.mu.Unlock()

	est := e.Get()
	if est.Signals != 10 {
		t.Errorf("signals before reset = %d, want 10", est.Signals)
	}
	if est.Percent == 0 {
		t.Error("percent should be > 0 before reset")
	}

	// Reset with new mean duration.
	e.Reset(10 * time.Second)

	est = e.Get()
	if est.Signals != 0 {
		t.Errorf("signals after reset = %d, want 0", est.Signals)
	}
	// After reset, percent should be close to 0 (just started).
	if est.Percent > 5 {
		t.Errorf("percent after reset = %.1f, want < 5", est.Percent)
	}
}

func TestCalibrator_MeanDuration(t *testing.T) {
	tests := []struct {
		name      string
		durations []time.Duration
		want      time.Duration
	}{
		{
			name:      "single duration",
			durations: []time.Duration{10 * time.Second},
			want:      10 * time.Second,
		},
		{
			name:      "two durations",
			durations: []time.Duration{10 * time.Second, 20 * time.Second},
			want:      15 * time.Second,
		},
		{
			name:      "three durations",
			durations: []time.Duration{6 * time.Second, 9 * time.Second, 12 * time.Second},
			want:      9 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCalibrator()
			for _, d := range tt.durations {
				c.Add("plan", d)
			}
			got := c.MeanDuration("plan")
			if got != tt.want {
				t.Errorf("MeanDuration = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalibrator_EmptyReturnsZero(t *testing.T) {
	c := NewCalibrator()
	got := c.MeanDuration("implement")
	if got != 0 {
		t.Errorf("MeanDuration = %v, want 0", got)
	}
}
