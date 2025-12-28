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
// On Unix (Linux/macOS), this uses os.Rename() which is atomic on POSIX systems.
// The running process keeps the old file open; new invocations use the new binary.
func (i *Installer) Install(downloadedPath string) error {
	// Get current binary path
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("%w: get executable path: %w", ErrInstallFailed, err)
	}

	// Ensure downloaded binary is executable
	if err := os.Chmod(downloadedPath, 0o755); err != nil {
		return fmt.Errorf("%w: chmod failed: %w", ErrInstallFailed, err)
	}

	// Atomic rename (works on Linux/macOS)
	// The running process keeps the old file open
	// New processes will get the new binary
	if err := os.Rename(downloadedPath, self); err != nil {
		return fmt.Errorf("%w: rename failed: %w", ErrInstallFailed, err)
	}

	return nil
}

// IsWritable checks if the binary directory is writable.
// Returns false if the user may need to run with sudo.
func (i *Installer) IsWritable() (bool, error) {
	self, err := os.Executable()
	if err != nil {
		return false, err
	}

	// Try to create a temporary file in the same directory
	dir := filepath.Dir(self)
	tmpFile, err := os.CreateTemp(dir, ".mehrhof-update-test-")
	if err != nil {
		return false, nil
	}

	// Clean up the test file
	_ = tmpFile.Close()
	_ = os.Remove(tmpFile.Name())

	return true, nil
}

// BinaryPath returns the path to the current binary.
func BinaryPath() (string, error) {
	return os.Executable()
}
