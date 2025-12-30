package storage

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileLock_LockUnlock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	lock := NewFileLock(lockPath)

	// Lock should succeed
	if err := lock.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Unlock should succeed
	if err := lock.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	// Lock file should exist
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file should exist after Lock/Unlock")
	}
}

func TestFileLock_TryLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "trylock.lock")

	lock1 := NewFileLock(lockPath)
	lock2 := NewFileLock(lockPath)

	// First lock should succeed
	acquired, err := lock1.TryLock()
	if err != nil {
		t.Fatalf("TryLock: %v", err)
	}
	if !acquired {
		t.Error("first TryLock should succeed")
	}

	// Second lock should fail (non-blocking)
	acquired, err = lock2.TryLock()
	if err != nil {
		t.Fatalf("TryLock second: %v", err)
	}
	if acquired {
		t.Error("second TryLock should fail when lock is held")
	}

	// After unlocking first, second should succeed
	if err := lock1.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	acquired, err = lock2.TryLock()
	if err != nil {
		t.Fatalf("TryLock after unlock: %v", err)
	}
	if !acquired {
		t.Error("TryLock should succeed after first lock released")
	}
	_ = lock2.Unlock()
}

func TestFileLock_LockWithTimeout(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "timeout.lock")

	lock1 := NewFileLock(lockPath)
	lock2 := NewFileLock(lockPath)

	// First lock
	if err := lock1.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Second lock with short timeout should fail
	start := time.Now()
	err := lock2.LockWithTimeout(100 * time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("LockWithTimeout should fail when lock is held")
		_ = lock2.Unlock()
	}

	// Should have waited approximately the timeout duration
	if elapsed < 90*time.Millisecond {
		t.Errorf("waited only %v, expected ~100ms", elapsed)
	}

	_ = lock1.Unlock()
}

func TestWithLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "withlock.lock")

	var counter int32
	var wg sync.WaitGroup

	// Run multiple goroutines that increment counter with lock
	for i := 0; i < 5; i++ {
		wg.Go(func() {
			err := WithLock(lockPath, func() error {
				// Read, increment, write (non-atomic without lock)
				val := atomic.LoadInt32(&counter)
				time.Sleep(10 * time.Millisecond) // Small delay to expose race
				atomic.StoreInt32(&counter, val+1)
				return nil
			})
			if err != nil {
				t.Errorf("WithLock: %v", err)
			}
		})
	}

	wg.Wait()

	// With proper locking, counter should be exactly 5
	if atomic.LoadInt32(&counter) != 5 {
		t.Errorf("counter = %d, want 5", counter)
	}
}

func TestFileLock_CreatesDirectory(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "subdir", "nested", "test.lock")

	lock := NewFileLock(lockPath)

	// Should create parent directories
	if err := lock.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	defer func() { _ = lock.Unlock() }()

	// Directory should exist
	dir := filepath.Dir(lockPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("lock directory should be created")
	}
}

func TestFileLock_DoubleUnlock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "double.lock")

	lock := NewFileLock(lockPath)

	if err := lock.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// First unlock
	if err := lock.Unlock(); err != nil {
		t.Fatalf("first Unlock: %v", err)
	}

	// Second unlock should be no-op (not error)
	if err := lock.Unlock(); err != nil {
		t.Errorf("second Unlock should be no-op, got: %v", err)
	}
}

func TestWithLockTimeout(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "withlocktimeout.lock")

	// Test successful lock acquisition and function execution
	executed := false
	err := WithLockTimeout(lockPath, 1*time.Second, func() error {
		executed = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithLockTimeout: %v", err)
	}
	if !executed {
		t.Error("function should have been executed")
	}
}

func TestWithLockTimeout_Timeout(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "withlocktimeout2.lock")

	// Hold the lock in another goroutine
	lock := NewFileLock(lockPath)
	if err := lock.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Try to acquire with short timeout - should fail
	start := time.Now()
	err := WithLockTimeout(lockPath, 100*time.Millisecond, func() error {
		t.Error("function should not be executed when lock times out")
		return nil
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Error("WithLockTimeout should fail when lock cannot be acquired")
	}

	// Should have waited approximately the timeout duration
	if elapsed < 90*time.Millisecond {
		t.Errorf("waited only %v, expected ~100ms", elapsed)
	}

	_ = lock.Unlock()
}

func TestWithLockTimeout_Concurrent(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "concurrent.lock")

	var counter int32
	var wg sync.WaitGroup

	// Run multiple goroutines that increment counter with lock timeout
	for i := 0; i < 3; i++ {
		wg.Go(func() {
			err := WithLockTimeout(lockPath, 5*time.Second, func() error {
				val := atomic.LoadInt32(&counter)
				time.Sleep(10 * time.Millisecond)
				atomic.StoreInt32(&counter, val+1)
				return nil
			})
			if err != nil {
				t.Errorf("WithLockTimeout: %v", err)
			}
		})
	}

	wg.Wait()

	// With proper locking, counter should be exactly 3
	if atomic.LoadInt32(&counter) != 3 {
		t.Errorf("counter = %d, want 3", counter)
	}
}
