package update

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"testing"
)

func TestNewChecker_WithToken(t *testing.T) {
	c := NewChecker("ghp_faketoken", "acme", "widget")
	if c == nil {
		t.Fatal("NewChecker returned nil")
	}
	if c.owner != "acme" || c.repo != "widget" {
		t.Errorf("owner/repo = %q/%q, want acme/widget", c.owner, c.repo)
	}
}

func TestNewChecker_Defaults(t *testing.T) {
	c := NewChecker("", "", "")
	if c.owner != "valksor" || c.repo != "kvelmo" {
		t.Errorf("defaults = %q/%q, want valksor/kvelmo", c.owner, c.repo)
	}
}

func TestCheck_SkipsDraftsAndPrereleases(t *testing.T) {
	assetName := "kvelmo-" + runtime.GOOS + "-" + runtime.GOARCH
	checker, closeFn := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/valksor/kvelmo/releases" {
			http.NotFound(w, r)

			return
		}
		releases := []map[string]any{
			{"tag_name": "v3.0.0-draft", "draft": true, "prerelease": false},
			{"tag_name": "v2.0.0-rc1", "draft": false, "prerelease": true},
			{
				"tag_name":   "v1.0.0",
				"draft":      false,
				"prerelease": false,
				"assets": []map[string]any{
					{"name": assetName, "browser_download_url": "https://x/" + assetName, "size": 10},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(releases)
	})
	defer closeFn()

	// Without nightly, should skip the draft and prerelease, land on v1.0.0.
	status, err := checker.Check(context.Background(), CheckOptions{CurrentVersion: "v0.9.0"})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if status.LatestVersion != "v1.0.0" {
		t.Errorf("LatestVersion = %q, want v1.0.0", status.LatestVersion)
	}
	if !status.IsNewer {
		t.Error("expected IsNewer=true")
	}
}

func TestCheck_IncludeNightlyPicksPrerelease(t *testing.T) {
	assetName := "kvelmo-" + runtime.GOOS + "-" + runtime.GOARCH
	checker, closeFn := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/valksor/kvelmo/releases" {
			http.NotFound(w, r)

			return
		}
		releases := []map[string]any{
			{
				"tag_name":   "v2.0.0-rc1",
				"draft":      false,
				"prerelease": true,
				"assets": []map[string]any{
					{"name": assetName, "browser_download_url": "https://x/" + assetName, "size": 5},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(releases)
	})
	defer closeFn()

	status, err := checker.Check(context.Background(), CheckOptions{
		CurrentVersion: "v1.0.0",
		IncludeNightly: true,
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if status.LatestVersion != "v2.0.0-rc1" {
		t.Errorf("LatestVersion = %q, want v2.0.0-rc1", status.LatestVersion)
	}
	if !status.IsPreRelease {
		t.Error("expected IsPreRelease=true")
	}
}

func TestCheck_NoUpdateAvailableWhenCurrent(t *testing.T) {
	assetName := "kvelmo-" + runtime.GOOS + "-" + runtime.GOARCH
	checker, closeFn := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/valksor/kvelmo/releases" {
			http.NotFound(w, r)

			return
		}
		releases := []map[string]any{
			{
				"tag_name":   "v1.0.0",
				"draft":      false,
				"prerelease": false,
				"assets": []map[string]any{
					{"name": assetName, "browser_download_url": "https://x/" + assetName, "size": 1},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(releases)
	})
	defer closeFn()

	status, err := checker.Check(context.Background(), CheckOptions{CurrentVersion: "v1.0.0"})
	if !errors.Is(err, ErrNoUpdateAvailable) {
		t.Fatalf("Check() error = %v, want ErrNoUpdateAvailable", err)
	}
	if status == nil || status.IsNewer {
		t.Error("expected non-nil status with IsNewer=false")
	}
}

func TestCheck_AssetNotFoundForPlatform(t *testing.T) {
	checker, closeFn := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/valksor/kvelmo/releases" {
			http.NotFound(w, r)

			return
		}
		releases := []map[string]any{
			{
				"tag_name":   "v1.2.3",
				"draft":      false,
				"prerelease": false,
				"assets": []map[string]any{
					{"name": "kvelmo-someother-arch", "browser_download_url": "https://x/y", "size": 1},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(releases)
	})
	defer closeFn()

	_, err := checker.Check(context.Background(), CheckOptions{CurrentVersion: "dev"})
	if !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("Check() error = %v, want ErrAssetNotFound", err)
	}
}

func TestCheck_NoReleases(t *testing.T) {
	checker, closeFn := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/valksor/kvelmo/releases" {
			http.NotFound(w, r)

			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	})
	defer closeFn()

	_, err := checker.Check(context.Background(), CheckOptions{CurrentVersion: "dev"})
	if err == nil {
		t.Fatal("expected error when no releases returned")
	}
}

func TestCheck_TargetTagWithVPrefixFallback(t *testing.T) {
	assetName := "kvelmo-" + runtime.GOOS + "-" + runtime.GOARCH
	checker, closeFn := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		// First candidate "1.5.0" 404s; fallback "v1.5.0" succeeds.
		if r.URL.Path == "/repos/valksor/kvelmo/releases/tags/1.5.0" {
			http.NotFound(w, r)

			return
		}
		if r.URL.Path == "/repos/valksor/kvelmo/releases/tags/v1.5.0" {
			release := map[string]any{
				"tag_name":   "v1.5.0",
				"draft":      false,
				"prerelease": false,
				"assets": []map[string]any{
					{"name": assetName, "browser_download_url": "https://x/" + assetName, "size": 7},
				},
			}
			_ = json.NewEncoder(w).Encode(release)

			return
		}
		http.NotFound(w, r)
	})
	defer closeFn()

	status, err := checker.Check(context.Background(), CheckOptions{
		CurrentVersion: "v1.0.0",
		TargetTag:      "1.5.0",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if status.LatestVersion != "v1.5.0" {
		t.Errorf("LatestVersion = %q, want v1.5.0", status.LatestVersion)
	}
}
