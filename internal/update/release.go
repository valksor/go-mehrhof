package update

import "time"

// ReleaseInfo represents a GitHub release.
type ReleaseInfo struct {
	TagName     string
	Name        string
	PreRelease  bool
	PublishedAt time.Time
	HTMLURL     string
	Body        string
	Assets      []Asset
}

// Asset represents a release asset (binary).
type Asset struct {
	Name string
	URL  string
	Size int64
}

// UpdateStatus represents the result of an update check.
type UpdateStatus struct {
	CurrentVersion string
	LatestVersion  string
	AssetName      string
	AssetURL       string
	AssetSize      int64
	IsNewer        bool
	IsPreRelease   bool
	ReleaseURL     string
	ReleaseNotes   string
}

// MinisignPublicKey is the public key used to verify release signatures.
// Matches the key used in install.sh.
// Key ID: 7527FDFF95C1D7BB.
const MinisignPublicKey = "RWS718GV//0ndQGx85bxSWMv+70MdX0OUELW1QJ3j6KC44Zj2R8pRbTx"

// CheckOptions configures the update check behavior.
type CheckOptions struct {
	CurrentVersion string
	IncludeNightly bool
	TargetTag      string
	Owner          string
	Repo           string
}
