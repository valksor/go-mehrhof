package update

import (
	"fmt"
	"os"
	"path/filepath"
)

// Installer performs atomic binary replacement.
type Installer struct{}

// NewInstaller creates a new installer.
func NewInstaller() *Installer {
	return &Installer{}
}

// Install replaces the current binary with the downloaded one.
// On Unix (Linux/macOS), os.Rename() is atomic on POSIX systems.
// The running process keeps the old file open; new invocations use the new binary.
func (i *Installer) Install(downloadedPath string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("%w: get executable path: %w", ErrInstallFailed, err)
	}

	// Resolve symlinks so we replace the actual binary, not the symlink itself.
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("%w: resolve symlinks: %w", ErrInstallFailed, err)
	}

	if err := os.Chmod(downloadedPath, 0o755); err != nil {
		return fmt.Errorf("%w: chmod failed: %w", ErrInstallFailed, err)
	}

	if err := os.Rename(downloadedPath, self); err != nil {
		return fmt.Errorf("%w: rename failed: %w", ErrInstallFailed, err)
	}

	return nil
}

// DownloadDir returns the directory where update downloads should be placed.
// This is the same filesystem as the binary to ensure os.Rename works atomically.
func (i *Installer) DownloadDir() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}

	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}

	return filepath.Dir(self), nil
}

// IsWritable checks if the binary directory is writable.
// Returns false if the user may need to run with sudo.
func (i *Installer) IsWritable() (bool, error) {
	self, err := os.Executable()
	if err != nil {
		return false, err
	}

	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return false, err
	}

	dir := filepath.Dir(self)
	tmpFile, err := os.CreateTemp(dir, ".kvelmo-update-test-")
	if err != nil {
		//nolint:nilerr // Can't create temp file = dir not writable
		return false, nil
	}

	_ = tmpFile.Close()
	_ = os.Remove(tmpFile.Name())

	return true, nil
}

// BinaryPath returns the path to the current binary.
func BinaryPath() (string, error) {
	return os.Executable()
}
