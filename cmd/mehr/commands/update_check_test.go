//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/config"
)

func TestShouldCheckForUpdates_DevBuild(t *testing.T) {
	// Save and restore original version
	originalVersion := Version
	defer func() { Version = originalVersion }()

	settings := &config.Settings{}

	// Test "dev" version
	Version = "dev"
	if shouldCheckForUpdates(settings) {
		t.Error("shouldCheckForUpdates returned true for 'dev' version")
	}

	// Test "none" version
	Version = "none"
	if shouldCheckForUpdates(settings) {
		t.Error("shouldCheckForUpdates returned true for 'none' version")
	}
}

func TestShouldCheckForUpdates_RecentCheck(t *testing.T) {
	// Save and restore original version
	originalVersion := Version
	defer func() { Version = originalVersion }()

	Version = "v1.0.0"

	settings := &config.Settings{
		LastUpdateCheck: time.Now().Add(-1 * time.Hour), // Checked 1 hour ago
	}

	if shouldCheckForUpdates(settings) {
		t.Error("shouldCheckForUpdates returned true when check was recent (1 hour ago)")
	}
}

func TestShouldCheckForUpdates_OldCheck(t *testing.T) {
	// Save and restore original version
	originalVersion := Version
	defer func() { Version = originalVersion }()

	Version = "v1.0.0"

	settings := &config.Settings{
		LastUpdateCheck: time.Now().Add(-25 * time.Hour), // Checked 25 hours ago
	}

	if !shouldCheckForUpdates(settings) {
		t.Error("shouldCheckForUpdates returned false when check was old (25 hours ago)")
	}
}

func TestShouldCheckForUpdates_NeverChecked(t *testing.T) {
	// Save and restore original version
	originalVersion := Version
	defer func() { Version = originalVersion }()

	Version = "v1.0.0"

	settings := &config.Settings{
		// LastUpdateCheck is zero value (never checked)
	}

	if !shouldCheckForUpdates(settings) {
		t.Error("shouldCheckForUpdates returned false when never checked before")
	}
}

func TestShouldCheckForUpdates_ExactlyAtInterval(t *testing.T) {
	// Save and restore original version
	originalVersion := Version
	defer func() { Version = originalVersion }()

	Version = "v1.0.0"

	// Check exactly 24 hours ago - should still be considered recent
	settings := &config.Settings{
		LastUpdateCheck: time.Now().Add(-24 * time.Hour),
	}

	// At exactly 24 hours, elapsed < checkInterval should be false (24h is not < 24h)
	// So shouldCheckForUpdates should return true
	if !shouldCheckForUpdates(settings) {
		t.Error("shouldCheckForUpdates returned false when check was exactly at interval boundary")
	}
}

func TestShouldCheckForUpdates_ReleaseBuild(t *testing.T) {
	// Save and restore original version
	originalVersion := Version
	defer func() { Version = originalVersion }()

	// Test various release version formats
	versions := []string{"v1.0.0", "1.0.0", "v2.3.4-beta", "0.1.0"}

	for _, v := range versions {
		Version = v
		settings := &config.Settings{
			LastUpdateCheck: time.Now().Add(-25 * time.Hour), // Old check
		}

		if !shouldCheckForUpdates(settings) {
			t.Errorf("shouldCheckForUpdates returned false for release version %q with old check", v)
		}
	}
}
