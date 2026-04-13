package conductor

import (
	"log/slog"
	"time"

	"github.com/valksor/kvelmo/internal/respcache"
	"github.com/valksor/kvelmo/settings"
)

// initResponseCache creates the response cache based on settings.
// Called during conductor construction.
func (c *Conductor) initResponseCache(s *settings.Settings) {
	if s == nil || s.Agent.ResponseCache == nil || !s.Agent.ResponseCache.Enabled {
		return
	}

	rc := s.Agent.ResponseCache

	maxEntries := rc.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 1000
	}

	ttlHours := rc.TTLHours
	if ttlHours <= 0 {
		ttlHours = 168
	}

	c.responseCache = respcache.New(maxEntries, time.Duration(ttlHours)*time.Hour)
	slog.Info("response cache enabled", "max_entries", maxEntries, "ttl_hours", ttlHours)
}

// ResponseCache returns the response cache, or nil if caching is disabled.
func (c *Conductor) ResponseCache() *respcache.Cache {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.responseCache
}

// ResponseCacheStats returns cache statistics, or nil if caching is disabled.
func (c *Conductor) ResponseCacheStats() *respcache.Stats {
	c.mu.RLock()
	cache := c.responseCache
	c.mu.RUnlock()

	if cache == nil {
		return nil
	}

	stats := cache.Stats()

	return &stats
}

// ClearResponseCache clears all cached entries if caching is enabled.
func (c *Conductor) ClearResponseCache() {
	c.mu.RLock()
	cache := c.responseCache
	c.mu.RUnlock()

	if cache != nil {
		cache.Clear()
		slog.Info("response cache cleared")
	}
}

// lookupResponseCache checks the cache for a matching prompt.
// Returns (response, true) on cache hit, ("", false) on miss or if caching is disabled.
func (c *Conductor) lookupResponseCache(prompt string) (string, bool) {
	if c.responseCache == nil {
		return "", false
	}

	response, ok := c.responseCache.Get(prompt)
	if ok {
		slog.Debug("response cache hit, skipping agent call")
	}

	return response, ok
}

// storeResponseCache stores a prompt-response pair in the cache.
// No-op if caching is disabled.
func (c *Conductor) storeResponseCache(prompt, response, phase string) {
	if c.responseCache == nil {
		return
	}

	c.responseCache.Put(prompt, response, phase)
}
