package settings

import (
	"strings"
	"testing"
)

// validSettings returns a Settings with all fields set to valid values.
func validSettings() Settings {
	return Settings{
		Workers: WorkerSettings{Max: 3},
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name       string
		modify     func(*Settings)
		wantCount  int
		wantSubstr []string
	}{
		{
			name:      "valid defaults produce no issues",
			modify:    func(_ *Settings) {},
			wantCount: 0,
		},
		{
			name:       "workers max zero",
			modify:     func(s *Settings) { s.Workers.Max = 0 },
			wantCount:  1,
			wantSubstr: []string{"workers.max"},
		},
		{
			name:       "workers max too high",
			modify:     func(s *Settings) { s.Workers.Max = 11 },
			wantCount:  1,
			wantSubstr: []string{"workers.max"},
		},
		{
			name:      "workers max valid boundary",
			modify:    func(s *Settings) { s.Workers.Max = 5 },
			wantCount: 0,
		},
		{
			name: "watchdog enabled with low threshold",
			modify: func(s *Settings) {
				s.Watchdog = WatchdogSettings{Enabled: true, ThresholdMB: 10, IntervalSec: 30, WindowSize: 10}
			},
			wantCount:  1,
			wantSubstr: []string{"watchdog.threshold_mb"},
		},
		{
			name: "watchdog enabled with low interval",
			modify: func(s *Settings) {
				s.Watchdog = WatchdogSettings{Enabled: true, ThresholdMB: 200, IntervalSec: 2, WindowSize: 10}
			},
			wantCount:  1,
			wantSubstr: []string{"watchdog.interval_sec"},
		},
		{
			name: "watchdog enabled with low window size",
			modify: func(s *Settings) {
				s.Watchdog = WatchdogSettings{Enabled: true, ThresholdMB: 200, IntervalSec: 30, WindowSize: 1}
			},
			wantCount:  1,
			wantSubstr: []string{"watchdog.window_size"},
		},
		{
			name: "watchdog disabled skips threshold validation",
			modify: func(s *Settings) {
				s.Watchdog = WatchdogSettings{Enabled: false, ThresholdMB: 1, IntervalSec: 1, WindowSize: 1}
			},
			wantCount: 0,
		},
		{
			name: "activity log enabled with zero max files",
			modify: func(s *Settings) {
				s.Storage.ActivityLog = ActivityLogSettings{Enabled: true, MaxFiles: 0}
			},
			wantCount:  1,
			wantSubstr: []string{"activity_log.max_files"},
		},
		{
			name: "activity log disabled with zero max files",
			modify: func(s *Settings) {
				s.Storage.ActivityLog = ActivityLogSettings{Enabled: false, MaxFiles: 0}
			},
			wantCount: 0,
		},
		{
			name:       "agent default unrecognized",
			modify:     func(s *Settings) { s.Agent.Default = "invalid-agent" },
			wantCount:  1,
			wantSubstr: []string{"agent.default"},
		},
		{
			name:      "agent default claude",
			modify:    func(s *Settings) { s.Agent.Default = "claude" },
			wantCount: 0,
		},
		{
			name:      "agent default empty is valid",
			modify:    func(s *Settings) { s.Agent.Default = "" },
			wantCount: 0,
		},
		{
			name:       "preset unrecognized",
			modify:     func(s *Settings) { s.Preset = "invalid" },
			wantCount:  1,
			wantSubstr: []string{"preset"},
		},
		{
			name:      "preset fast",
			modify:    func(s *Settings) { s.Preset = "fast" },
			wantCount: 0,
		},
		{
			name:      "preset empty is valid",
			modify:    func(s *Settings) { s.Preset = "" },
			wantCount: 0,
		},
		{
			name: "agent default matches custom agent name",
			modify: func(s *Settings) {
				s.Agent.Default = "my-agent"
				s.CustomAgents = map[string]CustomAgent{"my-agent": {Extends: "claude"}}
			},
			wantCount: 0,
		},
		{
			name: "multiple issues returned together",
			modify: func(s *Settings) {
				s.Workers.Max = 0
				s.Watchdog = WatchdogSettings{Enabled: true, ThresholdMB: 1, IntervalSec: 1, WindowSize: 1}
				s.Storage.ActivityLog = ActivityLogSettings{Enabled: true, MaxFiles: 0}
				s.Agent.Default = "bogus"
				s.Preset = "nope"
			},
			wantCount: 7,
			wantSubstr: []string{
				"workers.max",
				"watchdog.threshold_mb",
				"watchdog.interval_sec",
				"watchdog.window_size",
				"activity_log.max_files",
				"agent.default",
				"preset",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSettings()
			tt.modify(&s)

			issues := s.Validate()

			if len(issues) != tt.wantCount {
				t.Fatalf("got %d issues, want %d\nissues: %v", len(issues), tt.wantCount, issues)
			}

			all := strings.Join(issues, "\n")
			for _, sub := range tt.wantSubstr {
				if !strings.Contains(all, sub) {
					t.Errorf("expected issue containing %q, got: %v", sub, issues)
				}
			}
		})
	}
}
