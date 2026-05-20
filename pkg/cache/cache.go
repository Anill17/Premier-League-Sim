// Package cache wraps go-cache with a minimal API used by handlers.
// It is intentionally untyped so pkg/ stays free of internal/ imports.
package cache

import (
	"fmt"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

const (
	// TTL is the safety-net expiry. Explicit eviction on every write is
	// the primary invalidation path; TTL is a fallback.
	TTL             = 5 * time.Minute
	cleanupInterval = 10 * time.Minute
)

// Cache is a simple in-memory store backed by go-cache.
type Cache struct {
	c *gocache.Cache
}

// New returns a ready-to-use Cache.
func New() *Cache {
	return &Cache{c: gocache.New(TTL, cleanupInterval)}
}

// Set stores value under key with the default TTL.
func (c *Cache) Set(key string, value any) {
	c.c.SetDefault(key, value)
}

// Get returns the value stored under key, or (nil, false) on a miss.
func (c *Cache) Get(key string) (any, bool) {
	return c.c.Get(key)
}

// Evict removes the entry for key. Safe to call on a key that does not exist.
func (c *Cache) Evict(key string) {
	c.c.Delete(key)
}

// StandingsKey returns the canonical cache key for a season's standings.
func StandingsKey(seasonID int) string {
	return fmt.Sprintf("standings:%d", seasonID)
}
