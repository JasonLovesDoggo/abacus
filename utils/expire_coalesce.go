package utils

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// ExpireCoalescer suppresses redundant EXPIRE calls per key. The TTL we set
// is BaseTTLPeriod (6 months), so refreshing more than once per hour per key
// is pure waste — a key only loses its TTL if it's idle for the entire
// 6-month window. At ~32 RPS sustained, EXPIRE was running 5.6M times/day,
// nearly 1:1 with reads. This collapses it to roughly one call per active
// key per `interval`.
//
// Multi-instance safe: each app instance keeps its own cache. If two
// instances both decide to refresh in the same window for the same key, the
// duplicate EXPIRE is idempotent. There is no cross-instance coordination
// cost, which is the point.
type ExpireCoalescer struct {
	m sync.Map // key string -> int64 unix-nano of last refresh

	interval time.Duration

	// Bounded eviction: when entries exceed maxEntries, the periodic sweeper
	// drops anything older than 2*interval. Without this the map would grow
	// without bound across the long tail of one-shot keys.
	maxEntries int

	// size is kept in sync with the map so Prometheus scrapes don't have to
	// walk the whole sync.Map on every poll.
	size atomic.Int64

	Refreshed atomic.Uint64 // observed by Prometheus
	Skipped   atomic.Uint64

	// stop cancels the background sweeper. Allows InitExpireGate to be called
	// more than once without leaking goroutines on the orphaned coalescer.
	stop context.CancelFunc
}

// NewExpireCoalescer returns a coalescer that allows at most one EXPIRE per
// `interval` per key. Starts a background sweeper that GC's stale entries
// once per interval. Call Stop to release the sweeper goroutine.
func NewExpireCoalescer(interval time.Duration, maxEntries int) *ExpireCoalescer {
	ctx, cancel := context.WithCancel(context.Background())
	e := &ExpireCoalescer{interval: interval, maxEntries: maxEntries, stop: cancel}
	go e.sweepLoop(ctx)
	return e
}

// Stop terminates the background sweeper. Safe to call more than once.
func (e *ExpireCoalescer) Stop() {
	if e == nil || e.stop == nil {
		return
	}
	e.stop()
}

// ShouldRefresh returns true if `interval` has passed since the last refresh
// for `key`. The caller is expected to actually issue the EXPIRE on true.
// Updates the last-refresh timestamp atomically.
func (e *ExpireCoalescer) ShouldRefresh(key string) bool {
	now := time.Now().UnixNano()
	prev, loaded := e.m.LoadOrStore(key, now)
	if !loaded {
		// sync.Map.LoadOrStore is atomic: exactly one goroutine sees
		// loaded=false for a given key, so the first-insertion path has no
		// race.
		e.size.Add(1)
		e.Refreshed.Add(1)
		return true
	}
	if time.Duration(now-prev.(int64)) < e.interval {
		e.Skipped.Add(1)
		return false
	}
	// Real race lives here: under heavy contention several goroutines can
	// each pass the post-interval check, each Store their own `now`, and
	// each return true. That allows a small number of redundant EXPIREs on
	// the same key in the same window. Acceptable: EXPIRE is idempotent and
	// we avoid a per-key mutex on the hot path.
	e.m.Store(key, now)
	e.Refreshed.Add(1)
	return true
}

func (e *ExpireCoalescer) sweepLoop(ctx context.Context) {
	t := time.NewTicker(e.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.sweep()
		}
	}
}

func (e *ExpireCoalescer) sweep() {
	if e.size.Load() < int64(e.maxEntries) {
		return
	}
	cutoff := time.Now().Add(-2 * e.interval).UnixNano()
	e.m.Range(func(k, v any) bool {
		if v.(int64) < cutoff {
			if _, deleted := e.m.LoadAndDelete(k); deleted {
				e.size.Add(-1)
			}
		}
		return true
	})
}

// Size returns the current number of cached keys. O(1) via the atomic
// counter — safe to call from a Prometheus scrape.
func (e *ExpireCoalescer) Size() int {
	return int(e.size.Load())
}

// ExpireGate is the global coalescer used by the request handlers. Pre-
// initialized with conservative defaults so handlers can call it without a
// nil-check; main re-initializes with prod config on startup.
var (
	expireGateMu sync.Mutex
	ExpireGate   = NewExpireCoalescer(time.Hour, 1_000_000)
)

// InitExpireGate replaces the global coalescer. Stops the previous gate's
// sweeper so re-initialization doesn't leak goroutines.
func InitExpireGate(interval time.Duration, maxEntries int) {
	expireGateMu.Lock()
	defer expireGateMu.Unlock()
	if ExpireGate != nil {
		ExpireGate.Stop()
	}
	ExpireGate = NewExpireCoalescer(interval, maxEntries)
}
