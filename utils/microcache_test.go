package utils

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Basic invariant: first Fetch fills, second Fetch within TTL hits cache
// and doesn't run fill again.
func TestGetCache_HitOnSecondCall(t *testing.T) {
	c := NewGetCache(50*time.Millisecond, 100)
	defer c.Stop()

	var calls atomic.Int64
	fill := func() (string, bool, error) {
		calls.Add(1)
		return "42", false, nil
	}

	v, nf, err := c.Fetch("k", fill)
	require.NoError(t, err)
	require.False(t, nf)
	require.Equal(t, "42", v)
	require.Equal(t, int64(1), calls.Load())

	v, nf, err = c.Fetch("k", fill)
	require.NoError(t, err)
	require.False(t, nf)
	require.Equal(t, "42", v)
	require.Equal(t, int64(1), calls.Load(), "second call must hit cache, not call fill again")

	require.Equal(t, uint64(1), c.Hits.Load())
	require.Equal(t, uint64(1), c.Misses.Load())
}

// After TTL expires, the next Fetch must call fill again.
func TestGetCache_ExpiryTriggersRefill(t *testing.T) {
	c := NewGetCache(10*time.Millisecond, 100)
	defer c.Stop()

	var calls atomic.Int64
	fill := func() (string, bool, error) {
		calls.Add(1)
		return strconv.FormatInt(calls.Load(), 10), false, nil
	}

	v1, _, _ := c.Fetch("k", fill)
	require.Equal(t, "1", v1)
	time.Sleep(15 * time.Millisecond)

	v2, _, _ := c.Fetch("k", fill)
	require.Equal(t, "2", v2, "expired entry should trigger a fresh fill returning a new value")
	require.Equal(t, int64(2), calls.Load())
}

// Negative caching: redis.Nil (represented as notFound=true) must be
// cached so bot probes for nonexistent keys don't hit Redis on every call.
func TestGetCache_NegativeCachingWorks(t *testing.T) {
	c := NewGetCache(50*time.Millisecond, 100)
	defer c.Stop()

	var calls atomic.Int64
	fill := func() (string, bool, error) {
		calls.Add(1)
		return "", true, nil // simulates redis.Nil
	}

	_, nf1, err := c.Fetch("missing", fill)
	require.NoError(t, err)
	require.True(t, nf1)

	_, nf2, err := c.Fetch("missing", fill)
	require.NoError(t, err)
	require.True(t, nf2)
	require.Equal(t, int64(1), calls.Load(), "second call for missing key must hit cached not-found, not re-query")
}

// THE singleflight invariant. N concurrent Fetch calls for the same key
// must collapse to exactly ONE fill() invocation. This is what protects
// the cache stampede on TTL expiry under load.
func TestGetCache_ConcurrentFillsCollapseToOne(t *testing.T) {
	c := NewGetCache(50*time.Millisecond, 100)
	defer c.Stop()

	var calls atomic.Int64
	fill := func() (string, bool, error) {
		calls.Add(1)
		// Hold long enough to guarantee overlap.
		time.Sleep(20 * time.Millisecond)
		return "shared", false, nil
	}

	const N = 200
	var wg sync.WaitGroup
	results := make(chan string, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, _, err := c.Fetch("hot", fill)
			require.NoError(t, err)
			results <- v
		}()
	}
	wg.Wait()
	close(results)

	require.Equal(t, int64(1), calls.Load(),
		"200 concurrent Fetch calls for one key must collapse to exactly 1 fill, got %d", calls.Load())
	for r := range results {
		require.Equal(t, "shared", r, "every caller must get the same value")
	}
}

// Distinct keys under concurrency must NOT collapse — each gets its own fill.
func TestGetCache_DistinctKeysFillIndependently(t *testing.T) {
	c := NewGetCache(50*time.Millisecond, 1000)
	defer c.Stop()

	var calls atomic.Int64
	fill := func() (string, bool, error) {
		calls.Add(1)
		time.Sleep(5 * time.Millisecond)
		return "v", false, nil
	}

	const N = 100
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := c.Fetch("k"+strconv.Itoa(i), fill)
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	require.Equal(t, int64(N), calls.Load(),
		"%d distinct keys should each get one fill, got %d", N, calls.Load())
}

// Errors must NOT be cached. A transient Redis failure shouldn't poison
// the cache for the full TTL window — the next caller should retry.
func TestGetCache_ErrorsAreNotCached(t *testing.T) {
	c := NewGetCache(50*time.Millisecond, 100)
	defer c.Stop()

	boom := errors.New("redis down")
	var calls atomic.Int64
	fillFail := func() (string, bool, error) {
		calls.Add(1)
		return "", false, boom
	}

	_, _, err := c.Fetch("k", fillFail)
	require.ErrorIs(t, err, boom)
	require.Equal(t, int64(1), calls.Load())

	_, _, err = c.Fetch("k", fillFail)
	require.ErrorIs(t, err, boom)
	require.Equal(t, int64(2), calls.Load(),
		"error path must not cache; second call should re-invoke fill")
}

// Sweeper evicts only stale entries and only when above maxSize.
func TestGetCache_SweeperBoundedAndStaleOnly(t *testing.T) {
	c := NewGetCache(10*time.Millisecond, 5)
	defer c.Stop()

	// Fill past the cap.
	for i := 0; i < 10; i++ {
		_, _, _ = c.Fetch("k"+strconv.Itoa(i), func() (string, bool, error) {
			return "v", false, nil
		})
	}
	require.Equal(t, 10, c.Size())

	// Immediate sweep: above cap but entries are fresh, so nothing evicted.
	c.sweep()
	require.Equal(t, 10, c.Size(), "fresh entries above cap must survive sweep")

	// Wait past TTL, then sweep: stale + above cap = evict.
	time.Sleep(20 * time.Millisecond)
	c.sweep()
	require.Equal(t, 0, c.Size(), "stale entries above cap must be evicted")
	require.Greater(t, c.Evicted.Load(), uint64(0))
}

// Below the cap, the sweeper must do nothing — pure efficiency check.
func TestGetCache_SweeperBelowCapNoOp(t *testing.T) {
	c := NewGetCache(10*time.Millisecond, 1000)
	defer c.Stop()

	_, _, _ = c.Fetch("a", func() (string, bool, error) { return "v", false, nil })
	_, _, _ = c.Fetch("b", func() (string, bool, error) { return "v", false, nil })
	require.Equal(t, 2, c.Size())

	time.Sleep(20 * time.Millisecond)
	c.sweep()
	require.Equal(t, 2, c.Size(), "below cap, sweeper must not evict even stale entries")
	require.Equal(t, uint64(0), c.Evicted.Load())
}

// ttl<=0 disables caching entirely. Fetch should pass through to fill on
// every call and never store anything.
func TestGetCache_DisabledPassesThrough(t *testing.T) {
	c := NewGetCache(0, 100)
	defer c.Stop()

	require.False(t, c.Enabled())
	var calls atomic.Int64
	for i := 0; i < 5; i++ {
		_, _, _ = c.Fetch("k", func() (string, bool, error) {
			calls.Add(1)
			return "v", false, nil
		})
	}
	require.Equal(t, int64(5), calls.Load(), "disabled cache must call fill every time")
	require.Equal(t, 0, c.Size())
	require.Equal(t, uint64(0), c.Hits.Load())
}

// InitGetCache replaces the global cleanly without leaking goroutines.
// Smoke test: call it twice with different settings and verify the global
// reflects the latest config.
func TestInitGetCache_ReplacesCleanly(t *testing.T) {
	t.Cleanup(func() { InitGetCache(250*time.Millisecond, 100_000) })

	InitGetCache(100*time.Millisecond, 50)
	require.Equal(t, 100*time.Millisecond, GetCacheV.ttl)
	require.Equal(t, 50, GetCacheV.maxSize)

	InitGetCache(500*time.Millisecond, 200)
	require.Equal(t, 500*time.Millisecond, GetCacheV.ttl)
	require.Equal(t, 200, GetCacheV.maxSize)
}

// The default global cache must be non-nil at package load so handlers
// never deref nil before main runs InitGetCache.
func TestGetCacheV_NotNilByDefault(t *testing.T) {
	require.NotNil(t, GetCacheV)
	require.True(t, GetCacheV.Enabled(), "default global cache must be enabled")
}
