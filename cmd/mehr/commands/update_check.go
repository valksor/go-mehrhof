package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/valksor/go-mehrhof/internal/config"
	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/provider/github"
	"github.com/valksor/go-mehrhof/internal/update"
)

// checkForUpdatesInBackground performs an asynchronous update check.
// It respects the update check interval and only prints to stderr.
// This function should be called in a goroutine from PersistentPreRunE.
func checkForUpdatesInBackground(ctx context.Context) {
	// Use a short timeout to avoid slowing down CLI startup
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Get settings to check last update time
	settings, err := config.LoadSettings()
	if err != nil {
		return // Silently skip if settings can't be loaded
	}

	// Get workspace config to check if updates are enabled
	// We need to open a workspace to read the config
	// For now, use a default interval of 24 hours
	const checkInterval = 24 * time.Hour

	// Check if we've checked recently
	if !settings.LastUpdateCheck.IsZero() {
		elapsed := time.Since(settings.LastUpdateCheck)
		if elapsed < checkInterval {
			return // Already checked recently
		}
	}

	// Resolve GitHub token
	token, _ := github.ResolveToken("")

	// Create checker and check for updates
	checker := update.NewChecker(timeoutCtx, token, "valksor", "go-mehrhof")

	opts := update.CheckOptions{
		CurrentVersion:    Version,
		IncludePreRelease: false, // Only check for stable releases in background
	}

	status, err := checker.Check(timeoutCtx, opts)
	if err != nil {
		// Silently skip on errors - don't bother the user
		// Update the timestamp so we don't check again too soon
		_ = saveUpdateCheckTime(settings)

		return
	}

	// Update the timestamp
	_ = saveUpdateCheckTime(settings)

	// Only notify if there's actually an update available
	if status.IsNewer {
		// Print to stderr so it doesn't interfere with command output
		fmt.Fprintf(os.Stderr, "\n%s %s is available (you have %s)\n",
			display.Info("→"), display.Bold(status.LatestVersion), display.Muted(Version))
		fmt.Fprintf(os.Stderr, "%s Run 'mehr update' to install\n\n", display.Muted("→"))
	}
}

// saveUpdateCheckTime saves the current time as the last update check time.
func saveUpdateCheckTime(settings *config.Settings) error {
	settings.LastUpdateCheck = time.Now()

	return settings.Save()
}

// shouldCheckForUpdates returns true if update checks are enabled and it's time to check again.
func shouldCheckForUpdates(settings *config.Settings) bool {
	// Skip if this is a dev build
	if Version == "dev" || Version == "none" {
		return false
	}

	// Check if we've checked recently
	if !settings.LastUpdateCheck.IsZero() {
		const checkInterval = 24 * time.Hour
		elapsed := time.Since(settings.LastUpdateCheck)
		if elapsed < checkInterval {
			return false
		}
	}

	return true
}
