package update

import "errors"

var (
	// ErrNoUpdateAvailable is returned when the current version is already up to date.
	ErrNoUpdateAvailable = errors.New("update: no update available")

	// ErrDownloadFailed is returned when downloading the release binary fails.
	ErrDownloadFailed = errors.New("update: download failed")

	// ErrChecksumFailed is returned when checksum verification fails.
	ErrChecksumFailed = errors.New("update: checksum verification failed")

	// ErrInstallFailed is returned when installing the update fails.
	ErrInstallFailed = errors.New("update: installation failed")

	// ErrAssetNotFound is returned when no suitable asset is found for the current platform.
	ErrAssetNotFound = errors.New("update: no suitable asset found for platform")

	// ErrSignatureVerificationFailed is returned when Minisign signature verification fails.
	ErrSignatureVerificationFailed = errors.New("update: signature verification failed")

	// ErrReleaseNotFound is returned when a specific release tag cannot be found.
	ErrReleaseNotFound = errors.New("update: release not found")
)
