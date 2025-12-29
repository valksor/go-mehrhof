// Package cache provides a simple in-memory cache with TTL support.
package cache

import (
	"sync"
	"time"
)

// Default TTL values for different resource types
const (
	DefaultIssueTTL    = 5 * time.Minute
	DefaultCommentsTTL = 1 * time.Minute
	DefaultMetadataTTL = 30 * time.Minute
	DefaultDatabaseTTL = 1 * time.Hour
	DefaultPluginTTL   = 10 * time.Minute
)

// entry represents a cached item with expiration
type entry struct {
	data      any
	expiresAt time.Time
}

// isExpired returns true if the entry has expired
func (e *entry) isExpired() bool {
	return time.Now().After(e.expiresAt)
}

// Cache is a thread-safe in-memory cache with TTL support
type Cache struct {
	mu    sync.RWMutex
	store map[string]*entry

	// Enabled allows the cache to be disabled at runtime
	enabled bool
}

// New creates a new Cache instance
func New() *Cache {
	return &Cache{
		store:   make(map[string]*entry),
		enabled: true,
	}
}

// Enable enables the cache
func (c *Cache) Enable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = true
}

// Disable disables the cache (all Get operations will return cache miss)
func (c *Cache) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
}

// Enabled returns true if caching is enabled
func (c *Cache) Enabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

// Get retrieves a value from the cache by key.
// Returns the value and true if found and not expired, nil and false otherwise.
// The caller should type-assert the result to the expected type.
func (c *Cache) Get(key string) (any, bool) {
	if !c.Enabled() {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.store[key]
	if !ok {
		return nil, false
	}

	if e.isExpired() {
		// Clean up expired entry
		delete(c.store, key)
		return nil, false
	}

	return e.data, true
}

// Set stores a value in the cache with the given TTL
func (c *Cache) Set(key string, data any, ttl time.Duration) {
	if !c.Enabled() {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[key] = &entry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a value from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
}

// Clear removes all entries from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string]*entry)
}

// Size returns the number of entries in the cache
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
}

// Cleanup removes all expired entries from the cache
func (c *Cache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, e := range c.store {
		if e.isExpired() {
			delete(c.store, key)
		}
	}
}

// StartCleanupScheduler runs periodic cleanup of expired entries
// in a separate goroutine. The returned channel can be used to stop
// the scheduler by closing it.
func (c *Cache) StartCleanupScheduler(interval time.Duration) chan struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.Cleanup()
			case <-stop:
				return
			}
		}
	}()

	return stop
}
