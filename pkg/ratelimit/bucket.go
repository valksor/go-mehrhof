package ratelimit

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Bucket implements a token bucket rate limiter. Tokens are added at a constant
// rate up to a maximum burst capacity. Each Allow call consumes one token.
type Bucket struct {
	rate     float64   // tokens per second
	burst    float64   // max tokens (bucket capacity)
	tokens   float64   // current tokens
	lastTime time.Time // last refill time
	mu       sync.Mutex
	nowFunc  func() time.Time // override for testing
}

// NewBucket creates a token bucket that refills at rate tokens per second
// with a maximum capacity of burst tokens. The bucket starts full.
func NewBucket(rate float64, burst int) *Bucket {
	return &Bucket{
		rate:     rate,
		burst:    float64(burst),
		tokens:   float64(burst),
		lastTime: time.Now(),
		nowFunc:  time.Now,
	}
}

// Allow consumes one token and reports whether the request is allowed.
func (b *Bucket) Allow() bool {
	return b.AllowN(1)
}

// AllowN consumes n tokens and reports whether the request is allowed.
// If there are not enough tokens, no tokens are consumed and false is returned.
func (b *Bucket) AllowN(n int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.nowFunc()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens = min(b.tokens+elapsed*b.rate, b.burst)
	b.lastTime = now

	need := float64(n)
	if b.tokens < need {
		return false
	}

	b.tokens -= need

	return true
}

// bucketEntry wraps a Bucket with a last-used timestamp for cleanup.
// lastUsed is stored as atomic UnixNano to avoid data races between
// Allow (writer) and removeStale (reader).
type bucketEntry struct {
	bucket   *Bucket
	lastUsed atomic.Int64 // UnixNano
}

// Limiter manages per-key token buckets. Each unique key gets its own Bucket
// with shared rate and burst parameters.
type Limiter struct {
	rate      float64
	burst     int
	buckets   sync.Map // key -> *bucketEntry
	nowFunc   func() time.Time
	cancel    context.CancelFunc
	startOnce sync.Once
}

// NewLimiter creates a Limiter that assigns each key a token bucket with
// the given rate (tokens/sec) and burst capacity.
func NewLimiter(rate float64, burst int) *Limiter {
	return &Limiter{
		rate:    rate,
		burst:   burst,
		nowFunc: time.Now,
	}
}

// Allow consumes one token from the bucket for key and reports whether
// the request is allowed.
func (l *Limiter) Allow(key string) bool {
	now := l.nowFunc()

	val, loaded := l.buckets.Load(key)
	if loaded {
		entry := val.(*bucketEntry)
		entry.lastUsed.Store(now.UnixNano())

		return entry.bucket.Allow()
	}

	b := NewBucket(l.rate, l.burst)
	b.nowFunc = l.nowFunc
	entry := &bucketEntry{bucket: b}
	entry.lastUsed.Store(now.UnixNano())

	actual, loaded := l.buckets.LoadOrStore(key, entry)
	if loaded {
		existing := actual.(*bucketEntry)
		existing.lastUsed.Store(now.UnixNano())

		return existing.bucket.Allow()
	}

	return entry.bucket.Allow()
}

// Remove deletes the bucket for a given key.
func (l *Limiter) Remove(key string) {
	l.buckets.Delete(key)
}

// Start launches a background goroutine that periodically removes buckets
// that have not been used in the last 10 minutes. The goroutine stops when
// ctx is cancelled or Stop is called.
func (l *Limiter) Start(ctx context.Context) {
	l.startOnce.Do(func() {
		ctx, l.cancel = context.WithCancel(ctx)

		go l.cleanup(ctx)
	})
}

// Stop cancels the cleanup goroutine. Safe to call multiple times or if
// Start was never called.
func (l *Limiter) Stop() {
	if l.cancel != nil {
		l.cancel()
	}
}

const (
	cleanupInterval = 5 * time.Minute
	staleThreshold  = 10 * time.Minute
)

func (l *Limiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.removeStale()
		}
	}
}

func (l *Limiter) removeStale() {
	now := l.nowFunc()
	var removed int

	l.buckets.Range(func(key, value any) bool {
		entry := value.(*bucketEntry)
		lastUsed := time.Unix(0, entry.lastUsed.Load())
		if now.Sub(lastUsed) > staleThreshold {
			l.buckets.Delete(key)
			removed++
		}

		return true
	})

	if removed > 0 {
		slog.Debug("rate limiter cleanup", "removed", removed)
	}
}
