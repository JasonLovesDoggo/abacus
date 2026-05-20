package utils

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// GetCache is a tiny in-process micro-cache in front of GET-style reads.
// Sized for the /get/:namespace/:key hot path: 705-2800 active keys serving
// millions of reads. A 250ms TTL cuts Redis GET traffic by ~95-99% while
// keeping staleness invisible for counter semantics (values are
// monotonically increasing under normal use; readers only ever see an
// older-than-latest value, never a wrong one).
//
// Multi-instance safe by construction: each Fly machine has its own cache,
// no coordination. Cross-instance staleness is bounded by TTL.
//
// singleflight collapses concurrent fills for the same key into ONE Redis
// call. Without it, when a cached entry expires under high RPS, every
// concurrent reader would race to refill — a "cache stampede". One Redis
// call, N waiters that all get the same answer.
type GetCache struct {
	mu      sync.RWMutex
	entries map[string]getCacheEntry
	sf      singleflight.Group
	ttl     time.Duration
	maxSize int
	enabled bool

	size    atomic.Int64 // tracked alongside the map so Size() is O(1)
	Hits    atomic.Uint64
	Misses  atomic.Uint64
	Evicted atomic.Uint64

	stop context.CancelFunc
}

type getCacheEntry struct {
	val      string
	notFound bool // true = cached redis.Nil (key doesn't exist)
	expiry   time.Time
}

// getFillResult is what the singleflight Do() returns. Typed so we don't
// shuffle [2]interface{} around.
type getFillResult struct {
	val      string
	notFound bool
}

// NewGetCache returns a cache with the given TTL and maximum entry count.
// A background sweeper evicts entries when the cache exceeds maxSize.
// Disable with ttl<=0 — Fetch will then call fill directly without caching.
func NewGetCache(ttl time.Duration, maxSize int) *GetCache {
	ctx, cancel := context.WithCancel(context.Background())
	c := &GetCache{
		entries: make(map[string]getCacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
		enabled: ttl > 0,
		stop:    cancel,
	}
	if c.enabled {
		go c.sweepLoop(ctx)
	}
	return c
}

// Stop terminates the background sweeper. Safe to call multiple times.
func (c *GetCache) Stop() {
	if c == nil || c.stop == nil {
		return
	}
	c.stop()
}

// Size returns the current number of cached entries (O(1) via atomic).
func (c *GetCache) Size() int {
	return int(c.size.Load())
}

// Enabled reports whether the cache is doing any work. False when TTL<=0.
func (c *GetCache) Enabled() bool { return c != nil && c.enabled }

// Fetch returns a cached entry for key, or runs fill exactly once for
// concurrent callers (singleflight). fill returns (value, notFound, error).
// notFound=true represents a cached "key doesn't exist" (redis.Nil) result.
// Errors are NOT cached — only successful reads and explicit not-found.
func (c *GetCache) Fetch(key string, fill func() (string, bool, error)) (string, bool, error) {
	if !c.enabled {
		return fill()
	}

	if v, nf, hit := c.lookup(key); hit {
		c.Hits.Add(1)
		return v, nf, nil
	}

	// Coalesce concurrent fills for the same key into ONE call.
	res, err, _ := c.sf.Do(key, func() (any, error) {
		// Re-check under singleflight: another caller may have filled
		// between our lookup miss and entry into Do.
		if v, nf, hit := c.lookup(key); hit {
			c.Hits.Add(1)
			return getFillResult{v, nf}, nil
		}
		c.Misses.Add(1)
		v, nf, e := fill()
		if e != nil {
			return getFillResult{}, e
		}
		c.store(key, v, nf)
		return getFillResult{v, nf}, nil
	})
	if err != nil {
		return "", false, err
	}
	r := res.(getFillResult)
	return r.val, r.notFound, nil
}

// lookup returns (value, notFound, hit). hit=false if absent OR expired.
func (c *GetCache) lookup(key string) (string, bool, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiry) {
		return "", false, false
	}
	return e.val, e.notFound, true
}

func (c *GetCache) store(key, val string, notFound bool) {
	c.mu.Lock()
	_, existed := c.entries[key]
	c.entries[key] = getCacheEntry{val, notFound, time.Now().Add(c.ttl)}
	c.mu.Unlock()
	if !existed {
		c.size.Add(1)
	}
}

// sweepLoop runs once per TTL window to evict expired entries when the
// cache exceeds maxSize. Below the cap we skip the sweep entirely —
// stale-but-unaccessed entries cost nothing.
func (c *GetCache) sweepLoop(ctx context.Context) {
	t := time.NewTicker(c.ttl)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.sweep()
		}
	}
}

func (c *GetCache) sweep() {
	if c.size.Load() < int64(c.maxSize) {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.entries {
		if now.After(e.expiry) {
			delete(c.entries, k)
			c.size.Add(-1)
			c.Evicted.Add(1)
		}
	}
}

// RedisGetThrough is a convenience wrapper that calls Client.Get and adapts
// the result to Fetch's fill signature. Returns (value, notFound, error).
// Used by GetView and GetShieldView to keep the cache wiring in one place.
func RedisGetThrough(ctx context.Context, client *redis.Client, key string) (string, bool, error) {
	v, err := client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", true, nil
	}
	return v, false, err
}

// ===== Global cache instance =====
//
// Initialized with a tiny default at package load so handlers don't have
// to nil-check. main re-initializes with prod config on startup.

var (
	getCacheMu sync.Mutex
	GetCacheV  = NewGetCache(250*time.Millisecond, 100_000)
)

// InitGetCache replaces the global cache. Stops the previous one's sweeper
// so re-initialization doesn't leak goroutines.
func InitGetCache(ttl time.Duration, maxSize int) {
	getCacheMu.Lock()
	defer getCacheMu.Unlock()
	if GetCacheV != nil {
		GetCacheV.Stop()
	}
	GetCacheV = NewGetCache(ttl, maxSize)
}
