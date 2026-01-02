package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// FileLock provides file-based locking for concurrent access.
// Uses flock(2) for cross-process advisory locking.
type FileLock struct {
	file *os.File
	path string
}

// NewFileLock creates a new file lock for the given path.
// The lock file will be created if it doesn't exist.
func NewFileLock(path string) *FileLock {
	return &FileLock{path: path}
}

// Lock acquires an exclusive lock, blocking until available.
func (l *FileLock) Lock() error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}

	// Open or create the lock file
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}

	// Acquire exclusive lock (blocks until available)
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()

		return fmt.Errorf("acquire lock: %w", err)
	}

	l.file = f

	return nil
}

// TryLock attempts to acquire an exclusive lock without blocking.
// Returns false if the lock is held by another process.
func (l *FileLock) TryLock() (bool, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return false, fmt.Errorf("create lock directory: %w", err)
	}

	// Open or create the lock file
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return false, fmt.Errorf("open lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return false, nil // Lock held by another process
		}

		return false, fmt.Errorf("try lock: %w", err)
	}

	l.file = f

	return true, nil
}

// LockWithTimeout tries to acquire a lock with a timeout.
// Returns error if lock cannot be acquired within the timeout.
func (l *FileLock) LockWithTimeout(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	interval := 50 * time.Millisecond

	for {
		acquired, err := l.TryLock()
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("lock timeout after %v", timeout)
		}

		time.Sleep(interval)
		// Exponential backoff up to 500ms
		if interval < 500*time.Millisecond {
			interval = interval * 2
		}
	}
}

// Unlock releases the lock.
func (l *FileLock) Unlock() error {
	if l.file == nil {
		return nil
	}

	// Release the lock
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("release lock: %w", err)
	}

	// Close the file
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("close lock file: %w", err)
	}

	l.file = nil

	return nil
}

// WithLock executes a function while holding an exclusive lock.
// The lock is automatically released when the function returns.
func WithLock(lockPath string, fn func() error) error {
	lock := NewFileLock(lockPath)
	if err := lock.Lock(); err != nil {
		return err
	}
	defer func() { _ = lock.Unlock() }()

	return fn()
}

// WithLockTimeout executes a function while holding a lock,
// with a timeout for acquiring the lock.
func WithLockTimeout(lockPath string, timeout time.Duration, fn func() error) error {
	lock := NewFileLock(lockPath)
	if err := lock.LockWithTimeout(timeout); err != nil {
		return err
	}
	defer func() { _ = lock.Unlock() }()

	return fn()
}
