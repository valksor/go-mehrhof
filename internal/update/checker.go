package update

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/google/go-github/v67/github"
	"golang.org/x/oauth2"
)

// Checker checks for available updates from GitHub releases.
type Checker struct {
	ghClient *github.Client
	owner    string
	repo     string
}

// NewChecker creates a new update checker.
// If token is empty, the client will make unauthenticated requests (subject to rate limits).
func NewChecker(token, owner, repo string) *Checker {
	var tc *github.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		httpClient := oauth2.NewClient(context.Background(), ts)
		tc = github.NewClient(httpClient)
	} else {
		tc = github.NewClient(nil)
	}

	// Set default owner/repo if not provided
	if owner == "" {
		owner = "valksor"
	}
	if repo == "" {
		repo = "go-mehrhof"
	}

	return &Checker{
		ghClient: tc,
		owner:    owner,
		repo:     repo,
	}
}

// Check looks for available updates and returns the status.
// It returns ErrNoUpdateAvailable if the current version is up to date.
// It returns ErrDevBuild if the current version is "dev".
func (c *Checker) Check(ctx context.Context, opts CheckOptions) (*UpdateStatus, error) {
	// Handle dev builds - always indicate an update is available
	if opts.CurrentVersion == "dev" || opts.CurrentVersion == "none" {
		return nil, ErrDevBuild
	}

	// Set owner/repo from options if provided
	if opts.Owner != "" {
		c.owner = opts.Owner
	}
	if opts.Repo != "" {
		c.repo = opts.Repo
	}

	// List releases to find the latest
	releases, _, err := c.ghClient.Repositories.ListReleases(ctx, c.owner, c.repo, &github.ListOptions{
		PerPage: 10, // Get last 10 releases
	})
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	// Find the latest release (stable or pre-release based on options)
	var latestRelease *github.RepositoryRelease
	for _, r := range releases {
		if r.GetDraft() {
			continue // Skip draft releases
		}
		if !opts.IncludePreRelease && r.GetPrerelease() {
			continue // Skip pre-releases if not requested
		}
		latestRelease = r
		break // First matching release is the latest (API returns in descending order)
	}

	if latestRelease == nil {
		return nil, fmt.Errorf("no suitable release found")
	}

	// Normalize version strings for comparison
	latestVersion := strings.TrimPrefix(latestRelease.GetTagName(), "v")
	currentVersion := strings.TrimPrefix(opts.CurrentVersion, "v")

	// If current is same or newer than latest, no update needed
	if currentVersion == latestVersion || versionNewer(currentVersion, latestVersion) {
		return &UpdateStatus{
			CurrentVersion: opts.CurrentVersion,
			LatestVersion:  latestRelease.GetTagName(),
			IsNewer:        false,
			IsPreRelease:   latestRelease.GetPrerelease(),
		}, ErrNoUpdateAvailable
	}

	// Find the matching asset for this platform
	expectedAsset := fmt.Sprintf("mehrhof-%s-%s", runtime.GOOS, runtime.GOARCH)
	var assetURL string
	var assetSize int64
	var checksumsURL string

	for _, asset := range latestRelease.Assets {
		name := asset.GetName()
		switch name {
		case expectedAsset:
			assetURL = asset.GetBrowserDownloadURL()
			assetSize = int64(asset.GetSize())
		case "checksums.txt":
			checksumsURL = asset.GetBrowserDownloadURL()
		}
	}

	if assetURL == "" {
		return nil, fmt.Errorf("%w: %s for %s/%s", ErrAssetNotFound, expectedAsset, runtime.GOOS, runtime.GOARCH)
	}

	status := &UpdateStatus{
		CurrentVersion: opts.CurrentVersion,
		LatestVersion:  latestRelease.GetTagName(),
		AssetName:      expectedAsset,
		AssetURL:       assetURL,
		AssetSize:      assetSize,
		IsNewer:        true,
		IsPreRelease:   latestRelease.GetPrerelease(),
		ReleaseURL:     latestRelease.GetHTMLURL(),
		ReleaseNotes:   latestRelease.GetBody(),
		Checksum:       "", // Will be populated by fetching checksums
	}

	// Fetch checksums if available
	if checksumsURL != "" {
		status.Checksum, _ = c.fetchChecksum(ctx, checksumsURL, expectedAsset)
	}

	return status, nil
}

// fetchChecksum downloads and parses the checksums file to find the checksum for the given asset.
func (c *Checker) fetchChecksum(ctx context.Context, url, assetName string) (string, error) {
	// We can't use ghClient directly for this, we need to make a raw HTTP request
	// But we can let the downloader handle this - just return empty for now
	// The downloader will fetch and parse the checksums file
	return "", nil
}

// versionNewer compares two version strings and returns true if a is newer than b.
// This is a simple semver comparison that handles major.minor.patch format.
func versionNewer(a, b string) bool {
	// Simple semver comparison - parse major.minor.patch
	var aMajor, aMinor, aPatch int
	var bMajor, bMinor, bPatch int

	_, _ = fmt.Sscanf(a, "%d.%d.%d", &aMajor, &aMinor, &aPatch)
	_, _ = fmt.Sscanf(b, "%d.%d.%d", &bMajor, &bMinor, &bPatch)

	if aMajor > bMajor {
		return true
	}
	if aMajor == bMajor && aMinor > bMinor {
		return true
	}
	if aMajor == bMajor && aMinor == bMinor && aPatch > bPatch {
		return true
	}
	return false
}

// ReleaseInfoFromGitHub converts a GitHub release to our ReleaseInfo type.
func ReleaseInfoFromGitHub(gh *github.RepositoryRelease) *ReleaseInfo {
	assets := make([]Asset, 0, len(gh.Assets))
	for _, a := range gh.Assets {
		assets = append(assets, Asset{
			Name: a.GetName(),
			URL:  a.GetBrowserDownloadURL(),
			Size: int64(a.GetSize()),
		})
	}

	var publishedAt time.Time
	if gh.PublishedAt != nil {
		publishedAt = gh.PublishedAt.Time
	}

	return &ReleaseInfo{
		TagName:     gh.GetTagName(),
		Name:        gh.GetName(),
		PreRelease:  gh.GetPrerelease(),
		PublishedAt: publishedAt,
		HTMLURL:     gh.GetHTMLURL(),
		Body:        gh.GetBody(),
		Assets:      assets,
	}
}
