package settings

import (
	"fmt"
	"strings"
)

// Validate checks settings for invalid values and returns a list of issues.
// Returns nil if all settings are valid.
func (s *Settings) Validate() []string {
	var issues []string

	// Worker limits
	if s.Workers.Max < 1 {
		issues = append(issues, fmt.Sprintf("workers.max must be >= 1 (got %d)", s.Workers.Max))
	}
	if s.Workers.Max > 10 {
		issues = append(issues, fmt.Sprintf("workers.max should be <= 10 (got %d)", s.Workers.Max))
	}

	// Watchdog thresholds
	if s.Watchdog.Enabled {
		if s.Watchdog.ThresholdMB < 50 {
			issues = append(issues, fmt.Sprintf("watchdog.threshold_mb must be >= 50 (got %d)", s.Watchdog.ThresholdMB))
		}
		if s.Watchdog.IntervalSec < 5 {
			issues = append(issues, fmt.Sprintf("watchdog.interval_sec must be >= 5 (got %d)", s.Watchdog.IntervalSec))
		}
		if s.Watchdog.WindowSize < 3 {
			issues = append(issues, fmt.Sprintf("watchdog.window_size must be >= 3 (got %d)", s.Watchdog.WindowSize))
		}
	}

	// Activity log retention
	if s.Storage.ActivityLog.Enabled && s.Storage.ActivityLog.MaxFiles < 1 {
		issues = append(issues, fmt.Sprintf("storage.activity_log.max_files must be >= 1 (got %d)", s.Storage.ActivityLog.MaxFiles))
	}

	// Agent settings
	if s.Agent.Default != "" {
		validAgents := []string{"claude", "codex", "custom"}
		found := false
		for _, a := range validAgents {
			if s.Agent.Default == a {
				found = true

				break
			}
		}
		// Also check custom agents
		if !found {
			if _, ok := s.CustomAgents[s.Agent.Default]; ok {
				found = true
			}
		}
		if !found {
			issues = append(issues, fmt.Sprintf("agent.default %q is not a recognized agent (valid: claude, codex, custom, or a custom agent name)", s.Agent.Default))
		}
	}

	// Preset validation
	if s.Preset != "" {
		validPresets := []string{"compliance", "fast", "solo"}
		found := false
		for _, p := range validPresets {
			if s.Preset == p {
				found = true

				break
			}
		}
		if !found {
			issues = append(issues, fmt.Sprintf("preset %q is not recognized (valid: %s)", s.Preset, strings.Join(validPresets, ", ")))
		}
	}

	return issues
}
