package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBucketAllowUpToBurst(t *testing.T) {
	t.Parallel()

	b := NewBucket(10, 5)

	for i := range 5 {
		if !b.Allow() {
			t.Fatalf("expected Allow() to succeed on call %d", i+1)
		}
	}

	if b.Allow() {
		t.Fatal("expected Allow() to fail after exhausting burst")
	}
}

func TestBucketRefillsOverTime(t *testing.T) {
	t.Parallel()

	now := time.Now()
	b := NewBucket(10, 5)
	b.nowFunc = func() time.Time { return now }

	// Drain the bucket.
	for range 5 {
		b.Allow()
	}

	if b.Allow() {
		t.Fatal("bucket should be empty")
	}

	// Advance time by 300ms -> 3 tokens refilled (10 tokens/sec * 0.3s).
	now = now.Add(300 * time.Millisecond)

	for i := range 3 {
		if !b.Allow() {
			t.Fatalf("expected Allow() to succeed on refilled token %d", i+1)
		}
	}

	if b.Allow() {
		t.Fatal("expected Allow() to fail after consuming refilled tokens")
	}
}

func TestBucketDeniesWhenEmpty(t *testing.T) {
	t.Parallel()

	now := time.Now()
	b := NewBucket(1, 1)
	b.nowFunc = func() time.Time { return now }
	b.lastTime = now

	if !b.Allow() {
		t.Fatal("first Allow() should succeed")
	}

	// No time passes, bucket should be empty.
	if b.Allow() {
		t.Fatal("expected Allow() to fail when empty and no time has passed")
	}
}

func TestBucketAllowN(t *testing.T) {
	t.Parallel()

	b := NewBucket(10, 10)

	if !b.AllowN(7) {
		t.Fatal("AllowN(7) should succeed with 10 tokens")
	}

	if b.AllowN(5) {
		t.Fatal("AllowN(5) should fail with only 3 tokens remaining")
	}

	if !b.AllowN(3) {
		t.Fatal("AllowN(3) should succeed with 3 tokens remaining")
	}
}

func TestLimiterSeparateBucketsPerKey(t *testing.T) {
	t.Parallel()

	l := NewLimiter(10, 2)

	// Drain key "a".
	l.Allow("a")
	l.Allow("a")

	if l.Allow("a") {
		t.Fatal("key 'a' should be exhausted")
	}

	// Key "b" should be independent.
	if !l.Allow("b") {
		t.Fatal("key 'b' should have its own full bucket")
	}
}

func TestLimiterCleanupRemovesStaleBuckets(t *testing.T) {
	t.Parallel()

	now := time.Now()
	l := NewLimiter(10, 5)
	l.nowFunc = func() time.Time { return now }

	l.Allow("keep")
	l.Allow("stale")

	// Advance past stale threshold.
	now = now.Add(11 * time.Minute)

	// Touch "keep" so it stays fresh.
	l.Allow("keep")

	l.removeStale()

	// "stale" should be removed.
	_, staleExists := l.buckets.Load("stale")
	if staleExists {
		t.Fatal("stale bucket should have been removed")
	}

	// "keep" should still exist.
	_, keepExists := l.buckets.Load("keep")
	if !keepExists {
		t.Fatal("recently used bucket should not be removed")
	}
}

func TestLimiterStartStop(t *testing.T) {
	t.Parallel()

	l := NewLimiter(10, 5)
	l.Start(t.Context())

	// Verify the limiter works while running.
	if !l.Allow("test") {
		t.Fatal("Allow should succeed")
	}

	l.Stop()

	// Stop is safe to call multiple times.
	l.Stop()
}

func TestLimiterRemove(t *testing.T) {
	t.Parallel()

	l := NewLimiter(10, 1)

	l.Allow("conn1")

	if l.Allow("conn1") {
		t.Fatal("bucket should be exhausted")
	}

	// Remove and re-allow should get a fresh bucket.
	l.Remove("conn1")

	if !l.Allow("conn1") {
		t.Fatal("Allow should succeed after Remove (fresh bucket)")
	}
}

func TestLimiterConcurrentAccess(t *testing.T) {
	t.Parallel()

	l := NewLimiter(1000, 100)
	frozen := time.Now()
	l.nowFunc = func() time.Time { return frozen }

	var wg sync.WaitGroup
	var allowed atomic.Int64

	for range 10 {
		wg.Go(func() {
			for range 50 {
				if l.Allow("shared") {
					allowed.Add(1)
				}
			}
		})
	}

	wg.Wait()

	// With burst of 100 and 500 total attempts, at most 100 should succeed
	// (no time passes to refill since all goroutines run nearly simultaneously).
	total := allowed.Load()
	if total > 100 {
		t.Fatalf("expected at most 100 allowed, got %d", total)
	}

	if total < 1 {
		t.Fatal("expected at least 1 allowed request")
	}
}
