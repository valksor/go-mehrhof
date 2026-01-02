package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/provider/github"
	"github.com/valksor/go-mehrhof/internal/update"
)

var (
	updatePreRelease bool
	updateCheckOnly  bool
	updateYes        bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update mehr to the latest version",
	Long: `Update mehr to the latest version from GitHub releases.

By default, only stable releases are considered. Use --pre-release to include
pre-release versions.

The update process:
1. Checks for the latest release
2. Downloads the binary for your platform
3. Verifies checksum (if available)
4. Replaces the current binary atomically

After a successful update, restart mehr to use the new version.`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVarP(&updatePreRelease, "pre-release", "p", false,
		"Include pre-release versions")
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false,
		"Check for updates without installing")
	updateCmd.Flags().BoolVarP(&updateYes, "yes", "y", false,
		"Skip confirmation prompt")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	// Resolve GitHub token (anonymous access works for public repos)
	token, err := github.ResolveToken("")
	if err != nil {
		// Continue without token - may hit rate limits but works for public repos
		fmt.Fprintf(os.Stderr, "%s Running without authentication (rate limits may apply)\n",
			display.Warning("→"))
	}

	opts := update.CheckOptions{
		CurrentVersion:    Version,
		IncludePreRelease: updatePreRelease,
	}

	// Show checking message
	fmt.Println(display.Info("→") + " Checking for updates...")

	// Check for updates
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	// Create checker
	checker := update.NewChecker(ctx, token, "valksor", "go-mehrhof")

	status, err := checker.Check(ctx, opts)
	if err != nil {
		if errors.Is(err, update.ErrNoUpdateAvailable) {
			display.Success("Already up to date")
			fmt.Printf("Current version: %s\n", Version)

			return nil
		}
		if errors.Is(err, update.ErrDevBuild) {
			fmt.Printf(display.Warning("⚠")+" Dev build detected (%s)\n", Version)
			fmt.Println("Update checks are not available for dev builds.")
			fmt.Println("Install a release version to enable updates:")
			fmt.Println("  https://github.com/valksor/go-mehrhof/releases")

			return nil
		}

		return fmt.Errorf("check for updates: %w", err)
	}

	// Display update available
	fmt.Printf("\n%s %s\n", display.Success("✓"), display.Bold("Update available"))
	fmt.Printf("  Current:   %s\n", display.Muted(status.CurrentVersion))
	fmt.Printf("  Latest:    %s\n", display.Success(status.LatestVersion))
	if status.ReleaseURL != "" {
		fmt.Printf("  Release:   %s\n", display.Muted(status.ReleaseURL))
	}
	if status.AssetSize > 0 {
		sizeMB := float64(status.AssetSize) / 1024 / 1024
		fmt.Printf("  Download:  %s (%.1f MB)\n", display.Muted(status.AssetName), sizeMB)
	}

	if updateCheckOnly {
		return nil
	}

	// Confirm before downloading
	if !updateYes {
		prompt := fmt.Sprintf("Download and install %s?", status.LatestVersion)
		confirmed, err := confirmAction(prompt, false)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println(display.Muted("Update cancelled"))

			return nil
		}
	}

	// Check if writable
	installer := update.NewInstaller()
	writable, _ := installer.IsWritable()
	if !writable {
		return fmt.Errorf("%s\n\nTry running with sudo: sudo mehr update", display.ErrorMsg(
			"Cannot write to binary directory"))
	}

	// Download the update
	downloader := update.NewDownloader()
	spinner := display.NewSpinner("Downloading update")
	spinner.Start()

	// Fetch checksums URL from the release assets
	checksumsURL := getChecksumsURL(ctx, checker, status)

	downloadedPath, err := downloader.DownloadWithChecksums(
		ctx,
		status.AssetURL,
		checksumsURL,
		status.AssetName,
	)
	if err != nil {
		spinner.StopWithError(fmt.Sprintf("Download failed: %v", err))

		return err
	}

	spinner.StopWithSuccess("Download complete")

	// Warn if no checksum was available
	if checksumsURL != "" && status.Checksum == "" {
		fmt.Printf("\n%s Checksum verification unavailable - proceeding anyway\n",
			display.Warning("→"))
	}

	// Install the update
	spinner = display.NewSpinner("Installing update")
	spinner.Start()

	if err := installer.Install(downloadedPath); err != nil {
		spinner.StopWithError(fmt.Sprintf("Installation failed: %v", err))

		return err
	}

	spinner.StopWithSuccess("Installation complete")

	fmt.Printf("\n%s Updated to %s\n", display.SuccessMsg(""), display.Bold(status.LatestVersion))
	fmt.Printf("%s Restart mehr to use the new version\n\n", display.Muted("→"))

	return nil
}

// getChecksumsURL fetches the checksums file URL from the release assets.
func getChecksumsURL(_ context.Context, _ *update.Checker, status *update.UpdateStatus) string {
	// We need to fetch the release info to get the checksums URL
	// For now, construct it from the release URL pattern
	// GitHub releases follow: /owner/repo/releases/download/tag/asset
	// So checksums would be at: /owner/repo/releases/download/tag/checksums.txt

	// Extract tag from status.LatestVersion
	tag := status.LatestVersion

	return fmt.Sprintf("https://github.com/valksor/go-mehrhof/releases/download/%s/checksums.txt", tag)
}
