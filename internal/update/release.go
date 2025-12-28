package update

import "time"

// ReleaseInfo represents a GitHub release.
type ReleaseInfo struct {
	TagName     string    // e.g., "v1.2.3"
	Name        string    // e.g., "Release v1.2.3"
	PreRelease  bool      // true for pre-release versions
	PublishedAt time.Time // When the release was published
	HTMLURL     string    // URL to the release page
	Body        string    // Release notes
	Assets      []Asset   // Downloadable assets
}

// Asset represents a release asset (binary).
type Asset struct {
	Name string // e.g., "mehrhof-linux-amd64"
	URL  string // Browser download URL
	Size int64  // Size in bytes
}

// UpdateStatus represents the result of an update check.
type UpdateStatus struct {
	CurrentVersion string // Current version (from ldflags)
	LatestVersion  string // Latest available version
	AssetName      string // The binary asset for this platform
	AssetURL       string // Download URL
	AssetSize      int64  // Size in bytes
	Checksum       string // SHA256 checksum (empty if unavailable)
	IsNewer        bool   // true if LatestVersion > CurrentVersion
	IsPreRelease   bool   // true if latest is a pre-release
	ReleaseURL     string // URL to release page
	ReleaseNotes   string // Release body
}

// CheckOptions configures the update check behavior.
type CheckOptions struct {
	CurrentVersion    string // Current version (e.g., "v1.2.3" or "dev")
	IncludePreRelease bool   // If true, consider pre-release versions
	Owner             string // GitHub repo owner (default: "valksor")
	Repo              string // GitHub repo name (default: "go-mehrhof")
}
