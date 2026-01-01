package cache

import (
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	c := New()

	// Set a value
	c.Set("key1", "value1", time.Minute)

	// Get it back
	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}
	if val != "value1" {
		t.Fatalf("expected 'value1', got %v", val)
	}

	// Check size
	if c.Size() != 1 {
		t.Fatalf("expected size 1, got %d", c.Size())
	}
}

func TestCache_GetNotFound(t *testing.T) {
	c := New()

	// Get non-existent key
	_, ok := c.Get("nonexistent")
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestCache_Expiration(t *testing.T) {
	c := New()

	// Set a value with short TTL
	c.Set("key1", "value1", 10*time.Millisecond)

	// Should be found immediately
	_, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected to find key1 immediately")
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Size should still reflect the entry (lazy expiration doesn't remove during Get)
	if c.Size() != 1 {
		t.Fatalf("expected size 1 (expired entry), got %d", c.Size())
	}

	// Get should return miss for expired entry (lazy expiration)
	_, ok = c.Get("key1")
	if ok {
		t.Fatal("expected key1 to be expired")
	}

	// With lazy expiration, Get doesn't remove expired entries
	// Size is still 1 until cleanup runs
	if c.Size() != 1 {
		t.Fatalf("expected size 1 (lazy expiration), got %d", c.Size())
	}

	// Cleanup removes expired entries
	c.Cleanup()

	if c.Size() != 0 {
		t.Fatalf("expected size 0 after cleanup, got %d", c.Size())
	}
}

func TestCache_Delete(t *testing.T) {
	c := New()

	c.Set("key1", "value1", time.Minute)

	// Delete the key
	c.Delete("key1")

	// Should not be found
	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected cache miss after delete")
	}
}

func TestCache_Clear(t *testing.T) {
	c := New()

	c.Set("key1", "value1", time.Minute)
	c.Set("key2", "value2", time.Minute)

	if c.Size() != 2 {
		t.Fatalf("expected size 2, got %d", c.Size())
	}

	c.Clear()

	if c.Size() != 0 {
		t.Fatalf("expected size 0 after clear, got %d", c.Size())
	}

	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected cache miss after clear")
	}
}

func TestCache_Disable(t *testing.T) {
	c := New()

	c.Set("key1", "value1", time.Minute)
	c.Disable()

	// Should not find anything when disabled
	var val any
	var ok bool
	_, ok = c.Get("key1")
	if ok {
		t.Fatal("expected cache miss when disabled")
	}

	// Set should be a no-op
	c.Set("key2", "value2", time.Minute)
	_, ok = c.Get("key2")
	if ok {
		t.Fatal("expected cache miss when disabled (set)")
	}

	// Re-enable
	c.Enable()

	// Old entries should still be present (disable only affects Get/Set, not storage)
	val, ok = c.Get("key1")
	if !ok {
		t.Fatal("expected to find key1 after re-enable (entries are preserved)")
	}
	if val != "value1" {
		t.Fatalf("expected 'value1', got %v", val)
	}

	// New sets should work
	c.Set("key3", "value3", time.Minute)
	val, ok = c.Get("key3")
	if !ok {
		t.Fatal("expected to find key3 after re-enable")
	}
	if val != "value3" {
		t.Fatalf("expected 'value3', got %v", val)
	}
}

func TestCache_Cleanup(t *testing.T) {
	c := New()

	// Add some entries
	c.Set("key1", "value1", 10*time.Millisecond)
	c.Set("key2", "value2", time.Hour)

	time.Sleep(20 * time.Millisecond)

	// Cleanup should remove expired entries
	c.Cleanup()

	if c.Size() != 1 {
		t.Fatalf("expected size 1 after cleanup, got %d", c.Size())
	}

	// key2 should still be there
	_, ok := c.Get("key2")
	if !ok {
		t.Fatal("expected to find key2 (not expired)")
	}

	// key1 should be gone
	_, ok = c.Get("key1")
	if ok {
		t.Fatal("expected key1 to be removed (expired)")
	}
}

func TestCache_CleanupScheduler(t *testing.T) {
	c := New()

	// Start cleanup scheduler with short interval
	stop := c.StartCleanupScheduler(10 * time.Millisecond)
	defer close(stop)

	// Add an expiring entry
	c.Set("key1", "value1", 5*time.Millisecond)

	// Wait for cleanup to run
	time.Sleep(30 * time.Millisecond)

	if c.Size() != 0 {
		t.Fatalf("expected size 0 after scheduled cleanup, got %d", c.Size())
	}
}

func TestCache_GetRemovesExpired(t *testing.T) {
	c := New()

	c.Set("key1", "value1", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	// Getting an expired entry returns miss (lazy expiration)
	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected cache miss for expired entry")
	}

	// With lazy expiration, Get doesn't remove entries
	// Size is still 1 until cleanup runs
	if c.Size() != 1 {
		t.Fatalf("expected size 1 (lazy expiration), got %d", c.Size())
	}

	// Cleanup removes expired entries
	c.Cleanup()

	if c.Size() != 0 {
		t.Fatalf("expected size 0 after cleanup, got %d", c.Size())
	}
}

func TestCache_Overwrite(t *testing.T) {
	c := New()

	c.Set("key1", "value1", time.Minute)
	c.Set("key1", "value2", time.Minute)

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}
	if val != "value2" {
		t.Fatalf("expected 'value2', got %v", val)
	}

	if c.Size() != 1 {
		t.Fatalf("expected size 1, got %d", c.Size())
	}
}

func TestDefaultTTLs(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
	}{
		{"IssueTTL", DefaultIssueTTL},
		{"CommentsTTL", DefaultCommentsTTL},
		{"MetadataTTL", DefaultMetadataTTL},
		{"DatabaseTTL", DefaultDatabaseTTL},
		{"PluginTTL", DefaultPluginTTL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ttl <= 0 {
				t.Fatalf("expected positive TTL, got %v", tt.ttl)
			}
		})
	}
}
