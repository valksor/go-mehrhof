package tui

import (
	"strings"
	"testing"
)

func TestClassificationPrefix(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"flaky", "test failed {flaky}", "[FLAKY] test failed "},
		{"genuine", "{genuine} real bug", "[GENUINE]  real bug"},
		{"intermittent", "{intermittent} sometimes", "[INTERMITTENT]  sometimes"},
		{"plain", "just a line", "just a line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classificationPrefix(tt.line); got != tt.want {
				t.Errorf("classificationPrefix(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestAnnotateOutputLines(t *testing.T) {
	in := []string{"ok", "fail {flaky}", "boom {genuine}"}
	out := annotateOutputLines(in)
	if len(out) != 3 {
		t.Fatalf("len = %d", len(out))
	}
	if out[0] != "ok" {
		t.Errorf("out[0] = %q", out[0])
	}
	if !strings.HasPrefix(out[1], "[FLAKY]") {
		t.Errorf("out[1] = %q", out[1])
	}
	if !strings.HasPrefix(out[2], "[GENUINE]") {
		t.Errorf("out[2] = %q", out[2])
	}
}

func TestFormatETASeconds(t *testing.T) {
	tests := []struct {
		secs int
		want string
	}{
		{0, "0s"},
		{-5, "0s"},
		{45, "45s"},
		{90, "1m30s"},
		{120, "2m"},
		{3600, "1h"},
		{3660, "1h1m"},
		{7200, "2h"},
		{3900, "1h5m"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatETASeconds(tt.secs); got != tt.want {
				t.Errorf("formatETASeconds(%d) = %q, want %q", tt.secs, got, tt.want)
			}
		})
	}
}

func TestRenderProgressBarDetails(t *testing.T) {
	t.Run("calibrated includes ETA", func(t *testing.T) {
		out := renderProgressBar(50, 90, true)
		if !strings.Contains(out, "50%") || !strings.Contains(out, "~1m30s") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("uncalibrated omits ETA", func(t *testing.T) {
		out := renderProgressBar(30, 90, false)
		if strings.Contains(out, "~") {
			t.Errorf("uncalibrated bar should not include ETA: %q", out)
		}
	})
	t.Run("clamps over 100", func(t *testing.T) {
		out := renderProgressBar(150, 0, true)
		if !strings.Contains(out, "150%") {
			t.Errorf("out = %q", out)
		}
		// All ten cells should be filled (no empty cells).
		if strings.Contains(out, "░") {
			t.Errorf("over-100 bar should have no empty cells: %q", out)
		}
	})
	t.Run("clamps negative", func(t *testing.T) {
		out := renderProgressBar(-10, 0, true)
		if strings.Contains(out, "█") {
			t.Errorf("negative bar should have no filled cells: %q", out)
		}
	})
}

func TestRenderStatusBarFlags(t *testing.T) {
	mk := func(wt WorktreeState) *Model {
		m := NewModel("/proj", LayoutStacked)
		m.width = 200
		m.worktrees = []WorktreeState{wt}

		return &m
	}

	t.Run("failure class shown when failed", func(t *testing.T) {
		bar := mk(WorktreeState{Dir: "/p/proj", State: stateFailed, LastFailureClass: "hard_stop"}).renderStatusBar()
		if !strings.Contains(bar, "[hard_stop]") {
			t.Errorf("bar = %q", bar)
		}
	})

	t.Run("autofix progress shown", func(t *testing.T) {
		bar := mk(WorktreeState{Dir: "/p/proj", State: stateImplementing, AutoFixAttempt: 2, AutoFixMax: 3}).renderStatusBar()
		if !strings.Contains(bar, "[AUTO-FIX 2/3]") {
			t.Errorf("bar = %q", bar)
		}
	})

	t.Run("active forks shown", func(t *testing.T) {
		bar := mk(WorktreeState{Dir: "/p/proj", State: stateLoaded, ActiveForks: 2}).renderStatusBar()
		if !strings.Contains(bar, "[2 fork(s)]") {
			t.Errorf("bar = %q", bar)
		}
	})

	t.Run("risk level shown", func(t *testing.T) {
		bar := mk(WorktreeState{Dir: "/p/proj", State: stateLoaded, RiskLevel: "medium"}).renderStatusBar()
		if !strings.Contains(bar, "[risk:medium]") {
			t.Errorf("bar = %q", bar)
		}
	})

	t.Run("dry-run flag shown", func(t *testing.T) {
		m := mk(WorktreeState{Dir: "/p/proj", State: stateLoaded})
		m.dryRun = true
		if bar := m.renderStatusBar(); !strings.Contains(bar, "[DRY RUN]") {
			t.Errorf("bar = %q", bar)
		}
	})

	t.Run("progress bar shown when active", func(t *testing.T) {
		bar := mk(WorktreeState{
			Dir: "/p/proj", State: statePlanning,
			ProgressActive: true, ProgressPercent: 40, ProgressETASeconds: 60, ProgressCalibrated: true,
		}).renderStatusBar()
		if !strings.Contains(bar, "40%") {
			t.Errorf("bar should include progress: %q", bar)
		}
	})

	t.Run("no active task", func(t *testing.T) {
		m := NewModel("/proj", LayoutStacked)
		m.width = 80
		if bar := m.renderStatusBar(); !strings.Contains(bar, "no active tasks") {
			t.Errorf("bar = %q", bar)
		}
	})
}
