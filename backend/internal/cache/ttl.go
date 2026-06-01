// Package cache provides a tiny concurrency-safe in-memory TTL cache.
//
// Used for Apple Music catalog search results (docs/requirements.md F4: "对检索
// 结果做缓存，缓存 key 需含 storefront"). It deliberately holds NO user LLM keys
// and nothing secret — only public catalog data.
package cache

import (
	"sync"
	"time"
)

type entry[T any] struct {
	val T
	exp time.Time
}

// TTL is a map-backed cache with a single expiry duration for all entries.
// A non-positive ttl disables caching entirely (Set is a no-op, Get always misses).
type TTL[T any] struct {
	ttl time.Duration
	mu  sync.Mutex
	m   map[string]entry[T]
	now func() time.Time // seam for tests
}

func NewTTL[T any](ttl time.Duration) *TTL[T] {
	return &TTL[T]{ttl: ttl, m: make(map[string]entry[T]), now: time.Now}
}

// Get returns the cached value and true if present and unexpired.
func (c *TTL[T]) Get(key string) (T, bool) {
	var zero T
	if c == nil || c.ttl <= 0 {
		return zero, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		return zero, false
	}
	if c.now().After(e.exp) {
		delete(c.m, key)
		return zero, false
	}
	return e.val, true
}

// Set stores a value with the cache's TTL. No-op when caching is disabled.
func (c *TTL[T]) Set(key string, val T) {
	if c == nil || c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = entry[T]{val: val, exp: c.now().Add(c.ttl)}
}
