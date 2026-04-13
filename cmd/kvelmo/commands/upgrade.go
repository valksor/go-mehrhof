package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/cli"
	"github.com/valksor/kvelmo/internal/update"
	"github.com/valksor/kvelmo/meta"
	"github.com/valksor/kvelmo/settings"
)

var (
	upgradeNightly    bool
	upgradeCheck      bool
	upgradeYes        bool
	upgradeVersion    string
	upgradeSkipVerify bool
)

// UpgradeCmd updates the kvelmo binary to the latest version.
var UpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Update kvelmo to the latest version",
	Long: `Update kvelmo to the latest version from GitHub releases.

By default, only stable releases are considered. Use --nightly to include
nightly/pre-release versions.

The update process:
1. Checks for the latest release
2. Downloads the checksums file and verifies its Minisign signature
3. Downloads the binary for your platform
4. Verifies SHA256 checksum
5. Replaces the current binary atomically

After a successful update, restart kvelmo to use the new version.`,
	RunE: runUpgrade,
}

func init() {
	UpgradeCmd.Flags().BoolVarP(&upgradeNightly, "nightly", "n", false,
		"Install latest available release including pre-releases")
	UpgradeCmd.Flags().StringVarP(&upgradeVersion, "version", "v", "",
		"Install specific version tag (e.g. v1.2.3)")
	UpgradeCmd.Flags().BoolVar(&upgradeCheck, "check", false,
		"Check for updates without installing")
	UpgradeCmd.Flags().BoolVarP(&upgradeYes, "yes", "y", false,
		"Skip confirmation prompt")
	UpgradeCmd.Flags().BoolVar(&upgradeSkipVerify, "skip-verify", false,
		"Allow installation even if signature verification is unavailable")
}

func runUpgrade(cmd *cobra.Command, _ []string) error {
	if upgradeNightly && upgradeVersion != "" {
		return errors.New("--nightly and --version are mutually exclusive")
	}

	// Resolve GitHub token from .env files (not os.Getenv).
	token := resolveGitHubToken()

	opts := update.CheckOptions{
		CurrentVersion: meta.Version,
		IncludeNightly: upgradeNightly,
		TargetTag:      upgradeVersion,
	}

	fmt.Println("Checking for updates...")

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	checker := update.NewChecker(token, "valksor", "kvelmo")

	status, err := checker.Check(ctx, opts)
	if err != nil {
		if errors.Is(err, update.ErrNoUpdateAvailable) {
			fmt.Printf("Already up to date (%s)\n", meta.Version)

			return nil
		}
		if errors.Is(err, update.ErrReleaseNotFound) {
			return fmt.Errorf("requested release not found: %w", err)
		}
		if errors.Is(err, update.ErrAssetNotFound) {
			fmt.Println("No binary available for your platform.")
			fmt.Println("Check https://github.com/valksor/kvelmo/releases for supported platforms.")

			return err
		}

		return fmt.Errorf("check for updates: %w", err)
	}

	_, _ = cli.Green.Println("\n✓ Update available")
	fmt.Printf("  Current:   %s\n", status.CurrentVersion)
	fmt.Printf("  Latest:    %s\n", status.LatestVersion)
	if status.ReleaseURL != "" {
		fmt.Printf("  Release:   %s\n", status.ReleaseURL)
	}
	if status.AssetSize > 0 {
		sizeMB := float64(status.AssetSize) / 1024 / 1024
		fmt.Printf("  Download:  %s (%.1f MB)\n", status.AssetName, sizeMB)
	}
	if status.IsPreRelease {
		_, _ = cli.Yellow.Println("  ⚠ This is a pre-release version.")
	}

	if upgradeCheck {
		return nil
	}

	if !upgradeYes {
		fmt.Printf("\nDownload and install %s? [y/N]: ", status.LatestVersion)

		reader := bufio.NewReader(cmd.InOrStdin())
		response, readErr := reader.ReadString('\n')
		if readErr != nil {
			return readErr
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Update cancelled")

			return nil
		}
	}

	installer := update.NewInstaller()
	writable, err := installer.IsWritable()
	if err != nil {
		return fmt.Errorf("check write permissions: %w", err)
	}
	if !writable {
		fmt.Println("Cannot write to binary directory. Try running with sudo:")
		fmt.Println("  sudo kvelmo upgrade")

		return errors.New("insufficient permissions")
	}

	downloader := update.NewDownloader()

	// Stage downloads on the same filesystem as the binary for atomic rename.
	stageDir, err := installer.DownloadDir()
	if err != nil {
		return fmt.Errorf("determine download directory: %w", err)
	}
	downloader.SetStageDir(stageDir)

	if upgradeSkipVerify {
		_, _ = cli.Yellow.Println("\n⚠ --skip-verify: signature verification disabled. Only SHA256 checksum will be checked.")
		_, _ = cli.Yellow.Println("  This reduces security — use only if signature files are unavailable.")
	}

	spinner := cli.NewSpinner("Downloading and verifying update")
	spinner.Start()

	checksumsURL, signatureURL := releaseURLs(status)

	downloadedPath, verifyResult, err := downloader.DownloadWithSignature(
		ctx,
		status.AssetURL,
		checksumsURL,
		signatureURL,
		status.AssetName,
		update.MinisignPublicKey,
		!upgradeSkipVerify, // require signature unless user opted out
	)
	if err != nil {
		if errors.Is(err, update.ErrSignatureVerificationFailed) {
			spinner.Fail("Signature verification failed")
			fmt.Println("\nThe checksums file signature is invalid. This may indicate tampering.")
			fmt.Println("Update aborted for security. Please report this issue.")

			return err
		}
		spinner.Fail(fmt.Sprintf("Download failed: %v", err))
		fmt.Println("\nRetry the command, or download manually from:")
		fmt.Printf("  %s\n", status.ReleaseURL)

		return err
	}

	// Clean up the downloaded binary on any subsequent error.
	defer func() {
		if downloadedPath != "" {
			_ = os.Remove(downloadedPath)
		}
	}()

	spinner.Success("Download complete")

	if verifyResult.SignatureVerified {
		fmt.Println("✓ Signature verified")
	} else if verifyResult.SignatureSkipped {
		fmt.Println("⚠ Signature verification skipped (--skip-verify)")
	}
	if verifyResult.ChecksumVerified {
		fmt.Println("✓ Checksum verified")
	}

	spinner = cli.NewSpinner("Installing update")
	spinner.Start()

	if err := installer.Install(downloadedPath); err != nil {
		spinner.Fail(fmt.Sprintf("Installation failed: %v", err))

		return err
	}
	downloadedPath = "" // Installed successfully — prevent deferred cleanup

	spinner.Success("Installation complete")

	_, _ = cli.Green.Printf("\nUpdated to %s\n", status.LatestVersion)
	fmt.Println("If kvelmo serve is running, restart it: kvelmo shutdown && kvelmo serve")

	return nil
}

// releaseURLs returns the checksums and signature URLs for a release.
func releaseURLs(status *update.UpdateStatus) (string, string) {
	// Derive base URL from the asset URL to support repo transfers/mirrors.
	// AssetURL format: https://github.com/<owner>/<repo>/releases/download/<tag>/<asset>
	base := status.AssetURL
	if i := strings.LastIndex(base, "/"); i > 0 {
		base = base[:i]
	}

	return base + "/checksums.txt", base + "/checksums.txt.minisig"
}

// resolveGitHubToken loads the GitHub token from .env files.
// Returns empty string if unavailable (anonymous access works for public repos).
func resolveGitHubToken() string {
	env, err := settings.LoadEnvMap("")
	if err != nil {
		return ""
	}

	return env.Get("GITHUB_TOKEN")
}
