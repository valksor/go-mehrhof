package socket

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/valksor/kvelmo/paths"
)

// BaseDir returns the base directory for kvelmo data.
// Delegates to paths.Paths().BaseDir().
func BaseDir() string {
	return paths.BaseDir()
}

// GlobalSocketPath returns the path to the global socket.
// Delegates to paths.Paths().GlobalSocketPath().
func GlobalSocketPath() string {
	return paths.GlobalSocketPath()
}

// GlobalLockPath returns the path to the global lock file.
// Delegates to paths.Paths().GlobalLockPath().
func GlobalLockPath() string {
	return paths.GlobalLockPath()
}

// WorktreeSocketPath returns the socket path for a worktree directory.
// Delegates to paths.Paths().WorktreeSocketPath().
func WorktreeSocketPath(worktreeDir string) string {
	return paths.WorktreeSocketPath(worktreeDir)
}

// WorktreeIDFromPath returns a hash-based ID for a worktree directory.
// Delegates to paths.WorktreeIDFromPath().
func WorktreeIDFromPath(worktreeDir string) string {
	return paths.WorktreeIDFromPath(worktreeDir)
}

// SocketExists checks if a socket file exists at the given path.
func SocketExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeSocket != 0
}

// EnsureDir creates the required socket directories.
// Delegates to paths.Paths().EnsureDir().
func EnsureDir() error {
	return paths.EnsureDir()
}

// WorktreeLockPath returns the lock file path for a worktree socket.
// Placed alongside the socket file: {hash}.sock → {hash}.lock.
func WorktreeLockPath(worktreeDir string) string {
	sockPath := WorktreeSocketPath(worktreeDir)

	return strings.TrimSuffix(sockPath, ".sock") + ".lock"
}

// AcquireGlobalLock acquires an exclusive lock on the given lock file.
// Returns a release function that must be called to release the lock.
func AcquireGlobalLock(lockPath string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = file.Close()

		return nil, fmt.Errorf("acquire lock: %w", err)
	}

	release := func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}

	return release, nil
}
