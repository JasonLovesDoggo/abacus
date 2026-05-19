package utils

import (
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
	m        sync.Map // key string -> int64 unix-nano of last refresh
	interval time.Duration

	// Bounded eviction: when entries exceed maxEntries, the periodic sweeper
	// drops anything older than 2*interval. Without this the map would grow
	// without bound across the long tail of one-shot keys.
	maxEntries int

	Refreshed atomic.Uint64 // observed by Prometheus gauge
	Skipped   atomic.Uint64
}

// NewExpireCoalescer returns a coalescer that allows at most one EXPIRE per
// `interval` per key. Starts a background sweeper that GC's stale entries
// once per interval.
func NewExpireCoalescer(interval time.Duration, maxEntries int) *ExpireCoalescer {
	e := &ExpireCoalescer{interval: interval, maxEntries: maxEntries}
	go e.sweepLoop()
	return e
}

// ShouldRefresh returns true if `interval` has passed since the last refresh
// for `key`. The caller is expected to actually issue the EXPIRE on true.
// Updates the last-refresh timestamp atomically.
func (e *ExpireCoalescer) ShouldRefresh(key string) bool {
	now := time.Now().UnixNano()
	prev, loaded := e.m.LoadOrStore(key, now)
	if !loaded {
		e.Refreshed.Add(1)
		return true
	}
	if time.Duration(now-prev.(int64)) < e.interval {
		e.Skipped.Add(1)
		return false
	}
	// Race-y but correct: two concurrent goroutines could both win the
	// LoadOrStore path on the same key in the same interval. Result is one
	// extra EXPIRE on a key under contention. Acceptable — EXPIRE is
	// idempotent and this avoids a per-key mutex on the hot path.
	e.m.Store(key, now)
	e.Refreshed.Add(1)
	return true
}

func (e *ExpireCoalescer) sweepLoop() {
	t := time.NewTicker(e.interval)
	defer t.Stop()
	for range t.C {
		e.sweep()
	}
}

func (e *ExpireCoalescer) sweep() {
	count := 0
	e.m.Range(func(_, _ any) bool {
		count++
		return true
	})
	if count < e.maxEntries {
		return
	}
	cutoff := time.Now().Add(-2 * e.interval).UnixNano()
	e.m.Range(func(k, v any) bool {
		if v.(int64) < cutoff {
			e.m.Delete(k)
		}
		return true
	})
}

// Size returns the current number of cached keys. Used by metrics.
func (e *ExpireCoalescer) Size() int {
	n := 0
	e.m.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}

// ExpireGate is the global coalescer used by the request handlers. Pre-
// initialized with conservative defaults so handlers can call it without a
// nil-check; main re-initializes with prod config on startup.
var ExpireGate = NewExpireCoalescer(time.Hour, 1_000_000)

// InitExpireGate wires the global coalescer. Safe to call more than once;
// later calls replace the gate.
func InitExpireGate(interval time.Duration, maxEntries int) {
	ExpireGate = NewExpireCoalescer(interval, maxEntries)
}
