package update

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/google/go-github/v67/github"
	"golang.org/x/mod/semver"
	"golang.org/x/oauth2"
)

// Checker checks for available updates from GitHub releases.
type Checker struct {
	ghClient *github.Client
	owner    string
	repo     string
}

// NewChecker creates a new update checker.
// If token is empty, unauthenticated requests are used (subject to rate limits).
func NewChecker(token, owner, repo string) *Checker {
	var ghClient *github.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		httpClient := oauth2.NewClient(context.Background(), ts)
		ghClient = github.NewClient(httpClient)
	} else {
		ghClient = github.NewClient(nil)
	}

	owner = cmp.Or(owner, "valksor")
	repo = cmp.Or(repo, "kvelmo")

	return &Checker{
		ghClient: ghClient,
		owner:    owner,
		repo:     repo,
	}
}

// Check looks for available updates and returns the status.
// Returns ErrNoUpdateAvailable if the current version is up to date.
func (c *Checker) Check(ctx context.Context, opts CheckOptions) (*UpdateStatus, error) {
	owner := cmp.Or(opts.Owner, c.owner)
	repo := cmp.Or(opts.Repo, c.repo)

	var latestRelease *github.RepositoryRelease
	var err error

	if opts.TargetTag != "" {
		latestRelease, err = c.findReleaseByTag(ctx, opts.TargetTag, owner, repo)
	} else {
		latestRelease, err = c.findLatestRelease(ctx, opts.IncludeNightly, owner, repo)
	}
	if err != nil {
		return nil, err
	}

	if latestRelease == nil {
		return nil, errors.New("no suitable release found")
	}

	latestVersion := strings.TrimPrefix(latestRelease.GetTagName(), "v")
	currentVersion := strings.TrimPrefix(opts.CurrentVersion, "v")

	// For explicit targets, install the requested version even if already installed.
	if opts.TargetTag == "" {
		// For dev/none builds, always offer installing the latest release.
		if opts.CurrentVersion != "dev" && opts.CurrentVersion != "none" {
			if currentVersion == latestVersion || versionNewer(currentVersion, latestVersion) {
				return &UpdateStatus{
					CurrentVersion: opts.CurrentVersion,
					LatestVersion:  latestRelease.GetTagName(),
					IsNewer:        false,
					IsPreRelease:   latestRelease.GetPrerelease(),
				}, ErrNoUpdateAvailable
			}
		}
	}

	expectedAsset := fmt.Sprintf("kvelmo-%s-%s", runtime.GOOS, runtime.GOARCH)
	var assetURL string
	var assetSize int64

	for _, asset := range latestRelease.Assets {
		name := asset.GetName()
		if name == expectedAsset {
			assetURL = asset.GetBrowserDownloadURL()
			assetSize = int64(asset.GetSize())

			break
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
	}

	return status, nil
}

func (c *Checker) findLatestRelease(ctx context.Context, includeNightly bool, owner, repo string) (*github.RepositoryRelease, error) {
	// Fetch the 30 most recent releases (single page). If the latest stable
	// release is buried under 30+ pre-releases, it will not be found.
	// Pagination can be added if nightly release cadence grows beyond this.
	releases, _, err := c.ghClient.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{
		PerPage: 30,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}

	if len(releases) == 0 {
		return nil, errors.New("no releases found")
	}

	for _, r := range releases {
		if r.GetDraft() {
			continue
		}
		if !includeNightly && r.GetPrerelease() {
			continue
		}

		return r, nil
	}

	return nil, errors.New("no suitable release found")
}

func (c *Checker) findReleaseByTag(ctx context.Context, tag, owner, repo string) (*github.RepositoryRelease, error) {
	tags := []string{tag}
	if after, ok := strings.CutPrefix(tag, "v"); ok {
		tags = append(tags, after)
	} else if tag != "nightly" {
		tags = append(tags, "v"+tag)
	}

	for _, candidate := range tags {
		release, resp, err := c.ghClient.Repositories.GetReleaseByTag(ctx, owner, repo, candidate)
		if err == nil {
			return release, nil
		}
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			continue
		}

		return nil, fmt.Errorf("get release by tag %q: %w", candidate, err)
	}

	return nil, fmt.Errorf("%w: %q", ErrReleaseNotFound, tag)
}

// versionNewer returns true if a is newer than b using semantic versioning.
func versionNewer(a, b string) bool {
	if !strings.HasPrefix(a, "v") {
		a = "v" + a
	}
	if !strings.HasPrefix(b, "v") {
		b = "v" + b
	}

	return semver.Compare(a, b) > 0
}

// ReleaseInfoFromGitHub converts a GitHub release to ReleaseInfo.
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
