// Internal test package to access unexported isMonotonicallyGrowing.
package watchdog

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Interval != 30*time.Second {
		t.Errorf("Interval = %v, want 30s", cfg.Interval)
	}
	if cfg.WindowSize != 10 {
		t.Errorf("WindowSize = %d, want 10", cfg.WindowSize)
	}
	if cfg.ThresholdMB != 200 {
		t.Errorf("ThresholdMB = %v, want 200", cfg.ThresholdMB)
	}
	if cfg.NoiseMB != 5 {
		t.Errorf("NoiseMB = %v, want 5", cfg.NoiseMB)
	}
}

func TestIsMonotonicallyGrowing(t *testing.T) {
	tests := []struct {
		name        string
		samples     []float64
		thresholdMB float64
		noiseMB     float64
		want        bool
	}{
		{
			name:        "steady growth exceeds threshold",
			samples:     []float64{100, 102, 105, 110, 118, 130},
			thresholdMB: 20,
			noiseMB:     5,
			want:        true,
		},
		{
			name:        "growth below threshold",
			samples:     []float64{100, 101, 102, 103, 104, 105},
			thresholdMB: 100,
			noiseMB:     5,
			want:        false,
		},
		{
			name:        "large GC dip detected",
			samples:     []float64{100, 300, 50, 80, 110, 130},
			thresholdMB: 20,
			noiseMB:     5,
			want:        false,
		},
		{
			name:        "small dips within noise tolerance",
			samples:     []float64{100, 103, 102, 106, 109, 112},
			thresholdMB: 10,
			noiseMB:     5,
			want:        true,
		},
		{
			name:        "flat line no growth",
			samples:     []float64{100, 100, 100, 100, 100, 100},
			thresholdMB: 1,
			noiseMB:     5,
			want:        false,
		},
		{
			name:        "exactly at threshold",
			samples:     []float64{100, 110, 120, 130, 140, 150},
			thresholdMB: 50,
			noiseMB:     5,
			want:        true,
		},
		{
			name:        "just below threshold",
			samples:     []float64{100, 110, 120, 130, 140, 149},
			thresholdMB: 50,
			noiseMB:     5,
			want:        false,
		},
		{
			name:        "single dip exactly at noise boundary",
			samples:     []float64{100, 110, 105, 115, 120, 130},
			thresholdMB: 20,
			noiseMB:     5,
			want:        true, // dip is exactly 5 MB, which is not > noiseMB
		},
		{
			name:        "single dip exceeds noise",
			samples:     []float64{100, 110, 104, 115, 120, 130},
			thresholdMB: 20,
			noiseMB:     5,
			want:        false, // dip is 6 MB, exceeds noiseMB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMonotonicallyGrowing(tt.samples, tt.thresholdMB, tt.noiseMB)
			if got != tt.want {
				t.Errorf("isMonotonicallyGrowing(%v, threshold=%v, noise=%v) = %v, want %v",
					tt.samples, tt.thresholdMB, tt.noiseMB, got, tt.want)
			}
		})
	}
}

func TestStart_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := Config{
		Interval:    10 * time.Millisecond,
		WindowSize:  1000, // Large window — never fills during test
		ThresholdMB: 1000,
		NoiseMB:     5,
	}
	// Start should return immediately (goroutine spawned in background).
	done := Start(ctx, cfg)
	// Let the ticker fire a couple times.
	time.Sleep(50 * time.Millisecond)
	// Cancel the context; goroutine should stop without calling os.Exit.
	cancel()
	// Block until the goroutine has actually exited.
	<-done
}
