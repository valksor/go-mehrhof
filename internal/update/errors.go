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

	// ErrDevBuild is returned when checking updates from a dev build.
	ErrDevBuild = errors.New("update: dev build")
)
