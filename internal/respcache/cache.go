// Package respcache provides semantic response caching for agent prompts.
//
// When identical prompts are sent to agents for the same codebase, responses
// are regenerated from scratch. This cache stores prompt-response pairs keyed
// by SHA-256 hash, enabling exact-match deduplication with TTL-based expiration.
package respcache

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Entry stores a cached prompt-response pair.
type Entry struct {
	ID         string       `json:"id"`
	PromptHash string       `json:"prompt_hash"`
	Prompt     string       `json:"prompt"`
	Response   string       `json:"response"`
	Phase      string       `json:"phase,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	HitCount   atomic.Int64 `json:"hit_count"`
}

// Stats tracks cache performance.
type Stats struct {
	Entries     int     `json:"entries"`
	Hits        int     `json:"hits"`
	Misses      int     `json:"misses"`
	HitRate     float64 `json:"hit_rate"`
	TokensSaved int64   `json:"tokens_saved"`
}

// Cache provides semantic response caching for agent prompts.
// It uses exact SHA-256 hash matching for reliability and TTL-based
// expiration to prevent stale entries from being served.
type Cache struct {
	mu         sync.RWMutex
	entries    map[string]*Entry // keyed by prompt hash
	maxEntries int
	ttl        time.Duration

	// Atomic counters allow Get to use RLock for the map lookup and
	// update stats without upgrading to a full write lock.
	hits        atomic.Int64
	misses      atomic.Int64
	tokensSaved atomic.Int64
}

// New creates a Cache with the given configuration.
// maxEntries limits the number of cached entries (oldest evicted first).
// ttl controls how long entries remain valid.
func New(maxEntries int, ttl time.Duration) *Cache {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	if ttl <= 0 {
		ttl = 168 * time.Hour // 7 days default
	}

	return &Cache{
		entries:    make(map[string]*Entry),
		maxEntries: maxEntries,
		ttl:        ttl,
	}
}

// Get checks for a cached response matching the given prompt.
// Returns (response, true) on a cache hit, ("", false) on a miss.
// Expired entries are lazily removed on the next Put or evict cycle.
func (c *Cache) Get(prompt string) (string, bool) {
	hash := hashPrompt(prompt)

	c.mu.RLock()
	entry, ok := c.entries[hash]
	if !ok {
		c.mu.RUnlock()
		c.misses.Add(1)

		return "", false
	}

	// Check TTL expiration.
	if time.Since(entry.CreatedAt) > c.ttl {
		c.mu.RUnlock()
		// Remove expired entry under write lock.
		c.mu.Lock()
		delete(c.entries, hash)
		c.mu.Unlock()
		c.misses.Add(1)
		slog.Debug("cache entry expired", "hash", hash[:12], "age", time.Since(entry.CreatedAt))

		return "", false
	}

	entry.HitCount.Add(1)
	response := entry.Response
	phase := entry.Phase
	c.mu.RUnlock()

	c.hits.Add(1)
	// Estimate tokens saved (~4 chars per token).
	c.tokensSaved.Add(int64(len(response)) / 4)

	slog.Debug("cache hit", "hash", hash[:12], "phase", phase, "hit_count", entry.HitCount.Load())

	return response, true
}

// Put stores a prompt-response pair in the cache.
// If the cache is at capacity, the oldest entry is evicted first.
func (c *Cache) Put(prompt, response, phase string) {
	hash := hashPrompt(prompt)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing entry if same hash.
	if existing, ok := c.entries[hash]; ok {
		existing.Response = response
		existing.Phase = phase
		existing.CreatedAt = time.Now()

		return
	}

	// Evict oldest if at capacity.
	if len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	c.entries[hash] = &Entry{
		ID:         uuid.New().String(),
		PromptHash: hash,
		Prompt:     prompt,
		Response:   response,
		Phase:      phase,
		CreatedAt:  time.Now(),
	}

	slog.Debug("cache entry stored", "hash", hash[:12], "phase", phase)
}

// Stats returns cache performance statistics.
func (c *Cache) Stats() Stats {
	hits := c.hits.Load()
	misses := c.misses.Load()
	tokensSaved := c.tokensSaved.Load()

	total := hits + misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	c.mu.RLock()
	entries := len(c.entries)
	c.mu.RUnlock()

	return Stats{
		Entries:     entries,
		Hits:        int(hits),
		Misses:      int(misses),
		HitRate:     hitRate,
		TokensSaved: tokensSaved,
	}
}

// Clear removes all cached entries and resets statistics.
func (c *Cache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*Entry)
	c.mu.Unlock()

	c.hits.Store(0)
	c.misses.Store(0)
	c.tokensSaved.Store(0)

	slog.Debug("cache cleared")
}

// Len returns the number of entries currently in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// evictOldest removes the entry with the earliest CreatedAt timestamp.
// Must be called with c.mu held.
func (c *Cache) evictOldest() {
	var oldestHash string
	var oldestTime time.Time

	for hash, entry := range c.entries {
		if oldestHash == "" || entry.CreatedAt.Before(oldestTime) {
			oldestHash = hash
			oldestTime = entry.CreatedAt
		}
	}

	if oldestHash != "" {
		delete(c.entries, oldestHash)
		slog.Debug("cache entry evicted", "hash", oldestHash[:12])
	}
}

// hashPrompt returns the hex-encoded SHA-256 hash of a prompt string.
func hashPrompt(prompt string) string {
	h := sha256.Sum256([]byte(prompt))

	return hex.EncodeToString(h[:])
}

// FormatHitSuffix returns a display suffix indicating a cache hit.
// Used by TUI and CLI to annotate cached responses.
func FormatHitSuffix() string {
	return " [cached]"
}
