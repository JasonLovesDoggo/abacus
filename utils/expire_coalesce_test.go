package utils

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// First refresh for an unseen key must return true (gate starts open) and
// bump Refreshed. A second call inside the window must return false and bump
// Skipped without touching Refreshed.
func TestExpireCoalescer_FirstRefreshIsAllowed(t *testing.T) {
	e := NewExpireCoalescer(1*time.Hour, 1000)

	require.True(t, e.ShouldRefresh("k1"), "first call for an unseen key must refresh")
	require.Equal(t, uint64(1), e.Refreshed.Load())
	require.Equal(t, uint64(0), e.Skipped.Load())

	require.False(t, e.ShouldRefresh("k1"), "second call inside window must skip")
	require.Equal(t, uint64(1), e.Refreshed.Load())
	require.Equal(t, uint64(1), e.Skipped.Load())
}

// Per-key isolation: refreshing k1 must not affect k2's gate.
func TestExpireCoalescer_PerKey(t *testing.T) {
	e := NewExpireCoalescer(1*time.Hour, 1000)
	require.True(t, e.ShouldRefresh("k1"))
	require.True(t, e.ShouldRefresh("k2"), "k2 must refresh independently of k1")
	require.False(t, e.ShouldRefresh("k1"))
	require.False(t, e.ShouldRefresh("k2"))
}

// Once the window elapses, the next refresh must be allowed again.
func TestExpireCoalescer_WindowElapses(t *testing.T) {
	e := NewExpireCoalescer(10*time.Millisecond, 1000)
	require.True(t, e.ShouldRefresh("k"))
	require.False(t, e.ShouldRefresh("k"))
	time.Sleep(15 * time.Millisecond)
	require.True(t, e.ShouldRefresh("k"), "post-window refresh must be allowed again")
}

// Stress: many goroutines hammering the same key inside one window must yield
// a tiny number of refreshes (ideally 1; the LoadOrStore + race tolerance can
// allow a couple). The crucial property is that the gate is doing its job —
// not >50% of calls returning true.
func TestExpireCoalescer_ConcurrentSameKeySuppresses(t *testing.T) {
	e := NewExpireCoalescer(10*time.Second, 1000)

	const N = 2000
	var wg sync.WaitGroup
	var refreshed atomic.Int64
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e.ShouldRefresh("hot") {
				refreshed.Add(1)
			}
		}()
	}
	wg.Wait()

	// In practice you'll see 1 or 2 winners depending on goroutine scheduling.
	// The point is N=2000 callers must NOT all see "refresh".
	got := refreshed.Load()
	require.LessOrEqual(t, got, int64(10),
		"at most ~handful of refreshes should win, got %d/%d", got, N)
	require.GreaterOrEqual(t, got, int64(1), "at least one caller must refresh")
	require.Equal(t, int64(N)-got, int64(e.Skipped.Load()),
		"every non-refresh must be counted as skipped")
}

// Distinct keys under concurrency must all be allowed once. This pins the
// per-key isolation property — a buggy global mutex around the whole gate
// would serialize them but still allow all to refresh; a buggy shared-counter
// would let only one through.
func TestExpireCoalescer_ConcurrentDistinctKeysAllRefresh(t *testing.T) {
	e := NewExpireCoalescer(10*time.Second, 100_000)

	const N = 500
	var wg sync.WaitGroup
	var refreshed atomic.Int64
	for i := 0; i < N; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e.ShouldRefresh("k" + strconv.Itoa(i)) {
				refreshed.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int64(N), refreshed.Load(), "every distinct key must get exactly one refresh")
}

// Size grows with distinct keys and the sweeper drops stale entries when over
// the cap. We test sweep() directly so the test is fast and deterministic.
func TestExpireCoalescer_SweeperEvictsStaleAboveCap(t *testing.T) {
	e := NewExpireCoalescer(10*time.Millisecond, 5)

	// Insert 10 keys, all "fresh".
	for i := 0; i < 10; i++ {
		require.True(t, e.ShouldRefresh("fresh"+strconv.Itoa(i)))
	}
	require.Equal(t, 10, e.Size())

	// Sweep should be a no-op here — entries are fresher than 2*interval.
	e.sweep()
	require.Equal(t, 10, e.Size(), "fresh entries must survive sweep")

	// Force entries to be old by waiting past 2*interval.
	time.Sleep(25 * time.Millisecond)
	e.sweep()
	require.Equal(t, 0, e.Size(), "all stale entries above cap must be evicted")
}

// When map size is below cap, sweeper should not touch anything even if stale.
// This is a deliberate design choice: don't pay for eviction if you don't need to.
func TestExpireCoalescer_SweeperBelowCapDoesNotEvict(t *testing.T) {
	e := NewExpireCoalescer(10*time.Millisecond, 1000)
	e.ShouldRefresh("a")
	e.ShouldRefresh("b")
	time.Sleep(25 * time.Millisecond)
	e.sweep()
	require.Equal(t, 2, e.Size(), "below-cap sweeper must not touch stale entries")
}

// Default global gate exists immediately on package load so handlers never
// hit a nil deref before main() runs InitExpireGate.
func TestExpireGate_NotNilByDefault(t *testing.T) {
	require.NotNil(t, ExpireGate, "ExpireGate must be initialized at package load")
	require.True(t, ExpireGate.ShouldRefresh("sentinel-key-for-this-test-only"))
}
