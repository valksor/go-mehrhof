// Package cache tests
package cache

import (
	"sync"
	"testing"
	"time"
)

// Benchmark_Cache_Get_NoExpiration benchmarks cache hits without expiration.
func Benchmark_Cache_Get_NoExpiration(b *testing.B) {
	c := New()
	c.Set("key", "value", 1*time.Hour)

	b.ReportAllocs()
	for range b.N {
		_, _ = c.Get("key")
	}
}

// Benchmark_Cache_Get_WithExpiration benchmarks cache hits that check expiration.
func Benchmark_Cache_Get_WithExpiration(b *testing.B) {
	c := New()
	c.Set("key", "value", 100*time.Millisecond)

	b.ReportAllocs()
	for range b.N {
		_, _ = c.Get("key")
	}
}

// Benchmark_Cache_Get_Miss benchmarks cache misses.
func Benchmark_Cache_Get_Miss(b *testing.B) {
	c := New()
	c.Set("key", "value", 1*time.Hour)

	b.ReportAllocs()
	for range b.N {
		_, _ = c.Get("notfound")
	}
}

// Benchmark_Cache_Get_Expired benchmarks reads that encounter expired entries.
// With lazy expiration, this should be fast (no lock promotion).
func Benchmark_Cache_Get_Expired(b *testing.B) {
	c := New()
	// Set entries that will expire
	for i := range 100 {
		c.Set(string(rune('a'+i)), "value", 1*time.Millisecond)
	}
	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	b.ReportAllocs()
	for range b.N {
		// These will all be expired but lazy expiration avoids write locks
		_, _ = c.Get("a")
	}
}

// Benchmark_Cache_Set benchmarks setting cache entries.
func Benchmark_Cache_Set(b *testing.B) {
	c := New()
	b.ReportAllocs()

	for range b.N {
		c.Set("key", "value", 1*time.Hour)
	}
}

// Benchmark_Cache_GetSet_Concurrent benchmarks concurrent reads and writes.
func Benchmark_Cache_GetSet_Concurrent(b *testing.B) {
	c := New()
	// Pre-populate some entries
	for i := range 100 {
		c.Set(string(rune(i)), "value", 1*time.Hour)
	}

	b.RunParallel(func(pb *testing.PB) {
		even := true
		for pb.Next() {
			// Mix of reads and writes
			even = !even
			if even {
				_, _ = c.Get("key50")
			} else {
				c.Set("key", "value", 1*time.Hour)
			}
		}
	})
}

// Benchmark_Cache_Get_ConcurrentContention benchmarks concurrent reads on
// the same key to test lock contention.
func Benchmark_Cache_Get_ConcurrentContention(b *testing.B) {
	c := New()
	c.Set("hotkey", "value", 1*time.Hour)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = c.Get("hotkey")
		}
	})
}

// Benchmark_Cache_ExpiredWithContention benchmarks concurrent reads
// that encounter expired entries. This tests the lazy expiration optimization.
func Benchmark_Cache_ExpiredWithContention(b *testing.B) {
	c := New()
	// Create entries that will be expired
	for i := range 100 {
		c.Set(string(rune('a'+i)), "value", -1*time.Hour) // Already expired
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Lazy expiration: these should NOT cause write locks
			_, _ = c.Get("a")
		}
	})
}

// Benchmark_Cache_Cleanup benchmarks the cleanup operation.
func Benchmark_Cache_Cleanup(b *testing.B) {
	b.StopTimer()
	c := New()
	// Add many entries, half expired
	for i := range 1000 {
		ttl := 1 * time.Hour
		if i%2 == 0 {
			ttl = -1 * time.Hour // Expired
		}
		c.Set(string(rune(i)), "value", ttl)
	}
	b.StartTimer()

	b.ReportAllocs()
	for range b.N {
		c.Cleanup()
	}
}

// Benchmark_Cache_New benchmarks cache creation.
func Benchmark_Cache_New(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = New()
	}
}

// Benchmark_Cache_EnableDisable benchmarks enable/disable operations.
func Benchmark_Cache_EnableDisable(b *testing.B) {
	c := New()
	b.ReportAllocs()

	for range b.N {
		c.Enable()
		c.Disable()
	}
}

// Benchmark_Cache_Size benchmarks getting cache size.
func Benchmark_Cache_Size(b *testing.B) {
	c := New()
	for i := range 100 {
		c.Set(string(rune(i)), "value", 1*time.Hour)
	}

	b.ReportAllocs()
	for range b.N {
		_ = c.Size()
	}
}

// Benchmark_Cache_Delete benchmarks delete operations.
func Benchmark_Cache_Delete(b *testing.B) {
	b.StopTimer()
	c := New()
	for i := range 100 {
		c.Set(string(rune(i)), "value", 1*time.Hour)
	}
	b.StartTimer()

	b.ReportAllocs()
	for range b.N {
		c.Delete("key50")
		c.Set("key50", "value", 1*time.Hour) // Re-add for next iteration
	}
}

// Benchmark_Cache_Clear benchmarks clearing the entire cache.
func Benchmark_Cache_Clear(b *testing.B) {
	b.StopTimer()
	c := New()
	b.StartTimer()
	b.ReportAllocs()

	for range b.N {
		// Populate
		for j := range 100 {
			c.Set(string(rune(j)), "value", 1*time.Hour)
		}
		c.Clear()
	}
}

// Benchmark_Cache_MixedWorkload simulates a realistic cache workload.
func Benchmark_Cache_MixedWorkload(b *testing.B) {
	c := New()
	// Seed with initial data
	keys := make([]string, 100)
	for i := range 100 {
		keys[i] = string(rune('a' + i))
		c.Set(keys[i], "value", 1*time.Hour)
	}

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	for range 4 { // 4 goroutines
		wg.Go(func() {
			for i := range b.N / 4 {
				switch i % 5 {
				case 0, 1:
					// 40% reads (hot keys)
					_, _ = c.Get(keys[i%10])
				case 2:
					// 20% reads (cold keys)
					_, _ = c.Get("notfound")
				case 3:
					// 20% writes
					c.Set(keys[i%100], "value", 1*time.Hour)
				case 4:
					// 20% size checks
					_ = c.Size()
				}
			}
		})
	}
	wg.Wait()
}
